package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/websocket/v2"
	"github.com/joho/godotenv"
	"github.com/livekit/protocol/auth"
)

type Challenge struct {
	ID          string  `json:"id"`
	CreatorID   string  `json:"creatorId"`
	CreatorName string  `json:"creatorName"`
	Game        string  `json:"game"`
	Amount      float64 `json:"amount"`
	Timestamp   int64   `json:"timestamp"`
}

var (
	challenges = make(map[string]Challenge)
	mu         sync.Mutex
)

// Simple WebSocket hub for chat/signaling
var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan interface{})
var clientsMu sync.Mutex

func init() {
	godotenv.Load("../backend/.env") // Load existing .env
}

func main() {
	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
	}))

	// API Routes
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	app.Get("/api/settings", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"settings": fiber.Map{
				"paystackKey":   os.Getenv("PAYSTACK_PUBLIC_KEY"),
				"paystackKeyGH": os.Getenv("PAYSTACK_PUBLIC_KEY_GH"),
			},
		})
	})

	// Lobby API
	app.Get("/api/lobby/active", func(c *fiber.Ctx) error {
		mu.Lock()
		defer mu.Unlock()
		var list []Challenge
		for _, v := range challenges {
			list = append(list, v)
		}
		return c.JSON(fiber.Map{"ok": true, "challenges": list})
	})

	app.Post("/api/lobby/create", func(c *fiber.Ctx) error {
		var body struct {
			UID      string  `json:"uid"`
			Username string  `json:"username"`
			Game     string  `json:"game"`
			Amount   float64 `json:"amount"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"ok": false, "error": "Invalid request"})
		}
		id := fmt.Sprintf("challenge_%d", time.Now().UnixNano())
		chal := Challenge{
			ID:          id,
			CreatorID:   body.UID,
			CreatorName: body.Username,
			Game:        body.Game,
			Amount:      body.Amount,
			Timestamp:   time.Now().UnixMilli(),
		}
		mu.Lock()
		challenges[id] = chal
		mu.Unlock()

		// Broadcast new challenge via WS
		broadcast <- map[string]interface{}{
			"type":      "lobby_update",
			"challenge": chal,
		}

		return c.JSON(fiber.Map{"ok": true, "challenge": chal})
	})

	app.Post("/api/lobby/delete", func(c *fiber.Ctx) error {
		var body struct {
			ChallengeID string `json:"challengeId"`
			UID         string `json:"uid"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"ok": false})
		}
		mu.Lock()
		chal, exists := challenges[body.ChallengeID]
		if exists && chal.CreatorID == body.UID {
			delete(challenges, body.ChallengeID)
		}
		mu.Unlock()

		if exists {
			broadcast <- map[string]interface{}{
				"type": "lobby_challenge_removed",
				"id":   body.ChallengeID,
			}
		}

		return c.JSON(fiber.Map{"ok": true})
	})

	// LiveKit Token Generation
	app.Post("/api/stream/token", func(c *fiber.Ctx) error {
		var req struct {
			RoomName string `json:"roomName"`
			Identity string `json:"identity"`
			IsHost   bool   `json:"isHost"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid payload"})
		}

		apiKey := os.Getenv("LIVEKIT_API_KEY")
		apiSecret := os.Getenv("LIVEKIT_API_SECRET")
		if apiKey == "" || apiSecret == "" {
			return c.Status(500).JSON(fiber.Map{"error": "LiveKit keys not configured"})
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
			return c.Status(500).JSON(fiber.Map{"error": "Failed to create token"})
		}

		return c.JSON(fiber.Map{"token": token})
	})

	// WebSocket handler
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		clientsMu.Lock()
		clients[c] = true
		clientsMu.Unlock()

		defer func() {
			clientsMu.Lock()
			delete(clients, c)
			clientsMu.Unlock()
			c.Close()
		}()

		var msg map[string]interface{}
		for {
			if err := c.ReadJSON(&msg); err != nil {
				break
			}
			// Handle incoming ws events like chat messages
			broadcast <- msg
		}
	}))

	// Broadcast loop
	go func() {
		for {
			msg := <-broadcast
			clientsMu.Lock()
			for client := range clients {
				client.WriteJSON(msg)
			}
			clientsMu.Unlock()
		}
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("Starting Go backend on port %s", port)
	log.Fatal(app.Listen(":" + port))
}
