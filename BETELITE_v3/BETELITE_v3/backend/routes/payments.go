package routes

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/gofiber/fiber/v2"

	"betelite-go/config"
	"betelite-go/db"
	"betelite-go/middleware"
	"betelite-go/services"
)

func SetupPaymentRoutes(api fiber.Router) {
	payments := api.Group("/payments")

	// Get geo/currency info (Public)
	payments.Get("/geo", func(c *fiber.Ctx) error {
		ip := c.IP()
		if forwardedFor := c.Get("X-Forwarded-For"); forwardedFor != "" {
			ip = forwardedFor
		}
		
		geoInfo, err := services.DetectCurrency(ip)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to detect geo info"})
		}
		return c.JSON(fiber.Map{"success": true, "geo": geoInfo})
	})

	// Protected routes
	protected := payments.Group("/", middleware.AuthRequired())

	// Initialize Deposit
	protected.Post("/deposit", func(c *fiber.Ctx) error {
		var req struct {
			Amount int64 `json:"amount"` // in kobo/pesewas
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid payload"})
		}

		uid := middleware.GetUID(c)
		email := middleware.GetEmail(c)
		if email == "" {
			return c.Status(400).JSON(fiber.Map{"error": "User email required for payment"})
		}

		ip := c.IP()
		geoInfo, _ := services.DetectCurrency(ip)

		var secretKey string
		if geoInfo.Currency == "GHS" {
			secretKey = config.Cfg.PaystackSecretKeyGH
		} else {
			secretKey = config.Cfg.PaystackSecretKey
		}

		// Call Paystack /transaction/initialize
		payload := map[string]interface{}{
			"email":    email,
			"amount":   req.Amount,
			"currency": geoInfo.Currency,
			"metadata": map[string]string{
				"custom_fields": `[{"display_name": "User ID", "variable_name": "user_id", "value": "` + uid + `"}]`,
			},
		}

		payloadBytes, _ := json.Marshal(payload)
		httpReq, _ := http.NewRequest("POST", "https://api.paystack.co/transaction/initialize", bytes.NewBuffer(payloadBytes))
		httpReq.Header.Set("Authorization", "Bearer "+secretKey)
		httpReq.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(httpReq)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to connect to payment gateway"})
		}
		defer resp.Body.Close()

		var pResp struct {
			Status  bool   `json:"status"`
			Message string `json:"message"`
			Data    struct {
				AuthorizationUrl string `json:"authorization_url"`
				AccessCode       string `json:"access_code"`
				Reference        string `json:"reference"`
			} `json:"data"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&pResp); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to parse payment response"})
		}

		if !pResp.Status {
			return c.Status(400).JSON(fiber.Map{"error": pResp.Message})
		}

		return c.JSON(fiber.Map{"success": true, "data": pResp.Data})
	})

	// Webhook for Paystack (Public, protected by HMAC signature)
	payments.Post("/webhook", func(c *fiber.Ctx) error {
		body := c.Body()
		signature := c.Get("x-paystack-signature")

		// We need to check both NG and GH secrets because we don't know the origin from headers easily
		macNG := hmac.New(sha512.New, []byte(config.Cfg.PaystackSecretKey))
		macNG.Write(body)
		expectedMACNG := hex.EncodeToString(macNG.Sum(nil))

		macGH := hmac.New(sha512.New, []byte(config.Cfg.PaystackSecretKeyGH))
		macGH.Write(body)
		expectedMACGH := hex.EncodeToString(macGH.Sum(nil))

		if signature != expectedMACNG && signature != expectedMACGH {
			return c.SendStatus(401)
		}

		var event struct {
			Event string `json:"event"`
			Data  struct {
				Reference string `json:"reference"`
				Amount    int64  `json:"amount"`
				Status    string `json:"status"`
				Metadata  struct {
					CustomFields []struct {
						VariableName string `json:"variable_name"`
						Value        string `json:"value"`
					} `json:"custom_fields"`
				} `json:"metadata"`
			} `json:"data"`
		}

		if err := json.Unmarshal(body, &event); err != nil {
			return c.SendStatus(400)
		}

		if event.Event == "charge.success" && event.Data.Status == "success" {
			var uid string
			for _, field := range event.Data.Metadata.CustomFields {
				if field.VariableName == "user_id" {
					uid = field.Value
					break
				}
			}

			if uid != "" {
				// Process DB Update
				ctx := context.Background()
				tx, err := db.Pool.Begin(ctx)
				if err == nil {
					// Add to balance
					_, err1 := tx.Exec(ctx, "UPDATE users SET balance = balance + $1 WHERE id = $2", event.Data.Amount, uid)
					
					// Insert Transaction log
					_, err2 := tx.Exec(ctx, "INSERT INTO transactions (user_id, type, amount, ref_id) VALUES ($1, 'deposit', $2, $3)",
						uid, event.Data.Amount, event.Data.Reference)

					if err1 == nil && err2 == nil {
						tx.Commit(ctx)
					} else {
						tx.Rollback(ctx)
					}
				}
			}
		}

		return c.SendStatus(200)
	})
}
