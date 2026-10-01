package routes

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"

	"betelite-go/middleware"
	"betelite-go/models"
	"betelite-go/services"
	"betelite-go/utils"
)

func SetupDetectRoutes(api fiber.Router) {
	detect := api.Group("/detect", middleware.AuthRequired(), middleware.RateLimitOCR())

	// ── /frame: Lightweight detection endpoint for the Detect tab ──
	// Frontend sends: FormData with 'game', 'image_b64', optionally 'match_id'
	// Returns the raw AI result from the native Go Gemini OCR.
	detect.Post("/frame", func(c *fiber.Ctx) error {
		game := c.FormValue("game", "auto")
		imageB64 := c.FormValue("image_b64", "")
		targetGamertag := c.FormValue("target_gamertag", "")
		opponentGamertag := c.FormValue("opponent_gamertag", "")

		if imageB64 == "" {
			return utils.SendError(c, 400, "No image provided (image_b64 required)")
		}

		// Strip "data:image/jpeg;base64," or similar if present
		if idx := strings.Index(imageB64, ","); idx != -1 {
			imageB64 = imageB64[idx+1:]
		}

		imgData, err := base64.StdEncoding.DecodeString(imageB64)
		if err != nil {
			return utils.SendError(c, 400, "Invalid base64 image")
		}

		aiResult, err := services.VerifyMatchResultFromBytes(imgData, game, targetGamertag, opponentGamertag)
		if err != nil {
			// Never return placeholder scores: a fake 0-0 looks like a real result.
			log.Printf("[OCR] detection failed: %v", err)
			return utils.SendError(c, 502, "Score detection is unavailable right now. Please try again in a moment.")
		}

		// Map fields for frontend compatibility
		result := fiber.Map{
			"detected":      aiResult.Detected,
			"notes":         aiResult.Notes,
			"target_player": aiResult.TargetPlayer,
			"opponent":      aiResult.Opponent,
			"game_type":     aiResult.GameType,
		}

		if aiResult.TargetPlayer != nil {
			result["score1"] = aiResult.TargetPlayer.Score
			result["game_detected"] = aiResult.GameType
		}
		if aiResult.Opponent != nil {
			result["score2"] = aiResult.Opponent.Score
		}
		if aiResult.Detected {
			result["confidence"] = 85
			result["reasoning"] = aiResult.Notes
		}

		return c.JSON(result)
	})

	// ── /match-result: Full match verification with engine integration ──
	// Upload image and verify result against an active match
	detect.Post("/match-result", func(c *fiber.Ctx) error {
		matchID := c.FormValue("matchId")
		if matchID == "" {
			return utils.SendError(c, 400, "matchId is required")
		}

		gameType := c.FormValue("game", "")
		targetGamertag := c.FormValue("target_gamertag", "")
		opponentGamertag := c.FormValue("opponent_gamertag", "")

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

		// Send to native Go Gemini OCR
		aiResult, err := services.VerifyMatchResult(tempPath, gameType, targetGamertag, opponentGamertag)
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
