package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// RateLimiter returns a fiber rate limiting middleware
func RateLimiter() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        200,
		Expiration: time.Minute,
	})
}
