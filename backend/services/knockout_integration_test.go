package services

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"betelite-go/db"
)

// Runs only with a real database: TEST_DATABASE_URL=postgres://... go test ./services/
// A 4-player knockout: semi-finals, then the final and a 3rd-place match;
// positions and prizes must follow those two matches.
func TestKnockoutThirdPlace(t *testing.T) {
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
	p := fmt.Sprintf("test-ko3-%d", time.Now().UnixNano())
	var ids []string
	for i := 0; i < 4; i++ {
		id := fmt.Sprintf("%s-%d", p, i)
		ids = append(ids, id)
		if _, err := db.Pool.Exec(ctx, `INSERT INTO users (id, email, username, currency, terms_version, birth_date, balance)
			VALUES ($1, $1 || '@t.io', $1, 'NGN', $2, '1995-01-01', 0)`, id, TermsVersion); err != nil {
			t.Fatal(err)
		}
	}
	cup := p + "-cup"
	if _, err := db.Pool.Exec(ctx, `INSERT INTO tournaments (id, name, game, mode, format, entry_fee, currency, max_players, prize_pool, created_by, status, prizes)
		VALUES ($1, 'KO3', 'fc_mobile', 'knockout', 'knockout', 0, 'NGN', 4, 0, $2, 'open', '{"NGN":[3000,2000,1000],"GHS":[30,20,10]}'::jsonb)`, cup, ids[0]); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		db.Pool.Exec(ctx, "INSERT INTO tournament_players (tournament_id, user_id) VALUES ($1,$2)", cup, id)
	}
	tx, _ := db.Pool.Begin(ctx)
	if err := StartTournament(ctx, tx, cup); err != nil {
		t.Fatal(err)
	}
	tx.Commit(ctx)

	type m struct {
		id, home, away string
		third          bool
	}
	matches := func(round int) []m {
		rows, err := db.Pool.Query(ctx, "SELECT id, home_id, COALESCE(away_id,''), third_place FROM matches WHERE tournament_id = $1 AND round = $2 ORDER BY slot", cup, round)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []m
		for rows.Next() {
			var x m
			rows.Scan(&x.id, &x.home, &x.away, &x.third)
			out = append(out, x)
		}
		return out
	}
	// The home player wins every match.
	win := func(x m) {
		if _, err := db.Pool.Exec(ctx, "UPDATE matches SET status = 'confirmed', winner_id = home_id, score_home = 2, score_away = 1 WHERE id = $1", x.id); err != nil {
			t.Fatal(err)
		}
	}

	semis := matches(1)
	if len(semis) != 2 {
		t.Fatalf("want 2 semi-finals, got %d", len(semis))
	}
	for _, s := range semis {
		win(s)
	}
	if err := advanceTournament(ctx, cup); err != nil {
		t.Fatal(err)
	}
	r2 := matches(2)
	if len(r2) != 2 || r2[0].third || !r2[1].third {
		t.Fatalf("round 2 should be the final and a 3rd-place match, got %+v", r2)
	}
	final, third := r2[0], r2[1]
	if final.home != semis[0].home || final.away != semis[1].home || third.home != semis[0].away || third.away != semis[1].away {
		t.Fatalf("wrong pairings: final %+v third %+v", final, third)
	}

	// Only the final played: the cup must wait for the 3rd-place match.
	win(final)
	advanceTournament(ctx, cup)
	var status string
	db.Pool.QueryRow(ctx, "SELECT status FROM tournaments WHERE id = $1", cup).Scan(&status)
	if status != "active" {
		t.Fatalf("cup finished before the 3rd-place match, status %s", status)
	}
	win(third)
	if err := advanceTournament(ctx, cup); err != nil {
		t.Fatal(err)
	}
	db.Pool.QueryRow(ctx, "SELECT status FROM tournaments WHERE id = $1", cup).Scan(&status)
	if status != "finished" {
		t.Fatalf("cup should be finished, status %s", status)
	}
	want := map[string]struct {
		pos     int
		balance int64
	}{final.home: {1, 3000}, final.away: {2, 2000}, third.home: {3, 1000}, third.away: {4, 0}}
	for id, w := range want {
		var pos int
		var bal int64
		db.Pool.QueryRow(ctx, "SELECT COALESCE(final_position,0) FROM tournament_players WHERE tournament_id = $1 AND user_id = $2", cup, id).Scan(&pos)
		db.Pool.QueryRow(ctx, "SELECT balance FROM users WHERE id = $1", id).Scan(&bal)
		if pos != w.pos || bal != w.balance {
			t.Errorf("%s: position %d balance %d, want %d and %d", id, pos, bal, w.pos, w.balance)
		}
	}
}
