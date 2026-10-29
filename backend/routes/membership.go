package routes

import (
	"github.com/gofiber/fiber/v2"

	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/services"
	"betelite-go/utils"
)

type tierView struct {
	services.Tier
	Price int64 `json:"price"` // in the viewer's currency; 0 = free
}

func SetupMembershipRoutes(api fiber.Router) {
	mem := api.Group("/membership", middleware.AuthRequired(), dbRequired)

	// Plans in my currency, my current plan and how many entries I have left.
	mem.Get("/", func(c *fiber.Ctx) error {
		uid := middleware.GetUID(c)
		ctx := c.Context()
		var currency string
		var balance int64
		if err := db.Pool.QueryRow(ctx, "SELECT COALESCE(currency,'NGN'), balance FROM users WHERE id = $1", uid).Scan(&currency, &balance); err != nil {
			return fail(c, err)
		}
		usage, err := services.EntryUsage(ctx, db.Pool, uid)
		if err != nil {
			return fail(c, err)
		}
		tiers := make([]tierView, 0, len(services.Tiers))
		for _, t := range services.Tiers {
			tiers = append(tiers, tierView{Tier: t, Price: t.Prices[currency]})
		}
		return utils.SendSuccess(c, fiber.Map{"tiers": tiers, "currency": currency, "balance": balance, "usage": usage, "feeRule": feeRuleFor(currency), "minDeposit": minDeposit[currency], "payCurrencies": services.PayCurrencies(),
			"renewWindowDays": int(services.MembershipRenewWindow.Hours() / 24)})
	})

	// Buy or renew a plan. Paid from the wallet when it covers the price;
	// otherwise a checkout for the difference starts the plan once paid.
	mem.Post("/buy", func(c *fiber.Ctx) error {
		var req struct {
			Tier        string `json:"tier"`
			PayCurrency string `json:"payCurrency"`
		}
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid request")
		}
		uid := middleware.GetUID(c)
		ctx := c.Context()
		if err := services.RequireMoneyAccess(ctx, uid); err != nil {
			return fail(c, err)
		}
		tier := services.TierByID(req.Tier)
		if tier == nil || len(tier.Prices) == 0 {
			return utils.SendError(c, 400, "Choose a membership plan")
		}
		var currency string
		var balance int64
		db.Pool.QueryRow(ctx, "SELECT COALESCE(currency,'NGN'), balance FROM users WHERE id = $1", uid).Scan(&currency, &balance)
		price := tier.Prices[currency]

		if balance >= price {
			m, err := services.BuyMembership(ctx, uid, tier.ID)
			if err != nil {
				return fail(c, err)
			}
			return utils.SendSuccess(c, fiber.Map{"membership": m})
		}

		email := middleware.GetEmail(c)
		if email == "" {
			return utils.SendError(c, 400, "Add an email address to your account to pay")
		}
		// Check the plan can be bought before taking money for it.
		if err := services.CanBuyMembership(ctx, uid, tier.ID); err != nil {
			return fail(c, err)
		}
		amount := max(price-balance, minDeposit[currency])
		checkout, err := startDeposit(c, uid, email, amount, currency, req.PayCurrency, "membership:"+tier.ID)
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"checkout": checkout})
	})
}
