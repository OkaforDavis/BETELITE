package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Env                        string
	Port                       string
	DatabaseURL                string
	FirebaseProjectID          string
	FirebaseServiceAccountJSON string
	VAPIDPublicKey             string
	VAPIDPrivateKey            string
	PaystackPublicKey          string
	PaystackPublicKeyGH        string
	PaystackSecretKey          string
	PaystackSecretKeyGH        string
	AdminEmail                 string
	AllowedOrigins             []string
	APIFootballKey             string
	AnthropicAPIKey            string
	GeminiAPIKey               string
	GeminiModels               []string // tried in order; busy models fall back to the next
	ClaudeModel                string
	OCRProviders               []string // screenshot readers, tried in order
	AppURL                     string
	AppVersion                 string
	ForceUpdate                bool
	LiveKitAPIKey              string
	LiveKitAPISecret           string
	LiveKitURL                 string
	StakingEnabled             bool // real-money 1v1 stakes and paid tournament entries; off until licensed
	FlutterwaveSecretKey       string
	FlutterwavePublicKey       string
	FlutterwaveWebhookHash     string   // "secret hash" set in the Flutterwave dashboard
	FlutterwaveGHS             bool     // Flutterwave may collect and pay out GHS (once Flutterwave enables it)
	PaymentProviders           []string // tried in order, e.g. flutterwave,paystack
	FXMarginPct                float64  // margin on cross-currency payments and home-country switches
	FXNGNPerGHS                float64  // optional fixed rate; 0 = fetch the market rate
	FirebaseAuthDomain         string   // host that serves /__/auth (the app itself once set up), else <project>.firebaseapp.com
}

var Cfg Config

func Load() {
	// Try to load .env from current directory
	err := godotenv.Load(".env")
	if err != nil {
		log.Println("[WARN] No .env file found, relying on system environment variables")
	}

	originsStr := getEnv("ALLOWED_ORIGINS", "*")
	origins := strings.Split(originsStr, ",")

	Cfg = Config{
		Env:                        getEnv("GO_ENV", "development"),
		Port:                       getEnv("PORT", "3000"),
		DatabaseURL:                getEnv("DATABASE_URL", ""),
		FirebaseProjectID:          getEnv("FIREBASE_PROJECT_ID", ""),
		FirebaseServiceAccountJSON: getEnv("FIREBASE_SERVICE_ACCOUNT_JSON", ""),
		VAPIDPublicKey:             getEnv("VAPID_PUBLIC_KEY", ""),
		VAPIDPrivateKey:            getEnv("VAPID_PRIVATE_KEY", ""),
		PaystackPublicKey:          getEnv("PAYSTACK_PUBLIC_KEY", ""),
		PaystackPublicKeyGH:        getEnv("PAYSTACK_PUBLIC_KEY_GH", ""),
		PaystackSecretKey:          getEnv("PAYSTACK_SECRET_KEY", ""),
		PaystackSecretKeyGH:        getEnv("PAYSTACK_SECRET_KEY_GH", ""),
		AdminEmail:                 strings.TrimSpace(getEnv("ADMIN_EMAIL", "okafordavis8@gmail.com")),
		AllowedOrigins:             origins,
		APIFootballKey:             getEnv("API_FOOTBALL_KEY", ""),
		AnthropicAPIKey:            getEnv("ANTHROPIC_API_KEY", ""),
		GeminiAPIKey:               getEnv("GEMINI_API_KEY", ""),
		GeminiModels:               ParseGeminiModels(getEnv("GEMINI_MODEL", "")),
		ClaudeModel:                getEnv("CLAUDE_MODEL", "claude-opus-5"),
		OCRProviders:               splitList(getEnv("OCR_PROVIDERS", "gemini,claude")),
		AppURL:                     getEnv("APP_URL", ""),
		AppVersion:                 appVersion(),
		ForceUpdate:                getEnv("FORCE_UPDATE", "") == "true",
		LiveKitAPIKey:              getEnv("LIVEKIT_API_KEY", ""),
		LiveKitAPISecret:           getEnv("LIVEKIT_API_SECRET", ""),
		LiveKitURL:                 getEnv("LIVEKIT_URL", "wss://betelite-38umojt1.livekit.cloud"),
		StakingEnabled:             getEnv("STAKING_ENABLED", "") == "true",
		FirebaseAuthDomain:         getEnv("FIREBASE_AUTH_DOMAIN", ""),
		FlutterwaveSecretKey:       getEnv("FLUTTERWAVE_SECRET_KEY", ""),
		FlutterwavePublicKey:       getEnv("FLUTTERWAVE_PUBLIC_KEY", ""),
		FlutterwaveWebhookHash:     getEnv("FLUTTERWAVE_WEBHOOK_HASH", ""),
		FlutterwaveGHS:             getEnv("FLUTTERWAVE_GHS", "") == "true",
		PaymentProviders:           splitList(getEnv("PAYMENT_PROVIDERS", "flutterwave,paystack")),
		FXMarginPct:                parseFloat(getEnv("FX_MARGIN_PCT", "3"), 3),
		FXNGNPerGHS:                parseFloat(getEnv("FX_NGN_PER_GHS", "0"), 0),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// appVersion identifies the running build. Render sets RENDER_GIT_COMMIT;
// APP_VERSION overrides it. Clients compare it to know when to update.
func appVersion() string {
	if v := os.Getenv("APP_VERSION"); v != "" {
		return v
	}
	if v := os.Getenv("RENDER_GIT_COMMIT"); len(v) >= 7 {
		return v[:7]
	}
	return "dev"
}

// defaultGeminiModels are always available as fallbacks. Each model runs on
// separate capacity, so when one is overloaded the next usually answers.
var defaultGeminiModels = []string{"gemini-3.8-flash", "gemini-3.7-flash", "gemini-3.5-flash"}

// ParseGeminiModels parses GEMINI_MODEL (comma-separated, preferred first) and
// appends the defaults that aren't already listed.
func ParseGeminiModels(env string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(m string) {
		m = strings.TrimSpace(m)
		if m != "" && !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	for _, m := range strings.Split(env, ",") {
		add(m)
	}
	for _, m := range defaultGeminiModels {
		add(m)
	}
	return out
}

// splitList parses a comma-separated, case-insensitive list.
func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.ToLower(strings.TrimSpace(p)); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseFloat(v string, fallback float64) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return fallback
	}
	return f
}
