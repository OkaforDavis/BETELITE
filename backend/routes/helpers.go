package routes

import (
	"errors"
	"log"

	"github.com/gofiber/fiber/v2"

	"betelite-go/services"
	"betelite-go/utils"
)

// fail sends a UserError's message as-is and hides anything else behind a
// generic message (the details go to the log).
func fail(c *fiber.Ctx, err error) error {
	var ue *services.UserError
	if errors.As(err, &ue) {
		return utils.SendError(c, ue.Status, ue.Msg)
	}
	log.Printf("[ERROR] %s %s: %v", c.Method(), c.Path(), err)
	return utils.SendError(c, 500, "Something went wrong. Please try again.")
}

// dbRequired rejects requests while the database is not configured.
func dbRequired(c *fiber.Ctx) error {
	if !services.DBReady() {
		return utils.SendError(c, 503, "Service temporarily unavailable")
	}
	return c.Next()
}
