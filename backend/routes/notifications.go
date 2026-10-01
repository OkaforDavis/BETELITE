package routes

import (
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"

	"betelite-go/config"
	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/utils"
)

func SetupNotificationRoutes(api fiber.Router) {
	// Public: the browser needs this key before it can subscribe to push.
	api.Get("/notifications/vapid-public-key", func(c *fiber.Ctx) error {
		return utils.SendSuccess(c, fiber.Map{"publicKey": config.Cfg.VAPIDPublicKey})
	})

	n := api.Group("/notifications", middleware.AuthRequired(), dbRequired)

	n.Get("/", func(c *fiber.Ctx) error {
		uid := middleware.GetUID(c)
		rows, err := db.Pool.Query(c.Context(), `SELECT id, type, title, message, COALESCE(metadata,'{}'::jsonb), read, created_at
			FROM notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT 50`, uid)
		if err != nil {
			return fail(c, err)
		}
		defer rows.Close()
		list := []fiber.Map{}
		for rows.Next() {
			var id, typ, title, msg string
			var meta []byte
			var read bool
			var at time.Time
			if rows.Scan(&id, &typ, &title, &msg, &meta, &read, &at) == nil {
				list = append(list, fiber.Map{"id": id, "type": typ, "title": title, "message": msg, "metadata": json.RawMessage(meta), "read": read, "createdAt": at})
			}
		}
		var unread int
		db.Pool.QueryRow(c.Context(), "SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND NOT read", uid).Scan(&unread)
		return utils.SendSuccess(c, fiber.Map{"notifications": list, "unread": unread})
	})

	// Mark some (ids) or all (no ids) notifications as read.
	n.Post("/read", func(c *fiber.Ctx) error {
		var req struct {
			IDs []string `json:"ids"`
		}
		c.BodyParser(&req)
		uid := middleware.GetUID(c)
		var err error
		if len(req.IDs) == 0 {
			_, err = db.Pool.Exec(c.Context(), "UPDATE notifications SET read = TRUE WHERE user_id = $1 AND NOT read", uid)
		} else {
			_, err = db.Pool.Exec(c.Context(), "UPDATE notifications SET read = TRUE WHERE user_id = $1 AND id = ANY($2)", uid, req.IDs)
		}
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{})
	})

	n.Post("/subscribe", func(c *fiber.Ctx) error {
		var req struct {
			Endpoint string `json:"endpoint"`
			Keys     struct {
				P256dh string `json:"p256dh"`
				Auth   string `json:"auth"`
			} `json:"keys"`
		}
		if err := c.BodyParser(&req); err != nil || req.Endpoint == "" || req.Keys.P256dh == "" || req.Keys.Auth == "" {
			return utils.SendError(c, 400, "Invalid push subscription")
		}
		_, err := db.Pool.Exec(c.Context(), `INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth) VALUES ($1,$2,$3,$4)
			ON CONFLICT (endpoint) DO UPDATE SET user_id = $1, p256dh = $3, auth = $4`,
			middleware.GetUID(c), req.Endpoint, req.Keys.P256dh, req.Keys.Auth)
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{})
	})

	n.Post("/unsubscribe", func(c *fiber.Ctx) error {
		var req struct {
			Endpoint string `json:"endpoint"`
		}
		c.BodyParser(&req)
		db.Pool.Exec(c.Context(), "DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = $2", middleware.GetUID(c), req.Endpoint)
		return utils.SendSuccess(c, fiber.Map{})
	})
}
