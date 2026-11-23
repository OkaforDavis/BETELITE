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
func TestEndMatch(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	config.Load()
	ctx := context.Background()
	if err := db.Connect(ctx, url); err != nil {
		t.Fatal(err)
	}
	if err := db.RunMigrations(ctx); err != nil {
		t.Fatal(err)
	}
	hub := ws.NewHub()
	go hub.Run()
	services.Hub = hub
	app := fiber.New()
	api := app.Group("/api")
	SetupMatchRoutes(api)
	SetupMembershipRoutes(api)

	p := fmt.Sprintf("e%d", os.Getpid())
	call := func(user, path string, body any) (int, map[string]any) {
		b, _ := json.Marshal(body)
		var r io.Reader = strings.NewReader(string(b))
		req := httptest.NewRequest("POST", path, r)
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
	uid := func(u string) string { return "dev-" + p + u }
	for _, u := range []string{"a", "b", "c"} {
		req := httptest.NewRequest("GET", "/api/membership", nil)
		req.Header.Set("Authorization", "Bearer dev:"+p+u)
		app.Test(req, -1)
	}
	status := func(mid string) string {
		var s string
		db.Pool.QueryRow(ctx, "SELECT status FROM matches WHERE id = $1", mid).Scan(&s)
		return s
	}
	bal := func(u string) int64 {
		var b int64
		db.Pool.QueryRow(ctx, "SELECT balance FROM users WHERE id = $1", uid(u)).Scan(&b)
		return b
	}
	// newMatch: a host (a) vs challenger (b) 1v1; bounty > 0 means a's money is held.
	newMatch := func(name string, bounty int64) string {
		mid, cid := p+name, p+name+"-c"
		kind := "stake"
		if bounty > 0 {
			kind = "bounty"
		}
		db.Pool.Exec(ctx, `INSERT INTO escrow (challenge_id, creator_id, acceptor_id, amount, pool, status, game, currency, kind, fee, match_id)
			VALUES ($1,$2,$3,$4,$4,'held','dls','NGN',$5,$6,$7)`, cid, uid("a"), uid("b"), bounty, kind, bounty/5, mid)
		db.Pool.Exec(ctx, `INSERT INTO matches (id, kind, game, home_id, away_id, challenge_id, status) VALUES ($1,'p2p','dls',$2,$3,$4,'ready')`,
			mid, uid("a"), uid("b"), cid)
		return mid
	}

	// Friendly: either player ends it at once; outsiders can't.
	f := newMatch("-f", 0)
	if code, _ := call("c", "/api/matches/"+f+"/end", map[string]string{"reason": "x"}); code != 403 {
		t.Fatalf("outsider ended: %d", code)
	}
	if code, out := call("b", "/api/matches/"+f+"/end", map[string]string{"reason": "Accepted by mistake"}); code != 200 || status(f) != "void" {
		t.Fatalf("friendly end: %d %v %s", code, out, status(f))
	}

	// Bounty: needs the other player. Decline keeps it going; agree refunds the host.
	before := bal("a")
	bm := newMatch("-b", 100000)
	if code, _ := call("a", "/api/matches/"+bm+"/end", map[string]string{"reason": "Can't play"}); code != 200 || status(bm) != "ready" {
		t.Fatalf("request: %d %s", code, status(bm))
	}
	if code, _ := call("a", "/api/matches/"+bm+"/end/respond", map[string]bool{"agree": true}); code != 400 {
		t.Fatalf("asker agreed with self: %d", code)
	}
	if code, _ := call("b", "/api/matches/"+bm+"/end/respond", map[string]bool{"agree": false}); code != 200 || status(bm) != "ready" {
		t.Fatalf("decline: %d %s", code, status(bm))
	}
	if code, _ := call("b", "/api/matches/"+bm+"/end/respond", map[string]bool{"agree": true}); code != 409 {
		t.Fatalf("agree with no request: %d", code)
	}
	call("a", "/api/matches/"+bm+"/end", map[string]string{"reason": "Really can't"})
	if code, _ := call("b", "/api/matches/"+bm+"/end/respond", map[string]bool{"agree": true}); code != 200 || status(bm) != "void" {
		t.Fatalf("agree: %d %s", code, status(bm))
	}
	if got := bal("a") - before; got != 120000 {
		t.Fatalf("host refund %d, want bounty + fee 120000", got)
	}

	// Both ask: ends at once.
	b2 := newMatch("-b2", 50000)
	call("a", "/api/matches/"+b2+"/end", nil)
	if code, _ := call("b", "/api/matches/"+b2+"/end", nil); code != 200 || status(b2) != "void" {
		t.Fatalf("both asked: %d %s", code, status(b2))
	}

	// No answer in 15 minutes: the automation ends it.
	b3 := newMatch("-b3", 50000)
	call("a", "/api/matches/"+b3+"/end", nil)
	db.Pool.Exec(ctx, "UPDATE matches SET end_requested_at = NOW() - INTERVAL '16 minutes' WHERE id = $1", b3)
	services.EndUnansweredRequestsForTest(ctx)
	if status(b3) != "void" {
		t.Fatalf("unanswered request not ended: %s", status(b3))
	}

	// After a result is uploaded, ending is not allowed (dispute instead).
	b4 := newMatch("-b4", 50000)
	db.Pool.Exec(ctx, "UPDATE matches SET status = 'submitted' WHERE id = $1", b4)
	if code, _ := call("a", "/api/matches/"+b4+"/end", nil); code != 409 {
		t.Fatalf("ended a submitted match: %d", code)
	}
	// Tournament matches can't be ended by players.
	tm := p + "-t"
	db.Pool.Exec(ctx, `INSERT INTO matches (id, kind, game, home_id, away_id, status) VALUES ($1,'tournament','dls',$2,$3,'ready')`, tm, uid("a"), uid("b"))
	if code, _ := call("a", "/api/matches/"+tm+"/end", nil); code != 400 {
		t.Fatalf("ended a tournament match: %d", code)
	}
}
