package routes

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/gofiber/fiber/v2"

	"betelite-go/db"
	"betelite-go/middleware"
)

func generateReferralCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func SetupReferralRoutes(api fiber.Router) {
	referrals := api.Group("/referrals", middleware.AuthRequired())

	// Get my referral code and stats
	referrals.Get("/", func(c *fiber.Ctx) error {
		uid := middleware.GetUID(c)
		ctx := context.Background()

		var code string
		err := db.Pool.QueryRow(ctx, "SELECT referral_code FROM users WHERE id = $1", uid).Scan(&code)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch user"})
		}

		if code == "" {
			// Generate new code
			code = generateReferralCode()
			_, err = db.Pool.Exec(ctx, "UPDATE users SET referral_code = $1 WHERE id = $2", code, uid)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to generate code"})
			}
		}

		// Count referrals
		var count int
		db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE referred_by = $1", uid).Scan(&count)

		return c.JSON(fiber.Map{
			"success": true,
			"code":    code,
			"count":   count,
		})
	})

	// Process referral entry (called when someone signs up or claims a code)
	referrals.Post("/claim", func(c *fiber.Ctx) error {
		var req struct {
			Code string `json:"code"`
		}
		if err := c.BodyParser(&req); err != nil || req.Code == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid code"})
		}

		uid := middleware.GetUID(c)
		ctx := context.Background()

		// DB Transaction
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Database error"})
		}
		defer tx.Rollback(ctx)

		// 1. Check if user already claimed a code
		var existingRef string
		err = tx.QueryRow(ctx, "SELECT referred_by FROM users WHERE id = $1 FOR UPDATE", uid).Scan(&existingRef)
		if err == nil && existingRef != "" {
			return c.Status(400).JSON(fiber.Map{"error": "You have already claimed a referral code"})
		}

		// 2. Find referrer
		var referrerID string
		err = tx.QueryRow(ctx, "SELECT id FROM users WHERE referral_code = $1", req.Code).Scan(&referrerID)
		if err != nil || referrerID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid referral code"})
		}

		if referrerID == uid {
			return c.Status(400).JSON(fiber.Map{"error": "You cannot refer yourself"})
		}

		// 3. Update user
		_, err = tx.Exec(ctx, "UPDATE users SET referred_by = $1 WHERE id = $2", referrerID, uid)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to claim referral"})
		}

		// 4. Reward referrer (e.g., 500 kobo / 5 NGN or percentage of first deposit)
		// For simplicity, a flat bonus here.
		rewardAmount := int64(500)
		_, err = tx.Exec(ctx, "UPDATE users SET balance = balance + $1 WHERE id = $2", rewardAmount, referrerID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to reward referrer"})
		}

		refID := fmt.Sprintf("ref_%s", uid)
		_, err = tx.Exec(ctx, "INSERT INTO transactions (user_id, type, amount, ref_id) VALUES ($1, 'referral', $2, $3)",
			referrerID, rewardAmount, refID)

		if err := tx.Commit(ctx); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Transaction commit failed"})
		}

		return c.JSON(fiber.Map{"success": true})
	})
}
