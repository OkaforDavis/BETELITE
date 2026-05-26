package routes

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/livekit/protocol/auth"

	"betelite-go/config"
	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/models"
	"betelite-go/services"
	"betelite-go/utils"
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
			return utils.SendError(c, 400, "Invalid payload")
		}

		uid := middleware.GetUID(c)
		challengeID := fmt.Sprintf("challenge_%d", time.Now().UnixNano())

		// DB Transaction: Deduct wallet and create escrow
		ctx := context.Background()
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

		err = services.AdjustBalance(ctx, tx, uid, -req.Amount, "wager_hold", challengeID)
		if err != nil {
			return utils.SendError(c, 500, "Failed to update balance")
		}

		// Create Escrow
		_, err = tx.Exec(ctx, `INSERT INTO escrow (challenge_id, creator_id, amount, pool, status) 
			VALUES ($1, $2, $3, $4, 'waiting')`,
			challengeID, uid, req.Amount, req.Amount) // Pool is just creator's amount for now
		if err != nil {
			return utils.SendError(c, 500, "Failed to create escrow")
		}

		if err := tx.Commit(ctx); err != nil {
			return utils.SendError(c, 500, "Transaction commit failed")
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

		return utils.SendSuccess(c, fiber.Map{
			"challengeId": challengeID,
		})
	})

	// Accept challenge
	lobby.Post("/challenge/accept", func(c *fiber.Ctx) error {
		var req struct {
			ChallengeID string `json:"challengeId"`
			Username    string `json:"username"`
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

		// Get Escrow details
		var escrow models.Escrow
		var status string
		var player1ID string
		var amount int64
		err = tx.QueryRow(ctx, "SELECT id, creator_id, amount, status FROM escrow WHERE challenge_id = $1 FOR UPDATE", req.ChallengeID).
			Scan(&escrow.ID, &player1ID, &amount, &status)
		if err != nil || status != "pending" {
			return utils.SendError(c, 400, "Challenge not available")
		}

		if player1ID == uid {
			return utils.SendError(c, 400, "Cannot accept your own challenge")
		}

		// Check acceptor's balance
		var balance int64
		err = tx.QueryRow(ctx, "SELECT balance FROM users WHERE id = $1 FOR UPDATE", uid).Scan(&balance)
		if err != nil || balance < amount {
			return utils.SendError(c, 400, "Insufficient funds")
		}

		err = services.AdjustBalance(ctx, tx, uid, -amount, "wager_hold", req.ChallengeID)
		if err != nil {
			return utils.SendError(c, 500, "Failed to update balance")
		}

		// Update Escrow
		matchID := fmt.Sprintf("match_%d", time.Now().UnixNano())
		pool := amount * 2
		_, err = tx.Exec(ctx, "UPDATE escrow SET acceptor_id = $1, pool = $2, status = 'held', match_id = $3 WHERE id = $4",
			uid, pool, matchID, escrow.ID)
		if err != nil {
			return utils.SendError(c, 500, "Failed to update escrow")
		}

		if err := tx.Commit(ctx); err != nil {
			return utils.SendError(c, 500, "Transaction commit failed")
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

		return utils.SendSuccess(c, fiber.Map{"matchId": matchID})
	})

	// LiveKit Token Generation
	lobby.Post("/stream/token", func(c *fiber.Ctx) error {
		var req struct {
			RoomName string `json:"roomName"`
			Identity string `json:"identity"`
			IsHost   bool   `json:"isHost"`
		}
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid payload")
		}

		apiKey := config.Cfg.LiveKitAPIKey
		apiSecret := config.Cfg.LiveKitAPISecret
		if apiKey == "" || apiSecret == "" {
			return utils.SendError(c, 500, "LiveKit keys not configured")
		}

		at := auth.NewAccessToken(apiKey, apiSecret)
		grant := &auth.VideoGrant{
			RoomJoin: true,
			Room:     req.RoomName,
		}
		if req.IsHost {
			grant.CanPublish = &[]bool{true}[0]
			grant.CanPublishData = &[]bool{true}[0]
		} else {
			grant.CanPublish = &[]bool{false}[0]
			grant.CanPublishData = &[]bool{true}[0] // allow chatting
		}

		at.AddGrant(grant).
			SetIdentity(req.Identity).
			SetValidFor(time.Hour * 4)

		token, err := at.ToJWT()
		if err != nil {
			return utils.SendError(c, 500, "Failed to create token")
		}

		return utils.SendSuccess(c, fiber.Map{"token": token})
	})
}
