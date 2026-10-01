package routes

import (
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"

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
	ID            string    `json:"id"`
	CreatorID     string    `json:"creatorId"`
	CreatorName   string    `json:"creatorName"`
	CreatorTag    string    `json:"creatorTag"`
	CreatorAvatar string    `json:"creatorAvatar"`
	Game          string    `json:"game"`
	GameName      string    `json:"gameName"`
	Amount        int64     `json:"amount"`
	Currency      string    `json:"currency"`
	Prize         int64     `json:"prize"`
	CreatedAt     time.Time `json:"createdAt"`
}

func SetupLobbyRoutes(api fiber.Router, hub *ws.Hub) {
	lobby := api.Group("/lobby", middleware.AuthRequired(), dbRequired)

	// Open challenges waiting for an opponent.
	lobby.Get("/", func(c *fiber.Ctx) error {
		rows, err := db.Pool.Query(c.Context(), `
			SELECT e.challenge_id, e.creator_id, u.username, COALESCE(gp.gamertag,''), COALESCE(u.avatar_url,''), COALESCE(e.game,''), e.amount, COALESCE(e.currency,'NGN'), e.created_at
			FROM escrow e
			JOIN users u ON u.id = e.creator_id
			LEFT JOIN game_profiles gp ON gp.user_id = e.creator_id AND gp.game = e.game
			WHERE e.status = 'waiting' ORDER BY e.created_at DESC LIMIT 100`)
		if err != nil {
			return fail(c, err)
		}
		defer rows.Close()
		list := []challengeView{}
		for rows.Next() {
			var ch challengeView
			if rows.Scan(&ch.ID, &ch.CreatorID, &ch.CreatorName, &ch.CreatorTag, &ch.CreatorAvatar, &ch.Game, &ch.Amount, &ch.Currency, &ch.CreatedAt) == nil {
				if g := services.GameByID(ch.Game); g != nil {
					ch.GameName = g.Short
				}
				ch.Prize = ch.Amount * 2 * services.P2PWinnerPercent / 100
				list = append(list, ch)
			}
		}
		return utils.SendSuccess(c, fiber.Map{"challenges": list})
	})

	// Post a challenge: the stake is held in escrow until someone accepts.
	lobby.Post("/create", middleware.RateLimitMatchCreation(), func(c *fiber.Ctx) error {
		var req struct {
			Game   string `json:"game"`
			Amount int64  `json:"amount"`
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
		if err := services.RequireMoneyAccess(ctx, uid); err != nil {
			return fail(c, err)
		}
		if err := services.RequireGameProfile(ctx, db.Pool, uid, game.ID); err != nil {
			return fail(c, err)
		}
		var currency string
		db.Pool.QueryRow(ctx, "SELECT COALESCE(currency,'NGN') FROM users WHERE id = $1", uid).Scan(&currency)
		if req.Amount < minStake[currency] || req.Amount > maxStake[currency] {
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
		if err := services.AdjustBalance(ctx, tx, uid, -req.Amount, "wager_hold", id); err != nil {
			if errors.Is(err, services.ErrInsufficientFunds) {
				return utils.SendError(c, 400, "Insufficient balance. Deposit to post this challenge.")
			}
			return fail(c, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO escrow (challenge_id, creator_id, amount, pool, status, game, currency)
			VALUES ($1,$2,$3,$3,'waiting',$4,$5)`, id, uid, req.Amount, game.ID, currency); err != nil {
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
		if err := services.RequireMoneyAccess(ctx, uid); err != nil {
			return fail(c, err)
		}

		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return fail(c, err)
		}
		defer tx.Rollback(ctx)

		var escrowID, amount int64
		var creator, status, game, currency string
		err = tx.QueryRow(ctx, "SELECT id, creator_id, amount, status, COALESCE(game,''), COALESCE(currency,'NGN') FROM escrow WHERE challenge_id = $1 FOR UPDATE", req.ChallengeID).
			Scan(&escrowID, &creator, &amount, &status, &game, &currency)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && status != "waiting") {
			return utils.SendError(c, 409, "This challenge is no longer available")
		}
		if err != nil {
			return fail(c, err)
		}
		if creator == uid {
			return utils.SendError(c, 400, "You can't accept your own challenge")
		}
		if services.GameByID(game) == nil {
			return utils.SendError(c, 400, "This challenge is for an unsupported game")
		}
		var myCurrency string
		tx.QueryRow(ctx, "SELECT COALESCE(currency,'NGN') FROM users WHERE id = $1", uid).Scan(&myCurrency)
		if myCurrency != currency {
			return utils.SendError(c, 400, "This challenge is in "+currency+"; your wallet is in "+myCurrency)
		}
		if err := services.RequireGameProfile(ctx, tx, uid, game); err != nil {
			return fail(c, err)
		}
		if err := services.AdjustBalance(ctx, tx, uid, -amount, "wager_hold", req.ChallengeID); err != nil {
			if errors.Is(err, services.ErrInsufficientFunds) {
				return utils.SendError(c, 400, "Insufficient balance to match this stake")
			}
			return fail(c, err)
		}
		matchID, err := services.CreateMatch(ctx, tx, "p2p", game, creator, uid, req.ChallengeID, "", 0, 0, time.Now().Add(services.P2PPlayWindow))
		if err != nil {
			return fail(c, err)
		}
		if _, err := tx.Exec(ctx, "UPDATE escrow SET acceptor_id = $1, pool = amount * 2, status = 'held', match_id = $2 WHERE id = $3",
			uid, matchID, escrowID); err != nil {
			return fail(c, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fail(c, err)
		}

		ws.BroadcastEvent(hub, "lobby_challenge_removed", fiber.Map{"id": req.ChallengeID})
		services.Notify(creator, "challenge_accepted", "Challenge accepted! ⚔️",
			services.Username(ctx, uid)+" accepted your challenge. Play the match and upload the final result screen within 3 hours.",
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

		var amount int64
		var status string
		err = tx.QueryRow(ctx, "SELECT amount, status FROM escrow WHERE challenge_id = $1 AND creator_id = $2 FOR UPDATE", req.ChallengeID, uid).Scan(&amount, &status)
		if err != nil {
			return utils.SendError(c, 404, "Challenge not found")
		}
		if status != "waiting" {
			return utils.SendError(c, 409, "This challenge has already been accepted")
		}
		if err := services.AdjustBalance(ctx, tx, uid, amount, "wager_refund", req.ChallengeID); err != nil {
			return fail(c, err)
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
