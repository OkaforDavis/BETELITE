package routes

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"betelite-go/config"
	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/services"
	"betelite-go/utils"
)

// Wallet limits in minor units (kobo / pesewas).
var (
	minDeposit    = map[string]int64{"NGN": 100_00, "GHS": 10_00}
	maxDeposit    = map[string]int64{"NGN": 1_000_000_00, "GHS": 50_000_00}
	minWithdrawal = map[string]int64{"NGN": 1_000_00, "GHS": 50_00}
)

var accountNumberRe = regexp.MustCompile(`^[0-9]{6,20}$`)

func SetupPaymentRoutes(api fiber.Router) {
	// Public: geo hint for picking a currency at sign-up.
	api.Get("/payments/geo", func(c *fiber.Ctx) error {
		geo, _ := services.DetectCurrency(c.IP())
		return utils.SendSuccess(c, fiber.Map{"currency": geo.Currency, "country": geo.CountryCode})
	})

	// Paystack webhook (HMAC-signed).
	api.Post("/payments/webhook", paystackWebhook)

	wallet := api.Group("/wallet", middleware.AuthRequired(), dbRequired)

	wallet.Get("/", func(c *fiber.Ctx) error {
		uid := middleware.GetUID(c)
		var balance, held int64
		var currency string
		ctx := c.Context()
		if err := db.Pool.QueryRow(ctx, "SELECT balance, COALESCE(currency,'NGN') FROM users WHERE id = $1", uid).Scan(&balance, &currency); err != nil {
			return fail(c, err)
		}
		db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount),0) FROM escrow WHERE status IN ('waiting','held') AND (creator_id = $1 OR acceptor_id = $1)`, uid).Scan(&held)
		return utils.SendSuccess(c, fiber.Map{
			"balance": balance, "inPlay": held, "currency": currency,
			"limits": fiber.Map{"minDeposit": minDeposit[currency], "maxDeposit": maxDeposit[currency], "minWithdrawal": minWithdrawal[currency]},
		})
	})

	// Start a deposit. The wallet is credited only after Paystack confirms it.
	wallet.Post("/deposit", func(c *fiber.Ctx) error {
		var req struct {
			Amount int64 `json:"amount"`
		}
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid request")
		}
		uid, email := middleware.GetUID(c), middleware.GetEmail(c)
		ctx := c.Context()
		if err := services.RequireMoneyAccess(ctx, uid); err != nil {
			return fail(c, err)
		}
		if email == "" {
			return utils.SendError(c, 400, "Add an email address to your account to make deposits")
		}
		var currency string
		db.Pool.QueryRow(ctx, "SELECT COALESCE(currency,'NGN') FROM users WHERE id = $1", uid).Scan(&currency)
		if req.Amount < minDeposit[currency] || req.Amount > maxDeposit[currency] {
			return utils.SendError(c, 400, fmt.Sprintf("Deposits must be between %s and %s",
				services.FormatMoney(minDeposit[currency], currency), services.FormatMoney(maxDeposit[currency], currency)))
		}
		ref := "dep_" + utils.GenerateBaseID()
		if _, err := db.Pool.Exec(ctx, "INSERT INTO deposits (reference, user_id, amount, currency) VALUES ($1,$2,$3,$4)", ref, uid, req.Amount, currency); err != nil {
			return fail(c, err)
		}
		authURL, accessCode, err := services.PaystackInit(ctx, email, req.Amount, currency, ref, uid, appURL(c)+"/?deposit="+ref)
		if err != nil {
			return fail(c, err)
		}
		publicKey := config.Cfg.PaystackPublicKey
		if currency == "GHS" && config.Cfg.PaystackPublicKeyGH != "" {
			publicKey = config.Cfg.PaystackPublicKeyGH
		}
		return utils.SendSuccess(c, fiber.Map{
			"reference": ref, "authorizationUrl": authURL, "accessCode": accessCode, "publicKey": publicKey,
			"email": email, "amount": req.Amount, "currency": currency,
		})
	})

	// Called by the app after checkout; safe to call repeatedly.
	wallet.Post("/deposit/verify", func(c *fiber.Ctx) error {
		var req struct {
			Reference string `json:"reference"`
		}
		if err := c.BodyParser(&req); err != nil || req.Reference == "" {
			return utils.SendError(c, 400, "Missing reference")
		}
		var owner string
		db.Pool.QueryRow(c.Context(), "SELECT user_id FROM deposits WHERE reference = $1", req.Reference).Scan(&owner)
		if owner != middleware.GetUID(c) {
			return utils.SendError(c, 404, "Deposit not found")
		}
		status, err := services.CreditDeposit(c.Context(), req.Reference)
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"status": status})
	})

	wallet.Get("/banks", func(c *fiber.Ctx) error {
		var currency string
		db.Pool.QueryRow(c.Context(), "SELECT COALESCE(currency,'NGN') FROM users WHERE id = $1", middleware.GetUID(c)).Scan(&currency)
		banks, err := services.PaystackBanks(c.Context(), currency)
		if err != nil {
			return fail(c, err)
		}
		return utils.SendSuccess(c, fiber.Map{"banks": banks})
	})

	// Look up the account holder's name before withdrawing (Nigeria).
	wallet.Post("/resolve", func(c *fiber.Ctx) error {
		var req struct {
			AccountNumber string `json:"accountNumber"`
			BankCode      string `json:"bankCode"`
		}
		if err := c.BodyParser(&req); err != nil || !accountNumberRe.MatchString(req.AccountNumber) || req.BankCode == "" {
			return utils.SendError(c, 400, "Enter a valid account number and bank")
		}
		name, err := services.PaystackResolve(c.Context(), req.AccountNumber, req.BankCode)
		if err != nil {
			return utils.SendError(c, 400, "We couldn't find that account. Check the number and bank.")
		}
		return utils.SendSuccess(c, fiber.Map{"accountName": name})
	})

	// Request a withdrawal. Money is held immediately; an admin approves the payout.
	wallet.Post("/withdraw", func(c *fiber.Ctx) error {
		var req struct {
			Amount        int64  `json:"amount"`
			BankCode      string `json:"bankCode"`
			BankName      string `json:"bankName"`
			BankType      string `json:"bankType"`
			AccountNumber string `json:"accountNumber"`
			AccountName   string `json:"accountName"`
		}
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "Invalid request")
		}
		uid := middleware.GetUID(c)
		ctx := c.Context()
		if err := services.RequireMoneyAccess(ctx, uid); err != nil {
			return fail(c, err)
		}
		if !accountNumberRe.MatchString(req.AccountNumber) || req.BankCode == "" {
			return utils.SendError(c, 400, "Enter a valid account number and bank")
		}
		var currency string
		db.Pool.QueryRow(ctx, "SELECT COALESCE(currency,'NGN') FROM users WHERE id = $1", uid).Scan(&currency)
		if req.Amount < minWithdrawal[currency] {
			return utils.SendError(c, 400, "The minimum withdrawal is "+services.FormatMoney(minWithdrawal[currency], currency))
		}
		if currency == "NGN" {
			name, err := services.PaystackResolve(ctx, req.AccountNumber, req.BankCode)
			if err != nil {
				return utils.SendError(c, 400, "We couldn't verify that bank account")
			}
			req.AccountName = name
		}
		if strings.TrimSpace(req.AccountName) == "" {
			return utils.SendError(c, 400, "Enter the account holder's name")
		}

		id := "wd_" + utils.GenerateBaseID()
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return fail(c, err)
		}
		defer tx.Rollback(ctx)
		if err := services.AdjustBalance(ctx, tx, uid, -req.Amount, "withdrawal_hold", id); err != nil {
			if errors.Is(err, services.ErrInsufficientFunds) {
				return utils.SendError(c, 400, "Insufficient balance")
			}
			return fail(c, err)
		}
		_, err = tx.Exec(ctx, `INSERT INTO withdrawals (id, user_id, amount, currency, bank_code, bank_name, account_number, account_name, note)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, uid, req.Amount, currency, req.BankCode, req.BankName, req.AccountNumber, req.AccountName, req.BankType)
		if err != nil {
			return fail(c, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fail(c, err)
		}
		services.NotifyAdmins(ctx, "Withdrawal request", fmt.Sprintf("%s to %s (%s) is waiting for approval.",
			services.FormatMoney(req.Amount, currency), req.AccountName, req.BankName))
		return utils.SendSuccess(c, fiber.Map{"withdrawalId": id})
	})

	wallet.Get("/withdrawals", func(c *fiber.Ctx) error {
		rows, err := db.Pool.Query(c.Context(), `SELECT id, amount, currency, COALESCE(bank_name,''), account_number, status, created_at
			FROM withdrawals WHERE user_id = $1 ORDER BY created_at DESC LIMIT 20`, middleware.GetUID(c))
		if err != nil {
			return fail(c, err)
		}
		defer rows.Close()
		list := []fiber.Map{}
		for rows.Next() {
			var id, cur, bank, acct, status string
			var amount int64
			var at time.Time
			if rows.Scan(&id, &amount, &cur, &bank, &acct, &status, &at) == nil {
				if len(acct) > 4 {
					acct = "••••" + acct[len(acct)-4:]
				}
				list = append(list, fiber.Map{"id": id, "amount": amount, "currency": cur, "bankName": bank, "account": acct, "status": status, "createdAt": at})
			}
		}
		return utils.SendSuccess(c, fiber.Map{"withdrawals": list})
	})
}

func paystackWebhook(c *fiber.Ctx) error {
	body := c.Body()
	sig := c.Get("x-paystack-signature")
	valid := false
	for _, key := range []string{config.Cfg.PaystackSecretKey, config.Cfg.PaystackSecretKeyGH} {
		if key == "" {
			continue
		}
		mac := hmac.New(sha512.New, []byte(key))
		mac.Write(body)
		if hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(sig)) {
			valid = true
		}
	}
	if !valid {
		return c.SendStatus(401)
	}

	var ev struct {
		Event string `json:"event"`
		Data  struct {
			Reference string `json:"reference"`
			Reason    string `json:"reason"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &ev); err != nil {
		return c.SendStatus(400)
	}
	if db.Pool == nil {
		return c.SendStatus(503)
	}
	ctx := context.Background()

	switch ev.Event {
	case "charge.success":
		if _, err := services.CreditDeposit(ctx, ev.Data.Reference); err != nil {
			log.Printf("[PAYMENTS] webhook credit %s: %v", ev.Data.Reference, err)
			return c.SendStatus(500) // Paystack retries
		}
	case "transfer.success":
		var uid, cur string
		var amount int64
		err := db.Pool.QueryRow(ctx, "UPDATE withdrawals SET status = 'paid', updated_at = NOW() WHERE id = $1 AND status = 'processing' RETURNING user_id, amount, currency",
			ev.Data.Reference).Scan(&uid, &amount, &cur)
		if err == nil {
			services.Notify(uid, "withdrawal", "Withdrawal sent", services.FormatMoney(amount, cur)+" has been sent to your bank account.", "/wallet", nil)
		}
	case "transfer.failed", "transfer.reversed":
		if err := services.RefundWithdrawal(ctx, ev.Data.Reference, "Bank transfer failed: "+ev.Data.Reason, "processing"); err != nil {
			log.Printf("[PAYMENTS] refund withdrawal %s: %v", ev.Data.Reference, err)
			return c.SendStatus(500)
		}
	}
	return c.SendStatus(200)
}

// appURL is the public base URL used for Paystack redirects.
func appURL(c *fiber.Ctx) string {
	if config.Cfg.AppURL != "" {
		return strings.TrimRight(config.Cfg.AppURL, "/")
	}
	return c.BaseURL()
}
