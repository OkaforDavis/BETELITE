package routes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"

	"betelite-go/config"
	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/services"
	"betelite-go/utils"
)

type tournamentView struct {
	ID               string                  `json:"id"`
	Name             string                  `json:"name"`
	Description      string                  `json:"description,omitempty"`
	CoverURL         string                  `json:"coverUrl,omitempty"`
	Game             string                  `json:"game"`
	GameName         string                  `json:"gameName"`
	Format           string                  `json:"format"`
	Icon             string                  `json:"icon"`
	EntryFee         int64                   `json:"entryFee"`
	Currency         string                  `json:"currency"`
	MaxPlayers       int                     `json:"maxPlayers"`
	PlayerCount      int                     `json:"playerCount"`
	Status           string                  `json:"status"`
	CurrentRound     int                     `json:"currentRound"`
	RoundHours       int                     `json:"roundHours"`
	PrizeSplit       []float64               `json:"prizeSplit"`
	Prizes           []int64                 `json:"prizes"`                     // projected at full capacity
	FixedPrizes      services.FixedPrizes    `json:"fixedPrizes,omitempty"`      // per currency, 1st place first
	MembersOnlyUntil *time.Time              `json:"membersOnlyUntil,omitempty"` // early sign-up for members
	EarlyTier        string                  `json:"earlyTier"`                  // "pro" = any plan, "elite" = Elite only
	FreeSeats        *int                    `json:"freeSeats,omitempty"`        // max seats for free-plan players
	FreeTaken        int                     `json:"freeTaken"`
	TotalRounds      int                     `json:"totalRounds,omitempty"`
	Joined           bool                    `json:"joined"`
	WinnerID         string                  `json:"winnerId,omitempty"`
	CreatedAt        time.Time               `json:"createdAt"`
	StartedAt        *time.Time              `json:"startedAt,omitempty"`
	HostID           string                  `json:"hostId,omitempty"` // player-hosted cup: free entry, host funds the pool
	HostName         string                  `json:"hostName,omitempty"`
	HostPool         int64                   `json:"hostPool,omitempty"`
	HostFee          int64                   `json:"hostFee,omitempty"`
	FillDeadline     *time.Time              `json:"fillDeadline,omitempty"`
	MembersOnly      bool                    `json:"membersOnly,omitempty"` // host cup only paid members can join
	Settings         *services.MatchSettings `json:"settings,omitempty"`    // room settings (CODM)
	SettingsSummary  string                  `json:"settingsSummary,omitempty"`
	StartsAt         *time.Time              `json:"startsAt,omitempty"` // Free-for-All: when the room opens
	CreatedBy        string                  `json:"-"`
	Room             *services.CupRoom       `json:"room,omitempty"`      // Free-for-All: players and host only
	CanManage        bool                    `json:"canManage,omitempty"` // may post the room / the result
	HasResult        bool                    `json:"hasResult,omitempty"` // a final scoreboard was uploaded
}

const tournamentSelect = `SELECT t.id, t.name, t.game, t.format, COALESCE(t.icon,''), t.entry_fee, t.currency, t.max_players,
	(SELECT COUNT(*) FROM tournament_players p WHERE p.tournament_id = t.id), t.status, t.current_round, t.round_hours,
	t.prize_split, t.prize_pool, EXISTS(SELECT 1 FROM tournament_players p WHERE p.tournament_id = t.id AND p.user_id = $1),
	COALESCE(t.winner_id,''), t.created_at, t.started_at, t.prizes, t.members_only_until, t.early_tier, t.free_seats,
	(SELECT COUNT(*) FROM tournament_players p WHERE p.tournament_id = t.id AND p.free_entry),
	COALESCE(t.host_id,''), COALESCE((SELECT username FROM users h WHERE h.id = t.host_id),''), t.host_pool, t.host_fee, t.fill_deadline, t.members_only, COALESCE(t.description,''), COALESCE(t.cover_url,''),
	t.settings, t.starts_at, COALESCE(t.created_by,''), EXISTS(SELECT 1 FROM tournament_results r WHERE r.tournament_id = t.id) FROM tournaments t`

func scanTournament(row pgx.Row) (*tournamentView, error) {
	var t tournamentView
	var split, fixed, settings []byte
	var seed int64
	if err := row.Scan(&t.ID, &t.Name, &t.Game, &t.Format, &t.Icon, &t.EntryFee, &t.Currency, &t.MaxPlayers, &t.PlayerCount,
		&t.Status, &t.CurrentRound, &t.RoundHours, &split, &seed, &t.Joined, &t.WinnerID, &t.CreatedAt, &t.StartedAt, &fixed, &t.MembersOnlyUntil,
		&t.EarlyTier, &t.FreeSeats, &t.FreeTaken, &t.HostID, &t.HostName, &t.HostPool, &t.HostFee, &t.FillDeadline, &t.MembersOnly, &t.Description, &t.CoverURL,
		&settings, &t.StartsAt, &t.CreatedBy, &t.HasResult); err != nil {
		return nil, err
	}
	t.Settings = services.ParseSettings(settings)
	t.SettingsSummary = services.SettingsSummary(t.Game, t.Settings)
	t.FixedPrizes = services.ParseFixedPrizes(fixed)
	if t.Format == "swiss" {
		t.TotalRounds = services.SwissRounds(t.MaxPlayers)
	}
	t.PrizeSplit = services.DefaultSplit(t.Format)
	if len(split) > 0 {
		var s []float64
		if json.Unmarshal(split, &s) == nil && len(s) > 0 {
			t.PrizeSplit = s
		}
	}
	t.Prizes = services.PrizeAmounts(t.EntryFee, int64(t.MaxPlayers), seed, t.PrizeSplit)
	if g := services.GameByID(t.Game); g != nil {
		t.GameName = g.Short
	}
	return &t, nil
}

func SetupTournamentRoutes(api fiber.Router) {
	// Public: cup cover pictures (for <img> tags and link previews). The URL
	// carries ?v=<timestamp>, so it can be cached long.
	api.Get("/tournament-covers/:id", dbRequired, func(c *fiber.Ctx) error {
		img, updated, err := services.LoadCover(c.Context(), c.Params("id"))
		if err != nil {
			return fail(c, err)
		}
		if img == nil {
			return fiber.ErrNotFound
		}
		c.Set("Cache-Control", "public, max-age=604800, immutable")
		c.Set("Last-Modified", updated.UTC().Format(http.TimeFormat))
		c.Set("Content-Type", "image/jpeg")
		return c.Send(img)
	})

	tr := api.Group("/tournaments", middleware.AuthRequired(), dbRequired)

	// Cover picture: the host or an admin. Multipart "image".
	tr.Put("/:id/cover", func(c *fiber.Ctx) error {
		fh, err := c.FormFile("image")
		if err != nil {
			return utils.SendError(c, 400, "Choose a picture")
		}
		if fh.Size > services.CoverMaxUpload {
			return utils.SendError(c, 400, "Choose an image under 5 MB")
		}
		f, err := fh.Open()
		if err != nil {
			return fail(c, err)
		}
		data, err := io.ReadAll(io.LimitReader(f, services.CoverMaxUpload+1))
		f.Close()
		if err != nil {
			return fail(c, err)
		}
		jpg, err := services.ProcessCover(data)
		if err != nil {
			return fail(c, err)
		}
		url, err := services.SaveCover(c.Context(), c.Params("id"), middleware.GetUID(c), middleware.IsAdminCtx(c), jpg)
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"coverUrl": url})
	})
	tr.Delete("/:id/cover", func(c *fiber.Ctx) error {
		if err := services.RemoveCover(c.Context(), c.Params("id"), middleware.GetUID(c), middleware.IsAdminCtx(c)); err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{})
	})

	tr.Get("/", func(c *fiber.Ctx) error {
		rows, err := db.Pool.Query(c.Context(), tournamentSelect+`
			WHERE t.status <> 'finished' OR t.finished_at > NOW() - INTERVAL '7 days'
			ORDER BY (t.status = 'open') DESC, t.created_at DESC LIMIT 50`, middleware.GetUID(c))
		if err != nil {
			return fail(c, err)
		}
		defer rows.Close()
		list := []*tournamentView{}
		for rows.Next() {
			t, err := scanTournament(rows)
			if err != nil {
				return fail(c, err)
			}
			list = append(list, t)
		}
		return utils.SendSuccess(c, fiber.Map{"tournaments": list})
	})

	// What it costs me to host a cup right now.
	tr.Get("/host-options", func(c *fiber.Ctx) error {
		uid := middleware.GetUID(c)
		ctx := c.Context()
		var currency string
		var balance int64
		db.Pool.QueryRow(ctx, "SELECT COALESCE(currency,'NGN'), balance FROM users WHERE id = $1", uid).Scan(&currency, &balance)
		free, _ := services.FreeCupAvailable(ctx, db.Pool, uid)
		return utils.SendSuccess(c, fiber.Map{"enabled": config.Cfg.HostCupsEnabled, "currency": currency, "balance": balance,
			"minPool": services.MinCupPool[currency], "maxPool": services.MaxCupPool[currency], "feePercent": services.HostFeePercent,
			"freeCup": free, "fillDays": int(services.CupFillWindow.Hours() / 24), "maxOpen": services.MaxOpenHostCups,
			"commissionPercent": services.HostCommissionPercent, "commissionDays": int(services.HostCommissionWindow.Hours() / 24)})
	})

	// Host a cup: I fund the prize pool (+ hosting fee); players join free.
	tr.Post("/host", middleware.RateLimitMatchCreation(), func(c *fiber.Ctx) error {
		if !config.Cfg.HostCupsEnabled {
			return utils.SendError(c, 409, "Hosting cups is paused right now")
		}
		var req services.HostCupRequest
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid request")
		}
		id, err := services.HostCup(c.Context(), middleware.GetUID(c), req)
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"tournamentId": id})
	})

	// The host cancels their own cup before it starts: pool and fee come back.
	tr.Post("/:id/cancel", func(c *fiber.Ctx) error {
		uid := middleware.GetUID(c)
		var host string
		if err := db.Pool.QueryRow(c.Context(), "SELECT COALESCE(host_id,'') FROM tournaments WHERE id = $1", c.Params("id")).Scan(&host); err != nil || host != uid {
			return utils.SendError(c, 403, "Only the host can cancel this cup")
		}
		if err := services.CancelTournament(c.Context(), c.Params("id"), uid, "The host cancelled it."); err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{})
	})

	// Edit a cup after posting: the host, or an admin. Prizes never change.
	tr.Post("/:id/edit", func(c *fiber.Ctx) error {
		var req services.EditTournamentRequest
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid request")
		}
		tid, uid := c.Params("id"), middleware.GetUID(c)
		changes, err := services.EditTournament(c.Context(), tid, uid, middleware.IsAdminCtx(c), req)
		if err != nil {
			return fail(c, err)
		}
		if len(changes) > 0 {
			go notifyTournamentEdit(tid, uid, strings.Join(changes, ", "))
		}
		return utils.SendSuccess(c, fiber.Map{"changed": len(changes) > 0})
	})

	tr.Get("/:id", func(c *fiber.Ctx) error {
		ctx := c.Context()
		t, err := scanTournament(db.Pool.QueryRow(ctx, tournamentSelect+" WHERE t.id = $2", middleware.GetUID(c), c.Params("id")))
		if errors.Is(err, pgx.ErrNoRows) {
			return utils.SendError(c, 404, "Tournament not found")
		}
		if err != nil {
			return fail(c, err)
		}
		table, err := services.Standings(ctx, db.Pool, t.ID)
		if err != nil {
			return fail(c, err)
		}
		matches, err := services.TournamentMatches(ctx, t.ID)
		if err != nil {
			return fail(c, err)
		}
		if t.Format == "ffa" {
			uid, admin := middleware.GetUID(c), middleware.IsAdminCtx(c)
			t.CanManage = services.CanManageCup(t.HostID, uid, admin)
			if t.Room, err = services.GetCupRoom(ctx, t.ID, uid, admin); err != nil {
				return fail(c, err)
			}
		}
		var sponsor *services.Sponsor
		// Host cups and CrestArena's own cups (e.g. the End-of-Month Platter) can both have a sponsor.
		sponsor = services.GetSponsor(ctx, t.ID, (t.HostID != "" && t.HostID == middleware.GetUID(c)) || middleware.IsAdminCtx(c))
		return utils.SendSuccess(c, fiber.Map{"tournament": t, "standings": table, "matches": matches, "sponsor": sponsor})
	})

	// Free-for-All cups: the host posts the game room (this starts the cup and
	// alerts every player), then submits the final scoreboard.
	tr.Post("/:id/room", func(c *fiber.Ctx) error {
		var in struct {
			Code     string `json:"code"`
			Password string `json:"password"`
			Note     string `json:"note"`
		}
		if err := c.BodyParser(&in); err != nil {
			return utils.SendError(c, 400, "Invalid request")
		}
		room, err := services.PostCupRoom(c.Context(), c.Params("id"), middleware.GetUID(c), middleware.IsAdminCtx(c), in.Code, in.Password, in.Note)
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"room": room})
	})
	tr.Post("/:id/result", middleware.RateLimitOCR(), func(c *fiber.Ctx) error {
		fh, err := c.FormFile("image")
		if err != nil {
			return utils.SendError(c, 400, "Upload the final scoreboard screenshot")
		}
		if fh.Size > 8<<20 {
			return utils.SendError(c, 400, "The screenshot is larger than 8 MB")
		}
		f, err := fh.Open()
		if err != nil {
			return fail(c, err)
		}
		img, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return fail(c, err)
		}
		var res services.CupResult
		if err := json.Unmarshal([]byte(c.FormValue("result")), &res); err != nil {
			return utils.SendError(c, 400, "Invalid result")
		}
		if err := services.SubmitCupResult(c.Context(), c.Params("id"), middleware.GetUID(c), middleware.IsAdminCtx(c), res, img); err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"ok": true})
	})
	tr.Get("/:id/result-image", func(c *fiber.Ctx) error {
		img, err := services.CupResultImage(c.Context(), c.Params("id"), middleware.GetUID(c), middleware.IsAdminCtx(c))
		if err != nil {
			return fail(c, err)
		}
		c.Set("Content-Type", "image/jpeg")
		c.Set("Cache-Control", "private, max-age=300")
		return c.Send(img)
	})

	tr.Post("/:id/join", func(c *fiber.Ctx) error {
		uid := middleware.GetUID(c)
		ctx := c.Context()
		tid := c.Params("id")

		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return fail(c, err)
		}
		defer tx.Rollback(ctx)

		var fee int64
		var maxPlayers int
		var status, game, currency, name, host string
		var membersUntil *time.Time
		var earlyTier string
		var freeSeats *int
		var membersOnly bool
		err = tx.QueryRow(ctx, "SELECT entry_fee, max_players, status, game, currency, name, members_only_until, early_tier, free_seats, COALESCE(host_id,''), members_only FROM tournaments WHERE id = $1 FOR UPDATE", tid).
			Scan(&fee, &maxPlayers, &status, &game, &currency, &name, &membersUntil, &earlyTier, &freeSeats, &host, &membersOnly)
		if errors.Is(err, pgx.ErrNoRows) {
			return utils.SendError(c, 404, "Tournament not found")
		}
		if err != nil {
			return fail(c, err)
		}
		if status != "open" {
			return utils.SendError(c, 409, "Registration for this tournament is closed")
		}
		if err := services.RequireGameProfile(ctx, tx, uid, game); err != nil {
			return fail(c, err)
		}
		var count, freeTaken int
		var joined bool
		tx.QueryRow(ctx, "SELECT COUNT(*), COUNT(*) FILTER (WHERE free_entry), COALESCE(BOOL_OR(user_id = $2), false) FROM tournament_players WHERE tournament_id = $1", tid, uid).
			Scan(&count, &freeTaken, &joined)
		if joined {
			return utils.SendError(c, 409, "You're already registered")
		}
		if count >= maxPlayers {
			return utils.SendError(c, 409, "This tournament is full")
		}
		isFree := false
		if host != "" {
			// Player-hosted cup: free, no entry used, no seat caps. Just 18+ on
			// current terms, because the prizes are real money.
			if err := services.RequireMoneyAccess(ctx, uid); err != nil {
				return fail(c, err)
			}
			if membersOnly && host != uid && !services.HasPaidPlan(ctx, uid) {
				// Remember who brought them: the host earns a commission if
				// they now buy their first membership.
				services.RecordHostRef(ctx, db.Pool, uid, host)
				return utils.SendError(c, 403, "Members only: this cup is for players with a plan. Get any plan to join.")
			}
			services.RecordHostRef(ctx, tx, uid, host)
		} else if fee == 0 {
			// Prize tournaments: 18+ and terms, the members' early sign-up
			// window, and the monthly entries the player's plan allows.
			if err := services.RequireMoneyAccess(ctx, uid); err != nil {
				return fail(c, err)
			}
			if _, err := tx.Exec(ctx, "SELECT 1 FROM users WHERE id = $1 FOR UPDATE", uid); err != nil {
				return fail(c, err)
			}
			usage, err := services.EntryUsage(ctx, tx, uid)
			if err != nil {
				return fail(c, err)
			}
			if membersUntil != nil && time.Now().Before(*membersUntil) && services.TierRank(usage.Tier) < services.TierRank(earlyTier) {
				who := "Members get"
				if earlyTier == "elite" {
					who = "Elite members get"
				}
				return utils.SendError(c, 403, who+" early sign-up for this tournament. It opens to everyone "+membersUntil.Format("2 Jan, 15:04")+" (GMT).")
			}
			isFree = usage.Tier == services.FreeTier
			if isFree && freeSeats != nil && freeTaken >= *freeSeats {
				return utils.SendError(c, 403, "The free spots in this tournament are taken. Members can still join while seats are open.")
			}
			if usage.Left <= 0 {
				return utils.SendError(c, 403, fmt.Sprintf("You've used all %d tournament entries for this period. Upgrade your membership or wait until %s.",
					usage.Allowed, usage.ResetsAt.Format("2 Jan")))
			}
		} else {
			if !config.Cfg.StakingEnabled {
				return utils.SendError(c, 409, "Paid-entry tournaments are paused")
			}
			if err := services.RequireMoneyAccess(ctx, uid); err != nil {
				return fail(c, err)
			}
			var myCurrency string
			tx.QueryRow(ctx, "SELECT COALESCE(currency,'NGN') FROM users WHERE id = $1", uid).Scan(&myCurrency)
			if myCurrency != currency {
				return utils.SendError(c, 400, "This tournament is played in "+currency)
			}
			if err := services.AdjustBalance(ctx, tx, uid, -fee, "tournament_entry", tid); err != nil {
				if errors.Is(err, services.ErrInsufficientFunds) {
					return utils.SendError(c, 400, "Insufficient balance for the entry fee")
				}
				return fail(c, err)
			}
		}
		if _, err := tx.Exec(ctx, "INSERT INTO tournament_players (tournament_id, user_id, free_entry) VALUES ($1,$2,$3)", tid, uid, isFree); err != nil {
			return fail(c, err)
		}
		started := count+1 == maxPlayers
		if started {
			if err := services.StartTournament(ctx, tx, tid); err != nil {
				return fail(c, err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return fail(c, err)
		}
		if started {
			go notifyTournamentStart(tid, name)
		}
		return utils.SendSuccess(c, fiber.Map{"started": started})
	})

	// Leave before it starts (entry fee refunded).
	tr.Post("/:id/leave", func(c *fiber.Ctx) error {
		uid := middleware.GetUID(c)
		ctx := c.Context()
		tid := c.Params("id")
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return fail(c, err)
		}
		defer tx.Rollback(ctx)
		var fee int64
		var status string
		if err := tx.QueryRow(ctx, "SELECT entry_fee, status FROM tournaments WHERE id = $1 FOR UPDATE", tid).Scan(&fee, &status); err != nil {
			return utils.SendError(c, 404, "Tournament not found")
		}
		if status != "open" {
			return utils.SendError(c, 409, "You can't leave after the tournament has started")
		}
		tag, err := tx.Exec(ctx, "DELETE FROM tournament_players WHERE tournament_id = $1 AND user_id = $2", tid, uid)
		if err != nil {
			return fail(c, err)
		}
		if tag.RowsAffected() == 0 {
			return utils.SendError(c, 404, "You're not registered")
		}
		if fee > 0 {
			if err := services.AdjustBalance(ctx, tx, uid, fee, "tournament_refund", tid); err != nil {
				return fail(c, err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{})
	})
}

// notifyTournamentEdit tells the players who joined what the host changed.
func notifyTournamentEdit(tid, editor, what string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var name string
	db.Pool.QueryRow(ctx, "SELECT name FROM tournaments WHERE id = $1", tid).Scan(&name)
	rows, err := db.Pool.Query(ctx, "SELECT user_id FROM tournament_players WHERE tournament_id = $1 AND user_id <> $2", tid, editor)
	if err != nil {
		return
	}
	ids, _ := pgx.CollectRows(rows, pgx.RowTo[string])
	for _, id := range ids {
		services.Notify(id, "tournament_update", "Cup updated: "+name, "Changed: "+what+". Prizes stay the same.", "/tournaments/"+tid, map[string]any{"tournamentId": tid})
	}
}

func notifyTournamentStart(tid, name string) {
	ctx := context.Background()
	rows, err := db.Pool.Query(ctx, "SELECT user_id FROM tournament_players WHERE tournament_id = $1", tid)
	if err != nil {
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	var format string
	db.Pool.QueryRow(ctx, "SELECT format FROM tournaments WHERE id = $1", tid).Scan(&format)
	for _, id := range ids {
		if format == "ffa" {
			services.Notify(id, "tournament_started", name+" is full! 🎯",
				"All seats are taken. At start time the host posts the Room ID here and you'll get a loud alert.", "/tournaments/"+tid, map[string]any{"tournamentId": tid})
			continue
		}
		services.Notify(id, "tournament_started", name+" has started! 🏁",
			"Your first match is ready. Open the tournament to see your opponent and deadline.", "/tournaments/"+tid, map[string]any{"tournamentId": tid})
	}
	// Round 1 may contain only byes (odd player counts); move on if so.
	services.AdvanceTournament(ctx, tid)
}
