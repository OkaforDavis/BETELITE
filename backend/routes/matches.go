package routes

import (
	"io"
	"strconv"
	"strings"

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

	// Dispute: reason plus an optional screenshot as proof (multipart form,
	// or JSON {"reason"} from older app versions).
	m.Post("/:id/dispute", func(c *fiber.Ctx) error {
		reason := c.FormValue("reason")
		if reason == "" {
			var req struct {
				Reason string `json:"reason"`
			}
			c.BodyParser(&req)
			reason = req.Reason
		}
		if len(strings.TrimSpace(reason)) < 5 {
			return utils.SendError(c, 400, "Tell us briefly what is wrong with the result")
		}
		var evidence []byte
		if fh, err := c.FormFile("image"); err == nil {
			if fh.Size > services.MaxScreenshotLen {
				return utils.SendError(c, 400, "Screenshot is too large (max 8 MB)")
			}
			f, err := fh.Open()
			if err != nil {
				return fail(c, err)
			}
			evidence, err = io.ReadAll(io.LimitReader(f, services.MaxScreenshotLen+1))
			f.Close()
			if err != nil {
				return fail(c, err)
			}
		}
		match, err := services.DisputeResult(c.Context(), c.Params("id"), middleware.GetUID(c), reason, evidence)
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"match": match})
	})

	// Screenshot evidence: visible to the two players and admins for 30 days.
	m.Get("/:id/screenshots", func(c *fiber.Ctx) error {
		if !services.CanViewEvidence(c.Context(), c.Params("id"), middleware.GetUID(c), middleware.IsAdminCtx(c)) {
			return utils.SendError(c, 403, "Only the players in this match can view its screenshots")
		}
		list, err := services.ListEvidence(c.Context(), c.Params("id"))
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"screenshots": list})
	})

	m.Get("/:id/screenshots/:sid", func(c *fiber.Ctx) error {
		sid, err := strconv.ParseInt(c.Params("sid"), 10, 64)
		if err != nil {
			return utils.SendError(c, 400, "Invalid screenshot")
		}
		img, ctype, matchID, err := services.LoadEvidence(c.Context(), sid)
		if err != nil {
			return fail(c, err)
		}
		if matchID != c.Params("id") || !services.CanViewEvidence(c.Context(), matchID, middleware.GetUID(c), middleware.IsAdminCtx(c)) {
			return utils.SendError(c, 403, "Only the players in this match can view its screenshots")
		}
		c.Set("Cache-Control", "private, max-age=3600")
		c.Set("Content-Type", ctype)
		return c.Send(img)
	})
}
