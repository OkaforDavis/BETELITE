package routes

import (
	"github.com/gofiber/fiber/v2"

	"betelite-go/middleware"
	"betelite-go/services"
)

func SetupFootballRoutes(api fiber.Router) {
	football := api.Group("/football", middleware.AuthRequired())

	football.Get("/live", func(c *fiber.Ctx) error {
		fixtures, err := services.FetchLiveFixtures()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch live fixtures"})
		}
		return c.JSON(fiber.Map{"success": true, "fixtures": fixtures})
	})
}
