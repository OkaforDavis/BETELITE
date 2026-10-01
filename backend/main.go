package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/websocket/v2"

	"betelite-go/config"
	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/routes"
	"betelite-go/services"
	"betelite-go/ws"
)

func main() {
	config.Load()
	ctx := context.Background()

	if err := db.Connect(ctx, config.Cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	if err := db.RunMigrations(ctx); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	if err := middleware.InitFirebaseAuth(ctx); err != nil {
		log.Printf("[WARN] Failed to initialize Firebase Auth: %v", err)
	}

	app := fiber.New(fiber.Config{
		BodyLimit:               10 * 1024 * 1024,
		ProxyHeader:             fiber.HeaderXForwardedFor, // Render sits behind a proxy
		EnableIPValidation:      true,
		EnableTrustedProxyCheck: false,
		ReadTimeout:             90 * time.Second,
		WriteTimeout:            90 * time.Second,
	})
	app.Use(recover.New())
	app.Use(middleware.Cors())
	app.Use(securityHeaders)

	// WebSocket hub: live lobby, match updates, chat and in-app notifications.
	hub := ws.NewHub()
	go hub.Run()
	services.Hub = hub
	ws.VerifyToken = func(token string) (string, error) {
		id, err := middleware.VerifyToken(context.Background(), token)
		if err != nil {
			return "", err
		}
		return id.UID, nil
	}
	ws.DisplayName = func(uid string) string { return services.Username(context.Background(), uid) }

	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		client := &ws.Client{Hub: hub, Conn: c, Send: make(chan []byte, 256)}
		hub.Register <- client
		go client.WritePump()
		client.ReadPump()
	}))

	api := app.Group("/api", middleware.RateLimiter())
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "db": db.Pool != nil, "version": config.Cfg.AppVersion})
	})
	// The PWA polls this to show "New version available".
	api.Get("/version", func(c *fiber.Ctx) error {
		c.Set("Cache-Control", "no-store")
		return c.JSON(fiber.Map{"version": config.Cfg.AppVersion, "minVersion": config.Cfg.MinAppVersion})
	})
	api.Get("/settings", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true, "settings": fiber.Map{
			"paystackKey":   config.Cfg.PaystackPublicKey,
			"paystackKeyGH": config.Cfg.PaystackPublicKeyGH,
			"vapidKey":      config.Cfg.VAPIDPublicKey,
			"termsVersion":  services.TermsVersion,
		}})
	})

	routes.SetupMeRoutes(api)
	routes.SetupPaymentRoutes(api)
	routes.SetupMatchRoutes(api)
	routes.SetupLobbyRoutes(api, hub)
	routes.SetupTournamentRoutes(api)
	routes.SetupStreamRoutes(api, hub)
	routes.SetupNotificationRoutes(api)
	routes.SetupReferralRoutes(api)
	routes.SetupSettingsRoutes(api)
	routes.SetupFootballRoutes(api)
	routes.SetupAdminRoutes(api)

	setupStatic(app)

	services.StartAutomation(hub, services.DefaultAutomationConfig())

	go func() {
		log.Printf("Server %s listening on port %s", config.Cfg.AppVersion, config.Cfg.Port)
		if err := app.Listen(":" + config.Cfg.Port); err != nil {
			log.Fatalf("Error starting server: %v", err)
		}
	}()

	// Graceful shutdown so in-flight payouts finish on redeploy.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Shutting down...")
	app.ShutdownWithTimeout(20 * time.Second)
}

func securityHeaders(c *fiber.Ctx) error {
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	c.Set("X-Frame-Options", "SAMEORIGIN")
	c.Set("Permissions-Policy", "camera=(self), microphone=(self), geolocation=()")
	if config.Cfg.Env == "production" {
		c.Set("Strict-Transport-Security", "max-age=31536000")
	}
	return c.Next()
}

// setupStatic serves the PWA. index.html and sw.js get the build version
// stamped in (so each deploy installs a fresh service worker) and are never
// cached; other assets are cached briefly and busted with ?v=<version>.
func setupStatic(app *fiber.App) {
	versioned := map[string]string{}
	for _, f := range []string{"index.html", "sw.js"} {
		b, err := os.ReadFile("./static/" + f)
		if err != nil {
			log.Printf("[WARN] static/%s missing: %v", f, err)
			continue
		}
		versioned[f] = strings.ReplaceAll(string(b), "__APP_VERSION__", config.Cfg.AppVersion)
	}
	serve := func(name, ctype string) fiber.Handler {
		return func(c *fiber.Ctx) error {
			c.Set("Cache-Control", "no-cache")
			c.Type(ctype)
			return c.SendString(versioned[name])
		}
	}
	app.Get("/sw.js", func(c *fiber.Ctx) error {
		c.Set("Service-Worker-Allowed", "/")
		return serve("sw.js", "js")(c)
	})
	app.Get("/", serve("index.html", "html"))
	app.Get("/index.html", serve("index.html", "html"))

	app.Static("/", "./static", fiber.Static{Compress: true, MaxAge: 3600})

	// SPA fallback: any other non-API path renders the app (deep links like /match/123).
	app.Use(func(c *fiber.Ctx) error {
		p := c.Path()
		if strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/ws") || strings.Contains(p[strings.LastIndex(p, "/")+1:], ".") {
			return fiber.ErrNotFound
		}
		return serve("index.html", "html")(c)
	})
}
