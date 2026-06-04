package routes

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"betelite-go/config"
	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/utils"
)

func SetupNotificationRoutes(api fiber.Router) {
	notifications := api.Group("/notifications", middleware.AuthRequired())

	notifications.Get("/vapidPublicKey", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success":   true,
			"publicKey": config.Cfg.VAPIDPublicKey,
		})
	})

	notifications.Post("/subscribe", func(c *fiber.Ctx) error {
		var req struct {
			Endpoint string `json:"endpoint"`
			Keys     struct {
				P256dh string `json:"p256dh"`
				Auth   string `json:"auth"`
			} `json:"keys"`
		}

		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid subscription object")
		}

		uid := middleware.GetUID(c)
		ctx := context.Background()

		_, err := db.Pool.Exec(ctx, `
			INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth) 
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (endpoint) DO UPDATE SET user_id = $1, p256dh = $3, auth = $4`,
			uid, req.Endpoint, req.Keys.P256dh, req.Keys.Auth)

		if err != nil {
			return utils.SendError(c, 500, "Failed to save subscription")
		}

		return utils.SendSuccess(c, fiber.Map{})
	})
}
