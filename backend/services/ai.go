package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

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
	apiKey := config.Cfg.GeminiAPIKey
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is not configured")
	}

	schemaPath := "services/ai_schema.json"
	wrapperPath := "services/ocr_wrapper.py"

	cmd := exec.Command("python", wrapperPath,
		"--schema_path", schemaPath,
		"--model", "gemini-2.0-flash", // Use standard model or adjust if needed
		"--base_url", "https://generativelanguage.googleapis.com/v1beta/openai/",
		"--api_key", apiKey,
		imagePath,
	)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("receipt-ocr failed: %v, stderr: %s", err, stderr.String())
	}

	output := out.Bytes()

	var result AIResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse receipt-ocr output: %v, output: %s", err, string(output))
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
