package services

import (
	"context"
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
func TestDepositCrediting(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	if err := db.Connect(ctx, url); err != nil {
		t.Fatal(err)
	}
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatal(err)
	}
	config.Cfg.PaystackSecretKey = "sk_test"

	// Fake Paystack: each reference returns a scripted sequence of states.
	var mu sync.Mutex
	script := map[string][]string{}
	amounts := map[string]int64{}
	paystack := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ref := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		mu.Lock()
		states := script[ref]
		status := states[0]
		if len(states) > 1 {
			script[ref] = states[1:]
		}
		mu.Unlock()
		amt := amounts[ref]
		if amt == 0 {
			amt = 100000
		}
		json.NewEncoder(w).Encode(map[string]any{"status": true, "message": "ok",
			"data": map[string]any{"status": status, "reference": ref, "amount": amt, "currency": "NGN"}})
	}))
	defer paystack.Close()
	paystackBaseURL = paystack.URL

	uid := fmt.Sprintf("test-dep-%d", os.Getpid())
	db.Pool.Exec(ctx, "INSERT INTO users (id, email, username) VALUES ($1, $1 || '@t.io', 'Tester') ON CONFLICT DO NOTHING", uid)
	balance := func() int64 {
		var b int64
		db.Pool.QueryRow(ctx, "SELECT balance FROM users WHERE id = $1", uid).Scan(&b)
		return b
	}
	deposit := func(ref, status string) {
		db.Pool.Exec(ctx, "INSERT INTO deposits (reference, user_id, amount, currency, status, created_at) VALUES ($1,$2,100000,'NGN',$3, NOW() - INTERVAL '5 minutes')", ref, uid, status)
	}
	start := balance()

	// 1. The bug: checking while the customer is still paying returns
	// "abandoned". That must NOT close the deposit.
	deposit(uid+"-a", "pending")
	script[uid+"-a"] = []string{"abandoned", "success"}
	if s, err := CreditDeposit(ctx, uid+"-a"); err != nil || s != "pending" {
		t.Fatalf("abandoned should stay pending, got %q %v", s, err)
	}
	if s, _ := CreditDeposit(ctx, uid+"-a"); s != "paid" || balance() != start+100000 {
		t.Fatalf("success after abandoned should credit: %q balance %d", s, balance()-start)
	}

	// 2. The stuck deposit: the old code marked it failed; Paystack says paid.
	// The background reconciler must find and credit it.
	deposit(uid+"-stuck", "failed")
	script[uid+"-stuck"] = []string{"success"}
	ReconcileDeposits(ctx)
	if balance() != start+200000 {
		t.Fatalf("reconciler should credit the stuck deposit, balance +%d", balance()-start)
	}

	// 3. A genuinely declined card: marked failed, nothing credited.
	deposit(uid+"-declined", "pending")
	script[uid+"-declined"] = []string{"failed"}
	if s, _ := CreditDeposit(ctx, uid+"-declined"); s != "failed" || balance() != start+200000 {
		t.Fatalf("declined card must not credit: %q", s)
	}

	// 4. Many simultaneous checks + webhook + reconciler: credited exactly once.
	deposit(uid+"-race", "pending")
	script[uid+"-race"] = []string{"success"}
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); CreditDeposit(ctx, uid+"-race") }()
	}
	wg.Wait()
	ReconcileDeposits(ctx)
	if balance() != start+300000 {
		t.Fatalf("must credit exactly once, balance +%d", balance()-start)
	}

	// 5. Paid less than the deposit amount: not credited.
	deposit(uid+"-short", "pending")
	script[uid+"-short"] = []string{"success"}
	amounts[uid+"-short"] = 50000
	if _, err := CreditDeposit(ctx, uid+"-short"); err == nil || balance() != start+300000 {
		t.Fatalf("amount mismatch must not credit (err=%v)", err)
	}

	// Ledger: balance equals the sum of transactions.
	var sum int64
	db.Pool.QueryRow(ctx, "SELECT COALESCE(SUM(amount),0) FROM transactions WHERE user_id = $1", uid).Scan(&sum)
	if sum != balance() {
		t.Fatalf("ledger mismatch: transactions %d, balance %d", sum, balance())
	}
}
