package routes

import (
	"github.com/gofiber/fiber/v2"

	"betelite-go/config"
	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/services"
	"betelite-go/utils"
)

// setupAdminMoney adds per-currency money views and the manual home-country switch.
func setupAdminMoney(admin fiber.Router) {
	// What CrestArena owes players in each currency (wallets + pending withdrawals)
	// next to what came in and went out over the last 30 days.
	admin.Get("/money", func(c *fiber.Ctx) error {
		ctx := c.Context()
		type row struct {
			Currency        string `json:"currency"`
			Wallets         int64  `json:"wallets"`         // owed: total player balances
			PendingPayouts  int64  `json:"pendingPayouts"`  // owed: withdrawals not yet sent
			Memberships30d  int64  `json:"memberships30d"`  // in: plans sold
			Prizes30d       int64  `json:"prizes30d"`       // out: tournament prizes
			Cashback30d     int64  `json:"cashback30d"`     // out: renewal cashback
			PaidOut30d      int64  `json:"paidOut30d"`      // out: withdrawals sent
			CrossCurrency30 int64  `json:"crossCurrency30"` // top-ups paid in another currency (wallet amount)
		}
		out := []row{}
		for _, cur := range []string{"NGN", "GHS"} {
			r := row{Currency: cur}
			db.Pool.QueryRow(ctx, "SELECT COALESCE(SUM(balance),0) FROM users WHERE currency = $1 AND deleted_at IS NULL", cur).Scan(&r.Wallets)
			db.Pool.QueryRow(ctx, "SELECT COALESCE(SUM(amount),0) FROM withdrawals WHERE currency = $1 AND status IN ('pending','processing')", cur).Scan(&r.PendingPayouts)
			db.Pool.QueryRow(ctx, "SELECT COALESCE(SUM(price),0) FROM memberships WHERE currency = $1 AND created_at > NOW() - INTERVAL '30 days'", cur).Scan(&r.Memberships30d)
			db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(t.amount) FILTER (WHERE t.type = 'tournament_prize'),0), COALESCE(SUM(t.amount) FILTER (WHERE t.type = 'membership_cashback'),0)
				FROM transactions t JOIN users u ON u.id = t.user_id WHERE u.currency = $1 AND t.created_at > NOW() - INTERVAL '30 days'`, cur).Scan(&r.Prizes30d, &r.Cashback30d)
			db.Pool.QueryRow(ctx, "SELECT COALESCE(SUM(amount - fee),0) FROM withdrawals WHERE currency = $1 AND status = 'paid' AND updated_at > NOW() - INTERVAL '30 days'", cur).Scan(&r.PaidOut30d)
			db.Pool.QueryRow(ctx, "SELECT COALESCE(SUM(amount),0) FROM deposits WHERE currency = $1 AND status = 'paid' AND pay_currency IS NOT NULL AND paid_at > NOW() - INTERVAL '30 days'", cur).Scan(&r.CrossCurrency30)
			out = append(out, r)
		}
		rate, _ := services.NGNPerGHS(ctx)
		return utils.SendSuccess(c, fiber.Map{"currencies": out, "ngnPerGhs": rate, "marginPct": config.Cfg.FXMarginPct,
			"providers": fiber.Map{"NGN": services.ProvidersFor("NGN"), "GHS": services.ProvidersFor("GHS")}})
	})

	// Move a player's wallet to another country (after a complaint); skips the 30/90-day rules.
	admin.Post("/users/:id/currency", func(c *fiber.Ctx) error {
		var req struct {
			Currency string `json:"currency"`
		}
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid request")
		}
		balance, err := services.SwitchHomeCurrency(c.Context(), c.Params("id"), req.Currency, true)
		if err != nil {
			return fail(c, err)
		}
		services.NotifyAdmins(c.Context(), "Wallet country changed", middleware.GetEmail(c)+" moved a player's wallet to "+req.Currency)
		return utils.SendSuccess(c, fiber.Map{"balance": balance})
	})
}
