package services

import (
	"context"
	"fmt"
	"os"
	"testing"

	"betelite-go/db"
)

// Runs only with a real database: TEST_DATABASE_URL=postgres://... go test ./services/
func TestTips(t *testing.T) {
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
	p := fmt.Sprintf("test-tip-%d", os.Getpid())
	from, host, stranger := p+"-a", p+"-h", p+"-s"
	for _, id := range []string{from, host, stranger} {
		db.Pool.Exec(ctx, `INSERT INTO users (id, email, username, currency, terms_version, birth_date, balance)
			VALUES ($1, $1 || '@t.io', $1, 'NGN', $2, '1995-01-01', 0) ON CONFLICT DO NOTHING`, id, TermsVersion)
	}
	db.Pool.Exec(ctx, "UPDATE users SET balance = 60000_00 WHERE id = $1", from)
	cup := p + "-cup"
	db.Pool.Exec(ctx, `INSERT INTO tournaments (id, name, game, mode, format, entry_fee, currency, max_players, prize_pool, created_by, status, host_id)
		VALUES ($1, 'Tip cup', 'fc_mobile', 'knockout', 'knockout', 0, 'NGN', 4, 0, $2, 'open', $2)`, cup, host)

	bal := func(id string) (b int64) {
		db.Pool.QueryRow(ctx, "SELECT balance FROM users WHERE id = $1", id).Scan(&b)
		return
	}
	req := TipRequest{To: host, Amount: 500_00, Context: "tournament", RefID: cup}
	if err := TipAllowed(ctx, req); err != nil {
		t.Fatalf("host of the cup should be tippable: %v", err)
	}
	if err := TipAllowed(ctx, TipRequest{To: stranger, Context: "tournament", RefID: cup}); err == nil {
		t.Fatal("a stranger to the cup must not be tippable")
	}
	res, err := SendTip(ctx, from, req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Received != 500_00 || bal(host) != 500_00 || bal(from) != 59_500_00 {
		t.Fatalf("free tip: received %d, host %d, sender %d", res.Received, bal(host), bal(from))
	}
	if _, err := SendTip(ctx, from, TipRequest{To: from, Amount: 500_00}); err == nil {
		t.Fatal("self-tip must fail")
	}
	if _, err := SendTip(ctx, from, TipRequest{To: host, Amount: 50_00}); err == nil {
		t.Fatal("tip below the minimum must fail")
	}
	// Daily cap: 500 already sent today, so 20k + 20k + 10k (= 50.5k) must stop at the last one.
	for i, amt := range []int64{20_000_00, 20_000_00} {
		if _, err := SendTip(ctx, from, TipRequest{To: host, Amount: amt, Context: "tournament", RefID: cup}); err != nil {
			t.Fatalf("tip %d: %v", i, err)
		}
	}
	if _, err := SendTip(ctx, from, TipRequest{To: host, Amount: 10_000_00, Context: "tournament", RefID: cup}); err == nil {
		t.Fatal("daily tip cap not enforced")
	}
	if bal(from)+bal(host) != 60_000_00 {
		t.Fatalf("money created or lost: %d + %d", bal(from), bal(host))
	}
}
