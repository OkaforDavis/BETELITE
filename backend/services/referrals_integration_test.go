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
func TestReferralRewardsAfterFirstPaidMatch(t *testing.T) {
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
	p := fmt.Sprintf("rf%d", os.Getpid())
	inviter, friend, rival, oldie := p+"-inv", p+"-new", p+"-riv", p+"-old"
	for _, u := range []string{inviter, friend, rival, oldie} {
		db.Pool.Exec(ctx, "INSERT INTO users (id, email, username) VALUES ($1, $1 || '@t.io', $1) ON CONFLICT DO NOTHING", u)
	}
	db.Pool.Exec(ctx, "UPDATE users SET created_at = NOW() - INTERVAL '10 days' WHERE id = $1", oldie)
	bal := func(u string) (b int64) {
		db.Pool.QueryRow(ctx, "SELECT balance FROM users WHERE id = $1", u).Scan(&b)
		return
	}

	info, err := GetReferralInfo(ctx, inviter)
	if err != nil || len(info.Code) != 8 {
		t.Fatalf("invite code: %+v %v", info, err)
	}
	if again, _ := GetReferralInfo(ctx, inviter); again.Code != info.Code {
		t.Fatal("invite code must stay the same")
	}

	if _, err := ClaimReferral(ctx, inviter, info.Code); err == nil {
		t.Error("own code must be rejected")
	}
	if _, err := ClaimReferral(ctx, friend, "NOPE1234"); err == nil {
		t.Error("unknown code must be rejected")
	}
	if _, err := ClaimReferral(ctx, oldie, info.Code); err == nil {
		t.Error("accounts older than 7 days must not claim")
	}
	if name, err := ClaimReferral(ctx, friend, " "+info.Code+" "); err != nil || name != inviter {
		t.Fatalf("claim: %q %v", name, err)
	}
	if _, err := ClaimReferral(ctx, friend, info.Code); err == nil {
		t.Error("a second claim must be rejected")
	}
	if bal(inviter) != 0 || bal(friend) != 0 {
		t.Fatal("no money moves at sign-up")
	}

	// A free tournament match doesn't count; a paid 1v1 does.
	db.Pool.Exec(ctx, "INSERT INTO tournaments (id, name, game, entry_fee) VALUES ($1,'Free','dls',0)", p+"-t")
	db.Pool.Exec(ctx, `INSERT INTO matches (id, kind, game, home_id, away_id, tournament_id, status) VALUES ($1,'tournament','dls',$2,$3,$4,'confirmed')`, p+"-free", friend, rival, p+"-t")
	RewardReferral(ctx, friend)
	if bal(inviter) != 0 {
		t.Fatal("a free match must not unlock rewards")
	}
	db.Pool.Exec(ctx, `INSERT INTO matches (id, kind, game, home_id, away_id, status) VALUES ($1,'p2p','dls',$2,$3,'confirmed')`, p+"-paid", friend, rival)
	RewardReferral(ctx, friend)
	RewardReferral(ctx, friend) // must not pay twice
	if bal(inviter) != 200_00 || bal(friend) != 100_00 {
		t.Fatalf("after first paid match: inviter %d friend %d; want 20000/10000", bal(inviter), bal(friend))
	}
	if _, err := ClaimReferral(ctx, rival, info.Code); err == nil {
		t.Error("a player who already played a paid match must not claim")
	}
	time.Sleep(50 * time.Millisecond)
	if info, _ := GetReferralInfo(ctx, inviter); info.Joined != 1 || info.Rewarded != 1 || info.Earned != 200_00 {
		t.Fatalf("stats: %+v", info)
	}
}
