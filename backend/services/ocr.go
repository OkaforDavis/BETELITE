package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/genai"

	"betelite-go/config"
)

// ── Types ────────────────────────────────────────────────────────────

// PlayerScoreResult represents a detected player's score from a game screenshot.
type PlayerScoreResult struct {
	GamerTag string `json:"gamertag"`
	Score    int    `json:"score"`
	Side     string `json:"side,omitempty"` // LEFT or RIGHT for football games
}

// AIResult is the structured output from the Gemini vision OCR.
type AIResult struct {
	Detected     bool               `json:"detected"`
	GameType     string             `json:"game_type,omitempty"`
	TargetPlayer *PlayerScoreResult `json:"target_player,omitempty"`
	Opponent     *PlayerScoreResult `json:"opponent,omitempty"`
	Notes        string             `json:"notes"`
	Winner       string             `json:"-"` // Computed after parsing
	Score1       int                `json:"-"` // Computed after parsing
	Score2       int                `json:"-"` // Computed after parsing
}

// ── Singleton Gemini Client ──────────────────────────────────────────

var (
	geminiClient     *genai.Client
	geminiClientOnce sync.Once
	geminiClientErr  error
)

// getGeminiClient returns a lazily-initialised Gemini API client.
func getGeminiClient(ctx context.Context) (*genai.Client, error) {
	geminiClientOnce.Do(func() {
		apiKey := config.Cfg.GeminiAPIKey
		if apiKey == "" {
			geminiClientErr = fmt.Errorf("GEMINI_API_KEY is not configured")
			return
		}
		geminiClient, geminiClientErr = genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  apiKey,
			Backend: genai.BackendGeminiAPI,
		})
		if geminiClientErr == nil {
			log.Println("[INFO] Gemini API client initialised successfully")
		}
	})
	return geminiClient, geminiClientErr
}

// ── JSON Schema for Structured Output ────────────────────────────────

// gameScoreSchema defines the JSON response schema enforced via Gemini's
// structured output mode so we always get parseable JSON back.
var gameScoreSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"detected": {
			Type:        genai.TypeBoolean,
			Description: "True if game scores were detected in the screenshot",
		},
		"game_type": {
			Type:        genai.TypeString,
			Description: "The detected game type (e.g. EA FC 25, COD Mobile, DLS)",
		},
		"target_player": {
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"gamertag": {Type: genai.TypeString, Description: "The player name or gamertag"},
				"score":    {Type: genai.TypeInteger, Description: "The player's score or kills"},
				"side":     {Type: genai.TypeString, Description: "LEFT or RIGHT for football games only"},
			},
			Required: []string{"gamertag", "score"},
		},
		"opponent": {
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"gamertag": {Type: genai.TypeString, Description: "The opponent's name or gamertag"},
				"score":    {Type: genai.TypeInteger, Description: "The opponent's score or kills"},
				"side":     {Type: genai.TypeString, Description: "LEFT or RIGHT for football games only"},
			},
			Required: []string{"gamertag", "score"},
		},
		"notes": {
			Type:        genai.TypeString,
			Description: "Brief description of the match results detected or why detection failed",
		},
	},
	Required: []string{"detected", "notes"},
}

// ── Prompt ────────────────────────────────────────────────────────────

func buildDetectionPrompt(gameType, targetGamertag, opponentGamertag string) string {
	if gameType == "" || gameType == "auto" {
		gameType = "auto-detect"
	}
	if targetGamertag == "" {
		targetGamertag = "unknown"
	}
	if opponentGamertag == "" {
		opponentGamertag = "unknown"
	}

	return fmt.Sprintf(`You are a gaming score detection AI. Analyze this screenshot from a competitive mobile/console game match and extract the scores.

Game type: %s
Target player gamertag: %s
Opponent gamertag: %s

Instructions:
1. Identify the scoreboard or result screen in the screenshot
2. Find the scores for each player/team
3. If gamertags are provided, match them to the correct scores using fuzzy matching
4. For football games (FIFA, eFootball, EA FC, DLS, Dream League), identify LEFT and RIGHT sides
5. For FPS games (COD Mobile, PUBG, Free Fire), find kill counts or match scores
6. If there is a clear "Victory/Defeat" or "Win/Lose" indicator, use it to confirm the winner`, gameType, targetGamertag, opponentGamertag)
}

// ── Public API ────────────────────────────────────────────────────────

// VerifyMatchResult sends a game screenshot to Gemini vision and returns
// structured score detection results. This is a pure-Go implementation
// that eliminates the need for a separate Python detection service.
func VerifyMatchResult(imagePath string, gameType string, targetGamertag string, opponentGamertag string) (*AIResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := getGeminiClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("gemini client error: %w", err)
	}

	// Read the image file
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image file: %w", err)
	}

	// Detect MIME type from extension
	mimeType := "image/jpeg"
	lower := strings.ToLower(imagePath)
	switch {
	case strings.HasSuffix(lower, ".png"):
		mimeType = "image/png"
	case strings.HasSuffix(lower, ".webp"):
		mimeType = "image/webp"
	case strings.HasSuffix(lower, ".gif"):
		mimeType = "image/gif"
	}

	prompt := buildDetectionPrompt(gameType, targetGamertag, opponentGamertag)

	// Build multimodal content: text prompt + inline image
	parts := []*genai.Part{
		{Text: prompt},
		{InlineData: &genai.Blob{Data: imgData, MIMEType: mimeType}},
	}
	contents := []*genai.Content{{Parts: parts}}

	// Configure structured JSON output via response schema
	genConfig := &genai.GenerateContentConfig{
		Temperature:      genai.Ptr[float32](0.1), // Low temp for consistent structured output
		MaxOutputTokens:  1024,
		ResponseMIMEType: "application/json",
		ResponseSchema:   gameScoreSchema,
	}

	result, err := client.Models.GenerateContent(ctx, config.Cfg.GeminiModel, contents, genConfig)
	if err != nil {
		return nil, fmt.Errorf("gemini API call failed: %w", err)
	}

	// Extract text response
	responseText := result.Text()
	if responseText == "" {
		return nil, fmt.Errorf("gemini returned empty response")
	}

	log.Printf("[OCR] Gemini raw response: %s", responseText)

	// Parse structured JSON
	var aiResult AIResult
	if err := json.Unmarshal([]byte(responseText), &aiResult); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini JSON response: %w, raw: %s", err, responseText)
	}

	// Compute derived fields
	if aiResult.Detected && aiResult.TargetPlayer != nil && aiResult.Opponent != nil {
		aiResult.Score1 = aiResult.TargetPlayer.Score
		aiResult.Score2 = aiResult.Opponent.Score
		if aiResult.TargetPlayer.Score > aiResult.Opponent.Score {
			aiResult.Winner = aiResult.TargetPlayer.GamerTag
		} else if aiResult.Opponent.Score > aiResult.TargetPlayer.Score {
			aiResult.Winner = aiResult.Opponent.GamerTag
		} else {
			aiResult.Winner = "draw"
		}
	}

	return &aiResult, nil
}

// VerifyMatchResultFromBytes is a convenience wrapper that accepts raw image
// bytes instead of a file path. It writes to a temp file and delegates to
// VerifyMatchResult.
func VerifyMatchResultFromBytes(imgData []byte, gameType, targetGamertag, opponentGamertag string) (*AIResult, error) {
	tmpFile, err := os.CreateTemp("", "ocr_*.jpg")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(imgData); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	return VerifyMatchResult(tmpFile.Name(), gameType, targetGamertag, opponentGamertag)
}
