package routes

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"betelite-go/db"
	"betelite-go/middleware"
)

func SetupSettingsRoutes(api fiber.Router) {
	settings := api.Group("/user/settings", middleware.AuthRequired())

	settings.Get("/", func(c *fiber.Ctx) error {
		uid := middleware.GetUID(c)
		ctx := context.Background()

		var pushEnabled bool
		var emailEnabled bool
		err := db.Pool.QueryRow(ctx, "SELECT push_notifications, email_notifications FROM users WHERE id = $1", uid).Scan(&pushEnabled, &emailEnabled)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch settings"})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"settings": fiber.Map{
				"pushEnabled":  pushEnabled,
				"emailEnabled": emailEnabled,
			},
		})
	})

	settings.Post("/update", func(c *fiber.Ctx) error {
		var req struct {
			PushEnabled  *bool `json:"pushEnabled"`
			EmailEnabled *bool `json:"emailEnabled"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid payload"})
		}

		uid := middleware.GetUID(c)
		ctx := context.Background()

		if req.PushEnabled != nil {
			db.Pool.Exec(ctx, "UPDATE users SET push_notifications = $1 WHERE id = $2", *req.PushEnabled, uid)
		}
		if req.EmailEnabled != nil {
			db.Pool.Exec(ctx, "UPDATE users SET email_notifications = $1 WHERE id = $2", *req.EmailEnabled, uid)
		}

		return c.JSON(fiber.Map{"success": true})
	})
}
