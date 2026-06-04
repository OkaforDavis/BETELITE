package routes

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/services"
	"betelite-go/utils"
)

func SetupAdminRoutes(api fiber.Router) {
	admin := api.Group("/admin", middleware.AuthRequired(), middleware.AdminRequired())

	// Get overall stats
	admin.Get("/stats", func(c *fiber.Ctx) error {
		ctx := context.Background()

		var userCount int
		var totalEscrow int64
		var activeMatches int

		db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount)
		db.Pool.QueryRow(ctx, "SELECT COALESCE(SUM(pool), 0) FROM escrow WHERE status = 'held'").Scan(&totalEscrow)
		db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM escrow WHERE status = 'held'").Scan(&activeMatches)

		return utils.SendSuccess(c, fiber.Map{
			"stats": fiber.Map{
				"users":         userCount,
				"totalEscrow":   totalEscrow,
				"activeMatches": activeMatches,
			},
		})
	})

	// Get recent transactions
	admin.Get("/transactions", func(c *fiber.Ctx) error {
		ctx := context.Background()
		rows, err := db.Pool.Query(ctx, "SELECT id, user_id, type, amount, created_at FROM transactions ORDER BY created_at DESC LIMIT 50")
		if err != nil {
			return utils.SendError(c, 500, "Failed to fetch transactions")
		}
		defer rows.Close()

		var txs []fiber.Map
		for rows.Next() {
			var id int64
			var uid, tType string
			var amount int64
			var created string
			if err := rows.Scan(&id, &uid, &tType, &amount, &created); err == nil {
				txs = append(txs, fiber.Map{
					"id":        id,
					"userId":    uid,
					"type":      tType,
					"amount":    amount,
					"createdAt": created,
				})
			}
		}

		return utils.SendSuccess(c, fiber.Map{"transactions": txs})
	})

	// Add/Remove Balance
	admin.Post("/balance", func(c *fiber.Ctx) error {
		var req struct {
			UserID string `json:"userId"`
			Amount int64  `json:"amount"` // Can be negative to deduct
			Reason string `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid payload")
		}

		ctx := context.Background()
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return utils.SendError(c, 500, "Database error")
		}
		defer tx.Rollback(ctx)

		err = services.AdjustBalance(ctx, tx, req.UserID, req.Amount, "admin_adjustment", req.Reason)
		if err != nil {
			return utils.SendError(c, 500, "Failed to update balance")
		}

		if err := tx.Commit(ctx); err != nil {
			return utils.SendError(c, 500, "Transaction commit failed")
		}

		return utils.SendSuccess(c, fiber.Map{})
	})

	// ─── TOURNAMENT MANAGEMENT ──────────────────────────────────────

	// Create a tournament (free or gated with entry fee)
	// Free tournaments (entry_fee = 0) can only be posted ONCE per week
	admin.Post("/tournaments", func(c *fiber.Ctx) error {
		var req struct {
			Name       string `json:"name"`
			Game       string `json:"game"`
			Mode       string `json:"mode"`
			Icon       string `json:"icon"`
			EntryFee   int64  `json:"entryFee"`   // 0 = free, >0 = gated
			MaxPlayers int    `json:"maxPlayers"`
			PrizePool  int64  `json:"prizePool"`   // Admin-seeded prize for free tournaments
		}
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid payload")
		}

		if req.Name == "" || req.Game == "" {
			return utils.SendError(c, 400, "Name and game are required")
		}
		if req.MaxPlayers < 2 {
			req.MaxPlayers = 8
		}

		uid := middleware.GetUID(c)
		ctx := context.Background()

		// ── FREE TOURNAMENT WEEKLY LIMIT ──
		// Free tournaments (entry_fee = 0) can only be posted once per week
		if req.EntryFee <= 0 {
			req.EntryFee = 0 // Normalize

			// Check if a free tournament was already created this week
			startOfWeek := getStartOfCurrentWeek()
			var existingCount int
			err := db.Pool.QueryRow(ctx,
				"SELECT COUNT(*) FROM tournaments WHERE entry_fee = 0 AND created_at >= $1",
				startOfWeek,
			).Scan(&existingCount)
			if err != nil {
				return utils.SendError(c, 500, "Database error checking free tournament limit")
			}
			if existingCount > 0 {
				return utils.SendError(c, 429, "A free tournament has already been posted this week. Free tournaments are limited to once per week.")
			}
		}

		tournamentID := uuid.New().String()

		_, err := db.Pool.Exec(ctx,
			`INSERT INTO tournaments (id, name, game, mode, icon, entry_fee, max_players, prize_pool, status, created_by, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'open', $9, NOW())`,
			tournamentID, req.Name, req.Game, req.Mode, req.Icon, req.EntryFee, req.MaxPlayers, req.PrizePool, uid,
		)
		if err != nil {
			return utils.SendError(c, 500, "Failed to create tournament: "+err.Error())
		}

		feeType := "gated"
		if req.EntryFee == 0 {
			feeType = "free"
		}
		fmt.Printf("[ADMIN] Tournament created: %s (%s, %s, fee=%d, pool=%d) by %s\n",
			req.Name, req.Game, feeType, req.EntryFee, req.PrizePool, uid)

		return utils.SendSuccess(c, fiber.Map{
			"tournamentId": tournamentID,
			"type":         feeType,
		})
	})

	// List all tournaments (admin view — includes all statuses)
	admin.Get("/tournaments", func(c *fiber.Ctx) error {
		ctx := context.Background()
		rows, err := db.Pool.Query(ctx,
			`SELECT id, name, game, COALESCE(mode,''), COALESCE(icon,''), entry_fee, max_players, prize_pool, status, COALESCE(created_by,''), created_at
			 FROM tournaments ORDER BY created_at DESC`)
		if err != nil {
			return utils.SendError(c, 500, "Failed to fetch tournaments")
		}
		defer rows.Close()

		var results []fiber.Map
		for rows.Next() {
			var id, name, game, mode, icon, status, createdBy string
			var entryFee, prizePool int64
			var maxPlayers int
			var createdAt time.Time
			if err := rows.Scan(&id, &name, &game, &mode, &icon, &entryFee, &maxPlayers, &prizePool, &status, &createdBy, &createdAt); err == nil {
				// Get player count
				var playerCount int
				db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM tournament_players WHERE tournament_id = $1", id).Scan(&playerCount)

				feeType := "gated"
				if entryFee == 0 {
					feeType = "free"
				}
				results = append(results, fiber.Map{
					"id":          id,
					"name":        name,
					"game":        game,
					"mode":        mode,
					"icon":        icon,
					"entryFee":    entryFee,
					"maxPlayers":  maxPlayers,
					"prizePool":   prizePool,
					"status":      status,
					"type":        feeType,
					"playerCount": playerCount,
					"createdBy":   createdBy,
					"createdAt":   createdAt,
				})
			}
		}

		if results == nil {
			results = []fiber.Map{}
		}

		return utils.SendSuccess(c, fiber.Map{"tournaments": results})
	})

	// Delete a tournament (admin only, only if still open)
	admin.Delete("/tournaments/:id", func(c *fiber.Ctx) error {
		tID := c.Params("id")
		ctx := context.Background()

		// Only allow deleting open tournaments (not active/finished)
		var status string
		err := db.Pool.QueryRow(ctx, "SELECT status FROM tournaments WHERE id = $1", tID).Scan(&status)
		if err != nil {
			return utils.SendError(c, 404, "Tournament not found")
		}
		if status != "open" {
			return utils.SendError(c, 400, "Cannot delete a tournament that is already active or finished")
		}

		// Refund any players who already joined
		rows, err := db.Pool.Query(ctx, "SELECT user_id FROM tournament_players WHERE tournament_id = $1", tID)
		if err == nil {
			defer rows.Close()
			var entryFee int64
			db.Pool.QueryRow(ctx, "SELECT entry_fee FROM tournaments WHERE id = $1", tID).Scan(&entryFee)

			if entryFee > 0 {
				for rows.Next() {
					var userID string
					if rows.Scan(&userID) == nil {
						tx, err := db.Pool.Begin(ctx)
						if err == nil {
							services.AdjustBalance(ctx, tx, userID, entryFee, "tournament_refund", tID)
							tx.Commit(ctx)
						}
					}
				}
			}
		}

		// Delete players and tournament
		db.Pool.Exec(ctx, "DELETE FROM tournament_players WHERE tournament_id = $1", tID)
		db.Pool.Exec(ctx, "DELETE FROM fixtures WHERE tournament_id = $1", tID)
		_, err = db.Pool.Exec(ctx, "DELETE FROM tournaments WHERE id = $1", tID)
		if err != nil {
			return utils.SendError(c, 500, "Failed to delete tournament")
		}

		fmt.Printf("[ADMIN] Tournament %s deleted\n", tID)
		return utils.SendSuccess(c, fiber.Map{})
	})
}

// getStartOfCurrentWeek returns the start of the current ISO week (Monday 00:00 UTC)
func getStartOfCurrentWeek() time.Time {
	now := time.Now().UTC()
	weekday := now.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	daysToSubtract := int(weekday) - 1 // Monday = 0 days back
	startOfWeek := time.Date(now.Year(), now.Month(), now.Day()-daysToSubtract, 0, 0, 0, 0, time.UTC)
	return startOfWeek
}

