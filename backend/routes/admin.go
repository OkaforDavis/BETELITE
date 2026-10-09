package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"betelite-go/config"
	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/services"
	"betelite-go/utils"
)

func SetupAdminRoutes(api fiber.Router) {
	admin := api.Group("/admin", middleware.AuthRequired(), middleware.AdminRequired(), dbRequired)

	admin.Get("/stats", func(c *fiber.Ctx) error {
		ctx := c.Context()
		var users, activeMatches, needsReview, pendingWithdrawals int
		var held, wallets, feesToday int64
		db.Pool.QueryRow(ctx, "SELECT COUNT(*), COALESCE(SUM(balance),0) FROM users WHERE deleted_at IS NULL").Scan(&users, &wallets)
		db.Pool.QueryRow(ctx, "SELECT COALESCE(SUM(pool),0) FROM escrow WHERE status IN ('waiting','held')").Scan(&held)
		db.Pool.QueryRow(ctx, "SELECT COUNT(*) FILTER (WHERE status IN ('ready','submitted')), COUNT(*) FILTER (WHERE status IN ('disputed','review')) FROM matches").Scan(&activeMatches, &needsReview)
		db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM withdrawals WHERE status = 'pending'").Scan(&pendingWithdrawals)
		db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM((metadata->>'fee')::bigint),0) FROM transactions
			WHERE type = 'wager_win' AND created_at > NOW() - INTERVAL '1 day'`).Scan(&feesToday)
		return utils.SendSuccess(c, fiber.Map{"stats": fiber.Map{
			"users": users, "walletTotal": wallets, "inEscrow": held, "activeMatches": activeMatches,
			"needsReview": needsReview, "pendingWithdrawals": pendingWithdrawals, "feesToday": feesToday,
		}})
	})

	admin.Get("/transactions", func(c *fiber.Ctx) error {
		rows, err := db.Pool.Query(c.Context(), `SELECT t.id, t.user_id, COALESCE(u.username,''), t.type, t.amount, COALESCE(t.ref_id,''), t.created_at
			FROM transactions t LEFT JOIN users u ON u.id = t.user_id ORDER BY t.created_at DESC LIMIT 100`)
		if err != nil {
			return fail(c, err)
		}
		defer rows.Close()
		list := []fiber.Map{}
		for rows.Next() {
			var id, amount int64
			var uid, name, typ, ref string
			var at time.Time
			if rows.Scan(&id, &uid, &name, &typ, &amount, &ref, &at) == nil {
				list = append(list, fiber.Map{"id": id, "userId": uid, "username": name, "type": typ, "amount": amount, "ref": ref, "createdAt": at})
			}
		}
		return utils.SendSuccess(c, fiber.Map{"transactions": list})
	})

	// Find users by email / username (for balance adjustments and support).
	admin.Get("/users", func(c *fiber.Ctx) error {
		q := "%" + strings.ToLower(strings.TrimSpace(c.Query("q"))) + "%"
		rows, err := db.Pool.Query(c.Context(), `SELECT id, email, username, balance, COALESCE(currency,'NGN'), is_admin, created_at, COALESCE(avatar_url,'') FROM users
			WHERE deleted_at IS NULL AND (lower(email) LIKE $1 OR lower(username) LIKE $1 OR id = $2) ORDER BY created_at DESC LIMIT 25`, q, c.Query("q"))
		if err != nil {
			return fail(c, err)
		}
		defer rows.Close()
		list := []fiber.Map{}
		for rows.Next() {
			var id, email, name, cur, avatar string
			var bal int64
			var isAdmin bool
			var at time.Time
			if rows.Scan(&id, &email, &name, &bal, &cur, &isAdmin, &at, &avatar) == nil {
				list = append(list, fiber.Map{"id": id, "email": email, "username": name, "balance": bal, "currency": cur, "isAdmin": isAdmin, "createdAt": at, "avatarUrl": avatar})
			}
		}
		return utils.SendSuccess(c, fiber.Map{"users": list})
	})

	// Manual balance correction (always logged with the admin and reason).
	admin.Post("/balance", func(c *fiber.Ctx) error {
		var req struct {
			UserID string `json:"userId"`
			Amount int64  `json:"amount"`
			Reason string `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil || req.UserID == "" || req.Amount == 0 {
			return utils.SendError(c, 400, "User, amount and reason are required")
		}
		if len(strings.TrimSpace(req.Reason)) < 5 {
			return utils.SendError(c, 400, "Give a reason (at least 5 characters)")
		}
		ctx := c.Context()
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return fail(c, err)
		}
		defer tx.Rollback(ctx)
		meta, _ := json.Marshal(map[string]string{"admin": middleware.GetUID(c), "reason": req.Reason})
		if err := services.AdjustBalance(ctx, tx, req.UserID, req.Amount, "admin_adjustment", "admin", string(meta)); err != nil {
			return fail(c, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fail(c, err)
		}
		log.Printf("[ADMIN] %s adjusted %s by %d: %s", middleware.GetEmail(c), req.UserID, req.Amount, req.Reason)
		return utils.SendSuccess(c, fiber.Map{})
	})

	// Remove an offensive profile photo.
	admin.Delete("/users/:id/avatar", func(c *fiber.Ctx) error {
		if err := services.RemoveAvatar(c.Context(), c.Params("id")); err != nil {
			return fail(c, err)
		}
		log.Printf("[ADMIN] %s removed the profile photo of %s", middleware.GetEmail(c), c.Params("id"))
		services.Notify(c.Params("id"), "photo_removed", "Profile photo removed", "Your profile photo was removed because it broke our rules. You can upload a different one.", "/profile", nil)
		return utils.SendSuccess(c, fiber.Map{})
	})

	// ── Disputes & reviews ──────────────────────────────────────────

	admin.Get("/matches/review", func(c *fiber.Ctx) error {
		list, err := services.MatchesNeedingAdmin(c.Context())
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"matches": list})
	})

	// What the AI read from the submitted screenshot (for the review screen).
	admin.Get("/matches/:id/ocr", func(c *fiber.Ctx) error {
		var raw []byte
		db.Pool.QueryRow(c.Context(), "SELECT ocr_result FROM matches WHERE id = $1", c.Params("id")).Scan(&raw)
		return utils.SendSuccess(c, fiber.Map{"ocr": json.RawMessage(nonNullJSON(raw))})
	})

	admin.Post("/matches/:id/resolve", func(c *fiber.Ctx) error {
		var req struct {
			ScoreHome *int `json:"scoreHome"`
			ScoreAway *int `json:"scoreAway"`
			PensHome  *int `json:"pensHome"`
			PensAway  *int `json:"pensAway"`
		}
		if err := c.BodyParser(&req); err != nil || req.ScoreHome == nil || req.ScoreAway == nil || *req.ScoreHome < 0 || *req.ScoreAway < 0 {
			return utils.SendError(c, 400, "Enter both scores")
		}
		if err := services.AdminResolve(c.Context(), c.Params("id"), middleware.GetUID(c), *req.ScoreHome, *req.ScoreAway, req.PensHome, req.PensAway); err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{})
	})

	// Void: 1v1 → refund both; tournament → reset for a replay.
	admin.Post("/matches/:id/void", func(c *fiber.Ctx) error {
		var req struct {
			Reason string `json:"reason"`
		}
		c.BodyParser(&req)
		if err := services.VoidMatch(c.Context(), c.Params("id"), middleware.GetUID(c), req.Reason); err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{})
	})

	// ── Withdrawals ─────────────────────────────────────────────────

	admin.Get("/withdrawals", func(c *fiber.Ctx) error {
		status := c.Query("status", "pending")
		rows, err := db.Pool.Query(c.Context(), `SELECT w.id, w.user_id, COALESCE(u.username,''), COALESCE(u.email,''), w.amount, w.currency,
			COALESCE(w.bank_name,''), w.account_number, COALESCE(w.account_name,''), w.status, COALESCE(w.note,''), w.created_at
			FROM withdrawals w LEFT JOIN users u ON u.id = w.user_id WHERE w.status = $1 ORDER BY w.created_at LIMIT 100`, status)
		if err != nil {
			return fail(c, err)
		}
		defer rows.Close()
		list := []fiber.Map{}
		for rows.Next() {
			var id, uid, name, email, cur, bank, acct, acctName, st, note string
			var amount int64
			var at time.Time
			if rows.Scan(&id, &uid, &name, &email, &amount, &cur, &bank, &acct, &acctName, &st, &note, &at) == nil {
				list = append(list, fiber.Map{"id": id, "userId": uid, "username": name, "email": email, "amount": amount, "currency": cur,
					"bankName": bank, "accountNumber": acct, "accountName": acctName, "status": st, "note": note, "createdAt": at})
			}
		}
		return utils.SendSuccess(c, fiber.Map{"withdrawals": list})
	})

	admin.Post("/withdrawals/:id/approve", func(c *fiber.Ctx) error {
		if err := services.ApproveWithdrawal(c.Context(), c.Params("id"), middleware.GetUID(c)); err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{})
	})

	// Record a withdrawal paid by hand (bank transfer) instead of through Paystack.
	admin.Post("/withdrawals/:id/mark-paid", func(c *fiber.Ctx) error {
		var req struct {
			Note string `json:"note"`
		}
		c.BodyParser(&req)
		if len(strings.TrimSpace(req.Note)) < 3 {
			return utils.SendError(c, 400, "Add the bank transfer reference")
		}
		if err := services.MarkWithdrawalPaid(c.Context(), c.Params("id"), middleware.GetUID(c), req.Note); err != nil {
			return fail(c, err)
		}
		log.Printf("[ADMIN] %s marked withdrawal %s paid manually (%s)", middleware.GetEmail(c), c.Params("id"), req.Note)
		return utils.SendSuccess(c, fiber.Map{})
	})

	admin.Post("/withdrawals/:id/reject", func(c *fiber.Ctx) error {
		var req struct {
			Reason string `json:"reason"`
		}
		c.BodyParser(&req)
		if strings.TrimSpace(req.Reason) == "" {
			return utils.SendError(c, 400, "Give the user a reason")
		}
		if err := services.RefundWithdrawal(c.Context(), c.Params("id"), req.Reason, "pending"); err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{})
	})

	// ── Tournaments ─────────────────────────────────────────────────

	admin.Post("/tournaments", func(c *fiber.Ctx) error {
		var req struct {
			Name       string               `json:"name"`
			Game       string               `json:"game"`
			Format     string               `json:"format"` // knockout | league | swiss
			Icon       string               `json:"icon"`
			EntryFee   int64                `json:"entryFee"` // only when staking is enabled
			Currency   string               `json:"currency"` // entry fee currency
			MaxPlayers int                  `json:"maxPlayers"`
			Prizes     services.FixedPrizes `json:"prizes"`     // fixed prizes per currency, minor units
			PrizeSplit []float64            `json:"prizeSplit"` // paid-entry tournaments only
			RoundHours int                  `json:"roundHours"`
			EarlyHours int                  `json:"earlyHours"` // members-only sign-up window
		}
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid request")
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" || services.GameByID(req.Game) == nil {
			return utils.SendError(c, 400, "Name and a supported game are required")
		}
		switch req.Format {
		case "league", "swiss":
		default:
			req.Format = "knockout"
		}
		if req.Currency != "GHS" {
			req.Currency = "NGN"
		}
		if req.MaxPlayers < 2 || req.MaxPlayers > 64 {
			return utils.SendError(c, 400, "Players must be between 2 and 64")
		}
		if req.Format == "league" && req.MaxPlayers > 20 {
			return utils.SendError(c, 400, "Leagues are limited to 20 players (everyone plays everyone)")
		}
		if req.Format == "swiss" && (req.MaxPlayers < 4 || req.MaxPlayers%2 == 1) {
			return utils.SendError(c, 400, "Swiss tournaments need an even number of players (4 or more)")
		}
		if req.RoundHours <= 0 {
			req.RoundHours = 24
		}
		if req.EarlyHours < 0 || req.EarlyHours > 72 {
			return utils.SendError(c, 400, "Early sign-up must be between 0 and 72 hours")
		}
		if req.EntryFee < 0 {
			return utils.SendError(c, 400, "Amounts can't be negative")
		}
		if !config.Cfg.StakingEnabled {
			req.EntryFee = 0
		}

		var prizesJSON, splitJSON any
		if req.EntryFee == 0 {
			// Prize tournament: fixed prizes in both currencies, paid by us.
			places := req.Prizes.Places()
			if places == 0 || len(req.Prizes["GHS"]) != places || len(req.Prizes["NGN"]) != places {
				return utils.SendError(c, 400, "Set the prizes in both cedis and naira, one amount per paid place")
			}
			for _, list := range req.Prizes {
				for _, p := range list {
					if p <= 0 {
						return utils.SendError(c, 400, "Every paid place needs a prize above zero")
					}
				}
			}
			if places > req.MaxPlayers {
				return utils.SendError(c, 400, "More paid places than players")
			}
			if req.Format == "knockout" && places > 2 {
				return utils.SendError(c, 400, "Knockouts can pay 1st and 2nd only. Use Swiss or League to pay a top 3.")
			}
			b, _ := json.Marshal(map[string][]int64{"GHS": req.Prizes["GHS"], "NGN": req.Prizes["NGN"]})
			prizesJSON = string(b)
		} else {
			if len(req.PrizeSplit) == 0 {
				req.PrizeSplit = services.DefaultSplit(req.Format)
				if len(req.PrizeSplit) > req.MaxPlayers {
					req.PrizeSplit = req.PrizeSplit[:req.MaxPlayers]
				}
			}
			var total float64
			for _, p := range req.PrizeSplit {
				if p < 0 {
					return utils.SendError(c, 400, "Prize percentages can't be negative")
				}
				total += p
			}
			if total > 100 || len(req.PrizeSplit) > req.MaxPlayers {
				return utils.SendError(c, 400, "Prize split must add up to 100% or less and not exceed the number of players")
			}
			b, _ := json.Marshal(req.PrizeSplit)
			splitJSON = string(b)
		}

		var membersUntil any
		if req.EarlyHours > 0 {
			membersUntil = time.Now().Add(time.Duration(req.EarlyHours) * time.Hour)
		}
		ctx := c.Context()
		id := utils.GenerateTournamentID()
		_, err := db.Pool.Exec(ctx, `INSERT INTO tournaments (id, name, game, mode, format, icon, entry_fee, currency, max_players, prize_pool, prize_split, prizes, round_hours, members_only_until, created_by, status)
			VALUES ($1,$2,$3,$4,$4,$5,$6,$7,$8,0,$9::jsonb,$10::jsonb,$11,$12,$13,'open')`,
			id, req.Name, req.Game, req.Format, req.Icon, req.EntryFee, req.Currency, req.MaxPlayers, splitJSON, prizesJSON, req.RoundHours, membersUntil, middleware.GetUID(c))
		if err != nil {
			return fail(c, err)
		}
		log.Printf("[ADMIN] tournament %s created: %s %s fee=%d players=%d prizes=%v", id, req.Format, req.Game, req.EntryFee, req.MaxPlayers, req.Prizes)
		if req.EntryFee == 0 && req.EarlyHours > 0 {
			go announceEarlySignup(id, req.Name)
		}
		return utils.SendSuccess(c, fiber.Map{"tournamentId": id})
	})

	// Cancel an open tournament and refund everyone who joined.
	admin.Delete("/tournaments/:id", func(c *fiber.Ctx) error {
		ctx := c.Context()
		tid := c.Params("id")
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return fail(c, err)
		}
		defer tx.Rollback(ctx)
		var status, name string
		var fee int64
		if err := tx.QueryRow(ctx, "SELECT status, entry_fee, name FROM tournaments WHERE id = $1 FOR UPDATE", tid).Scan(&status, &fee, &name); err != nil {
			return utils.SendError(c, 404, "Tournament not found")
		}
		if status != "open" {
			return utils.SendError(c, 409, "Only tournaments that haven't started can be cancelled")
		}
		rows, err := tx.Query(ctx, "SELECT user_id FROM tournament_players WHERE tournament_id = $1", tid)
		if err != nil {
			return fail(c, err)
		}
		var players []string
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				players = append(players, id)
			}
		}
		rows.Close()
		for _, p := range players {
			if fee > 0 {
				if err := services.AdjustBalance(ctx, tx, p, fee, "tournament_refund", tid); err != nil {
					return fail(c, err)
				}
			}
		}
		if _, err := tx.Exec(ctx, "DELETE FROM tournament_players WHERE tournament_id = $1", tid); err != nil {
			return fail(c, err)
		}
		if _, err := tx.Exec(ctx, "UPDATE tournaments SET status = 'cancelled' WHERE id = $1", tid); err != nil {
			return fail(c, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fail(c, err)
		}
		for _, p := range players {
			services.Notify(p, "tournament_cancelled", "Tournament cancelled", name+" was cancelled. Any entry fee has been refunded.", "/tournaments", nil)
		}
		return utils.SendSuccess(c, fiber.Map{})
	})

	// Test the OCR on a screenshot without touching any match.
	admin.Post("/ocr-test", func(c *fiber.Ctx) error {
		game := services.GameByID(c.FormValue("game"))
		if game == nil {
			return utils.SendError(c, 400, "Choose a game")
		}
		fh, err := c.FormFile("image")
		if err != nil {
			return utils.SendError(c, 400, "Attach a screenshot")
		}
		f, err := fh.Open()
		if err != nil {
			return fail(c, err)
		}
		img, _ := io.ReadAll(io.LimitReader(f, services.MaxScreenshotLen))
		f.Close()
		var expected []services.ExpectedPlayer
		for _, n := range []string{c.FormValue("player1"), c.FormValue("player2")} {
			if n = strings.TrimSpace(n); n != "" {
				expected = append(expected, services.ExpectedPlayer{Gamertag: n})
			}
		}
		res, err := services.AnalyzeScreenshot(c.Context(), img, game, expected)
		if err != nil {
			return utils.SendError(c, 502, fmt.Sprintf("OCR failed: %v", err))
		}
		verdict := "accepted"
		switch {
		case !res.IsVersusHuman:
			verdict = "rejected: not a player-vs-player match"
		case !res.IsResultScreen:
			verdict = "rejected: not a final result screen"
		case res.Confidence < services.MinOCRConfidence:
			verdict = "rejected: low confidence"
		case len(expected) == 2:
			if _, _, err := services.MatchPlayers(res, expected[0], expected[1]); err != nil {
				verdict = "rejected: " + err.Error()
			}
		}
		return utils.SendSuccess(c, fiber.Map{"ocr": res, "verdict": verdict})
	})
}

func nonNullJSON(b []byte) []byte {
	if len(b) == 0 {
		return []byte("null")
	}
	return b
}

// startOfWeek returns Monday 00:00 UTC of the current week.
func startOfWeek() time.Time {
	now := time.Now().UTC()
	wd := int(now.Weekday())
	if wd == 0 {
		wd = 7
	}
	return time.Date(now.Year(), now.Month(), now.Day()-(wd-1), 0, 0, 0, 0, time.UTC)
}

// announceEarlySignup tells current members a tournament is open to them first.
func announceEarlySignup(tid, name string) {
	ctx := context.Background()
	rows, err := db.Pool.Query(ctx, "SELECT DISTINCT user_id FROM memberships WHERE starts_at <= NOW() AND ends_at > NOW()")
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
	for _, id := range ids {
		services.Notify(id, "tournament_new", "Early sign-up: "+name, "Members can join before everyone else. Grab your spot.", "/tournaments/"+tid, map[string]any{"tournamentId": tid})
	}
}
