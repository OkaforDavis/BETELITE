package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"betelite-go/config"
	"betelite-go/db"
	"betelite-go/services"
	"betelite-go/ws"
)

// Runs only with a real database: TEST_DATABASE_URL=postgres://... go test ./routes/
func TestJoinRulesAndFreeChallenges(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	config.Load()
	config.Cfg.StakingEnabled = false
	ctx := context.Background()
	if err := db.Connect(ctx, url); err != nil {
		t.Fatal(err)
	}
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	api := app.Group("/api")
	SetupTournamentRoutes(api)
	SetupMembershipRoutes(api)
	hub := ws.NewHub()
	go hub.Run()
	SetupLobbyRoutes(api, hub)

	p := fmt.Sprintf("j%d", os.Getpid())
	call := func(user, method, path string, body any) (int, map[string]any) {
		var r io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			r = strings.NewReader(string(b))
		}
		req := httptest.NewRequest(method, path, r)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer dev:"+p+user)
		res, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]any{}
		json.NewDecoder(res.Body).Decode(&out)
		return res.StatusCode, out
	}
	uid := func(user string) string { return "dev-" + p + user }
	ready := func(user, cur string) {
		call(user, "GET", "/api/membership", nil) // creates the user row
		db.Pool.Exec(ctx, `UPDATE users SET currency = $2, terms_version = $3, birth_date = '1995-01-01' WHERE id = $1`, uid(user), cur, services.TermsVersion)
		db.Pool.Exec(ctx, `INSERT INTO game_profiles (user_id, game, gamertag) VALUES ($1,'dls',$1) ON CONFLICT DO NOTHING`, uid(user))
	}
	ready("free", "GHS")
	ready("pro", "NGN")

	newTournament := func(name string, early bool) string {
		id := p + name
		membersUntil := "NULL"
		if early {
			membersUntil = "NOW() + INTERVAL '6 hours'"
		}
		db.Pool.Exec(ctx, `INSERT INTO tournaments (id, name, game, format, entry_fee, max_players, prizes, members_only_until)
			VALUES ($1,$1,'dls','knockout',0,8,'{"GHS":[6000],"NGN":[600000]}'::jsonb,`+membersUntil+`)`, id)
		return id
	}

	// Free player: one entry a month.
	t1, t2 := newTournament("t1", false), newTournament("t2", false)
	if code, out := call("free", "POST", "/api/tournaments/"+t1+"/join", nil); code != 200 {
		t.Fatalf("first join: %d %v", code, out)
	}
	if code, out := call("free", "POST", "/api/tournaments/"+t2+"/join", nil); code != 403 || !strings.Contains(fmt.Sprint(out["error"]), "entries") {
		t.Fatalf("second join should be refused for lack of entries: %d %v", code, out)
	}
	// Leaving before the start gives the entry back.
	call("free", "POST", "/api/tournaments/"+t1+"/leave", nil)
	if code, out := call("free", "POST", "/api/tournaments/"+t2+"/join", nil); code != 200 {
		t.Fatalf("join after leaving: %d %v", code, out)
	}

	// Members-only early sign-up.
	early := newTournament("early", true)
	if code, out := call("free", "POST", "/api/tournaments/"+early+"/join", nil); code != 403 || !strings.Contains(fmt.Sprint(out["error"]), "Members") {
		t.Fatalf("free player during early sign-up: %d %v", code, out)
	}

	// Pro paid from the wallet: 2 entries, early sign-up.
	db.Pool.Exec(ctx, "UPDATE users SET balance = 3_000_00 WHERE id = $1", uid("pro"))
	code, out := call("pro", "POST", "/api/membership/buy", map[string]string{"tier": "pro"})
	if code != 200 || out["membership"] == nil {
		t.Fatalf("buy pro from wallet: %d %v", code, out)
	}
	for _, tid := range []string{early, newTournament("t3", false)} {
		if code, out := call("pro", "POST", "/api/tournaments/"+tid+"/join", nil); code != 200 {
			t.Fatalf("pro join %s: %d %v", tid, code, out)
		}
	}
	if code, _ := call("pro", "POST", "/api/tournaments/"+newTournament("t4", false)+"/join", nil); code != 403 {
		t.Fatalf("pro third join should be refused, got %d", code)
	}
	code, out = call("pro", "GET", "/api/membership", nil)
	usage := out["usage"].(map[string]any)
	if code != 200 || usage["tier"] != "pro" || usage["used"].(float64) != 2 || usage["left"].(float64) != 0 {
		t.Fatalf("membership view: %d %v", code, out)
	}

	// Staking off: challenges are free whatever amount is sent, and nobody is charged.
	db.Pool.Exec(ctx, "UPDATE users SET balance = 0 WHERE id IN ($1,$2)", uid("free"), uid("pro"))
	code, out = call("free", "POST", "/api/lobby/create", map[string]any{"game": "dls", "amount": 500_00})
	if code != 200 {
		t.Fatalf("free challenge: %d %v", code, out)
	}
	cid := out["challengeId"].(string)
	code, out = call("pro", "POST", "/api/lobby/accept", map[string]any{"challengeId": cid})
	if code != 200 {
		t.Fatalf("accept free challenge: %d %v", code, out)
	}
	var amount int64
	db.Pool.QueryRow(ctx, "SELECT amount FROM escrow WHERE challenge_id = $1", cid).Scan(&amount)
	if amount != 0 {
		t.Fatalf("escrow amount %d, want 0", amount)
	}
	matchID := out["matchId"].(string)
	if err := services.AdminResolve(ctx, matchID, "test", 2, 1, nil, nil); err != nil {
		t.Fatal(err)
	}
	var balances, txns int64
	db.Pool.QueryRow(ctx, "SELECT COALESCE(SUM(balance),0) FROM users WHERE id IN ($1,$2)", uid("free"), uid("pro")).Scan(&balances)
	db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM transactions WHERE ref_id = $1", cid).Scan(&txns)
	if balances != 0 || txns != 0 {
		t.Fatalf("a free match moved money: balances %d, transactions %d", balances, txns)
	}
}
