package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"google.golang.org/genai"

	"betelite-go/config"
)

// When every Gemini model is busy, the upload must still succeed through
// Claude, which itself retries Anthropic's 529 "overloaded".
func TestOCRFallsBackToClaudeWhenGeminiBusy(t *testing.T) {
	gemini := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		fmt.Fprint(w, `{"error":{"code":503,"message":"This model is currently experiencing high demand.","status":"UNAVAILABLE"}}`)
	}))
	defer gemini.Close()

	var claudeCalls atomic.Int32
	var lastBody map[string]any
	var lastBeta string
	result := `{"is_result_screen":true,"is_versus_human":true,"mode":"Dream League Live","ingame_match_id":"","players":[{"name":"REAL MADRID","ingame_id":"","score":2,"penalty_score":null,"side":"LEFT"},{"name":"NARIONA","ingame_id":"","score":1,"penalty_score":null,"side":"RIGHT"}],"confidence":98,"reject_reason":"","notes":"ok"}`
	claude := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := claudeCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			w.WriteHeader(529)
			fmt.Fprint(w, `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		json.Unmarshal(raw, &lastBody)
		lastBeta = r.Header.Get("anthropic-beta")
		resp, _ := json.Marshal(map[string]any{
			"id": "msg_1", "type": "message", "role": "assistant", "model": "claude-opus-5",
			"content":     []any{map[string]any{"type": "text", "text": result}},
			"stop_reason": "end_turn", "usage": map[string]any{"input_tokens": 1, "output_tokens": 1},
		})
		w.Write(resp)
	}))
	defer claude.Close()

	gc, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: "x", Backend: genai.BackendGeminiAPI, HTTPOptions: genai.HTTPOptions{BaseURL: gemini.URL + "/"}})
	if err != nil {
		t.Fatal(err)
	}
	geminiClientOnce.Do(func() {})
	geminiClient, geminiClientErr = gc, nil
	claudeClientOnce.Do(func() {})
	claudeClient = anthropic.NewClient(option.WithAPIKey("x"), option.WithBaseURL(claude.URL), option.WithMaxRetries(2))
	config.Cfg.GeminiAPIKey, config.Cfg.AnthropicAPIKey = "x", "x"
	config.Cfg.GeminiModels = []string{"m1", "m2"}
	config.Cfg.ClaudeModel = "claude-opus-5"
	config.Cfg.OCRProviders = []string{"gemini", "claude"}

	png := []byte("\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 32))
	res, err := AnalyzeScreenshot(context.Background(), png, GameByID("dls"), []ExpectedPlayer{{Gamertag: "REAL MADRID"}, {Gamertag: "NARIONA"}})
	if err != nil {
		t.Fatalf("expected Claude fallback to succeed: %v", err)
	}
	if res.Model != "claude-opus-5" || res.Players[0].Name != "REAL MADRID" || res.Players[1].Score != 1 {
		t.Fatalf("unexpected result %+v", res)
	}
	if claudeCalls.Load() != 2 {
		t.Fatalf("Claude called %d times; want 2 (529 retried once)", claudeCalls.Load())
	}
	if h, a, err := MatchPlayers(res, ExpectedPlayer{Gamertag: "REAL MADRID"}, ExpectedPlayer{Gamertag: "NARIONA"}); err != nil || h != 0 || a != 1 {
		t.Fatalf("names should map to sides: h=%d a=%d err=%v", h, a, err)
	}

	// The request must carry the image, the JSON schema and refusal fallbacks.
	b, _ := json.Marshal(lastBody)
	for _, want := range []string{`"type":"image"`, `"media_type":"image/png"`, `"json_schema"`, `"fallbacks":"default"`, `"model":"claude-opus-5"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("request body missing %s", want)
		}
	}
	if !strings.Contains(lastBeta, "server-side-fallback-2026-07-01") {
		t.Errorf("missing fallback beta header, got %q", lastBeta)
	}
}
