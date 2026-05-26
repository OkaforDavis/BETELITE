package routes

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/models"
	"betelite-go/services"
	"betelite-go/ws"
)

func SetupLobbyRoutes(api fiber.Router, hub *ws.Hub) {
	lobby := api.Group("/lobby", middleware.AuthRequired())

	// Create a new challenge (Escrow hold)
	lobby.Post("/challenge", func(c *fiber.Ctx) error {
		var req struct {
			Game     string `json:"game"`
			Amount   int64  `json:"amount"` // Note: Frontend might send string/float, ensure it's converted to int64 kobo
			Username string `json:"username"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid payload"})
		}

		uid := middleware.GetUID(c)
		challengeID := fmt.Sprintf("challenge_%d", time.Now().UnixNano())

		// DB Transaction: Deduct wallet and create escrow
		ctx := context.Background()
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

		// Create Escrow
		_, err = tx.Exec(ctx, `INSERT INTO escrow (challenge_id, creator_id, amount, pool, status) 
			VALUES ($1, $2, $3, $4, 'waiting')`,
			challengeID, uid, req.Amount, req.Amount) // Pool is just creator's amount for now
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to create escrow"})
		}

		// Log transaction
		_, err = tx.Exec(ctx, "INSERT INTO transactions (user_id, type, amount, ref_id) VALUES ($1, 'wager_hold', $2, $3)",
			uid, -req.Amount, challengeID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to log transaction"})
		}

		if err := tx.Commit(ctx); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Transaction commit failed"})
		}

		challenge := models.Challenge{
			ID:          challengeID,
			CreatorID:   uid,
			CreatorName: req.Username,
			Game:        req.Game,
			Amount:      req.Amount,
			Currency:    "NGN",
			Timestamp:   time.Now().UnixMilli(),
		}

		// Broadcast new challenge
		ws.BroadcastEvent(hub, "lobby_update", challenge)

		return c.JSON(fiber.Map{"success": true, "challenge": challenge})
	})

	// Accept challenge
	lobby.Post("/challenge/accept", func(c *fiber.Ctx) error {
		var req struct {
			ChallengeID string `json:"challengeId"`
			Username    string `json:"username"`
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

		// Get Escrow details
		var escrow models.Escrow
		err = tx.QueryRow(ctx, "SELECT id, creator_id, amount, status FROM escrow WHERE challenge_id = $1 FOR UPDATE", req.ChallengeID).
			Scan(&escrow.ID, &escrow.CreatorID, &escrow.Amount, &escrow.Status)
		if err != nil || escrow.Status != "waiting" {
			return c.Status(400).JSON(fiber.Map{"error": "Challenge not available"})
		}

		if escrow.CreatorID == uid {
			return c.Status(400).JSON(fiber.Map{"error": "Cannot accept your own challenge"})
		}

		// Check acceptor's balance
		var balance int64
		err = tx.QueryRow(ctx, "SELECT balance FROM users WHERE id = $1 FOR UPDATE", uid).Scan(&balance)
		if err != nil || balance < escrow.Amount {
			return c.Status(400).JSON(fiber.Map{"error": "Insufficient funds"})
		}

		// Deduct balance
		_, err = tx.Exec(ctx, "UPDATE users SET balance = balance - $1 WHERE id = $2", escrow.Amount, uid)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to update balance"})
		}

		// Update Escrow
		matchID := fmt.Sprintf("match_%d", time.Now().UnixNano())
		pool := escrow.Amount * 2
		_, err = tx.Exec(ctx, "UPDATE escrow SET acceptor_id = $1, pool = $2, status = 'held', match_id = $3 WHERE id = $4",
			uid, pool, matchID, escrow.ID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to update escrow"})
		}

		// Log transaction
		_, err = tx.Exec(ctx, "INSERT INTO transactions (user_id, type, amount, ref_id) VALUES ($1, 'wager_hold', $2, $3)",
			uid, -escrow.Amount, req.ChallengeID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to log transaction"})
		}

		if err := tx.Commit(ctx); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Transaction commit failed"})
		}

		// Create match in Engine
		match := &models.Match{
			ID:           matchID,
			Game:         "EA FC 24", // Would normally pull from challenge
			HomeID:       escrow.CreatorID,
			AwayID:       uid,
			Status:       "live",
			Minute:       0,
			WagerPool:    pool,
			WagerAmount:  escrow.Amount,
			IsP2P:        true,
			ChallengeID:  req.ChallengeID,
		}
		services.Engine.AddMatch(match)

		// Broadcast removal of challenge
		ws.BroadcastEvent(hub, "lobby_challenge_removed", map[string]string{"id": req.ChallengeID})

		return c.JSON(fiber.Map{"success": true, "matchId": matchID})
	})
}
