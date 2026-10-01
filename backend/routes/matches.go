package routes

import (
	"io"

	"github.com/gofiber/fiber/v2"

	"betelite-go/middleware"
	"betelite-go/services"
	"betelite-go/utils"
)

func SetupMatchRoutes(api fiber.Router) {
	m := api.Group("/matches", middleware.AuthRequired(), dbRequired)

	// Public live board: unfinished matches.
	m.Get("/", func(c *fiber.Ctx) error {
		list, err := services.LiveMatches(c.Context(), 50)
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"matches": list})
	})

	// My matches: active first, then history.
	m.Get("/mine", func(c *fiber.Ctx) error {
		list, err := services.UserMatches(c.Context(), middleware.GetUID(c), 50)
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"matches": list})
	})

	// The one match I'm meant to play / act on right now (or null).
	m.Get("/current", func(c *fiber.Ctx) error {
		cur, err := services.CurrentMatch(c.Context(), middleware.GetUID(c))
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"match": cur})
	})

	m.Get("/:id", func(c *fiber.Ctx) error {
		match, err := services.GetMatch(c.Context(), c.Params("id"))
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"match": match})
	})

	// Upload the final result screenshot for this match (multipart "image").
	m.Post("/:id/result", middleware.RateLimitOCR(), func(c *fiber.Ctx) error {
		fh, err := c.FormFile("image")
		if err != nil {
			return utils.SendError(c, 400, "Please attach a screenshot of the final result screen")
		}
		if fh.Size > services.MaxScreenshotLen {
			return utils.SendError(c, 400, "Screenshot is too large (max 8 MB)")
		}
		f, err := fh.Open()
		if err != nil {
			return fail(c, err)
		}
		img, err := io.ReadAll(io.LimitReader(f, services.MaxScreenshotLen+1))
		f.Close()
		if err != nil {
			return fail(c, err)
		}
		match, err := services.SubmitResult(c.Context(), c.Params("id"), middleware.GetUID(c), img)
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"match": match})
	})

	m.Post("/:id/confirm", func(c *fiber.Ctx) error {
		match, err := services.ConfirmResult(c.Context(), c.Params("id"), middleware.GetUID(c))
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"match": match})
	})

	m.Post("/:id/dispute", func(c *fiber.Ctx) error {
		var req struct {
			Reason string `json:"reason"`
		}
		c.BodyParser(&req)
		if len(req.Reason) < 5 {
			return utils.SendError(c, 400, "Tell us briefly what is wrong with the result")
		}
		match, err := services.DisputeResult(c.Context(), c.Params("id"), middleware.GetUID(c), req.Reason)
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"match": match})
	})
}
