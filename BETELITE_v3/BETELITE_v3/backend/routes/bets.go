package routes

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/services"
	"betelite-go/utils"
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
			return utils.SendError(c, 400, "Invalid payload")
		}

		if req.Amount <= 0 {
			return utils.SendError(c, 400, "Amount must be greater than 0")
		}

		// Verify match is live and taking bets
		match := services.Engine.GetMatch(req.MatchID)
		if match == nil {
			return utils.SendError(c, 400, "Match not available for betting")
		}
		if match.Minute >= 80 {
			return utils.SendError(c, 400, "Betting is closed for this match (80+ mins)")
		}

		uid := middleware.GetUID(c)
		ctx := context.Background()

		// DB Transaction
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return utils.SendError(c, 500, "Database error")
		}
		defer tx.Rollback(ctx)

		// Check balance
		var balance int64
		err = tx.QueryRow(ctx, "SELECT balance FROM users WHERE id = $1 FOR UPDATE", uid).Scan(&balance)
		if err != nil || balance < req.Amount {
			return utils.SendError(c, 400, "Insufficient funds")
		}

		// We create the bet first so we have the betID for the transaction log
		betID := fmt.Sprintf("bet_%d", time.Now().UnixNano())
		potentialWin := int64(float64(req.Amount) * float64(req.Odds))

		_, err = tx.Exec(ctx, `
			INSERT INTO bets (id, user_id, match_id, pick, odds, amount, potential_win, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'live')`,
			betID, uid, req.MatchID, req.Pick, req.Odds, req.Amount, potentialWin)
		if err != nil {
			return utils.SendError(c, 500, "Failed to place bet")
		}

		err = services.AdjustBalance(ctx, tx, uid, -req.Amount, "bet_place", betID)
		if err != nil {
			return utils.SendError(c, 500, "Failed to update balance")
		}

		if err := tx.Commit(ctx); err != nil {
			return utils.SendError(c, 500, "Transaction commit failed")
		}

		return utils.SendSuccess(c, fiber.Map{"betId": betID})
	})
}
