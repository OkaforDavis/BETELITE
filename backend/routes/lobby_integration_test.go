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
	"time"

	"github.com/gofiber/fiber/v2"

	"betelite-go/config"
	"betelite-go/db"
	"betelite-go/services"
	"betelite-go/ws"
)

// Runs only with a real database: TEST_DATABASE_URL=postgres://... go test ./routes/
// One 1v1 at a time: once a challenge is accepted, both players' other open
// challenges close, and neither can post or accept until the match is done.
func TestOneOneVOneAtATime(t *testing.T) {
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
	SetupMembershipRoutes(api)
	hub := ws.NewHub()
	go hub.Run()
	SetupLobbyRoutes(api, hub)

	p := fmt.Sprintf("l%d", time.Now().UnixNano())
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
	for _, u := range []string{"a", "b", "c"} {
		call(u, "GET", "/api/membership", nil) // creates the user row
		db.Pool.Exec(ctx, `UPDATE users SET currency = 'NGN', terms_version = $2, birth_date = '1995-01-01' WHERE id = $1`, uid(u), services.TermsVersion)
		for _, g := range []string{"dls", "fc_mobile"} {
			db.Pool.Exec(ctx, `INSERT INTO game_profiles (user_id, game, gamertag) VALUES ($1,$2,$1) ON CONFLICT DO NOTHING`, uid(u), g)
		}
	}
	post := func(user, game string) string {
		code, out := call(user, "POST", "/api/lobby/create", map[string]any{"game": game})
		if code != 200 {
			t.Fatalf("%s posting %s: %d %v", user, game, code, out)
		}
		return out["challengeId"].(string)
	}
	open := func() map[string]bool {
		_, out := call("c", "GET", "/api/lobby/", nil)
		seen := map[string]bool{}
		for _, ch := range out["challenges"].([]any) {
			seen[ch.(map[string]any)["id"].(string)] = true
		}
		return seen
	}

	a1, a2 := post("a", "dls"), post("a", "fc_mobile")
	b1 := post("b", "dls")
	// b accepts a's first challenge: a's other one and b's own one close.
	if code, out := call("b", "POST", "/api/lobby/accept", map[string]any{"challengeId": a1}); code != 200 {
		t.Fatalf("accept: %d %v", code, out)
	}
	var st2, stB string
	db.Pool.QueryRow(ctx, "SELECT status FROM escrow WHERE challenge_id = $1", a2).Scan(&st2)
	db.Pool.QueryRow(ctx, "SELECT status FROM escrow WHERE challenge_id = $1", b1).Scan(&stB)
	if st2 != "cancelled" || stB != "cancelled" {
		t.Fatalf("other open challenges should close: a2=%s b1=%s", st2, stB)
	}
	if o := open(); o[a2] || o[b1] {
		t.Fatalf("closed challenges still listed: %v", o)
	}
	// Neither a nor b can post or accept while their 1v1 is unfinished.
	if code, _ := call("a", "POST", "/api/lobby/create", map[string]any{"game": "dls"}); code != 409 {
		t.Fatalf("a posted during a match: %d", code)
	}
	c1 := post("c", "dls")
	if code, _ := call("b", "POST", "/api/lobby/accept", map[string]any{"challengeId": c1}); code != 409 {
		t.Fatalf("b accepted during a match: %d", code)
	}
	// Finish the match: they can play again.
	db.Pool.Exec(ctx, "UPDATE matches SET status = 'confirmed' WHERE home_id = $1 AND away_id = $2", uid("a"), uid("b"))
	if code, out := call("b", "POST", "/api/lobby/accept", map[string]any{"challengeId": c1}); code != 200 {
		t.Fatalf("b should accept once the match is done: %d %v", code, out)
	}
	// A challenge left over from a busy player (data from before this rule) is hidden.
	db.Pool.Exec(ctx, `INSERT INTO escrow (challenge_id, creator_id, amount, pool, status, game, currency, kind)
		VALUES ($1, $2, 0, 0, 'waiting', 'dls', 'NGN', 'stake')`, p+"-old", uid("c"))
	if open()[p+"-old"] {
		t.Fatal("a busy player's challenge should be hidden")
	}
	if code, _ := call("a", "POST", "/api/lobby/accept", map[string]any{"challengeId": p + "-old"}); code != 409 {
		t.Fatalf("accepting a busy player's challenge should fail: %d", code)
	}
}
