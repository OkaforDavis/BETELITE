package services

import (
	"testing"
)

func TestNamesMatch(t *testing.T) {
	cases := []struct {
		read, registered string
		want             bool
	}{
		{"DAVIS_99", "davis99", true},
		{"Davis 99", "Davis99", true},
		{"BobKlng99", "BobKing99", true}, // one OCR slip in a long name
		{"BobKing", "BobKing99", false},
		{"Bob", "Rob", false}, // short names must match exactly
		{"UKRAINE", "AliceFC", false},
		{"", "AliceFC", false},
	}
	for _, c := range cases {
		if got := NamesMatch(c.read, c.registered); got != c.want {
			t.Errorf("NamesMatch(%q, %q) = %v, want %v", c.read, c.registered, got, c.want)
		}
	}
}

func TestMatchPlayersMapsSides(t *testing.T) {
	res := &OCRResult{Players: []OCRPlayer{{Name: "BobKing99", Score: 1}, {Name: "AliceFC", Score: 3}}}
	h, a, err := MatchPlayers(res, ExpectedPlayer{Gamertag: "AliceFC"}, ExpectedPlayer{Gamertag: "BobKing99"})
	if err != nil || h != 1 || a != 0 {
		t.Fatalf("got h=%d a=%d err=%v; want home on the right side", h, a, err)
	}

	// The scenario screenshot: national team names, no usernames.
	res = &OCRResult{Players: []OCRPlayer{{Name: "UKRAINE"}, {Name: "FRANCE"}}}
	if _, _, err := MatchPlayers(res, ExpectedPlayer{Gamertag: "AliceFC"}, ExpectedPlayer{Gamertag: "BobKing99"}); err == nil {
		t.Fatal("team names must not match registered players")
	}
}

func TestRoundRobin(t *testing.T) {
	for _, n := range []int{2, 3, 4, 7, 16} {
		players := make([]string, n)
		for i := range players {
			players[i] = string(rune('A' + i))
		}
		seen := map[string]bool{}
		for _, round := range roundRobin(players) {
			inRound := map[string]bool{}
			for _, p := range round {
				if inRound[p[0]] || inRound[p[1]] {
					t.Fatalf("n=%d: player plays twice in one round", n)
				}
				inRound[p[0]], inRound[p[1]] = true, true
				key := min(p[0], p[1]) + max(p[0], p[1])
				if seen[key] {
					t.Fatalf("n=%d: pairing %s repeated", n, key)
				}
				seen[key] = true
			}
		}
		if want := n * (n - 1) / 2; len(seen) != want {
			t.Fatalf("n=%d: %d pairings, want %d", n, len(seen), want)
		}
	}
}

func TestPrizeAmounts(t *testing.T) {
	// Knockout, 16 players × ₦1,000: winner gets 70% = ₦11,200.
	if got := PrizeAmounts(1000_00, 16, 0, DefaultKnockoutSplit); got[0] != 11_200_00 {
		t.Errorf("knockout prize = %d", got[0])
	}
	// League, 16 × ₦1,000 = ₦16,000: 30% / 10% / 4.5% / 4.5%.
	got := PrizeAmounts(1000_00, 16, 0, DefaultLeagueSplit)
	want := []int64{4_800_00, 1_600_00, 720_00, 720_00}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("league prize %d = %d, want %d", i+1, got[i], want[i])
		}
	}
	// Free tournament: the seed is fully distributed in the split's proportions.
	if got := PrizeAmounts(0, 8, 5000_00, []float64{70}); got[0] != 5000_00 {
		t.Errorf("free knockout prize = %d", got[0])
	}
}

func TestFormatMoney(t *testing.T) {
	if s := FormatMoney(1_234_567_00, "NGN"); s != "₦1,234,567" {
		t.Errorf("got %s", s)
	}
	if s := FormatMoney(50_50, "GHS"); s != "₵50.50" {
		t.Errorf("got %s", s)
	}
}
