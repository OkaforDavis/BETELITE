package routes

import (
	"github.com/gofiber/fiber/v2"

	"betelite-go/middleware"
	"betelite-go/services"
)

func SetupMatchRoutes(api fiber.Router) {
	matchGroup := api.Group("/matches", middleware.AuthRequired())

	// Get all active matches
	matchGroup.Get("/", func(c *fiber.Ctx) error {
		activeMatches := services.Engine.GetActiveMatches()
		return c.JSON(fiber.Map{"success": true, "matches": activeMatches})
	})

	// Get specific match by ID
	matchGroup.Get("/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		match := services.Engine.GetMatch(id)
		if match == nil {
			return c.Status(404).JSON(fiber.Map{"error": "Match not found"})
		}
		return c.JSON(fiber.Map{"success": true, "match": match})
	})
}
