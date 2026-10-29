package routes

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"

	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/services"
	"betelite-go/utils"
)

// Complaint categories players can choose from.
var supportCategories = map[string]string{
	"match":      "Match result or dispute",
	"tournament": "Tournament",
	"payment":    "Payment or membership",
	"withdrawal": "Withdrawal",
	"account":    "Account or sign-in",
	"location":   "Country or currency",
	"report":     "Report a player",
	"other":      "Something else",
}

type ticketView struct {
	ID         string     `json:"id"`
	UserID     string     `json:"userId,omitempty"`
	Username   string     `json:"username,omitempty"`
	Email      string     `json:"email,omitempty"`
	Category   string     `json:"category"`
	Subject    string     `json:"subject"`
	Message    string     `json:"message"`
	RefID      string     `json:"refId,omitempty"`
	Status     string     `json:"status"`
	AdminReply string     `json:"adminReply,omitempty"`
	RepliedAt  *time.Time `json:"repliedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

func SetupSupportRoutes(api fiber.Router) {
	sup := api.Group("/support", middleware.AuthRequired(), dbRequired)

	sup.Get("/categories", func(c *fiber.Ctx) error {
		return utils.SendSuccess(c, fiber.Map{"categories": supportCategories})
	})

	// My complaints and the admin's replies.
	sup.Get("/", func(c *fiber.Ctx) error {
		rows, err := db.Pool.Query(c.Context(), `SELECT id, category, subject, message, COALESCE(ref_id,''), status,
			COALESCE(admin_reply,''), replied_at, created_at FROM support_tickets WHERE user_id = $1 ORDER BY created_at DESC LIMIT 30`, middleware.GetUID(c))
		if err != nil {
			return fail(c, err)
		}
		defer rows.Close()
		list := []ticketView{}
		for rows.Next() {
			var t ticketView
			if rows.Scan(&t.ID, &t.Category, &t.Subject, &t.Message, &t.RefID, &t.Status, &t.AdminReply, &t.RepliedAt, &t.CreatedAt) == nil {
				list = append(list, t)
			}
		}
		return utils.SendSuccess(c, fiber.Map{"tickets": list})
	})

	// Send a complaint to the admins.
	sup.Post("/", func(c *fiber.Ctx) error {
		var req struct {
			Category string `json:"category"`
			Subject  string `json:"subject"`
			Message  string `json:"message"`
			RefID    string `json:"refId"` // optional match/tournament/withdrawal ID
		}
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid request")
		}
		req.Subject, req.Message, req.RefID = strings.TrimSpace(req.Subject), strings.TrimSpace(req.Message), strings.TrimSpace(req.RefID)
		if _, ok := supportCategories[req.Category]; !ok {
			return utils.SendError(c, 400, "Choose what your message is about")
		}
		if req.Subject == "" || utf8.RuneCountInString(req.Subject) > 100 {
			return utils.SendError(c, 400, "Add a short subject (up to 100 characters)")
		}
		if utf8.RuneCountInString(req.Message) < 10 || utf8.RuneCountInString(req.Message) > 2000 {
			return utils.SendError(c, 400, "Describe the problem in 10 to 2,000 characters")
		}
		if utf8.RuneCountInString(req.RefID) > 64 {
			req.RefID = ""
		}
		uid := middleware.GetUID(c)
		ctx := c.Context()
		var open int
		db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM support_tickets WHERE user_id = $1 AND status = 'open'", uid).Scan(&open)
		if open >= 5 {
			return utils.SendError(c, 429, "You have 5 open messages. Please wait for a reply before sending more.")
		}
		id := "tk_" + utils.GenerateBaseID()
		if _, err := db.Pool.Exec(ctx, `INSERT INTO support_tickets (id, user_id, category, subject, message, ref_id)
			VALUES ($1,$2,$3,$4,$5,NULLIF($6,''))`, id, uid, req.Category, req.Subject, req.Message, req.RefID); err != nil {
			return fail(c, err)
		}
		services.NotifyAdmins(ctx, "New complaint: "+supportCategories[req.Category],
			fmt.Sprintf("%s: %s", services.Username(ctx, uid), req.Subject))
		return utils.SendSuccess(c, fiber.Map{"ticketId": id})
	})
}

// setupAdminSupport adds the admin side: list complaints and reply.
func setupAdminSupport(admin fiber.Router) {
	admin.Get("/support", func(c *fiber.Ctx) error {
		status := c.Query("status", "open")
		rows, err := db.Pool.Query(c.Context(), `SELECT t.id, t.user_id, COALESCE(u.username,''), COALESCE(u.email,''), t.category, t.subject, t.message,
			COALESCE(t.ref_id,''), t.status, COALESCE(t.admin_reply,''), t.replied_at, t.created_at
			FROM support_tickets t LEFT JOIN users u ON u.id = t.user_id WHERE t.status = $1 ORDER BY t.created_at LIMIT 100`, status)
		if err != nil {
			return fail(c, err)
		}
		defer rows.Close()
		list := []ticketView{}
		for rows.Next() {
			var t ticketView
			if rows.Scan(&t.ID, &t.UserID, &t.Username, &t.Email, &t.Category, &t.Subject, &t.Message, &t.RefID, &t.Status,
				&t.AdminReply, &t.RepliedAt, &t.CreatedAt) == nil {
				list = append(list, t)
			}
		}
		return utils.SendSuccess(c, fiber.Map{"tickets": list, "categories": supportCategories})
	})

	// Reply to a complaint; the player is notified. close=true marks it resolved.
	admin.Post("/support/:id/reply", func(c *fiber.Ctx) error {
		var req struct {
			Reply string `json:"reply"`
			Close bool   `json:"close"`
		}
		if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Reply) == "" {
			return utils.SendError(c, 400, "Write a reply")
		}
		status := "answered"
		if req.Close {
			status = "closed"
		}
		ctx := c.Context()
		var uid, subject string
		err := db.Pool.QueryRow(ctx, `UPDATE support_tickets SET admin_reply = $2, status = $3, replied_by = $4, replied_at = NOW()
			WHERE id = $1 RETURNING user_id, subject`, c.Params("id"), strings.TrimSpace(req.Reply), status, middleware.GetUID(c)).Scan(&uid, &subject)
		if err != nil {
			return utils.SendError(c, 404, "Message not found")
		}
		services.Notify(uid, "support", "Reply from CrestArena support", subject+": tap to read the reply.", "/support", nil)
		return utils.SendSuccess(c, fiber.Map{})
	})
}
