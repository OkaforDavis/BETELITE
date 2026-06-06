package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
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
	LiveKitAPIKey              string
	LiveKitAPISecret           string
	DetectionServiceURL        string
	DetectionAPISecret         string
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
		AdminEmail:                 getEnv("ADMIN_EMAIL", "okafordavis8@gmail.com"),
		AllowedOrigins:             origins,
		APIFootballKey:             getEnv("API_FOOTBALL_KEY", ""),
		AnthropicAPIKey:            getEnv("ANTHROPIC_API_KEY", ""),
		GeminiAPIKey:               getEnv("GEMINI_API_KEY", ""),
		LiveKitAPIKey:              getEnv("LIVEKIT_API_KEY", ""),
		LiveKitAPISecret:           getEnv("LIVEKIT_API_SECRET", ""),
		DetectionServiceURL:        getEnv("DETECTION_SERVICE_URL", "http://localhost:5000"),
		DetectionAPISecret:         getEnv("DETECTION_API_SECRET", "betelite_internal_secret_key_123"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
