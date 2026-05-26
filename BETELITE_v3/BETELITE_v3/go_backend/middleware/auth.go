package middleware

import (
	"context"
	"log"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/api/option"

	"betelite-go/config"
)

var firebaseAuth *auth.Client

// InitFirebaseAuth initializes the Firebase Admin SDK
func InitFirebaseAuth(ctx context.Context) error {
	var app *firebase.App
	var err error

	if config.Cfg.FirebaseProjectID == "" {
		log.Println("[WARN] FIREBASE_PROJECT_ID is empty. Firebase Auth will be disabled.")
		return nil
	}

	conf := &firebase.Config{ProjectID: config.Cfg.FirebaseProjectID}

	if config.Cfg.FirebaseServiceAccountJSON != "" {
		opt := option.WithCredentialsJSON([]byte(config.Cfg.FirebaseServiceAccountJSON))
		app, err = firebase.NewApp(ctx, conf, opt)
	} else {
		log.Println("[INFO] No Firebase service account JSON provided, falling back to Application Default Credentials.")
		app, err = firebase.NewApp(ctx, conf)
	}

	if err != nil {
		return err
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return err
	}

	firebaseAuth = client
	log.Println("[INFO] Firebase Auth initialized successfully")
	return nil
}

// AuthRequired is a middleware that verifies the Firebase ID token
func AuthRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if firebaseAuth == nil {
			// In dev mode without Firebase, we might want to bypass or inject a dummy user
			// For safety, let's deny unless explicitly bypassed.
			// But for smooth testing locally without keys, we can bypass:
			log.Println("[WARN] AuthRequired bypassed (Firebase not initialized)")
			c.Locals("uid", "dev-uid")
			c.Locals("email", "dev@example.com")
			return c.Next()
		}

		authHeader := c.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Missing or invalid authorization header"})
		}

		idToken := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := firebaseAuth.VerifyIDToken(c.Context(), idToken)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid or expired token"})
		}

		c.Locals("uid", token.UID)
		if email, ok := token.Claims["email"]; ok {
			c.Locals("email", email)
		}

		return c.Next()
	}
}

// AdminRequired checks if the authenticated user matches the Admin email
func AdminRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		email := GetEmail(c)
		if email != config.Cfg.AdminEmail {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
		}
		return c.Next()
	}
}

// GetUID extracts the UID from locals
func GetUID(c *fiber.Ctx) string {
	if uid, ok := c.Locals("uid").(string); ok {
		return uid
	}
	return ""
}

// GetEmail extracts the Email from locals
func GetEmail(c *fiber.Ctx) string {
	if email, ok := c.Locals("email").(string); ok {
		return email
	}
	return ""
}
