package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"betelite-go/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// RateLimiter is the global API limit. Mobile networks in Nigeria and Ghana put
// many customers behind one IP address (carrier NAT), so the main limit is per
// signed-in user (keyed by their auth token) and only a generous ceiling
// applies per IP, which still stops floods of fake tokens from one address.
func RateLimiter() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          3000,
		Expiration:   time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string { return "ip:" + c.IP() },
		LimitReached: func(c *fiber.Ctx) error {
			return utils.SendError(c, 429, "Too many requests from your network. Please try again shortly.")
		},
	})
}

// RateLimiterUser is the per-user part of the global limit (see RateLimiter).
func RateLimiterUser() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          300,
		Expiration:   time.Minute,
		KeyGenerator: requesterKey,
		LimitReached: func(c *fiber.Ctx) error {
			return utils.SendError(c, 429, "Too many requests. Please try again later.")
		},
	})
}

// requesterKey identifies the caller: the verified UID when known, otherwise a
// hash of the bearer token (one per user session), otherwise the IP.
func requesterKey(c *fiber.Ctx) string {
	if uid := GetUID(c); uid != "" {
		return "u:" + uid
	}
	if h := c.Get("Authorization"); len(h) > 20 {
		sum := sha256.Sum256([]byte(h))
		return "t:" + hex.EncodeToString(sum[:8])
	}
	return "ip:" + c.IP()
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
