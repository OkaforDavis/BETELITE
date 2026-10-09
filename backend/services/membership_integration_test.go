package services

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"betelite-go/db"
)

func testDB(t *testing.T) context.Context {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	if db.Pool == nil {
		if err := db.Connect(ctx, url); err != nil {
			t.Fatal(err)
		}
		if err := db.RunMigrations(ctx); err != nil {
			t.Fatal(err)
		}
	}
	return ctx
}

func balanceOf(ctx context.Context, uid string) (b int64) {
	db.Pool.QueryRow(ctx, "SELECT balance FROM users WHERE id = $1", uid).Scan(&b)
	return
}

func makeUsers(ctx context.Context, prefix string, n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("%s-%02d", prefix, i)
		cur := "GHS"
		if i%2 == 1 {
			cur = "NGN"
		}
		db.Pool.Exec(ctx, `INSERT INTO users (id, email, username, currency, terms_version, birth_date)
			VALUES ($1, $1 || '@t.io', $1, $2, $3, '1995-01-01') ON CONFLICT DO NOTHING`, ids[i], cur, TermsVersion)
	}
	return ids
}

// Runs only with a real database: TEST_DATABASE_URL=postgres://... go test ./services/
func TestMembershipPlansAndEntries(t *testing.T) {
	ctx := testDB(t)
	u := makeUsers(ctx, fmt.Sprintf("mb%d", os.Getpid()), 2)
	gh, ng := u[0], u[1]

	usage, err := EntryUsage(ctx, db.Pool, gh)
	if err != nil || usage.Tier != FreeTier || usage.Allowed != 1 || usage.Left != 1 {
		t.Fatalf("free usage: %+v %v", usage, err)
	}

	if _, err := BuyMembership(ctx, gh, "pro"); err == nil {
		t.Fatal("buying without balance must fail")
	}
	db.Pool.Exec(ctx, "UPDATE users SET balance = 50_00 WHERE id = $1", gh)
	m, err := BuyMembership(ctx, gh, "pro")
	if err != nil || m.Price != 30_00 || m.Currency != "GHS" {
		t.Fatalf("buy pro: %+v %v", m, err)
	}
	if b := balanceOf(ctx, gh); b != 20_00 {
		t.Fatalf("balance after pro = %d, want 2000", b)
	}
	if _, err := BuyMembership(ctx, gh, "elite"); err == nil {
		t.Error("switching tier during a plan must fail")
	}
	if _, err := BuyMembership(ctx, gh, "pro"); err == nil {
		t.Error("renewing more than 7 days early must fail")
	}
	usage, _ = EntryUsage(ctx, db.Pool, gh)
	if usage.Tier != "pro" || usage.Allowed != 2 || usage.Left != 2 {
		t.Fatalf("pro usage: %+v", usage)
	}

	// Renewal in the last week extends from the current end date.
	db.Pool.Exec(ctx, "UPDATE memberships SET starts_at = NOW() - INTERVAL '25 days', ends_at = NOW() + INTERVAL '5 days' WHERE user_id = $1", gh)
	db.Pool.Exec(ctx, "UPDATE users SET balance = 30_00 WHERE id = $1", gh)
	renew, err := BuyMembership(ctx, gh, "pro")
	if err != nil {
		t.Fatalf("renew: %v", err)
	}
	if d := time.Until(renew.StartsAt); d < 4*24*time.Hour || d > 6*24*time.Hour {
		t.Errorf("renewal should start when the current plan ends, starts in %v", d)
	}
	usage, _ = EntryUsage(ctx, db.Pool, gh)
	if usage.RenewedTo == nil {
		t.Error("usage should show the renewal")
	}

	// Entries count tournament joins in the plan period; cancelled ones don't.
	tid := "tm-" + gh
	db.Pool.Exec(ctx, "INSERT INTO tournaments (id, name, game, entry_fee) VALUES ($1,'T','dls',0)", tid)
	db.Pool.Exec(ctx, "INSERT INTO tournament_players (tournament_id, user_id) VALUES ($1,$2)", tid, gh)
	usage, _ = EntryUsage(ctx, db.Pool, gh)
	if usage.Used != 1 || usage.Left != 1 {
		t.Fatalf("after one join: %+v", usage)
	}
	db.Pool.Exec(ctx, "UPDATE tournaments SET status = 'cancelled' WHERE id = $1", tid)
	if usage, _ = EntryUsage(ctx, db.Pool, gh); usage.Used != 0 {
		t.Fatalf("cancelled tournament still counted: %+v", usage)
	}

	// Naira prices for a Nigerian wallet.
	db.Pool.Exec(ctx, "UPDATE users SET balance = 8_000_00 WHERE id = $1", ng)
	if m, err := BuyMembership(ctx, ng, "elite"); err != nil || m.Price != 8_000_00 || balanceOf(ctx, ng) != 0 {
		t.Fatalf("elite in NGN: %+v %v", m, err)
	}
}

func TestSwissTournamentPaysFixedPrizes(t *testing.T) {
	ctx := testDB(t)
	players := makeUsers(ctx, fmt.Sprintf("sw%d", os.Getpid()), 16)
	tid := fmt.Sprintf("sw-%d", os.Getpid())
	prizes := `{"GHS":[6000,2500,1500],"NGN":[600000,250000,150000]}`
	if _, err := db.Pool.Exec(ctx, `INSERT INTO tournaments (id, name, game, format, entry_fee, max_players, prizes)
		VALUES ($1,'Points Cup','dls','swiss',0,16,$2::jsonb)`, tid, prizes); err != nil {
		t.Fatal(err)
	}
	for _, p := range players {
		db.Pool.Exec(ctx, "INSERT INTO tournament_players (tournament_id, user_id) VALUES ($1,$2)", tid, p)
	}
	tx, _ := db.Pool.Begin(ctx)
	if err := StartTournament(ctx, tx, tid); err != nil {
		t.Fatal(err)
	}
	tx.Commit(ctx)

	// Play every round: the home player wins, except every third match is a 1–1 draw.
	for round := 1; round <= 10; round++ {
		var status string
		db.Pool.QueryRow(ctx, "SELECT status FROM tournaments WHERE id = $1", tid).Scan(&status)
		if status == "finished" {
			break
		}
		rows, _ := db.Pool.Query(ctx, "SELECT id FROM matches WHERE tournament_id = $1 AND status = 'ready'", tid)
		var ids []string
		for rows.Next() {
			var id string
			rows.Scan(&id)
			ids = append(ids, id)
		}
		rows.Close()
		if len(ids) != 8 {
			t.Fatalf("round %d has %d matches, want 8", round, len(ids))
		}
		for i, id := range ids {
			sh, sa := 2, 0
			if i%3 == 0 {
				sh, sa = 1, 1
			}
			if err := AdminResolve(ctx, id, "test", sh, sa, nil, nil); err != nil {
				t.Fatalf("resolve: %v", err)
			}
		}
		AdvanceTournament(ctx, tid)
		time.Sleep(100 * time.Millisecond) // let the async advance from SettleMatch finish
	}

	var status string
	var rounds int
	db.Pool.QueryRow(ctx, "SELECT status, current_round FROM tournaments WHERE id = $1", tid).Scan(&status, &rounds)
	if status != "finished" || rounds != SwissRounds(16) {
		t.Fatalf("status %s after %d rounds; want finished after %d", status, rounds, SwissRounds(16))
	}
	var rematches int
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM (SELECT LEAST(home_id, away_id), GREATEST(home_id, away_id) FROM matches
		WHERE tournament_id = $1 GROUP BY 1, 2 HAVING COUNT(*) > 1) x`, tid).Scan(&rematches)
	if rematches != 0 {
		t.Errorf("%d rematches", rematches)
	}

	table, _ := Standings(ctx, db.Pool, tid)
	want := map[string][]int64{"GHS": {6000, 2500, 1500}, "NGN": {600000, 250000, 150000}}
	for i, s := range table {
		var cur string
		db.Pool.QueryRow(ctx, "SELECT currency FROM users WHERE id = $1", s.UserID).Scan(&cur)
		got := balanceOf(ctx, s.UserID)
		var expect int64
		if i < 3 {
			expect = want[cur][i]
		}
		if got != expect {
			t.Errorf("position %d (%s, %s): balance %d, want %d", i+1, s.UserID, cur, got, expect)
		}
	}
}

func TestKnockoutWinnerTakesFixedPrize(t *testing.T) {
	ctx := testDB(t)
	players := makeUsers(ctx, fmt.Sprintf("ko%d", os.Getpid()), 8)
	tid := fmt.Sprintf("ko-%d", os.Getpid())
	db.Pool.Exec(ctx, `INSERT INTO tournaments (id, name, game, format, entry_fee, max_players, prizes)
		VALUES ($1,'KO Cup','dls','knockout',0,8,'{"GHS":[6000],"NGN":[600000]}'::jsonb)`, tid)
	for _, p := range players {
		db.Pool.Exec(ctx, "INSERT INTO tournament_players (tournament_id, user_id) VALUES ($1,$2)", tid, p)
	}
	tx, _ := db.Pool.Begin(ctx)
	if err := StartTournament(ctx, tx, tid); err != nil {
		t.Fatal(err)
	}
	tx.Commit(ctx)
	for i := 0; i < 5; i++ {
		rows, _ := db.Pool.Query(ctx, "SELECT id FROM matches WHERE tournament_id = $1 AND status = 'ready'", tid)
		var ids []string
		for rows.Next() {
			var id string
			rows.Scan(&id)
			ids = append(ids, id)
		}
		rows.Close()
		for _, id := range ids {
			AdminResolve(ctx, id, "test", 3, 1, nil, nil)
		}
		AdvanceTournament(ctx, tid)
		time.Sleep(100 * time.Millisecond)
	}
	var winner, status string
	db.Pool.QueryRow(ctx, "SELECT status, COALESCE(winner_id,'') FROM tournaments WHERE id = $1", tid).Scan(&status, &winner)
	if status != "finished" || winner == "" {
		t.Fatalf("status %s winner %q", status, winner)
	}
	var cur string
	db.Pool.QueryRow(ctx, "SELECT currency FROM users WHERE id = $1", winner).Scan(&cur)
	want := map[string]int64{"GHS": 6000, "NGN": 600000}[cur]
	var paid, total int64
	db.Pool.QueryRow(ctx, "SELECT balance FROM users WHERE id = $1", winner).Scan(&paid)
	db.Pool.QueryRow(ctx, "SELECT COALESCE(SUM(amount),0) FROM transactions WHERE ref_id = $1 AND type = 'tournament_prize'", tid).Scan(&total)
	if paid != want || total != want {
		t.Fatalf("winner paid %d (total %d), want %d %s", paid, total, want, cur)
	}
}

func TestMembershipUnlocksReferralReward(t *testing.T) {
	ctx := testDB(t)
	u := makeUsers(ctx, fmt.Sprintf("mr%d", os.Getpid()), 2)
	inviter, friend := u[0], u[1] // GHS inviter, NGN friend
	db.Pool.Exec(ctx, "UPDATE users SET referred_by = $2, balance = 3_000_00 WHERE id = $1", friend, inviter)
	if _, err := BuyMembership(ctx, friend, "pro"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond) // reward runs in the background
	if b := balanceOf(ctx, inviter); b != ReferralInviterReward["GHS"] {
		t.Errorf("inviter balance %d, want %d", b, ReferralInviterReward["GHS"])
	}
	if b := balanceOf(ctx, friend); b != ReferralFriendReward["NGN"] {
		t.Errorf("friend balance %d, want %d", b, ReferralFriendReward["NGN"])
	}
}
