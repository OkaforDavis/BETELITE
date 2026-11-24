package routes

import (
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"

	"betelite-go/config"
	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/services"
	"betelite-go/utils"
	"betelite-go/ws"
)

// Stake limits per player, in minor units.
var (
	minStake = map[string]int64{"NGN": 100_00, "GHS": 5_00}
	maxStake = map[string]int64{"NGN": 100_000_00, "GHS": 2_000_00}
)

type challengeView struct {
	ID              string                  `json:"id"`
	CreatorID       string                  `json:"creatorId"`
	CreatorName     string                  `json:"creatorName"`
	CreatorTag      string                  `json:"creatorTag"`
	CreatorAvatar   string                  `json:"creatorAvatar"`
	Game            string                  `json:"game"`
	GameName        string                  `json:"gameName"`
	Kind            string                  `json:"kind"` // stake | bounty (amount 0 = friendly)
	Amount          int64                   `json:"amount"`
	Currency        string                  `json:"currency"`
	Prize           int64                   `json:"prize"`
	CreatedAt       time.Time               `json:"createdAt"`
	Settings        *services.MatchSettings `json:"settings,omitempty"` // room settings the poster picked (CODM)
	SettingsSummary string                  `json:"settingsSummary,omitempty"`
}

// closedNote tells a poster that their other open challenges were closed.
func closedNote(n int) string {
	switch n {
	case 0:
		return ""
	case 1:
		return " Your other open challenge was closed (one 1v1 at a time)."
	}
	return fmt.Sprintf(" Your %d other open challenges were closed (one 1v1 at a time).", n)
}

func SetupLobbyRoutes(api fiber.Router, hub *ws.Hub) {
	lobby := api.Group("/lobby", middleware.AuthRequired(), dbRequired)

	// Open challenges waiting for an opponent.
	lobby.Get("/", func(c *fiber.Ctx) error {
		rows, err := db.Pool.Query(c.Context(), `
			SELECT e.challenge_id, e.creator_id, u.username, COALESCE(gp.gamertag,''), COALESCE(u.avatar_url,''), COALESCE(e.game,''), e.kind, e.amount, COALESCE(e.currency,'NGN'), e.created_at, e.settings
			FROM escrow e
			JOIN users u ON u.id = e.creator_id
			LEFT JOIN game_profiles gp ON gp.user_id = e.creator_id AND gp.game = e.game
			WHERE e.status = 'waiting'
			AND NOT EXISTS (SELECT 1 FROM matches m WHERE m.kind = 'p2p' AND m.status IN `+services.UnfinishedP2PStatuses+`
				AND (m.home_id = e.creator_id OR m.away_id = e.creator_id))
			ORDER BY e.created_at DESC LIMIT 100`)
		if err != nil {
			return fail(c, err)
		}
		defer rows.Close()
		list := []challengeView{}
		for rows.Next() {
			var ch challengeView
			var settings []byte
			if rows.Scan(&ch.ID, &ch.CreatorID, &ch.CreatorName, &ch.CreatorTag, &ch.CreatorAvatar, &ch.Game, &ch.Kind, &ch.Amount, &ch.Currency, &ch.CreatedAt, &settings) == nil {
				ch.Settings = services.ParseSettings(settings)
				ch.SettingsSummary = services.SettingsSummary(ch.Game, ch.Settings)
				if g := services.GameByID(ch.Game); g != nil {
					ch.GameName = g.Short
				}
				if ch.Kind == "bounty" {
					ch.Prize = ch.Amount // the challenger wins the whole bounty
				} else {
					ch.Prize = ch.Amount * 2 * services.P2PWinnerPercent / 100
				}
				list = append(list, ch)
			}
		}
		return utils.SendSuccess(c, fiber.Map{"challenges": list})
	})

	// Post a challenge. Friendly: nothing held. Bounty: the host's prize and
	// hosting fee are held until the match is settled; the challenger plays
	// free. Stake (both pay): only when staking is licensed and enabled.
	lobby.Post("/create", middleware.RateLimitMatchCreation(), func(c *fiber.Ctx) error {
		var req struct {
			Game   string `json:"game"`
			Amount int64  `json:"amount"`
			Kind   string `json:"kind"` // "" | stake | bounty
			// Room settings (CODM): mode, map, score/time limit, loadout rules.
			Settings *services.MatchSettings `json:"settings"`
		}
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid request")
		}
		uid := middleware.GetUID(c)
		ctx := c.Context()
		game := services.GameByID(req.Game)
		if game == nil {
			return utils.SendError(c, 400, "Choose a supported game")
		}
		settings, err := services.CleanSettings(game.ID, req.Settings, "1v1")
		if err != nil {
			return fail(c, err)
		}
		kind := "stake"
		if req.Kind == "bounty" && req.Amount > 0 {
			if !config.Cfg.BountyEnabled {
				return utils.SendError(c, 409, "Bounty matches are paused right now")
			}
			kind = "bounty"
		} else if !config.Cfg.StakingEnabled {
			// Without a gaming licence, 1v1s are free friendly matches (no stakes).
			req.Amount = 0
		}
		if req.Amount > 0 {
			if err := services.RequireMoneyAccess(ctx, uid); err != nil {
				return fail(c, err)
			}
		}
		if err := services.RequireGameProfile(ctx, db.Pool, uid, game.ID); err != nil {
			return fail(c, err)
		}
		var currency string
		db.Pool.QueryRow(ctx, "SELECT COALESCE(currency,'NGN') FROM users WHERE id = $1", uid).Scan(&currency)
		if kind == "bounty" && (req.Amount < services.MinBounty[currency] || req.Amount > services.MaxBounty[currency]) {
			return utils.SendError(c, 400, fmt.Sprintf("A bounty must be between %s and %s",
				services.FormatMoney(services.MinBounty[currency], currency), services.FormatMoney(services.MaxBounty[currency], currency)))
		}
		if kind == "stake" && req.Amount > 0 && (req.Amount < minStake[currency] || req.Amount > maxStake[currency]) {
			return utils.SendError(c, 400, fmt.Sprintf("Stake must be between %s and %s",
				services.FormatMoney(minStake[currency], currency), services.FormatMoney(maxStake[currency], currency)))
		}
		var open int
		db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM escrow WHERE creator_id = $1 AND status = 'waiting'", uid).Scan(&open)
		if open >= 3 {
			return utils.SendError(c, 400, "You can have at most 3 open challenges")
		}

		id := utils.GenerateChallengeID()
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return fail(c, err)
		}
		defer tx.Rollback(ctx)
		if err := services.LockPlayers(ctx, tx, uid); err != nil {
			return fail(c, err)
		}
		if busy, err := services.ActiveP2PMatch(ctx, tx, uid); err != nil {
			return fail(c, err)
		} else if busy != "" {
			return utils.SendError(c, 409, "Finish your current 1v1 first (upload or confirm its result), then post a new challenge")
		}
		var fee int64
		if kind == "bounty" {
			fee = services.HostFee(req.Amount)
			if err := services.AdjustBalance(ctx, tx, uid, -req.Amount, "bounty_hold", id); err == nil {
				err = services.AdjustBalance(ctx, tx, uid, -fee, "bounty_fee", id)
			} else if err != nil {
				if errors.Is(err, services.ErrInsufficientFunds) {
					return utils.SendError(c, 400, fmt.Sprintf("You need %s to post this bounty (%s prize + %s hosting fee). Top up first.",
						services.FormatMoney(req.Amount+fee, currency), services.FormatMoney(req.Amount, currency), services.FormatMoney(fee, currency)))
				}
				return fail(c, err)
			}
		} else if req.Amount > 0 {
			if err := services.AdjustBalance(ctx, tx, uid, -req.Amount, "wager_hold", id); err != nil {
				if errors.Is(err, services.ErrInsufficientFunds) {
					return utils.SendError(c, 400, "Insufficient balance. Deposit to post this challenge.")
				}
				return fail(c, err)
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO escrow (challenge_id, creator_id, amount, pool, status, game, currency, kind, fee, settings)
			VALUES ($1,$2,$3,$3,'waiting',$4,$5,$6,$7,$8::jsonb)`, id, uid, req.Amount, game.ID, currency, kind, fee, services.SettingsJSON(settings)); err != nil {
			return fail(c, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fail(c, err)
		}

		ws.BroadcastEvent(hub, "lobby_new_challenge", fiber.Map{"id": id, "game": game.ID})
		return utils.SendSuccess(c, fiber.Map{"challengeId": id})
	})

	// Accept a challenge: stake is matched and a verified match is created.
	lobby.Post("/accept", middleware.RateLimitMatchCreation(), func(c *fiber.Ctx) error {
		var req struct {
			ChallengeID string `json:"challengeId"`
		}
		if err := c.BodyParser(&req); err != nil || req.ChallengeID == "" {
			return utils.SendError(c, 400, "Invalid request")
		}
		uid := middleware.GetUID(c)
		ctx := c.Context()

		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return fail(c, err)
		}
		defer tx.Rollback(ctx)

		// Lock both players before the challenge, so two accepts involving the
		// same player run one after the other.
		var poster string
		if err := tx.QueryRow(ctx, "SELECT creator_id FROM escrow WHERE challenge_id = $1", req.ChallengeID).Scan(&poster); err != nil {
			return utils.SendError(c, 409, "This challenge is no longer available")
		}
		if poster == uid {
			return utils.SendError(c, 400, "You can't accept your own challenge")
		}
		if err := services.LockPlayers(ctx, tx, poster, uid); err != nil {
			return fail(c, err)
		}

		var escrowID, amount int64
		var creator, status, game, currency, kind string
		err = tx.QueryRow(ctx, "SELECT id, creator_id, amount, status, COALESCE(game,''), COALESCE(currency,'NGN'), kind FROM escrow WHERE challenge_id = $1 FOR UPDATE", req.ChallengeID).
			Scan(&escrowID, &creator, &amount, &status, &game, &currency, &kind)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && status != "waiting") {
			return utils.SendError(c, 409, "This challenge is no longer available")
		}
		if err != nil {
			return fail(c, err)
		}
		// One 1v1 at a time, for both players.
		if busy, err := services.ActiveP2PMatch(ctx, tx, uid); err != nil {
			return fail(c, err)
		} else if busy != "" {
			return utils.SendError(c, 409, "Finish your current 1v1 first (upload or confirm its result), then accept another")
		}
		if busy, err := services.ActiveP2PMatch(ctx, tx, creator); err != nil {
			return fail(c, err)
		} else if busy != "" {
			return utils.SendError(c, 409, "This player is in another match right now. Try another challenge.")
		}
		if services.GameByID(game) == nil {
			return utils.SendError(c, 400, "This challenge is for an unsupported game")
		}
		bounty := kind == "bounty"
		if amount > 0 && bounty {
			// Free to accept; the challenger must still be 18+ on current terms to win money.
			if err := services.RequireMoneyAccess(ctx, uid); err != nil {
				return fail(c, err)
			}
		} else if amount > 0 {
			if !config.Cfg.StakingEnabled {
				return utils.SendError(c, 409, "Staked challenges are paused. Post a free challenge instead.")
			}
			if err := services.RequireMoneyAccess(ctx, uid); err != nil {
				return fail(c, err)
			}
			var myCurrency string
			tx.QueryRow(ctx, "SELECT COALESCE(currency,'NGN') FROM users WHERE id = $1", uid).Scan(&myCurrency)
			if myCurrency != currency {
				return utils.SendError(c, 400, "This challenge is in "+currency+"; your wallet is in "+myCurrency)
			}
		}
		if err := services.RequireGameProfile(ctx, tx, uid, game); err != nil {
			return fail(c, err)
		}
		if amount > 0 && !bounty {
			if err := services.AdjustBalance(ctx, tx, uid, -amount, "wager_hold", req.ChallengeID); err != nil {
				if errors.Is(err, services.ErrInsufficientFunds) {
					return utils.SendError(c, 400, "Insufficient balance to match this stake")
				}
				return fail(c, err)
			}
		}
		matchID, err := services.CreateMatch(ctx, tx, "p2p", game, creator, uid, req.ChallengeID, "", 0, 0, time.Now().Add(services.P2PPlayWindow))
		if err != nil {
			return fail(c, err)
		}
		pool := "amount * 2"
		if bounty {
			pool = "amount"
		}
		if _, err := tx.Exec(ctx, "UPDATE escrow SET acceptor_id = $1, pool = "+pool+", status = 'held', match_id = $2 WHERE id = $3",
			uid, matchID, escrowID); err != nil {
			return fail(c, err)
		}
		closedPoster, err := services.CloseOpenChallenges(ctx, tx, creator, req.ChallengeID)
		if err != nil {
			return fail(c, err)
		}
		closedMine, err := services.CloseOpenChallenges(ctx, tx, uid, "")
		if err != nil {
			return fail(c, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fail(c, err)
		}

		ws.BroadcastEvent(hub, "lobby_challenge_removed", fiber.Map{"id": req.ChallengeID})
		for _, id := range append(closedPoster, closedMine...) {
			ws.BroadcastEvent(hub, "lobby_challenge_removed", fiber.Map{"id": id})
		}
		what := "challenge"
		if bounty {
			what = "bounty"
		}
		services.Notify(creator, "challenge_accepted", "Challenge accepted! ⚔️",
			services.Username(ctx, uid)+" accepted your "+what+". Play the match and upload the final result screen within 3 hours."+closedNote(len(closedPoster)),
			"/match/"+matchID, map[string]any{"matchId": matchID})
		return utils.SendSuccess(c, fiber.Map{"matchId": matchID})
	})

	// Cancel my open challenge and get the stake back.
	lobby.Post("/delete", func(c *fiber.Ctx) error {
		var req struct {
			ChallengeID string `json:"challengeId"`
		}
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid request")
		}
		uid := middleware.GetUID(c)
		ctx := c.Context()
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return fail(c, err)
		}
		defer tx.Rollback(ctx)
		if err := services.LockPlayers(ctx, tx, uid); err != nil {
			return fail(c, err)
		}

		var amount, fee int64
		var status, kind string
		err = tx.QueryRow(ctx, "SELECT amount, fee, status, kind FROM escrow WHERE challenge_id = $1 AND creator_id = $2 FOR UPDATE", req.ChallengeID, uid).Scan(&amount, &fee, &status, &kind)
		if err != nil {
			return utils.SendError(c, 404, "Challenge not found")
		}
		if status != "waiting" {
			return utils.SendError(c, 409, "This challenge has already been accepted")
		}
		if kind == "bounty" && amount+fee > 0 {
			if err := services.AdjustBalance(ctx, tx, uid, amount+fee, "bounty_refund", req.ChallengeID); err != nil {
				return fail(c, err)
			}
		} else if amount > 0 {
			if err := services.AdjustBalance(ctx, tx, uid, amount, "wager_refund", req.ChallengeID); err != nil {
				return fail(c, err)
			}
		}
		if _, err := tx.Exec(ctx, "UPDATE escrow SET status = 'cancelled' WHERE challenge_id = $1", req.ChallengeID); err != nil {
			return fail(c, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fail(c, err)
		}
		ws.BroadcastEvent(hub, "lobby_challenge_removed", fiber.Map{"id": req.ChallengeID})
		return utils.SendSuccess(c, fiber.Map{})
	})
}
