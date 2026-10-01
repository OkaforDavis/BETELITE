package routes

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

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
			Name       string    `json:"name"`
			Game       string    `json:"game"`
			Format     string    `json:"format"` // knockout | league
			Icon       string    `json:"icon"`
			EntryFee   int64     `json:"entryFee"`
			Currency   string    `json:"currency"`
			MaxPlayers int       `json:"maxPlayers"`
			PrizePool  int64     `json:"prizePool"` // seed for free tournaments
			PrizeSplit []float64 `json:"prizeSplit"`
			RoundHours int       `json:"roundHours"`
		}
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid request")
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" || services.GameByID(req.Game) == nil {
			return utils.SendError(c, 400, "Name and a supported game are required")
		}
		if req.Format != "league" {
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
		if req.RoundHours <= 0 {
			req.RoundHours = 24
		}
		if req.EntryFee < 0 || req.PrizePool < 0 {
			return utils.SendError(c, 400, "Amounts can't be negative")
		}
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
		ctx := c.Context()
		if req.EntryFee == 0 {
			var count int
			db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM tournaments WHERE entry_fee = 0 AND created_at >= $1", startOfWeek()).Scan(&count)
			if count > 0 {
				return utils.SendError(c, 429, "Only one free tournament can be posted per week")
			}
		}
		split, _ := json.Marshal(req.PrizeSplit)
		id := utils.GenerateTournamentID()
		_, err := db.Pool.Exec(ctx, `INSERT INTO tournaments (id, name, game, mode, format, icon, entry_fee, currency, max_players, prize_pool, prize_split, round_hours, created_by, status)
			VALUES ($1,$2,$3,$4,$4,$5,$6,$7,$8,$9,$10::jsonb,$11,$12,'open')`,
			id, req.Name, req.Game, req.Format, req.Icon, req.EntryFee, req.Currency, req.MaxPlayers, req.PrizePool, string(split), req.RoundHours, middleware.GetUID(c))
		if err != nil {
			return fail(c, err)
		}
		log.Printf("[ADMIN] tournament %s created: %s %s fee=%d players=%d split=%v", id, req.Format, req.Game, req.EntryFee, req.MaxPlayers, req.PrizeSplit)
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
