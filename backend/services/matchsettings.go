package services

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// Match settings: how a game room must be set up (mode, map, score and time
// limits, loadout rules). The cup host or challenge poster picks a mode; the
// preset fills the rest, which they can change. Players see a one-line
// summary and can open "See all settings".

type MatchSettings struct {
	Mode       string   `json:"mode"`
	Map        string   `json:"map,omitempty"`
	ScoreLimit int      `json:"scoreLimit,omitempty"`
	TimeLimit  int      `json:"timeLimit,omitempty"` // minutes; 0 = not used (round-based modes)
	Rules      []string `json:"rules,omitempty"`
}

type ModePreset struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	ScoreLabel string   `json:"scoreLabel"` // "kills", "rounds", "points"
	Score      int      `json:"score"`
	MaxScore   int      `json:"maxScore"`
	Time       int      `json:"time"`  // default minutes (0 = round-based)
	Sizes      []string `json:"sizes"` // "1v1", "2v2", "3v3", "5v5", "ffa"
	Tip        string   `json:"tip,omitempty"`
}

type RuleOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type SettingsSpec struct {
	Modes []ModePreset `json:"modes"`
	Maps  []string     `json:"maps"`
	Rules []RuleOption `json:"rules"`
}

const (
	MapHostPicks = "Host picks"
	MapRandom    = "Random"
)

var codmSettings = &SettingsSpec{
	Modes: []ModePreset{
		{ID: "ffa", Name: "Free for All", ScoreLabel: "kills", Score: 30, MaxScore: 100, Time: 10, Sizes: []string{"ffa", "1v1"},
			Tip: "Everyone for themselves. First to the kill limit wins; if time runs out, the most kills wins."},
		{ID: "gunfight", Name: "Gunfight", ScoreLabel: "rounds", Score: 6, MaxScore: 10, Sizes: []string{"1v1", "2v2"},
			Tip: "Short rounds, same random loadout for both sides. First to the round limit wins."},
		{ID: "tdm", Name: "Team Deathmatch", ScoreLabel: "kills", Score: 50, MaxScore: 150, Time: 10, Sizes: []string{"1v1", "2v2", "3v3", "5v5"},
			Tip: "First side to the kill limit wins; if time runs out, the most kills wins."},
		{ID: "snd", Name: "Search & Destroy", ScoreLabel: "rounds", Score: 4, MaxScore: 7, Sizes: []string{"1v1", "2v2", "3v3", "5v5"},
			Tip: "Plant or defuse the bomb. No respawns. First to the round limit wins."},
		{ID: "hardpoint", Name: "Hardpoint", ScoreLabel: "points", Score: 150, MaxScore: 250, Time: 10, Sizes: []string{"1v1", "2v2", "3v3", "5v5"},
			Tip: "Hold the moving hill to score. First to the point limit wins."},
		{ID: "domination", Name: "Domination", ScoreLabel: "points", Score: 150, MaxScore: 250, Time: 10, Sizes: []string{"1v1", "2v2", "3v3", "5v5"},
			Tip: "Capture and hold flags A, B and C. First to the point limit wins."},
	},
	Maps: []string{MapHostPicks, MapRandom, "Nuketown", "Crash", "Crossfire", "Firing Range", "Killhouse", "Shipment", "Raid", "Standoff",
		"Summit", "Hijacked", "Terminal", "Rust", "Meltdown", "Tunisia", "Scrapyard", "Hackney Yard"},
	Rules: []RuleOption{
		{"no_scorestreaks", "No scorestreaks"},
		{"no_operator", "No operator skills"},
		{"no_snipers", "No snipers"},
		{"no_shotguns", "No shotguns"},
		{"no_launchers", "No launchers"},
		{"no_lethals", "No lethal grenades"},
	},
}

// SettingsFor returns the settings spec of a game (nil when the game has no
// room settings to pick).
func SettingsFor(game string) *SettingsSpec {
	if game == "codm" {
		return codmSettings
	}
	return nil
}

func (s *SettingsSpec) mode(id string) *ModePreset {
	for i := range s.Modes {
		if s.Modes[i].ID == id {
			return &s.Modes[i]
		}
	}
	return nil
}

// CleanSettings checks settings for a game and fills gaps from the preset.
// size is "1v1" for challenges, "ffa" for Free-for-All cups, "" for any.
// It returns nil (no settings) for games without a settings spec.
func CleanSettings(game string, in *MatchSettings, size string) (*MatchSettings, error) {
	spec := SettingsFor(game)
	if spec == nil || in == nil || in.Mode == "" {
		if spec != nil && size == "ffa" {
			return nil, userErr(400, "Pick the game mode for this cup")
		}
		return nil, nil
	}
	m := spec.mode(in.Mode)
	if m == nil {
		return nil, userErr(400, "Pick one of the listed game modes")
	}
	if size != "" && !slices.Contains(m.Sizes, size) {
		return nil, userErr(400, "%s can't be played as %s", m.Name, sizeName(size))
	}
	out := &MatchSettings{Mode: m.ID, ScoreLimit: in.ScoreLimit, TimeLimit: in.TimeLimit}
	if out.ScoreLimit == 0 {
		out.ScoreLimit = m.Score
	}
	if out.ScoreLimit < 1 || out.ScoreLimit > m.MaxScore {
		return nil, userErr(400, "The %s limit for %s must be between 1 and %d", strings.TrimSuffix(m.ScoreLabel, "s"), m.Name, m.MaxScore)
	}
	if m.Time == 0 {
		out.TimeLimit = 0
	} else if out.TimeLimit == 0 {
		out.TimeLimit = m.Time
	} else if out.TimeLimit < 1 || out.TimeLimit > 30 {
		return nil, userErr(400, "The time limit must be between 1 and 30 minutes")
	}
	out.Map = strings.TrimSpace(in.Map)
	if out.Map == "" {
		out.Map = MapHostPicks
	}
	if !slices.Contains(spec.Maps, out.Map) {
		return nil, userErr(400, "Pick one of the listed maps")
	}
	for _, r := range spec.Rules {
		if slices.Contains(in.Rules, r.ID) {
			out.Rules = append(out.Rules, r.ID)
		}
	}
	return out, nil
}

func sizeName(size string) string {
	if size == "ffa" {
		return "a Free-for-All"
	}
	return "a " + size
}

// SettingsSummary is the one-line summary, e.g.
// "Free for All · First to 50 kills · 10 min · Crash".
func SettingsSummary(game string, s *MatchSettings) string {
	spec := SettingsFor(game)
	if spec == nil || s == nil {
		return ""
	}
	m := spec.mode(s.Mode)
	if m == nil {
		return ""
	}
	parts := []string{m.Name, fmt.Sprintf("First to %d %s", s.ScoreLimit, m.ScoreLabel)}
	if s.TimeLimit > 0 {
		parts = append(parts, fmt.Sprintf("%d min", s.TimeLimit))
	}
	if s.Map != "" && s.Map != MapHostPicks {
		parts = append(parts, s.Map)
	}
	return strings.Join(parts, " · ")
}

// settingsJSON stores settings (nil → SQL NULL).
func settingsJSON(s *MatchSettings) any {
	if s == nil {
		return nil
	}
	b, _ := json.Marshal(s)
	return string(b)
}

// SettingsJSON is settingsJSON for the routes package.
func SettingsJSON(s *MatchSettings) any { return settingsJSON(s) }

// ParseSettings reads stored settings (nil when empty).
func ParseSettings(b []byte) *MatchSettings {
	if len(b) == 0 {
		return nil
	}
	var s MatchSettings
	if json.Unmarshal(b, &s) != nil || s.Mode == "" {
		return nil
	}
	return &s
}
