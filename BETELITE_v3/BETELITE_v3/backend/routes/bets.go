package routes

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/services"
)

func SetupBetRoutes(api fiber.Router) {
	bets := api.Group("/bets", middleware.AuthRequired())

	bets.Post("/place", func(c *fiber.Ctx) error {
		var req struct {
			MatchID string  `json:"matchId"`
			Pick    string  `json:"pick"` // home, away, draw
			Odds    float32 `json:"odds"`
			Amount  int64   `json:"amount"` // Note: kobo/pesewas
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid payload"})
		}

		if req.Amount <= 0 {
			return c.Status(400).JSON(fiber.Map{"error": "Amount must be greater than 0"})
		}

		// Verify match is live and taking bets
		match := services.Engine.GetMatch(req.MatchID)
		if match == nil || match.Status != "live" {
			return c.Status(400).JSON(fiber.Map{"error": "Match not available for betting"})
		}
		if match.Minute >= 80 {
			return c.Status(400).JSON(fiber.Map{"error": "Betting is closed for this match (80+ mins)"})
		}

		uid := middleware.GetUID(c)
		ctx := context.Background()

		// DB Transaction
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Database error"})
		}
		defer tx.Rollback(ctx)

		// Check balance
		var balance int64
		err = tx.QueryRow(ctx, "SELECT balance FROM users WHERE id = $1 FOR UPDATE", uid).Scan(&balance)
		if err != nil || balance < req.Amount {
			return c.Status(400).JSON(fiber.Map{"error": "Insufficient funds"})
		}

		// Deduct balance
		_, err = tx.Exec(ctx, "UPDATE users SET balance = balance - $1 WHERE id = $2", req.Amount, uid)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to update balance"})
		}

		// Insert Bet
		betID := fmt.Sprintf("bet_%d", time.Now().UnixNano())
		potentialWin := int64(float32(req.Amount) * req.Odds)

		_, err = tx.Exec(ctx, `INSERT INTO bets (id, user_id, match_id, pick, odds, amount, potential_win, status) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'live')`,
			betID, uid, req.MatchID, req.Pick, req.Odds, req.Amount, potentialWin)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to place bet"})
		}

		// Log transaction
		_, err = tx.Exec(ctx, "INSERT INTO transactions (user_id, type, amount, ref_id) VALUES ($1, 'bet_place', $2, $3)",
			uid, -req.Amount, betID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to log transaction"})
		}

		if err := tx.Commit(ctx); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Transaction commit failed"})
		}

		return c.JSON(fiber.Map{"success": true, "betId": betID})
	})
}
