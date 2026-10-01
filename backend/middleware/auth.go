package middleware

import (
	"context"
	"errors"
	"log"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/api/option"

	"betelite-go/config"
	"betelite-go/services"
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

// Identity is the verified caller behind a Firebase ID token.
type Identity struct {
	UID           string
	Email         string
	Name          string
	EmailVerified bool
}

// VerifyToken checks a Firebase ID token. Outside production, with Firebase
// not configured, it returns a fixed dev identity for local testing.
func VerifyToken(ctx context.Context, idToken string) (*Identity, error) {
	if firebaseAuth == nil {
		if config.Cfg.Env == "production" {
			return nil, errors.New("authentication service unavailable")
		}
		// "dev:<name>" tokens let local tests act as several users.
		uid := "dev-uid"
		if strings.HasPrefix(idToken, "dev:") && len(idToken) > 4 {
			uid = "dev-" + idToken[4:]
		}
		return &Identity{UID: uid, Email: uid + "@example.com", Name: uid, EmailVerified: true}, nil
	}
	token, err := firebaseAuth.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, err
	}
	id := &Identity{UID: token.UID}
	id.Email, _ = token.Claims["email"].(string)
	id.Name, _ = token.Claims["name"].(string)
	id.EmailVerified, _ = token.Claims["email_verified"].(bool)
	return id, nil
}

// AuthRequired verifies the Firebase ID token and makes sure the caller has a
// row in Postgres (the wallet of record).
func AuthRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if firebaseAuth == nil && config.Cfg.Env == "production" {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Authentication service unavailable"})
		}

		idToken := ""
		if h := c.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			idToken = strings.TrimPrefix(h, "Bearer ")
		}
		if idToken == "" && firebaseAuth != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Please sign in"})
		}

		id, err := VerifyToken(c.Context(), idToken)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Your session has expired. Please sign in again."})
		}

		if err := services.EnsureUser(c.Context(), id.UID, id.Email, id.Name); err != nil {
			log.Printf("[ERROR] EnsureUser %s: %v", id.UID, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not load your account"})
		}
		if services.IsDeleted(c.Context(), id.UID) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "This account has been deleted"})
		}

		c.Locals("uid", id.UID)
		c.Locals("email", id.Email)
		c.Locals("emailVerified", id.EmailVerified)
		return c.Next()
	}
}

// IsAdminCtx reports whether the authenticated caller is an admin: either the
// configured ADMIN_EMAIL with a verified address, or flagged is_admin in the DB.
func IsAdminCtx(c *fiber.Ctx) bool {
	verified, _ := c.Locals("emailVerified").(bool)
	email := GetEmail(c)
	if email != "" && verified && strings.EqualFold(email, config.Cfg.AdminEmail) {
		return true
	}
	return services.IsAdmin(c.Context(), GetUID(c))
}

// AdminRequired rejects callers who are not admins. Use after AuthRequired.
func AdminRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !IsAdminCtx(c) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
		}
		return c.Next()
	}
}

// DeleteFirebaseUser removes the sign-in account after account deletion.
func DeleteFirebaseUser(ctx context.Context, uid string) {
	if firebaseAuth == nil {
		return
	}
	if err := firebaseAuth.DeleteUser(ctx, uid); err != nil {
		log.Printf("[WARN] delete firebase user %s: %v", uid, err)
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
