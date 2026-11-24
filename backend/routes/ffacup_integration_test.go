package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
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
func TestFreeForAllCup(t *testing.T) {
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
	SetupTournamentRoutes(api)
	SetupMembershipRoutes(api)

	p := fmt.Sprintf("f%d", os.Getpid())
	do := func(user, method, path string, body io.Reader, ctype string) (int, map[string]any) {
		req := httptest.NewRequest(method, path, body)
		if ctype != "" {
			req.Header.Set("Content-Type", ctype)
		}
		req.Header.Set("Authorization", "Bearer dev:"+p+user)
		res, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]any{}
		json.NewDecoder(res.Body).Decode(&out)
		return res.StatusCode, out
	}
	post := func(user, path string, body any) (int, map[string]any) {
		b, _ := json.Marshal(body)
		return do(user, "POST", path, strings.NewReader(string(b)), "application/json")
	}
	uid := func(u string) string { return "dev-" + p + u }
	for _, u := range []string{"host", "a", "b", "c", "x"} {
		do(u, "GET", "/api/membership", nil, "")
		db.Pool.Exec(ctx, "UPDATE users SET currency = 'GHS' WHERE id = $1", uid(u))
	}

	tid := p + "-cup"
	if _, err := db.Pool.Exec(ctx, `INSERT INTO tournaments (id, name, game, mode, format, currency, max_players, prizes, status, host_id, created_by, settings)
		VALUES ($1,'Black Friday FFA','codm','ffa','ffa','GHS',8,'{"GHS":[15000],"NGN":[1600000]}'::jsonb,'open',$2,$2,
		'{"mode":"ffa","scoreLimit":50,"timeLimit":10,"map":"Host picks","rules":["no_scorestreaks","no_operator"]}'::jsonb)`, tid, uid("host")); err != nil {
		t.Fatal(err)
	}
	path := "/api/tournaments/" + tid

	// One player can't start a room game.
	db.Pool.Exec(ctx, "INSERT INTO tournament_players (tournament_id, user_id) VALUES ($1,$2)", tid, uid("a"))
	if code, _ := post("host", path+"/room", map[string]string{"code": "24827", "password": "1234"}); code != 409 {
		t.Fatalf("room with 1 player: %d", code)
	}
	for _, u := range []string{"b", "c"} {
		db.Pool.Exec(ctx, "INSERT INTO tournament_players (tournament_id, user_id) VALUES ($1,$2)", tid, uid(u))
	}
	if code, _ := post("a", path+"/room", map[string]string{"code": "24827"}); code != 403 {
		t.Fatalf("player posted the room: %d", code)
	}
	if code, _ := post("host", path+"/room", map[string]string{"code": "24<827>"}); code != 400 {
		t.Fatalf("bad room id accepted: %d", code)
	}
	if code, out := post("host", path+"/room", map[string]string{"code": "24827", "password": "1234", "note": "Map: Crash"}); code != 200 {
		t.Fatalf("post room: %d %v", code, out)
	}
	var alerts int
	db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM notifications WHERE type = 'match_ping' AND metadata->>'tournamentId' = $1", tid).Scan(&alerts)
	if alerts != 3 {
		t.Fatalf("%d room alerts, want 3", alerts)
	}

	// Players see the room and settings; outsiders don't see the room.
	_, out := do("a", "GET", path, nil, "")
	tv := out["tournament"].(map[string]any)
	room, _ := tv["room"].(map[string]any)
	if tv["status"] != "active" || room == nil || room["code"] != "24827" || room["password"] != "1234" {
		t.Fatalf("player view: %v", tv)
	}
	if tv["settingsSummary"] != "Free for All · First to 50 kills · 10 min" {
		t.Fatalf("summary: %v", tv["settingsSummary"])
	}
	_, out = do("x", "GET", path, nil, "")
	if out["tournament"].(map[string]any)["room"] != nil {
		t.Fatal("outsider saw the room")
	}

	// Result: screenshot + scores; the winner must have the top score.
	var pngBuf bytes.Buffer
	png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 40, 20)))
	result := func(user string, res services.CupResult) int {
		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		fw, _ := w.CreateFormFile("image", "board.png")
		fw.Write(pngBuf.Bytes())
		b, _ := json.Marshal(res)
		w.WriteField("result", string(b))
		w.Close()
		code, _ := do(user, "POST", path+"/result", &body, w.FormDataContentType())
		return code
	}
	scores := map[string]int{uid("a"): 50, uid("b"): 31, uid("c"): 12}
	if code := result("a", services.CupResult{Scores: scores, Winner: uid("a")}); code != 403 {
		t.Fatalf("player submitted result: %d", code)
	}
	if code := result("host", services.CupResult{Scores: scores, Winner: uid("b")}); code != 400 {
		t.Fatalf("winner without top score accepted: %d", code)
	}
	if code := result("host", services.CupResult{Scores: map[string]int{uid("a"): 51}, Winner: uid("a")}); code != 400 {
		t.Fatalf("score above limit accepted: %d", code)
	}
	if code := result("host", services.CupResult{Scores: map[string]int{uid("x"): 5}, Winner: uid("a")}); code != 400 {
		t.Fatalf("outsider score accepted: %d", code)
	}
	var before int64
	db.Pool.QueryRow(ctx, "SELECT balance FROM users WHERE id = $1", uid("a")).Scan(&before)
	if code := result("host", services.CupResult{Scores: scores, Winner: uid("a")}); code != 200 {
		t.Fatalf("result: %d", code)
	}
	var after int64
	var status, winner string
	db.Pool.QueryRow(ctx, "SELECT balance FROM users WHERE id = $1", uid("a")).Scan(&after)
	db.Pool.QueryRow(ctx, "SELECT status, COALESCE(winner_id,'') FROM tournaments WHERE id = $1", tid).Scan(&status, &winner)
	if after-before != 15000 || status != "finished" || winner != uid("a") {
		t.Fatalf("payout %d, status %s, winner %s", after-before, status, winner)
	}
	var pos, score int
	db.Pool.QueryRow(ctx, "SELECT final_position, score FROM tournament_players WHERE tournament_id = $1 AND user_id = $2", tid, uid("b")).Scan(&pos, &score)
	if pos != 2 || score != 31 {
		t.Fatalf("b: position %d score %d", pos, score)
	}
	if code, _ := do("c", "GET", path+"/result-image", nil, ""); code != 200 {
		t.Fatalf("player can't see scoreboard: %d", code)
	}
	if code, _ := do("x", "GET", path+"/result-image", nil, ""); code != 403 {
		t.Fatalf("outsider saw scoreboard: %d", code)
	}
	if code := result("host", services.CupResult{Scores: scores, Winner: uid("a")}); code != 409 {
		t.Fatalf("second result accepted: %d", code)
	}
}
