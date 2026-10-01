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
			HostName string `json:"hostName"`
			HostId   string `json:"hostId"`
			GameType string `json:"gameType"`
			RoomId   string `json:"roomId"`
		}
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid payload")
		}

		if req.RoomId == "" || req.HostId == "" {
			return utils.SendError(c, 400, "Missing required fields")
		}

		match := &models.Match{
			ID:        req.RoomId,
			Label:     req.GameType + " Stream",
			Home:      req.HostName,
			HomeID:    req.HostId,
			Away:      "Waiting...",
			AwayID:    "",
			ScoreHome: 0,
			ScoreAway: 0,
			Minute:    0,
			Status:    "live",
			IsP2P:     true,
		}

		// Register in the engine so it appears in live matches
		services.Engine.AddMatch(match)

		return utils.SendSuccess(c, fiber.Map{"ok": true, "match": match})
	})

	// Submit score via native Go Gemini OCR
	matchGroup.Post("/submit-score", func(c *fiber.Ctx) error {
		matchId := c.FormValue("matchId")
		if matchId == "" {
			return utils.SendError(c, 400, "Missing matchId")
		}

		match := services.Engine.GetMatch(matchId)
		if match == nil {
			return utils.SendError(c, 404, "Match not found")
		}
		if match.Status != "live" {
			return utils.SendError(c, 400, "Match is not live")
		}

		fileHeader, err := c.FormFile("image")
		if err != nil {
			return utils.SendError(c, 400, "Missing image file")
		}

		// Save file temporarily
		tempPath := filepath.Join(os.TempDir(), fmt.Sprintf("submit_%s_%s", matchId, fileHeader.Filename))
		if err := c.SaveFile(fileHeader, tempPath); err != nil {
			return utils.SendError(c, 500, "Error saving file")
		}
		defer os.Remove(tempPath)

		// Call native Go Gemini OCR
		aiResult, err := services.VerifyMatchResult(tempPath, match.Game, "", "")
		if err != nil {
			return utils.SendError(c, 500, "AI detection failed: "+err.Error())
		}

		if !aiResult.Detected {
			return utils.SendError(c, 422, "Could not detect scores from screenshot: "+aiResult.Notes)
		}

		// Update match with AI scores
		match.ScoreHome = aiResult.Score1
		match.ScoreAway = aiResult.Score2
		match.Status = "finished"

		// Finalize match in engine
		services.Engine.HandleMatchEnd(match)

		return utils.SendSuccess(c, fiber.Map{
			"scoreHome": aiResult.Score1,
			"scoreAway": aiResult.Score2,
			"winner":    aiResult.Winner,
			"gameType":  aiResult.GameType,
			"notes":     aiResult.Notes,
		})
	})
}
