package config

import (
	"log"
	"os"
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
var defaultGeminiModels = []string{"gemini-3.8-flash", "gemini-3.5-flash"}

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
