package routes

import "testing"

func TestByteRange(t *testing.T) {
	cases := []struct {
		h          string
		start, end int
		ok         bool
	}{
		{"bytes=0-", 0, 99, true},
		{"bytes=0-1", 0, 1, true},
		{"bytes=10-500", 10, 99, true},
		{"bytes=-10", 90, 99, true},
		{"bytes=100-", 0, 0, false},
		{"bytes=5-2", 0, 0, false},
		{"bytes=0-1,5-6", 0, 0, false},
		{"", 0, 0, false},
	}
	for _, c := range cases {
		s, e, ok := byteRange(c.h, 100)
		if ok != c.ok || (ok && (s != c.start || e != c.end)) {
			t.Errorf("%q: got %d-%d %v, want %d-%d %v", c.h, s, e, ok, c.start, c.end, c.ok)
		}
	}
}
