package services

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"testing"

	"betelite-go/db"
)

// Runs only with a real database: TEST_DATABASE_URL=postgres://... go test ./services/
func TestEvidenceStorageAndRetention(t *testing.T) {
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

	p := fmt.Sprintf("ev%d", os.Getpid())
	for _, u := range []string{p + "a", p + "b"} {
		db.Pool.Exec(ctx, "INSERT INTO users (id, email, username) VALUES ($1, $1 || '@t.io', $1) ON CONFLICT DO NOTHING", u)
	}
	match := func(id, status, settledAgo string) {
		settled := "NULL"
		if settledAgo != "" {
			settled = "NOW() - INTERVAL '" + settledAgo + "'"
		}
		db.Pool.Exec(ctx, fmt.Sprintf(`INSERT INTO matches (id, kind, game, home_id, away_id, status, settled_at)
			VALUES ($1,'p2p','dls',$2,$3,$4,%s)`, settled), id, p+"a", p+"b", status)
	}
	// A 2400x1080 phone screenshot with EXIF GPS data.
	var buf bytes.Buffer
	jpeg.Encode(&buf, testImage(2400, 1080), nil)
	shot := withEXIF(buf.Bytes())

	match(p+"-old", "confirmed", "31 days")
	match(p+"-recent", "confirmed", "2 days")
	match(p+"-disputed", "disputed", "")
	for _, m := range []string{p + "-old", p + "-recent", p + "-disputed"} {
		if err := SaveEvidence(ctx, m, p+"a", "result", shot); err != nil {
			t.Fatal(err)
		}
	}
	// The disputed match's screenshot is also 40 days old.
	db.Pool.Exec(ctx, "UPDATE match_screenshots SET created_at = NOW() - INTERVAL '40 days' WHERE match_id = $1", p+"-disputed")

	list, err := ListEvidence(ctx, p+"-recent")
	if err != nil || len(list) != 1 || list[0].Uploader != p+"a" {
		t.Fatalf("list: %+v %v", list, err)
	}
	img, _, matchID, err := LoadEvidence(ctx, list[0].ID)
	if err != nil || matchID != p+"-recent" {
		t.Fatalf("load: %v", err)
	}
	if bytes.Contains(img, []byte("GPS")) {
		t.Fatal("stored evidence must not contain location metadata")
	}
	if cfg, _, _ := image.DecodeConfig(bytes.NewReader(img)); cfg.Width != 1920 {
		t.Fatalf("evidence should be resized to 1920 wide, got %d", cfg.Width)
	}

	if !CanViewEvidence(ctx, p+"-recent", p+"a", false) || !CanViewEvidence(ctx, p+"-recent", p+"b", false) {
		t.Fatal("both players must be able to view")
	}
	if CanViewEvidence(ctx, p+"-recent", "someone-else", false) || !CanViewEvidence(ctx, p+"-recent", "someone-else", true) {
		t.Fatal("only players and admins may view")
	}

	purgeOldEvidence(ctx)
	count := func(m string) (n int) {
		db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM match_screenshots WHERE match_id = $1", m).Scan(&n)
		return
	}
	if count(p+"-old") != 0 {
		t.Error("evidence 31 days after a final match must be deleted")
	}
	if count(p+"-recent") != 1 {
		t.Error("recent evidence must be kept")
	}
	if count(p+"-disputed") != 1 {
		t.Error("evidence for an unresolved dispute must be kept, however old")
	}
}
