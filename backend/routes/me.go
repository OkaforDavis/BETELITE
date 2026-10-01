package routes

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgconn"

	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/services"
	"betelite-go/utils"
)

func SetupMeRoutes(api fiber.Router) {
	api.Get("/games", func(c *fiber.Ctx) error {
		return utils.SendSuccess(c, fiber.Map{"games": services.Games()})
	})

	me := api.Group("/me", middleware.AuthRequired(), dbRequired)

	me.Get("/", func(c *fiber.Ctx) error {
		p, err := services.LoadProfile(c.Context(), middleware.GetUID(c))
		if err != nil {
			return fail(c, err)
		}
		if middleware.IsAdminCtx(c) {
			p.IsAdmin = true
		}
		return utils.SendSuccess(c, fiber.Map{"profile": p, "termsVersion": services.TermsVersion})
	})

	// Update display name / avatar. Currency can only change before any money moved.
	me.Put("/", func(c *fiber.Ctx) error {
		var req struct {
			Username  *string `json:"username"`
			AvatarURL *string `json:"avatarUrl"`
			Currency  *string `json:"currency"`
		}
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid request")
		}
		uid := middleware.GetUID(c)
		ctx := c.Context()
		if req.Username != nil {
			name := strings.TrimSpace(*req.Username)
			if n := utf8.RuneCountInString(name); n < 3 || n > 24 {
				return utils.SendError(c, 400, "Display name must be 3–24 characters")
			}
			if _, err := db.Pool.Exec(ctx, "UPDATE users SET username = $2, updated_at = NOW() WHERE id = $1", uid, name); err != nil {
				return fail(c, err)
			}
		}
		if req.AvatarURL != nil {
			url := strings.TrimSpace(*req.AvatarURL)
			if url != "" && !strings.HasPrefix(url, "https://") {
				return utils.SendError(c, 400, "Avatar must be an https image link")
			}
			if len(url) > 1000 {
				return utils.SendError(c, 400, "Avatar link is too long")
			}
			if _, err := db.Pool.Exec(ctx, "UPDATE users SET avatar_url = NULLIF($2,''), updated_at = NOW() WHERE id = $1", uid, url); err != nil {
				return fail(c, err)
			}
		}
		if req.Currency != nil {
			cur := *req.Currency
			if cur != "NGN" && cur != "GHS" {
				return utils.SendError(c, 400, "Supported currencies are NGN and GHS")
			}
			var moved bool
			db.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM transactions WHERE user_id = $1)", uid).Scan(&moved)
			if moved {
				return utils.SendError(c, 409, "Your wallet currency can't be changed after your first transaction")
			}
			country := "NG"
			if cur == "GHS" {
				country = "GH"
			}
			if _, err := db.Pool.Exec(ctx, "UPDATE users SET currency = $2, country = $3, updated_at = NOW() WHERE id = $1", uid, cur, country); err != nil {
				return fail(c, err)
			}
		}
		p, err := services.LoadProfile(ctx, uid)
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"profile": p})
	})

	// Accept Terms + Privacy and confirm age (date of birth).
	me.Post("/consent", func(c *fiber.Ctx) error {
		var req struct {
			BirthDate string `json:"birthDate"` // YYYY-MM-DD
			Accept    bool   `json:"accept"`
			Marketing bool   `json:"marketing"`
		}
		if err := c.BodyParser(&req); err != nil || !req.Accept {
			return utils.SendError(c, 400, "You must accept the Terms and Privacy Policy")
		}
		birth, err := time.Parse("2006-01-02", req.BirthDate)
		if err != nil {
			return utils.SendError(c, 400, "Please enter your date of birth")
		}
		if err := services.AcceptTerms(c.Context(), middleware.GetUID(c), birth, req.Marketing); err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{})
	})

	// Save the in-game name / ID used to verify match screenshots.
	me.Put("/games/:game", func(c *fiber.Ctx) error {
		game := services.GameByID(c.Params("game"))
		if game == nil {
			return utils.SendError(c, 404, "Unsupported game")
		}
		var req struct {
			Gamertag string `json:"gamertag"`
			InGameID string `json:"inGameId"`
		}
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid request")
		}
		tag := strings.TrimSpace(req.Gamertag)
		inGameID := strings.TrimSpace(req.InGameID)
		if n := utf8.RuneCountInString(tag); n < 2 || n > 32 {
			return utils.SendError(c, 400, "Enter your in-game name (2–32 characters)")
		}
		if len(inGameID) > 40 {
			return utils.SendError(c, 400, "In-game ID is too long")
		}
		uid := middleware.GetUID(c)
		ctx := c.Context()

		var locked bool
		db.Pool.QueryRow(ctx, "SELECT locked FROM game_profiles WHERE user_id = $1 AND game = $2", uid, game.ID).Scan(&locked)
		if locked {
			return utils.SendError(c, 409, "Your "+game.Short+" name is locked because you've played a match with it. Contact support to change it.")
		}
		_, err := db.Pool.Exec(ctx, `INSERT INTO game_profiles (user_id, game, gamertag, ingame_id) VALUES ($1,$2,$3,NULLIF($4,''))
			ON CONFLICT (user_id, game) DO UPDATE SET gamertag = $3, ingame_id = NULLIF($4,''), updated_at = NOW()`,
			uid, game.ID, tag, inGameID)
		if err != nil {
			if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
				return utils.SendError(c, 409, "That "+game.Short+" name is already registered by another player")
			}
			return fail(c, err)
		}
		list, err := services.ListGameProfiles(ctx, uid)
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"gameProfiles": list})
	})

	me.Get("/transactions", func(c *fiber.Ctx) error {
		rows, err := db.Pool.Query(c.Context(), `SELECT id, type, amount, COALESCE(ref_id,''), created_at FROM transactions
			WHERE user_id = $1 ORDER BY created_at DESC LIMIT 50`, middleware.GetUID(c))
		if err != nil {
			return fail(c, err)
		}
		defer rows.Close()
		list := []fiber.Map{}
		for rows.Next() {
			var id, amount int64
			var typ, ref string
			var at time.Time
			if rows.Scan(&id, &typ, &amount, &ref, &at) == nil {
				list = append(list, fiber.Map{"id": id, "type": typ, "amount": amount, "ref": ref, "createdAt": at})
			}
		}
		return utils.SendSuccess(c, fiber.Map{"transactions": list})
	})

	// Download everything we store about you (NDPA / GDPR data portability).
	me.Get("/export", func(c *fiber.Ctx) error {
		data, err := services.ExportUserData(c.Context(), middleware.GetUID(c))
		if err != nil {
			return fail(c, err)
		}
		c.Set("Content-Disposition", `attachment; filename="crestarena-my-data.json"`)
		return c.JSON(data)
	})

	// Delete account: anonymise personal data, then remove the Firebase login.
	me.Delete("/", func(c *fiber.Ctx) error {
		uid := middleware.GetUID(c)
		if err := services.DeleteAccount(c.Context(), uid); err != nil {
			return fail(c, err)
		}
		go middleware.DeleteFirebaseUser(context.Background(), uid)
		return utils.SendSuccess(c, fiber.Map{})
	})
}
