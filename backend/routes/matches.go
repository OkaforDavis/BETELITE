package routes

import (

	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"

	"betelite-go/config"
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
	// Submit score via AI Detection Service
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

		file, err := fileHeader.Open()
		if err != nil {
			return utils.SendError(c, 500, "Error opening file")
		}
		defer file.Close()

		// Read file into buffer
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("image", fileHeader.Filename)
		if err != nil {
			return utils.SendError(c, 500, "Error creating form file")
		}
		if _, err := io.Copy(part, file); err != nil {
			return utils.SendError(c, 500, "Error copying file")
		}
		writer.Close()

		// Send to Python Detection Service
		req, err := http.NewRequest("POST", config.Cfg.DetectionServiceURL+"/predict", body)
		if err != nil {
			return utils.SendError(c, 500, "Error creating detection request")
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		// Add API Key (We'll define DetectionAPISecret in config)
		req.Header.Set("X-API-Key", config.Cfg.DetectionAPISecret)

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return utils.SendError(c, 500, "Error contacting detection service")
		}
		defer resp.Body.Close()

		var detRes struct {
			ScoreHome int `json:"score_home"`
			ScoreAway int `json:"score_away"`
			Time      int `json:"time"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&detRes); err != nil {
			return utils.SendError(c, 500, "Error decoding detection result")
		}

		// Update match with AI scores
		match.ScoreHome = detRes.ScoreHome
		match.ScoreAway = detRes.ScoreAway
		match.Minute = detRes.Time
		match.Status = "finished"

		// Finalize match in engine
		services.Engine.HandleMatchEnd(match)

		return utils.SendSuccess(c, fiber.Map{
			"scoreHome": detRes.ScoreHome,
			"scoreAway": detRes.ScoreAway,
			"time":      detRes.Time,
		})
	})
}
