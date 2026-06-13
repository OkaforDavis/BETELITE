package main

import (
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"

	"betelite-go/config"
	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/routes"
	"betelite-go/services"
	"betelite-go/ws"
)

func main() {
	// 1. Load config
	config.Load()

	// 2. Initialize PostgreSQL
	ctx := context.Background()
	if err := db.Connect(ctx, config.Cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 3. Run Migrations
	if err := db.RunMigrations(ctx); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	// 4. Initialize Firebase Auth
	if err := middleware.InitFirebaseAuth(ctx); err != nil {
		log.Printf("[WARN] Failed to initialize Firebase Auth: %v", err)
	}

	// 5. Setup Fiber
	app := fiber.New(fiber.Config{
		BodyLimit: 10 * 1024 * 1024, // 10MB limit
	})

	// Middleware
	app.Use(middleware.Cors())

	// Serve PWA static frontend
	app.Static("/", "./static", fiber.Static{
		Compress:  true,
		ByteRange: true,
		Index:     "index.html",
	})

	// SPA fallback — any non-API route serves index.html
	app.Use(func(c *fiber.Ctx) error {
		path := string(c.Request().URI().Path())
		if len(path) < 4 || path[:4] != "/api" {
			return c.SendFile("./static/index.html")
		}
		return c.Next()
	})

	// 6. Setup WebSocket Hub and Engine
	hub := ws.NewHub()
	go hub.Run()

	services.InitEngine(hub)

	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		client := &ws.Client{
			Hub:  hub,
			Conn: c,
			Send: make(chan []byte, 256),
		}
		hub.Register <- client

		go client.WritePump()
		client.ReadPump()
	}))

	// 7. Routes (Phase 1 basics)
	api := app.Group("/api", middleware.RateLimiter())

	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "db": db.Pool != nil})
	})

	api.Get("/settings", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"settings": fiber.Map{
				"paystackKey":   config.Cfg.PaystackPublicKey,
				"paystackKeyGH": config.Cfg.PaystackPublicKeyGH,
			},
		})
	})

	api.Get("/streams", func(c *fiber.Ctx) error {
		// Return live P2P matches as active streams
		activeMatches := services.Engine.GetActiveMatches()
		var streams []fiber.Map
		for _, m := range activeMatches {
			if m.IsP2P && m.Status == "live" {
				streams = append(streams, fiber.Map{
					"id":       m.ID,
					"host":     m.Home,
					"hostId":   m.HomeID,
					"game":     m.Label,
					"viewers":  0,
					"status":   "live",
					"matchId":  m.ID,
				})
			}
		}
		if streams == nil {
			streams = []fiber.Map{}
		}
		return c.JSON(fiber.Map{"streams": streams})
	})

	// Setup Routes
	routes.SetupMatchRoutes(api)
	routes.SetupLobbyRoutes(api, hub)
	routes.SetupBetRoutes(api)
	routes.SetupTournamentRoutes(api)
	routes.SetupPaymentRoutes(api)
	routes.SetupDetectRoutes(api)
	routes.SetupFootballRoutes(api)
	routes.SetupNotificationRoutes(api)
	routes.SetupReferralRoutes(api)
	routes.SetupAdminRoutes(api)
	routes.SetupSettingsRoutes(api)

	log.Printf("Server listening on port %s", config.Cfg.Port)
	if err := app.Listen(":" + config.Cfg.Port); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
