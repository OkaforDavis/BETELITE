package routes

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"betelite-go/config"
)

// A Paystack approval call with a wrong signature must be refused before
// anything else is looked at (no database needed for that path).
func TestPaystackApproveRejectsBadSignature(t *testing.T) {
	config.Cfg.PaystackSecretKey = "sk_test_approve"
	app := fiber.New()
	app.Post("/approve", paystackApprove)

	body := `{"reference":"wd_x","amount":100000,"currency":"NGN"}`
	req := httptest.NewRequest("POST", "/approve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-paystack-signature", "deadbeef")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("bad signature: status %d, want 400", resp.StatusCode)
	}

	// A correct signature gets past the signature check (then fails on the
	// missing database, still a refusal, never a 200).
	mac := hmac.New(sha512.New, []byte("sk_test_approve"))
	mac.Write([]byte(body))
	req = httptest.NewRequest("POST", "/approve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-paystack-signature", hex.EncodeToString(mac.Sum(nil)))
	resp, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode == 200 {
		t.Fatal("approved a transfer with no matching withdrawal")
	}
}
