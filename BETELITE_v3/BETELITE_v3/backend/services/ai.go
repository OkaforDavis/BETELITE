package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"betelite-go/config"
)

type PlayerScoreResult struct {
	GamerTag string `json:"gamertag"`
	Score    int    `json:"score"`
	Side     string `json:"side,omitempty"`
}

type AIResult struct {
	Detected     bool               `json:"detected"`
	GameType     string             `json:"game_type,omitempty"`
	TargetPlayer *PlayerScoreResult `json:"target_player,omitempty"`
	Opponent     *PlayerScoreResult `json:"opponent,omitempty"`
	Notes        string             `json:"notes"`
	Winner       string             `json:"-"` // Computed from result
	Score1       int                `json:"-"` // Computed from result
	Score2       int                `json:"-"` // Computed from result
}

// VerifyMatchResult calls the external AI verification service
// (LLM vision-based detection service)
func VerifyMatchResult(imagePath string, gameType string, targetGamertag string, opponentGamertag string) (*AIResult, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add the image file
	part, err := writer.CreateFormFile("file", "match_result.jpg")
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return nil, err
	}

	// Add form fields for game context
	if gameType != "" {
		writer.WriteField("game", gameType)
	}
	if targetGamertag != "" {
		writer.WriteField("target_gamertag", targetGamertag)
	}
	if opponentGamertag != "" {
		writer.WriteField("opponent_gamertag", opponentGamertag)
	}

	writer.Close()

	detectionURL := config.Cfg.DetectionServiceURL
	if detectionURL == "" {
		detectionURL = "http://localhost:5000"
	}
	req, err := http.NewRequest("POST", detectionURL+"/api/detect/frame", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 60 * time.Second} // LLM calls can take longer
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("AI service returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result AIResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Compute legacy fields from structured result
	if result.Detected && result.TargetPlayer != nil && result.Opponent != nil {
		result.Score1 = result.TargetPlayer.Score
		result.Score2 = result.Opponent.Score
		if result.TargetPlayer.Score > result.Opponent.Score {
			result.Winner = result.TargetPlayer.GamerTag
		} else if result.Opponent.Score > result.TargetPlayer.Score {
			result.Winner = result.Opponent.GamerTag
		} else {
			result.Winner = "draw"
		}
	}

	return &result, nil
}
