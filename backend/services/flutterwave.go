package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"betelite-go/config"
)

// Flutterwave v3 API: checkout, verification, payouts. Amounts in the API are
// in major units (₦12.50 = 12.5); everything else in the app uses minor units.

var flutterwaveHTTP = &http.Client{Timeout: 25 * time.Second}

// flutterwaveBaseURL is a variable so tests can point it at a fake server.
var flutterwaveBaseURL = "https://api.flutterwave.com/v3"

// FlutterwaveReady reports whether Flutterwave keys are configured.
func FlutterwaveReady() bool { return config.Cfg.FlutterwaveSecretKey != "" }

func flutterwave(ctx context.Context, method, path string, body any, out any) error {
	if !FlutterwaveReady() {
		return fmt.Errorf("flutterwave is not configured")
	}
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, flutterwaveBaseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+config.Cfg.FlutterwaveSecretKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := flutterwaveHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var env struct {
		Status  string          `json:"status"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("flutterwave %s: bad response (%d)", path, resp.StatusCode)
	}
	if env.Status != "success" {
		return &UserError{Status: 400, Msg: env.Message}
	}
	if out != nil && len(env.Data) > 0 {
		return json.Unmarshal(env.Data, out)
	}
	return nil
}

func toMajor(minor int64) float64 { return float64(minor) / 100 }
func toMinor(major float64) int64 { return int64(math.Round(major * 100)) }

// flutterwavePaymentOptions are the methods offered at checkout per currency.
func flutterwavePaymentOptions(currency string) string {
	if currency == "GHS" {
		return "card,mobilemoneyghana"
	}
	return "card,banktransfer,ussd"
}

// FlutterwaveInit starts a hosted checkout and returns its link.
func FlutterwaveInit(ctx context.Context, email, name string, amount int64, currency, reference, uid, redirectURL string) (string, error) {
	var data struct {
		Link string `json:"link"`
	}
	err := flutterwave(ctx, "POST", "/payments", map[string]any{
		"tx_ref":          reference,
		"amount":          toMajor(amount),
		"currency":        currency,
		"redirect_url":    redirectURL,
		"payment_options": flutterwavePaymentOptions(currency),
		"customer":        map[string]any{"email": email, "name": name},
		"customizations":  map[string]any{"title": "CrestArena", "description": "CrestArena wallet and membership"},
		"meta":            map[string]any{"user_id": uid},
	}, &data)
	return data.Link, err
}

// FlutterwaveVerify fetches the authoritative state of a payment by our reference.
// Status is normalised to Paystack's words: success, failed, abandoned (not finished).
func FlutterwaveVerify(ctx context.Context, reference string) (*PaystackTxn, error) {
	var d struct {
		Status   string  `json:"status"`
		TxRef    string  `json:"tx_ref"`
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
	}
	if err := flutterwave(ctx, "GET", "/transactions/verify_by_reference?tx_ref="+url.QueryEscape(reference), nil, &d); err != nil {
		if ue, ok := err.(*UserError); ok && strings.Contains(strings.ToLower(ue.Msg), "no transaction") {
			return &PaystackTxn{Status: "abandoned", Reference: reference}, nil
		}
		return nil, err
	}
	status := strings.ToLower(d.Status)
	switch status {
	case "successful":
		status = "success"
	case "failed", "cancelled":
		status = "failed"
	default:
		status = "abandoned"
	}
	return &PaystackTxn{Status: status, Reference: d.TxRef, Amount: toMinor(d.Amount), Currency: d.Currency}, nil
}

// GhanaMobileMoney are the networks Flutterwave pays out to in Ghana.
var GhanaMobileMoney = []Bank{
	{Name: "MTN Mobile Money", Code: "MTN", Type: "mobile_money"},
	{Name: "Telecel Cash", Code: "VODAFONE", Type: "mobile_money"},
	{Name: "AirtelTigo Money", Code: "TIGO", Type: "mobile_money"},
}

// FlutterwaveBanks lists payout destinations (Ghana: mobile money first, then banks).
func FlutterwaveBanks(ctx context.Context, currency string) ([]Bank, error) {
	country := "NG"
	if currency == "GHS" {
		country = "GH"
	}
	var raw []struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	if err := flutterwave(ctx, "GET", "/banks/"+country, nil, &raw); err != nil {
		return nil, err
	}
	var out []Bank
	if currency == "GHS" {
		out = append(out, GhanaMobileMoney...)
	}
	for _, b := range raw {
		out = append(out, Bank{Name: b.Name, Code: b.Code, Type: "bank"})
	}
	return out, nil
}

// FlutterwaveResolve returns the account holder's name (Nigerian banks).
func FlutterwaveResolve(ctx context.Context, accountNumber, bankCode string) (string, error) {
	var d struct {
		AccountName string `json:"account_name"`
	}
	err := flutterwave(ctx, "POST", "/accounts/resolve", map[string]any{"account_number": accountNumber, "account_bank": bankCode}, &d)
	return d.AccountName, err
}

// FlutterwavePayout sends a transfer; the result arrives by webhook
// (transfer.completed) with our reference. Returns Flutterwave's transfer ID.
func FlutterwavePayout(ctx context.Context, currency, name, accountNumber, bankCode string, amount int64, reference, narration string) (string, error) {
	var d struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	err := flutterwave(ctx, "POST", "/transfers", map[string]any{
		"account_bank":     bankCode,
		"account_number":   accountNumber,
		"amount":           toMajor(amount),
		"currency":         currency,
		"debit_currency":   currency,
		"beneficiary_name": name,
		"reference":        reference,
		"narration":        narration,
	}, &d)
	return fmt.Sprint(d.ID), err
}
