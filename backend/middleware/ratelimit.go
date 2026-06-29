package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"betelite-go/utils"
)

// RateLimiter returns a fiber rate limiting middleware (Global Default)
func RateLimiter() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        200,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			uid := GetUID(c)
			if uid != "" {
				return uid
			}
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return utils.SendError(c, 429, "Too many requests. Please try again later.")
		},
	})
}

// RateLimitOCR returns a strict fiber rate limiting middleware for OCR/AI functions
func RateLimitOCR() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        5, // 5 requests per minute
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			uid := GetUID(c)
			if uid != "" {
				return uid
			}
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return utils.SendError(c, 429, "OCR Rate Limit Exceeded. You can only perform 5 verifications per minute.")
		},
	})
}

// RateLimitMatchCreation limits match and challenge creation to prevent spam
func RateLimitMatchCreation() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        3, // 3 matches/challenges per minute
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			uid := GetUID(c)
			if uid != "" {
				return uid
			}
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return utils.SendError(c, 429, "You are creating challenges too quickly. Please slow down.")
		},
	})
}
