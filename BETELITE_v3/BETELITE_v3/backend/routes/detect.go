package routes

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"

	"betelite-go/middleware"
	"betelite-go/models"
	"betelite-go/services"
	"betelite-go/utils"
)

func SetupDetectRoutes(api fiber.Router) {
	detect := api.Group("/detect", middleware.AuthRequired())

	// Upload image and verify result
	detect.Post("/match-result", func(c *fiber.Ctx) error {
		matchID := c.FormValue("matchId")
		if matchID == "" {
			return utils.SendError(c, 400, "matchId is required")
		}

		file, err := c.FormFile("image")
		if err != nil {
			return utils.SendError(c, 400, "image file is required")
		}

		// Save file temporarily
		tempPath := filepath.Join(os.TempDir(), fmt.Sprintf("upload_%s_%s", matchID, file.Filename))
		if err := c.SaveFile(file, tempPath); err != nil {
			return utils.SendError(c, 500, "failed to save image")
		}
		defer os.Remove(tempPath)

		// Send to AI service
		aiResult, err := services.VerifyMatchResult(tempPath)
		if err != nil {
			return utils.SendError(c, 500, "AI detection failed: "+err.Error())
		}

		// Verify against Engine
		match := services.Engine.GetMatch(matchID)
		if match == nil {
			return utils.SendError(c, 404, "Match not found in engine")
		}

		// Set the match result
		match.Result = &models.MatchResult{
			Winner:         aiResult.Winner,
			FinalScoreHome: aiResult.Score1,
			FinalScoreAway: aiResult.Score2,
			Duration:       90,
		}
		match.Status = "finished"

		// Handle payout
		if match.ChallengeID != "" {
			go services.HandleEscrowPayout(match)
		}

		return utils.SendSuccess(c, fiber.Map{
			"result": aiResult,
		})
	})
}
