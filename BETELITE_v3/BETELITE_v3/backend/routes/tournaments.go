package routes

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/models"
)

func SetupTournamentRoutes(api fiber.Router) {
	tournaments := api.Group("/tournaments", middleware.AuthRequired())

	// Get active tournaments
	tournaments.Get("/", func(c *fiber.Ctx) error {
		ctx := context.Background()
		rows, err := db.Pool.Query(ctx, "SELECT id, name, game, entry_fee, max_players, prize_pool, status FROM tournaments WHERE status != 'finished'")
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch tournaments"})
		}
		defer rows.Close()

		var results []models.Tournament
		for rows.Next() {
			var t models.Tournament
			err := rows.Scan(&t.ID, &t.Name, &t.Game, &t.EntryFee, &t.MaxPlayers, &t.PrizePool, &t.Status)
			if err == nil {
				results = append(results, t)
			}
		}

		return c.JSON(fiber.Map{"success": true, "tournaments": results})
	})

	// Join tournament
	tournaments.Post("/join", func(c *fiber.Ctx) error {
		var req struct {
			TournamentID string `json:"tournamentId"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid payload"})
		}

		uid := middleware.GetUID(c)
		ctx := context.Background()

		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Database error"})
		}
		defer tx.Rollback(ctx)

		// 1. Get Tournament
		var t models.Tournament
		err = tx.QueryRow(ctx, "SELECT entry_fee, max_players, prize_pool, status FROM tournaments WHERE id = $1 FOR UPDATE", req.TournamentID).
			Scan(&t.EntryFee, &t.MaxPlayers, &t.PrizePool, &t.Status)
		if err != nil || t.Status != "open" {
			return c.Status(400).JSON(fiber.Map{"error": "Tournament not available for joining"})
		}

		// 2. Check player count
		var playerCount int
		err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM tournament_players WHERE tournament_id = $1", req.TournamentID).Scan(&playerCount)
		if err != nil || playerCount >= t.MaxPlayers {
			return c.Status(400).JSON(fiber.Map{"error": "Tournament is full"})
		}

		// 3. Check if already joined
		var existing int
		tx.QueryRow(ctx, "SELECT 1 FROM tournament_players WHERE tournament_id = $1 AND user_id = $2", req.TournamentID, uid).Scan(&existing)
		if existing == 1 {
			return c.Status(400).JSON(fiber.Map{"error": "You have already joined this tournament"})
		}

		// 4. Deduct entry fee if > 0
		if t.EntryFee > 0 {
			var balance int64
			err = tx.QueryRow(ctx, "SELECT balance FROM users WHERE id = $1 FOR UPDATE", uid).Scan(&balance)
			if err != nil || balance < t.EntryFee {
				return c.Status(400).JSON(fiber.Map{"error": "Insufficient funds for entry fee"})
			}

			_, err = tx.Exec(ctx, "UPDATE users SET balance = balance - $1 WHERE id = $2", t.EntryFee, uid)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to deduct entry fee"})
			}

			// Add to prize pool (e.g., 80% to prize pool)
			addedToPrize := int64(float64(t.EntryFee) * 0.8)
			_, err = tx.Exec(ctx, "UPDATE tournaments SET prize_pool = prize_pool + $1 WHERE id = $2", addedToPrize, req.TournamentID)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to update prize pool"})
			}

			_, err = tx.Exec(ctx, "INSERT INTO transactions (user_id, type, amount, ref_id) VALUES ($1, 'tournament_entry', $2, $3)",
				uid, -t.EntryFee, req.TournamentID)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to log transaction"})
			}
		}

		// 5. Add player to tournament
		_, err = tx.Exec(ctx, "INSERT INTO tournament_players (tournament_id, user_id) VALUES ($1, $2)", req.TournamentID, uid)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to add player to tournament"})
		}

		// Check if tournament is now full, start it
		if playerCount+1 == t.MaxPlayers {
			_, err = tx.Exec(ctx, "UPDATE tournaments SET status = 'active', current_round = 1 WHERE id = $1", req.TournamentID)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to start tournament"})
			}
			// In a real implementation, you would also generate the first round fixtures here
			fmt.Printf("Tournament %s is now active\n", req.TournamentID)
		}

		if err := tx.Commit(ctx); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Transaction commit failed"})
		}

		return c.JSON(fiber.Map{"success": true})
	})
}
