package services

import (
	"fmt"
	"testing"
)

func TestSwissRounds(t *testing.T) {
	for players, want := range map[int]int{4: 2, 8: 3, 16: 4, 32: 5, 64: 6} {
		if got := SwissRounds(players); got != want {
			t.Errorf("SwissRounds(%d) = %d, want %d", players, got, want)
		}
	}
}

func TestSwissPairsAvoidRematches(t *testing.T) {
	ids := []string{"a", "b", "c", "d"}
	met := map[[2]string]bool{}
	meet := func(x, y string) { met[[2]string{x, y}], met[[2]string{y, x}] = true, true }
	meet("a", "b")
	meet("c", "d")
	// Greedy a–c would leave b–d (fine); a must not meet b again.
	pairs := swissPairs(ids, met)
	if len(pairs) != 2 {
		t.Fatalf("got %v", pairs)
	}
	for _, p := range pairs {
		if met[[2]string{p[0], p[1]}] {
			t.Errorf("rematch %v in %v", p, pairs)
		}
	}
}

func TestSwissPairsBacktracks(t *testing.T) {
	// a has met b and c; a greedy b–c pairing would strand a with d... which is fine,
	// but if d has also met everyone except a, every player must still be paired once.
	ids := []string{"a", "b", "c", "d", "e", "f"}
	met := map[[2]string]bool{}
	meet := func(x, y string) { met[[2]string{x, y}], met[[2]string{y, x}] = true, true }
	meet("a", "b")
	meet("a", "c")
	meet("e", "f")
	meet("d", "e")
	pairs := swissPairs(ids, met)
	seen := map[string]bool{}
	for _, p := range pairs {
		if met[[2]string{p[0], p[1]}] {
			t.Errorf("rematch %v in %v", p, pairs)
		}
		seen[p[0]], seen[p[1]] = true, true
	}
	if len(seen) != len(ids) {
		t.Errorf("not everyone paired: %v", pairs)
	}
}

func TestSwissPairsAllowsRematchWhenUnavoidable(t *testing.T) {
	met := map[[2]string]bool{{"a", "b"}: true, {"b", "a"}: true}
	pairs := swissPairs([]string{"a", "b"}, met)
	if fmt.Sprint(pairs) != "[[a b]]" {
		t.Errorf("got %v", pairs)
	}
}

func TestFixedPrizes(t *testing.T) {
	p := ParseFixedPrizes([]byte(`{"GHS":[8000,4000,3000],"NGN":[800000,400000,300000]}`))
	if p.Places() != 3 || p["GHS"][0] != 8000 {
		t.Errorf("parsed %v", p)
	}
	if ParseFixedPrizes(nil) != nil || ParseFixedPrizes([]byte(`{}`)) != nil {
		t.Error("empty prizes should be nil")
	}
}

func TestTiers(t *testing.T) {
	want := map[string][3]int64{"free": {1, 0, 0}, "pro": {2, 30_00, 3_000_00}, "pro_plus": {3, 50_00, 5_000_00}, "elite": {5, 80_00, 8_000_00}}
	for id, w := range want {
		tier := TierByID(id)
		if tier == nil || int64(tier.Entries) != w[0] || tier.Prices["GHS"] != w[1] || tier.Prices["NGN"] != w[2] {
			t.Errorf("tier %s = %+v", id, tier)
		}
	}
	if err := checkPlanChange(&Membership{Tier: "pro"}, TierByID("elite")); err == nil {
		t.Error("switching tier during an active plan should be refused")
	}
}
