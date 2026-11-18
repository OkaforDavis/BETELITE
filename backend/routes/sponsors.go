package routes

import (
	"io"
	"mime/multipart"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/services"
	"betelite-go/utils"
)

// SetupSponsorRoutes: hosts add a sponsor to their cup, admins approve it,
// and approved pictures/videos are served publicly (img/video tags can't send
// a login token).
func SetupSponsorRoutes(api fiber.Router) {
	api.Get("/sponsor-media/:mid", dbRequired, func(c *fiber.Ctx) error {
		return serveSponsorMedia(c, false)
	})

	host := api.Group("/tournaments/:id/sponsor", middleware.AuthRequired(), dbRequired)
	host.Post("/", func(c *fiber.Ctx) error {
		form, err := c.MultipartForm()
		if err != nil {
			return utils.SendError(c, 400, "Add at least one picture or a short video")
		}
		var images []services.SponsorUpload
		for _, fh := range form.File["images"] {
			data, err := readUpload(fh, services.SponsorImageMaxLen)
			if err != nil {
				return utils.SendError(c, 400, "Each picture must be under 5 MB")
			}
			images = append(images, services.SponsorUpload{Data: data})
		}
		var video *services.SponsorUpload
		if v := form.File["video"]; len(v) > 0 {
			data, err := readUpload(v[0], services.SponsorVideoMaxLen)
			if err != nil {
				return utils.SendError(c, 400, "The video is too large (max 5 MB). Use a short, low-quality clip.")
			}
			video = &services.SponsorUpload{Data: data}
		}
		if err := services.SetSponsor(c.Context(), middleware.GetUID(c), c.Params("id"), c.FormValue("name"), c.FormValue("link"), images, video); err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"sponsor": services.GetSponsor(c.Context(), c.Params("id"), true)})
	})
	host.Delete("/", func(c *fiber.Ctx) error {
		var hostID string
		db.Pool.QueryRow(c.Context(), "SELECT COALESCE(host_id,'') FROM tournaments WHERE id = $1", c.Params("id")).Scan(&hostID)
		if hostID != middleware.GetUID(c) && !middleware.IsAdminCtx(c) {
			return utils.SendError(c, 403, "Only the cup's host can remove the sponsor")
		}
		if err := services.RemoveSponsor(c.Context(), c.Params("id")); err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{})
	})

	admin := api.Group("/admin/sponsors", middleware.AuthRequired(), middleware.AdminRequired(), dbRequired)
	admin.Get("/", func(c *fiber.Ctx) error {
		list, err := services.ListSponsors(c.Context())
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"sponsors": list})
	})
	admin.Get("/media/:mid", func(c *fiber.Ctx) error { return serveSponsorMedia(c, true) })
	admin.Post("/:tid/review", func(c *fiber.Ctx) error {
		var req struct {
			Approve bool   `json:"approve"`
			Reason  string `json:"reason"`
		}
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid request")
		}
		if err := services.ReviewSponsor(c.Context(), c.Params("tid"), middleware.GetUID(c), req.Approve, req.Reason); err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{})
	})
}

func serveSponsorMedia(c *fiber.Ctx, privileged bool) error {
	id, err := strconv.ParseInt(c.Params("mid"), 10, 64)
	if err != nil {
		return utils.SendError(c, 404, "Not found")
	}
	data, ctype, err := services.LoadSponsorMedia(c.Context(), id, privileged)
	if err != nil {
		return fail(c, err)
	}
	c.Set("Content-Type", ctype)
	c.Set("X-Content-Type-Options", "nosniff")
	if privileged {
		c.Set("Cache-Control", "private, no-store")
	} else {
		c.Set("Cache-Control", "public, max-age=3600")
	}
	// Phones (iPhone especially) only play videos served in byte ranges.
	c.Set("Accept-Ranges", "bytes")
	if start, end, ok := byteRange(c.Get("Range"), len(data)); ok {
		c.Set("Content-Range", "bytes "+strconv.Itoa(start)+"-"+strconv.Itoa(end)+"/"+strconv.Itoa(len(data)))
		return c.Status(fiber.StatusPartialContent).Send(data[start : end+1])
	}
	return c.Send(data)
}

// byteRange parses a single "bytes=start-end" Range header.
func byteRange(h string, size int) (start, end int, ok bool) {
	spec, found := strings.CutPrefix(h, "bytes=")
	if !found || size == 0 || strings.Contains(spec, ",") {
		return 0, 0, false
	}
	a, b, _ := strings.Cut(spec, "-")
	end = size - 1
	switch {
	case a == "": // last N bytes
		n, err := strconv.Atoi(b)
		if err != nil || n <= 0 {
			return 0, 0, false
		}
		start = max(size-n, 0)
	default:
		var err error
		if start, err = strconv.Atoi(a); err != nil || start >= size {
			return 0, 0, false
		}
		if b != "" {
			if e, err := strconv.Atoi(b); err == nil && e < end {
				end = e
			}
		}
	}
	return start, end, start <= end
}

func readUpload(fh *multipart.FileHeader, limit int64) ([]byte, error) {
	if fh.Size > limit {
		return nil, io.ErrShortBuffer
	}
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, io.ErrShortBuffer
	}
	return data, nil
}
