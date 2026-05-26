package routes

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/models"
	"betelite-go/services"
	"betelite-go/utils"
)

func SetupTournamentRoutes(api fiber.Router) {
	tournaments := api.Group("/tournaments", middleware.AuthRequired())

	// Get active tournaments
	tournaments.Get("/", func(c *fiber.Ctx) error {
		ctx := context.Background()
		rows, err := db.Pool.Query(ctx, "SELECT id, name, game, entry_fee, max_players, prize_pool, status FROM tournaments WHERE status != 'finished'")
		if err != nil {
			return utils.SendError(c, 500, "Failed to fetch tournaments")
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

		return utils.SendSuccess(c, fiber.Map{"tournaments": results})
	})

	// Join tournament
	tournaments.Post("/join", func(c *fiber.Ctx) error {
		var req struct {
			TournamentID string `json:"tournamentId"`
		}
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid payload")
		}

		uid := middleware.GetUID(c)
		ctx := context.Background()

		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return utils.SendError(c, 500, "Database error")
		}
		defer tx.Rollback(ctx)

		// 1. Get Tournament
		var t models.Tournament
		err = tx.QueryRow(ctx, "SELECT entry_fee, max_players, prize_pool, status FROM tournaments WHERE id = $1 FOR UPDATE", req.TournamentID).
			Scan(&t.EntryFee, &t.MaxPlayers, &t.PrizePool, &t.Status)
		if err != nil {
			return utils.SendError(c, 404, "Tournament not found")
		}
		if t.Status != "open" {
			return utils.SendError(c, 400, "Tournament not available for joining")
		}

		// 2. Check player count
		var playerCount int
		err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM tournament_players WHERE tournament_id = $1", req.TournamentID).Scan(&playerCount)
		if err != nil || playerCount >= t.MaxPlayers {
			return utils.SendError(c, 400, "Tournament is full")
		}

		// 3. Check if already joined
		var existing bool
		tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM tournament_players WHERE tournament_id = $1 AND user_id = $2)", req.TournamentID, uid).Scan(&existing)
		if existing {
			return utils.SendError(c, 400, "Already registered for this tournament")
		}

		// 4. Deduct entry fee if > 0
		if t.EntryFee > 0 {
			var balance int64
			err = tx.QueryRow(ctx, "SELECT balance FROM users WHERE id = $1 FOR UPDATE", uid).Scan(&balance)
			if err != nil || balance < t.EntryFee {
				return utils.SendError(c, 400, "Insufficient funds for entry fee")
			}

			err = services.AdjustBalance(ctx, tx, uid, -t.EntryFee, "tournament_entry", req.TournamentID)
			if err != nil {
				return utils.SendError(c, 500, "Failed to update balance")
			}

			// Add to prize pool (e.g., 80% to prize pool)
			addedToPrize := int64(float64(t.EntryFee) * 0.8)
			_, err = tx.Exec(ctx, "UPDATE tournaments SET prize_pool = prize_pool + $1 WHERE id = $2", addedToPrize, req.TournamentID)
			if err != nil {
				return utils.SendError(c, 500, "Failed to update prize pool")
			}
		}

		// 5. Add player to tournament
		_, err = tx.Exec(ctx, "INSERT INTO tournament_players (tournament_id, user_id) VALUES ($1, $2)", req.TournamentID, uid)
		if err != nil {
			return utils.SendError(c, 500, "Failed to add player to tournament")
		}

		// Check if tournament is now full, start it
		if playerCount+1 == t.MaxPlayers {
			_, err = tx.Exec(ctx, "UPDATE tournaments SET status = 'active', current_round = 1 WHERE id = $1", req.TournamentID)
			if err != nil {
				return utils.SendError(c, 500, "Failed to start tournament")
			}
			// In a real implementation, you would also generate the first round fixtures here
			fmt.Printf("Tournament %s is now active\n", req.TournamentID)
		}

		if err := tx.Commit(ctx); err != nil {
			return utils.SendError(c, 500, "Transaction commit failed")
		}

		return utils.SendSuccess(c, fiber.Map{})
	})
}
