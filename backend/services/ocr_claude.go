package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"

	"betelite-go/config"
)

var (
	claudeClient     anthropic.Client
	claudeClientOnce sync.Once
)

func getClaudeClient() *anthropic.Client {
	claudeClientOnce.Do(func() {
		claudeClient = anthropic.NewClient(
			option.WithAPIKey(config.Cfg.AnthropicAPIKey),
			option.WithMaxRetries(3), // retries 429, 5xx and 529 "overloaded" with backoff
		)
	})
	return &claudeClient
}

// claudeSchema mirrors OCRResult. Structured outputs guarantee the reply
// parses; optional values are nullable rather than omitted.
var claudeSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"is_result_screen", "is_versus_human", "mode", "ingame_match_id", "players", "confidence", "reject_reason", "notes"},
	"properties": map[string]any{
		"is_result_screen": map[string]any{"type": "boolean"},
		"is_versus_human":  map[string]any{"type": "boolean"},
		"mode":             map[string]any{"type": "string"},
		"ingame_match_id":  map[string]any{"type": "string", "description": "Match ID / code shown by the game, or empty"},
		"players": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"name", "ingame_id", "score", "penalty_score", "side"},
				"properties": map[string]any{
					"name":          map[string]any{"type": "string"},
					"ingame_id":     map[string]any{"type": "string", "description": "Player ID if displayed, or empty"},
					"score":         map[string]any{"type": "integer"},
					"penalty_score": map[string]any{"type": []string{"integer", "null"}, "description": "Shoot-out goals, null if no shoot-out"},
					"side":          map[string]any{"type": "string", "enum": []string{"LEFT", "RIGHT"}},
				},
			},
		},
		"confidence":    map[string]any{"type": "integer", "description": "0-100"},
		"reject_reason": map[string]any{"type": "string"},
		"notes":         map[string]any{"type": "string"},
	},
}

// analyzeClaude reads a result screenshot with Claude. Used as the second
// provider so a Google outage alone can't block result uploads.
func analyzeClaude(ctx context.Context, img []byte, mime, prompt string) (*OCRResult, error) {
	if config.Cfg.AnthropicAPIKey == "" {
		return nil, errProviderNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	resp, err := getClaudeClient().Beta.Messages.New(ctx, anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(config.Cfg.ClaudeModel),
		MaxTokens: 16000,
		// If a safety classifier declines, the API re-serves the request with
		// a suitable fallback model inside the same call.
		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01},
		Fallbacks: anthropic.BetaFallbacksParamUnion{OfDefault: constant.ValueOf[constant.Default]()},
		OutputConfig: anthropic.BetaOutputConfigParam{
			Effort: anthropic.BetaOutputConfigEffortMedium,
			Format: anthropic.BetaJSONOutputFormatParam{Schema: claudeSchema},
		},
		Messages: []anthropic.BetaMessageParam{
			anthropic.NewBetaUserMessage(
				anthropic.NewBetaImageBlock(anthropic.BetaBase64ImageSourceParam{
					Data:      base64.StdEncoding.EncodeToString(img),
					MediaType: anthropic.BetaBase64ImageSourceMediaType(mime),
				}),
				anthropic.NewBetaTextBlock(prompt),
			),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("claude: %w", err)
	}
	if resp.StopReason == anthropic.BetaStopReasonRefusal {
		return nil, fmt.Errorf("claude declined to read the screenshot")
	}

	for _, block := range resp.Content {
		if text, ok := block.AsAny().(anthropic.BetaTextBlock); ok {
			var out OCRResult
			if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
				return nil, fmt.Errorf("parse claude response: %w", err)
			}
			out.Model = string(resp.Model)
			return &out, nil
		}
	}
	return nil, errEmptyResponse
}
