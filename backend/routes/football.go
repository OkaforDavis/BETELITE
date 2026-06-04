package routes

import (
	"github.com/gofiber/fiber/v2"

	"betelite-go/middleware"
	"betelite-go/services"
	"betelite-go/utils"
)

func SetupFootballRoutes(api fiber.Router) {
	football := api.Group("/football", middleware.AuthRequired())

	football.Get("/live", func(c *fiber.Ctx) error {
		fixtures, err := services.FetchLiveFixtures()
		if err != nil {
			return utils.SendError(c, 500, "Failed to fetch live fixtures")
		}
		return utils.SendSuccess(c, fiber.Map{"fixtures": fixtures})
	})
}
