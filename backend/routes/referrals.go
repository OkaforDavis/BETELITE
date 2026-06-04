package routes

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/gofiber/fiber/v2"

	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/services"
	"betelite-go/utils"
)

func generateReferralCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 8)
	// seed rand
	rand.Seed(time.Now().UnixNano())
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
			return utils.SendError(c, 500, "Failed to fetch user")
		}

		if code == "" {
			// Generate new code
			code = generateReferralCode()
			_, err = db.Pool.Exec(ctx, "UPDATE users SET referral_code = $1 WHERE id = $2", code, uid)
			if err != nil {
				return utils.SendError(c, 500, "Failed to generate code")
			}
		}

		// Count referrals
		var count int
		db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE referred_by = $1", uid).Scan(&count)

		return utils.SendSuccess(c, fiber.Map{
			"code":      code,
			"referrals": count,
		})
	})

	// Process referral entry (called when someone signs up or claims a code)
	referrals.Post("/claim", func(c *fiber.Ctx) error {
		var req struct {
			Code string `json:"code"`
		}
		if err := c.BodyParser(&req); err != nil || req.Code == "" {
			return utils.SendError(c, 400, "Invalid payload")
		}

		uid := middleware.GetUID(c)
		ctx := context.Background()

		// DB Transaction
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return utils.SendError(c, 500, "Database error")
		}
		defer tx.Rollback(ctx)

		// 1. Check if user already claimed a code
		var existingRef *string
		err = tx.QueryRow(ctx, "SELECT referred_by FROM users WHERE id = $1 FOR UPDATE", uid).Scan(&existingRef)
		if err == nil && existingRef != nil {
			return utils.SendError(c, 400, "You have already claimed a referral code")
		}

		// 2. Find referrer
		var referrerID string
		err = tx.QueryRow(ctx, "SELECT id FROM users WHERE referral_code = $1", req.Code).Scan(&referrerID)
		if err != nil || referrerID == "" {
			return utils.SendError(c, 400, "Invalid referral code")
		}

		if referrerID == uid {
			return utils.SendError(c, 400, "You cannot refer yourself")
		}

		// 3. Update user
		_, err = tx.Exec(ctx, "UPDATE users SET referred_by = $1 WHERE id = $2", referrerID, uid)
		if err != nil {
			return utils.SendError(c, 500, "Failed to claim referral")
		}

		// 4. Reward referrer (e.g., 500 kobo / 5 NGN)
		rewardAmount := int64(500)
		refID := fmt.Sprintf("ref_%s", uid)
		
		err = services.AdjustBalance(ctx, tx, referrerID, rewardAmount, "referral", refID)
		if err != nil {
			return utils.SendError(c, 500, "Failed to reward referrer")
		}

		if err := tx.Commit(ctx); err != nil {
			return utils.SendError(c, 500, "Transaction commit failed")
		}

		return utils.SendSuccess(c, fiber.Map{})
	})
}
