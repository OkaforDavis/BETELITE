package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"google.golang.org/genai"

	"betelite-go/config"
)

// A busy model (503 "high demand") must be retried and then fall back to the
// next model instead of failing the player's upload.
func TestAnalyzeScreenshotFallsBackWhenModelBusy(t *testing.T) {
	var busyCalls, goodCalls atomic.Int32
	result := `{"is_result_screen":true,"is_versus_human":true,"mode":"Head-to-Head","players":[{"name":"AliceFC","score":3,"side":"LEFT"},{"name":"BobKing99","score":1,"side":"RIGHT"}],"confidence":95,"notes":"ok"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "busy-model"):
			busyCalls.Add(1)
			w.WriteHeader(503)
			fmt.Fprint(w, `{"error":{"code":503,"message":"This model is currently experiencing high demand. Spikes in demand are usually temporary.","status":"UNAVAILABLE"}}`)
		case strings.Contains(r.URL.Path, "good-model"):
			goodCalls.Add(1)
			body, _ := json.Marshal(map[string]any{"candidates": []any{map[string]any{
				"content": map[string]any{"role": "model", "parts": []any{map[string]any{"text": result}}},
			}}})
			w.Write(body)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey: "test", Backend: genai.BackendGeminiAPI, HTTPOptions: genai.HTTPOptions{BaseURL: srv.URL + "/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	geminiClientOnce.Do(func() {}) // use our client, not the real one
	geminiClient, geminiClientErr = client, nil
	config.Cfg.GeminiModels = []string{"busy-model", "good-model"}
	config.Cfg.GeminiAPIKey = "test"
	config.Cfg.OCRProviders = []string{"gemini"}

	png := []byte("\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 32))
	res, err := AnalyzeScreenshot(context.Background(), png, GameByID("fc_mobile"), nil)
	if err != nil {
		t.Fatalf("expected fallback to succeed, got %v", err)
	}
	if res.Model != "good-model" || len(res.Players) != 2 || res.Players[0].Score != 3 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if busyCalls.Load() != 2 || goodCalls.Load() != 1 {
		t.Fatalf("busy model called %d times (want 2 retries), good model %d (want 1)", busyCalls.Load(), goodCalls.Load())
	}
}

func TestRetryable(t *testing.T) {
	for _, s := range []string{"Error 503, Status: UNAVAILABLE", "Error 429 RESOURCE_EXHAUSTED", "Error 500 INTERNAL"} {
		if !retryable(fmt.Errorf("%s", s)) {
			t.Errorf("%q should be retryable", s)
		}
	}
	if retryable(fmt.Errorf("Error 400, API key not valid")) {
		t.Error("a bad API key must not be retried")
	}
}

func TestGeminiModelsAlwaysHaveFallback(t *testing.T) {
	cases := map[string][]string{
		"":                                     {"gemini-3.8-flash", "gemini-3.5-flash"},
		"gemini-3.8-flash":                     {"gemini-3.8-flash", "gemini-3.5-flash"},
		" gemini-3.7-flash , gemini-3.8-flash": {"gemini-3.7-flash", "gemini-3.8-flash", "gemini-3.5-flash"},
	}
	for in, want := range cases {
		got := config.ParseGeminiModels(in)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("GEMINI_MODEL=%q → %v, want %v", in, got, want)
		}
	}
}
