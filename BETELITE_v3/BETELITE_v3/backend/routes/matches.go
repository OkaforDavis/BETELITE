package routes

import (
	"github.com/gofiber/fiber/v2"

	"betelite-go/middleware"
	"betelite-go/services"
	"betelite-go/utils"
)

func SetupMatchRoutes(api fiber.Router) {
	matchGroup := api.Group("/matches", middleware.AuthRequired())

	// Get all active matches
	matchGroup.Get("/", func(c *fiber.Ctx) error {
		activeMatches := services.Engine.GetActiveMatches()
		return utils.SendSuccess(c, fiber.Map{"matches": activeMatches})
	})

	// Get specific match by ID
	matchGroup.Get("/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		match := services.Engine.GetMatch(id)
		if match == nil {
			return utils.SendError(c, 404, "Match not found")
		}
		return utils.SendSuccess(c, fiber.Map{"match": match})
	})
}
