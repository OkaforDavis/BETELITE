package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"betelite-go/config"
)

var paystackHTTP = &http.Client{Timeout: 20 * time.Second}

// PaystackSecret returns the secret key for a currency (Ghana may use a
// separate Paystack account).
func PaystackSecret(currency string) string {
	if currency == "GHS" && config.Cfg.PaystackSecretKeyGH != "" {
		return config.Cfg.PaystackSecretKeyGH
	}
	return config.Cfg.PaystackSecretKey
}

// paystack calls the Paystack API and decodes the "data" field into out.
func paystack(ctx context.Context, currency, method, path string, body any, out any) error {
	secret := PaystackSecret(currency)
	if secret == "" {
		return fmt.Errorf("paystack is not configured")
	}
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://api.paystack.co"+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set("Content-Type", "application/json")
	resp, err := paystackHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var env struct {
		Status  bool            `json:"status"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("paystack %s: bad response (%d)", path, resp.StatusCode)
	}
	if !env.Status {
		return &UserError{Status: 400, Msg: env.Message}
	}
	if out != nil && len(env.Data) > 0 {
		return json.Unmarshal(env.Data, out)
	}
	return nil
}

// PaystackInit starts a checkout and returns the authorization URL.
func PaystackInit(ctx context.Context, email string, amount int64, currency, reference, uid, callbackURL string) (string, error) {
	var data struct {
		AuthorizationURL string `json:"authorization_url"`
	}
	err := paystack(ctx, currency, "POST", "/transaction/initialize", map[string]any{
		"email":        email,
		"amount":       amount,
		"currency":     currency,
		"reference":    reference,
		"callback_url": callbackURL,
		"metadata":     map[string]any{"user_id": uid},
	}, &data)
	return data.AuthorizationURL, err
}

// PaystackTxn is the part of a verified transaction we rely on.
type PaystackTxn struct {
	Status    string `json:"status"`
	Reference string `json:"reference"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
}

// PaystackVerify fetches the authoritative state of a transaction.
func PaystackVerify(ctx context.Context, currency, reference string) (*PaystackTxn, error) {
	var t PaystackTxn
	err := paystack(ctx, currency, "GET", "/transaction/verify/"+url.PathEscape(reference), nil, &t)
	return &t, err
}

// Bank is a payout destination offered to users.
type Bank struct {
	Name string `json:"name"`
	Code string `json:"code"`
	Type string `json:"type"`
}

// PaystackBanks lists banks (and mobile money providers in Ghana).
func PaystackBanks(ctx context.Context, currency string) ([]Bank, error) {
	country := "nigeria"
	if currency == "GHS" {
		country = "ghana"
	}
	var banks []Bank
	err := paystack(ctx, currency, "GET", "/bank?perPage=200&country="+country+"&currency="+currency, nil, &banks)
	return banks, err
}

// PaystackResolve returns the account holder's name (Nigeria only).
func PaystackResolve(ctx context.Context, accountNumber, bankCode string) (string, error) {
	var data struct {
		AccountName string `json:"account_name"`
	}
	err := paystack(ctx, "NGN", "GET", "/bank/resolve?account_number="+url.QueryEscape(accountNumber)+"&bank_code="+url.QueryEscape(bankCode), nil, &data)
	return data.AccountName, err
}

// PaystackPayout creates a recipient and starts a transfer; returns the
// recipient and transfer codes.
func PaystackPayout(ctx context.Context, currency, bankType, name, accountNumber, bankCode string, amount int64, reference, reason string) (string, string, error) {
	recipientType := "nuban"
	switch {
	case currency == "GHS" && bankType == "mobile_money":
		recipientType = "mobile_money"
	case currency == "GHS":
		recipientType = "ghipss"
	}
	var rec struct {
		RecipientCode string `json:"recipient_code"`
	}
	if err := paystack(ctx, currency, "POST", "/transferrecipient", map[string]any{
		"type": recipientType, "name": name, "account_number": accountNumber, "bank_code": bankCode, "currency": currency,
	}, &rec); err != nil {
		return "", "", err
	}
	var tr struct {
		TransferCode string `json:"transfer_code"`
	}
	err := paystack(ctx, currency, "POST", "/transfer", map[string]any{
		"source": "balance", "amount": amount, "recipient": rec.RecipientCode, "reference": reference, "reason": reason,
	}, &tr)
	return rec.RecipientCode, tr.TransferCode, err
}
