package routes

import (
	"context"

	"github.com/gofiber/fiber/v2"

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
}
