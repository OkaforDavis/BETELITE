package routes

import (
	"github.com/gofiber/fiber/v2"

	"betelite-go/middleware"
	"betelite-go/services"
	"betelite-go/utils"
)

func SetupReferralRoutes(api fiber.Router) {
	referrals := api.Group("/referrals", middleware.AuthRequired(), dbRequired)

	// My invite code, link stats and whether I can still add a code.
	referrals.Get("/", func(c *fiber.Ctx) error {
		info, err := services.GetReferralInfo(c.Context(), middleware.GetUID(c))
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{
			"code": info.Code, "referrals": info.Joined, "rewarded": info.Rewarded, "earned": info.Earned,
			"currency": info.Currency, "inviterReward": info.InviterGets, "friendReward": info.FriendGets,
			"referredBy": info.ReferredBy, "canClaim": info.CanClaim,
		})
	})

	// Link a new account to the friend who invited it (from an invite link or
	// a typed code). Rewards are paid after the first paid match.
	referrals.Post("/claim", func(c *fiber.Ctx) error {
		var req struct {
			Code string `json:"code"`
		}
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid request")
		}
		name, err := services.ClaimReferral(c.Context(), middleware.GetUID(c), req.Code)
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"referredBy": name})
	})
}
