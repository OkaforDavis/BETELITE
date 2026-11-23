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
func TestMatchChatAndPing(t *testing.T) {
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

	p := fmt.Sprintf("c%d", os.Getpid())
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
	}
	mid := p + "-m"
	if _, err := db.Pool.Exec(ctx, `INSERT INTO matches (id, kind, game, home_id, away_id, status) VALUES ($1,'p2p','dls',$2,$3,'ready')`,
		mid, uid("a"), uid("b")); err != nil {
		t.Fatal(err)
	}
	path := "/api/matches/" + mid

	if code, _ := call("c", "GET", path+"/chat", nil); code != 403 {
		t.Fatalf("outsider read chat: %d", code)
	}
	if code, _ := call("c", "POST", path+"/chat", map[string]string{"text": "hi"}); code != 403 {
		t.Fatalf("outsider sent chat: %d", code)
	}
	if code, out := call("a", "POST", path+"/chat", map[string]string{"text": "  ready in 5?  "}); code != 200 {
		t.Fatalf("send: %d %v", code, out)
	}
	if code, _ := call("a", "POST", path+"/chat", map[string]string{"text": "join https://evil.example"}); code != 400 {
		t.Fatalf("link allowed: %d", code)
	}
	if code, _ := call("a", "POST", path+"/chat", map[string]string{"text": strings.Repeat("x", 301)}); code != 400 {
		t.Fatalf("long message allowed: %d", code)
	}
	if code, _ := call("a", "POST", path+"/chat", map[string]string{"text": "   "}); code != 400 {
		t.Fatalf("empty message allowed: %d", code)
	}

	// Alerts: one per minute per player.
	if code, out := call("a", "POST", path+"/ping", nil); code != 200 {
		t.Fatalf("ping: %d %v", code, out)
	}
	if code, _ := call("a", "POST", path+"/ping", nil); code != 429 {
		t.Fatalf("second ping not limited: %d", code)
	}
	if code, _ := call("b", "POST", path+"/ping", nil); code != 200 {
		t.Fatalf("opponent ping: %d", code)
	}
	var notes int
	db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND type = 'match_ping'", uid("b")).Scan(&notes)
	if notes != 1 {
		t.Fatalf("b got %d ping notifications, want 1", notes)
	}

	code, out := call("b", "GET", path+"/chat", nil)
	chat, _ := out["chat"].(map[string]any)
	msgs, _ := chat["messages"].([]any)
	if code != 200 || len(msgs) != 3 {
		t.Fatalf("read chat: %d %v", code, out)
	}
	if first := msgs[0].(map[string]any); first["body"] != "ready in 5?" || first["kind"] != "text" {
		t.Fatalf("first message: %v", first)
	}
	if chat["canSend"] != true || chat["canPing"] != true || chat["pingWait"].(float64) < 50 {
		t.Fatalf("flags: %v", chat)
	}

	// After the result: no more alerts; chat open for a short while, then closed.
	db.Pool.Exec(ctx, "UPDATE matches SET status = 'confirmed', settled_at = NOW() WHERE id = $1", mid)
	db.Pool.Exec(ctx, "UPDATE match_messages SET created_at = created_at - INTERVAL '2 minutes' WHERE match_id = $1", mid)
	if code, _ := call("a", "POST", path+"/ping", nil); code != 409 {
		t.Fatalf("ping after result: %d", code)
	}
	if code, _ := call("b", "POST", path+"/chat", map[string]string{"text": "GG"}); code != 200 {
		t.Fatalf("GG after result: %d", code)
	}
	db.Pool.Exec(ctx, "UPDATE matches SET settled_at = NOW() - INTERVAL '2 hours' WHERE id = $1", mid)
	if code, _ := call("b", "POST", path+"/chat", map[string]string{"text": "late"}); code != 409 {
		t.Fatalf("chat after close: %d", code)
	}
	_, out = call("a", "GET", path+"/chat", nil)
	if c := out["chat"].(map[string]any); c["canSend"] != false || c["canPing"] != false {
		t.Fatalf("closed flags: %v", c)
	}

	// Spam limit: 15 messages a minute.
	db.Pool.Exec(ctx, "UPDATE matches SET status = 'ready', settled_at = NULL WHERE id = $1", mid)
	limited := false
	for i := 0; i < 20; i++ {
		if code, _ := call("a", "POST", path+"/chat", map[string]string{"text": fmt.Sprint("m", i)}); code == 429 {
			limited = true
			break
		}
	}
	if !limited {
		t.Fatal("no spam limit")
	}
}
