package routes

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"betelite-go/db"
	"betelite-go/middleware"
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

		return c.JSON(fiber.Map{
			"success": true,
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
			return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch transactions"})
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

		return c.JSON(fiber.Map{"success": true, "transactions": txs})
	})

	// Add/Remove Balance
	admin.Post("/balance", func(c *fiber.Ctx) error {
		var req struct {
			UserID string `json:"userId"`
			Amount int64  `json:"amount"` // Can be negative to deduct
			Reason string `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid payload"})
		}

		ctx := context.Background()
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Database error"})
		}
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx, "UPDATE users SET balance = balance + $1 WHERE id = $2", req.Amount, req.UserID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to update balance"})
		}

		_, err = tx.Exec(ctx, "INSERT INTO transactions (user_id, type, amount, ref_id) VALUES ($1, 'admin_adjustment', $2, $3)",
			req.UserID, req.Amount, req.Reason)

		if err := tx.Commit(ctx); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Transaction commit failed"})
		}

		return c.JSON(fiber.Map{"success": true})
	})
}
