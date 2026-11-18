package services

import (
	"strings"
	"testing"
)

func TestRoomSpecsComplete(t *testing.T) {
	for _, g := range Games() {
		r := g.Room
		switch r.Mode {
		case "shared":
		case "host":
			if r.Host != "home" && r.Host != "either" {
				t.Errorf("%s: host must be home or either, got %q", g.ID, r.Host)
			}
			if len(r.JoinSteps) == 0 {
				t.Errorf("%s: host games need join steps", g.ID)
			}
		default:
			t.Errorf("%s: unknown room mode %q", g.ID, r.Mode)
		}
		if len(r.HostSteps) == 0 || r.CodeLabel == "" {
			t.Errorf("%s: missing steps or code label", g.ID)
		}
	}
}

func TestValidRoomText(t *testing.T) {
	for s, want := range map[string]bool{
		"12345678": true, "AB-12_cd": true, "Room #4": true,
		"ab": false, strings.Repeat("9", 25): false, "<script>": false, "12;DROP": false,
	} {
		if got := validRoomText(s, 3, 24); got != want {
			t.Errorf("validRoomText(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestCleanJoinLink(t *testing.T) {
	for in, want := range map[string]string{
		"https://example.com/join?c=AB12": "https://example.com/join?c=AB12",
		"http://example.com/join":         "",
		"javascript:alert(1)":             "",
		"https://user:pw@example.com/x":   "",
		"https://localhost/x":             "",
		"example.com/x":                   "",
	} {
		if got := cleanJoinLink(in); got != want {
			t.Errorf("cleanJoinLink(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRandomCode(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		c := randomCode(6)
		if len(c) != 6 || strings.ContainsAny(c, "01OI") {
			t.Fatalf("bad code %q", c)
		}
		seen[c] = true
	}
	if len(seen) < 190 {
		t.Fatalf("codes repeat too often: %d unique of 200", len(seen))
	}
}
