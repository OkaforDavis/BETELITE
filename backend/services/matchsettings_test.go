package services

import "testing"

func TestCleanSettings(t *testing.T) {
	// Preset fills the gaps; unknown rules are dropped.
	s, err := CleanSettings("codm", &MatchSettings{Mode: "tdm", Rules: []string{"no_snipers", "made_up"}}, "1v1")
	if err != nil || s.ScoreLimit != 50 || s.TimeLimit != 10 || s.Map != MapHostPicks || len(s.Rules) != 1 {
		t.Fatalf("tdm preset: %+v %v", s, err)
	}
	// Round-based modes have no time limit.
	s, _ = CleanSettings("codm", &MatchSettings{Mode: "gunfight", TimeLimit: 9}, "1v1")
	if s.TimeLimit != 0 || s.ScoreLimit != 6 {
		t.Fatalf("gunfight: %+v", s)
	}
	bad := []struct {
		in   MatchSettings
		size string
	}{
		{MatchSettings{Mode: "battle_royale"}, ""},
		{MatchSettings{Mode: "ffa", ScoreLimit: 500}, "ffa"},
		{MatchSettings{Mode: "tdm", TimeLimit: 90}, ""},
		{MatchSettings{Mode: "tdm", Map: "Not a map"}, ""},
		{MatchSettings{Mode: "snd"}, "ffa"}, // S&D isn't a free-for-all
	}
	for _, b := range bad {
		if _, err := CleanSettings("codm", &b.in, b.size); err == nil {
			t.Fatalf("accepted %+v as %q", b.in, b.size)
		}
	}
	// Games without settings ignore them; FFA cups must pick a mode.
	if s, err := CleanSettings("efootball", &MatchSettings{Mode: "tdm"}, "1v1"); s != nil || err != nil {
		t.Fatalf("efootball: %+v %v", s, err)
	}
	if _, err := CleanSettings("codm", nil, "ffa"); err == nil {
		t.Fatal("ffa cup without a mode accepted")
	}
	if got := SettingsSummary("codm", &MatchSettings{Mode: "ffa", ScoreLimit: 50, TimeLimit: 10, Map: "Crash"}); got != "Free for All · First to 50 kills · 10 min · Crash" {
		t.Fatalf("summary: %q", got)
	}
}
