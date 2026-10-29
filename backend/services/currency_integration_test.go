package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"betelite-go/config"
	"betelite-go/db"
)

// Runs only with a real database: TEST_DATABASE_URL=postgres://... go test ./services/
func TestCrossCurrencyTopUpViaFlutterwave(t *testing.T) {
	ctx := testDB(t)
	old := config.Cfg
	t.Cleanup(func() { config.Cfg = old })
	config.Cfg.FlutterwaveSecretKey = "FLWSECK-test"

	// Fake Flutterwave: verify_by_reference answers with a scripted payment.
	var mu sync.Mutex
	paid := map[string][2]any{} // ref -> {amount (major), currency}
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ref := r.URL.Query().Get("tx_ref")
		mu.Lock()
		p, ok := paid[ref]
		mu.Unlock()
		if !ok {
			json.NewEncoder(w).Encode(map[string]any{"status": "error", "message": "No transaction was found for this id"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{
			"status": "successful", "tx_ref": ref, "amount": p[0], "currency": p[1]}})
	}))
	defer fake.Close()
	flutterwaveBaseURL = fake.URL

	u := makeUsers(ctx, fmt.Sprintf("cx%d", os.Getpid()), 1)[0] // a GHS wallet
	ref := "dep_" + u
	// ₵100 to the wallet, paid in naira: ₦11,845 plus Flutterwave's fee = ₦12,014.19.
	charged := GrossUpFor(ProviderFlutterwave, 11_845_00, "NGN")
	db.Pool.Exec(ctx, `INSERT INTO deposits (reference, user_id, amount, currency, charged, provider, pay_currency, created_at)
		VALUES ($1,$2,10000,'GHS',$3,'flutterwave','NGN', NOW() - INTERVAL '5 minutes')`, ref, u, charged)

	if s, err := CreditDeposit(ctx, ref); err != nil || s != "pending" {
		t.Fatalf("unpaid checkout should stay pending: %q %v", s, err)
	}
	// Paid in the wrong currency: must not credit.
	paid[ref] = [2]any{float64(charged) / 100, "GHS"}
	if _, err := CreditDeposit(ctx, ref); err == nil || balanceOf(ctx, u) != 0 {
		t.Fatalf("payment in the wrong currency was credited (balance %d)", balanceOf(ctx, u))
	}
	// Paid correctly in naira: the cedi wallet gets exactly ₵100, once.
	paid[ref] = [2]any{float64(charged) / 100, "NGN"}
	if s, err := CreditDeposit(ctx, ref); err != nil || s != "paid" {
		t.Fatalf("credit: %q %v", s, err)
	}
	CreditDeposit(ctx, ref)
	if b := balanceOf(ctx, u); b != 100_00 {
		t.Fatalf("balance %d, want 10000 (₵100)", b)
	}
}

func TestHomeCurrencySwitchRules(t *testing.T) {
	ctx := testDB(t)
	old := config.Cfg
	t.Cleanup(func() { config.Cfg = old })
	config.Cfg.FXNGNPerGHS, config.Cfg.FXMarginPct = 115, 3

	u := makeUsers(ctx, fmt.Sprintf("hs%d", os.Getpid()), 1)[0] // GHS wallet
	db.Pool.Exec(ctx, "UPDATE users SET balance = 100_00 WHERE id = $1", u)

	// Just arrived in Nigeria: not yet.
	RecordSeenCountry(ctx, u, "NG")
	if hs := HomeSwitchStatus(ctx, u); hs.Eligible || hs.To != "NGN" || hs.DaysLeft < 29 {
		t.Fatalf("new arrival: %+v", hs)
	}
	if _, err := SwitchHomeCurrency(ctx, u, "NGN", false); err == nil {
		t.Fatal("switch must wait 30 days")
	}
	// Seeing them in Nigeria again doesn't restart the clock; 31 days later it's allowed.
	db.Pool.Exec(ctx, "UPDATE users SET seen_country_since = NOW() - INTERVAL '31 days' WHERE id = $1", u)
	RecordSeenCountry(ctx, u, "NG")
	if hs := HomeSwitchStatus(ctx, u); !hs.Eligible {
		t.Fatalf("after 31 days: %+v", hs)
	}
	// A pending withdrawal blocks it.
	db.Pool.Exec(ctx, `INSERT INTO withdrawals (id, user_id, amount, currency, bank_code, account_number, status) VALUES ($1,$2,1000,'GHS','MTN','0240000000','pending')`, "wd_"+u, u)
	if _, err := SwitchHomeCurrency(ctx, u, "NGN", false); err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatalf("pending withdrawal should block: %v", err)
	}
	db.Pool.Exec(ctx, "UPDATE withdrawals SET status = 'paid' WHERE id = $1", "wd_"+u)

	nb, err := SwitchHomeCurrency(ctx, u, "NGN", false)
	if err != nil || nb != 11_155_00 {
		t.Fatalf("switch: balance %d %v, want 1115500 (₦11,155)", nb, err)
	}
	var cur string
	db.Pool.QueryRow(ctx, "SELECT currency FROM users WHERE id = $1", u).Scan(&cur)
	if cur != "NGN" || balanceOf(ctx, u) != 11_155_00 {
		t.Fatalf("after switch: %s %d", cur, balanceOf(ctx, u))
	}
	// Back in Ghana for 31 days, but only once every 90 days without an admin.
	RecordSeenCountry(ctx, u, "GH")
	db.Pool.Exec(ctx, "UPDATE users SET seen_country_since = NOW() - INTERVAL '31 days' WHERE id = $1", u)
	if _, err := SwitchHomeCurrency(ctx, u, "GHS", false); err == nil {
		t.Fatal("second switch within 90 days must be refused")
	}
	if _, err := SwitchHomeCurrency(ctx, u, "GHS", true); err != nil {
		t.Fatalf("admin switch: %v", err)
	}
}
