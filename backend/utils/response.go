package utils

import "github.com/gofiber/fiber/v2"

// SendError formats and returns a standard JSON error response
func SendError(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": message})
}

// SendSuccess formats and returns a standard JSON success response
func SendSuccess(c *fiber.Ctx, data fiber.Map) error {
	data["success"] = true
	return c.JSON(data)
}
