package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"

	"betelite-go/config"
)

// Cors returns a fiber CORS middleware configured from env
func Cors() fiber.Handler {
	allowedOrigins := strings.Join(config.Cfg.AllowedOrigins, ",")
	return cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
	})
}
