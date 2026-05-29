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

	// Locked room (private match)
	matchGroup.Post("/locked-room", func(c *fiber.Ctx) error {
		var req struct {
			OpponentId string `json:"opponentId"`
			Wager      int64  `json:"wager"`
		}
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid payload")
		}
		
		// Normally this would create a locked room challenge
		// For now just return a success
		return utils.SendSuccess(c, fiber.Map{"ok": true, "matchId": "locked_" + req.OpponentId})
	})
}
