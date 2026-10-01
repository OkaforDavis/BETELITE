package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"google.golang.org/genai"

	"betelite-go/config"
)

// ── Types ────────────────────────────────────────────────────────────

// OCRPlayer is one player/club as read from a result screen.
type OCRPlayer struct {
	Name         string `json:"name"`
	InGameID     string `json:"ingame_id,omitempty"`
	Score        int    `json:"score"`
	PenaltyScore *int   `json:"penalty_score,omitempty"`
	Side         string `json:"side,omitempty"` // LEFT or RIGHT
}

// OCRResult is the structured output Gemini returns for a screenshot.
type OCRResult struct {
	IsResultScreen bool        `json:"is_result_screen"`
	IsVersusHuman  bool        `json:"is_versus_human"`
	Mode           string      `json:"mode"`
	InGameMatchID  string      `json:"ingame_match_id,omitempty"`
	Players        []OCRPlayer `json:"players"`
	Confidence     int         `json:"confidence"`
	RejectReason   string      `json:"reject_reason,omitempty"`
	Notes          string      `json:"notes"`
	Model          string      `json:"model,omitempty"` // which Gemini model answered
}

// ExpectedPlayer is a registered player we expect to find on the screenshot.
type ExpectedPlayer struct {
	Gamertag string
	InGameID string
}

// ── Gemini client ────────────────────────────────────────────────────

var (
	geminiClient     *genai.Client
	geminiClientOnce sync.Once
	geminiClientErr  error
)

func getGeminiClient(ctx context.Context) (*genai.Client, error) {
	geminiClientOnce.Do(func() {
		if config.Cfg.GeminiAPIKey == "" {
			geminiClientErr = fmt.Errorf("GEMINI_API_KEY is not configured")
			return
		}
		geminiClient, geminiClientErr = genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  config.Cfg.GeminiAPIKey,
			Backend: genai.BackendGeminiAPI,
		})
		if geminiClientErr == nil {
			log.Printf("[INFO] Gemini client ready (models %v)", config.Cfg.GeminiModels)
		}
	})
	return geminiClient, geminiClientErr
}

var playerSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"name":          {Type: genai.TypeString, Description: "Username / club name exactly as displayed"},
		"ingame_id":     {Type: genai.TypeString, Description: "Player ID number if displayed, else empty"},
		"score":         {Type: genai.TypeInteger, Description: "Final goals scored (excluding penalty shoot-out)"},
		"penalty_score": {Type: genai.TypeInteger, Description: "Penalty shoot-out goals, only if a shoot-out happened", Nullable: genai.Ptr(true)},
		"side":          {Type: genai.TypeString, Enum: []string{"LEFT", "RIGHT"}},
	},
	Required: []string{"name", "score", "side"},
}

var resultSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"is_result_screen": {Type: genai.TypeBoolean, Description: "True only for the final full-time result screen of a completed match"},
		"is_versus_human":  {Type: genai.TypeBoolean, Description: "True only if both sides are human players (not CPU/AI, not a scenario/event/campaign)"},
		"mode":             {Type: genai.TypeString, Description: "Game mode shown, e.g. Head-to-Head, Friend Match, Scenario, vs AI"},
		"ingame_match_id":  {Type: genai.TypeString, Description: "Match ID / code shown by the game, else empty"},
		"players":          {Type: genai.TypeArray, Items: playerSchema, Description: "Exactly the two sides, LEFT first"},
		"confidence":       {Type: genai.TypeInteger, Description: "0-100 confidence that names and scores are read correctly"},
		"reject_reason":    {Type: genai.TypeString, Description: "If the screenshot is not a valid human-vs-human final result, say why in one short sentence"},
		"notes":            {Type: genai.TypeString},
	},
	Required: []string{"is_result_screen", "is_versus_human", "mode", "players", "confidence", "notes"},
}

func buildPrompt(game *Game, expected []ExpectedPlayer) string {
	var exp strings.Builder
	for i, p := range expected {
		fmt.Fprintf(&exp, "  Player %d: name %q", i+1, p.Gamertag)
		if p.InGameID != "" {
			fmt.Fprintf(&exp, ", in-game ID %q", p.InGameID)
		}
		exp.WriteString("\n")
	}
	if exp.Len() == 0 {
		exp.WriteString("  (not provided)\n")
	}
	return fmt.Sprintf(`You verify esports match results for a real-money platform. Be strict: a wrong "valid" answer costs players money.

Game: %s
%s

Registered players for this match:
%s
Read the screenshot and report exactly what is displayed. Do not guess or invent names, IDs or scores; if something is unreadable, lower the confidence and explain in notes.
- is_result_screen: true only for the final full-time result of a finished match.
- is_versus_human: false for CPU/AI opponents, scenarios, events, campaigns or objectives (e.g. "TARGET: WIN BY 2").
- players: the two sides as displayed (LEFT first), with the names exactly as shown.
- If the screen shows only national/club team names and not the players' usernames (where the game normally shows usernames), set is_result_screen=false and explain in reject_reason.`,
		game.Name, game.ocrHints, exp.String())
}

// AnalyzeScreenshot sends a result screenshot to Gemini and returns what it read.
func AnalyzeScreenshot(ctx context.Context, img []byte, game *Game, expected []ExpectedPlayer) (*OCRResult, error) {
	mime := http.DetectContentType(img)
	switch mime {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return nil, fmt.Errorf("unsupported image type %s", mime)
	}
	prompt := buildPrompt(game, expected)

	// Providers are tried in order (OCR_PROVIDERS, default gemini,claude). Each
	// retries its own temporary errors; a second company means one provider's
	// outage can't block result uploads.
	var errs []string
	for _, p := range config.Cfg.OCRProviders {
		var res *OCRResult
		var err error
		switch p {
		case "gemini":
			res, err = analyzeGemini(ctx, img, mime, prompt)
		case "claude":
			res, err = analyzeClaude(ctx, img, mime, prompt)
		default:
			continue
		}
		if err == nil {
			return res, nil
		}
		if errors.Is(err, errProviderNotConfigured) {
			continue
		}
		log.Printf("[OCR] %s failed: %v", p, err)
		errs = append(errs, err.Error())
		if ctx.Err() != nil {
			break
		}
	}
	if len(errs) == 0 {
		return nil, fmt.Errorf("no OCR provider configured (set GEMINI_API_KEY and/or ANTHROPIC_API_KEY)")
	}
	return nil, fmt.Errorf("%s", strings.Join(errs, " | "))
}

var errProviderNotConfigured = errors.New("provider not configured")

// analyzeGemini reads a screenshot with Gemini, retrying busy models and
// falling back across GEMINI_MODEL.
func analyzeGemini(ctx context.Context, img []byte, mime, prompt string) (*OCRResult, error) {
	if config.Cfg.GeminiAPIKey == "" {
		return nil, errProviderNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()

	client, err := getGeminiClient(ctx)
	if err != nil {
		return nil, err
	}

	contents := []*genai.Content{{Parts: []*genai.Part{
		{Text: prompt},
		{InlineData: &genai.Blob{Data: img, MIMEType: mime}},
	}}}
	cfg := &genai.GenerateContentConfig{
		Temperature:      genai.Ptr[float32](0),
		MaxOutputTokens:  1024,
		ResponseMIMEType: "application/json",
		ResponseSchema:   resultSchema,
	}

	// Google sometimes answers 503 "high demand" or 429. Retry briefly, then fall
	// back to the next model, which runs on separate capacity.
	var lastErr error
	for _, model := range config.Cfg.GeminiModels {
		for attempt := 0; attempt < 2; attempt++ {
			if ctx.Err() != nil {
				return nil, fmt.Errorf("gemini: timed out (last error: %v)", lastErr)
			}
			callCtx, cancelCall := context.WithTimeout(ctx, 20*time.Second)
			resp, err := client.Models.GenerateContent(callCtx, model, contents, cfg)
			cancelCall()
			if err == nil {
				out, perr := parseOCR(resp)
				if perr == nil {
					out.Model = model
					return out, nil
				}
				err = perr
			}
			lastErr = err
			if !retryable(err) {
				return nil, fmt.Errorf("gemini (%s): %w", model, err)
			}
			log.Printf("[OCR] %s busy (attempt %d): %v", model, attempt+1, err)
			select {
			case <-time.After(time.Duration(attempt+1) * 1500 * time.Millisecond):
			case <-ctx.Done():
			}
		}
	}
	return nil, fmt.Errorf("gemini: all models busy: %w", lastErr)
}

func parseOCR(resp *genai.GenerateContentResponse) (*OCRResult, error) {
	text := resp.Text()
	if text == "" {
		return nil, errEmptyResponse
	}
	var out OCRResult
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return nil, fmt.Errorf("parse gemini response: %w", err)
	}
	return &out, nil
}

var errEmptyResponse = errors.New("gemini returned an empty response")

// retryable reports whether an error is temporary on Google's side
// (overloaded, rate limited, internal error, timeout or empty answer).
func retryable(err error) bool {
	if errors.Is(err, errEmptyResponse) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	s := err.Error()
	for _, k := range []string{"503", "UNAVAILABLE", "429", "RESOURCE_EXHAUSTED", "500", "INTERNAL", "overloaded", "high demand"} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

// ── Name matching ────────────────────────────────────────────────────

// normalizeName lowercases and strips everything but letters and digits so
// "Davis_99 " and "DAVIS99" compare equal.
func normalizeName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}

// NamesMatch tolerates small OCR slips (one wrong character per 6) but never
// matches very short names loosely.
func NamesMatch(read, registered string) bool {
	a, b := normalizeName(read), normalizeName(registered)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	if len([]rune(b)) < 6 {
		return false
	}
	return levenshtein(a, b) <= len([]rune(b))/6
}

// MatchPlayers finds which read player belongs to each registered player.
// It returns indexes into res.Players for home and away, or an error that is
// safe to show the user.
func MatchPlayers(res *OCRResult, home, away ExpectedPlayer) (int, int, error) {
	if len(res.Players) != 2 {
		return -1, -1, fmt.Errorf("we could not find exactly two players on the screenshot")
	}
	find := func(p ExpectedPlayer) int {
		for i, rp := range res.Players {
			if p.InGameID != "" && rp.InGameID != "" && normalizeName(p.InGameID) == normalizeName(rp.InGameID) {
				return i
			}
		}
		for i, rp := range res.Players {
			if NamesMatch(rp.Name, p.Gamertag) {
				return i
			}
		}
		return -1
	}
	h, a := find(home), find(away)
	switch {
	case h < 0 && a < 0:
		return -1, -1, fmt.Errorf("neither player's name (%s, %s) is on the screenshot", home.Gamertag, away.Gamertag)
	case h < 0:
		return -1, -1, fmt.Errorf("%s's name is not on the screenshot", home.Gamertag)
	case a < 0:
		return -1, -1, fmt.Errorf("%s's name is not on the screenshot", away.Gamertag)
	case h == a:
		return -1, -1, fmt.Errorf("both registered names matched the same player on the screenshot")
	}
	return h, a, nil
}
