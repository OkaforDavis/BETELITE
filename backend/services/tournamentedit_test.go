package services

import "testing"

func TestValidCupSize(t *testing.T) {
	ok := map[string][]int{"knockout": {4, 8, 64}, "swiss": {8, 32}, "league": {4, 8}}
	bad := map[string][]int{"knockout": {6, 128}, "swiss": {4, 12}, "league": {3, 9}, "other": {8}}
	for f, ns := range ok {
		for _, n := range ns {
			if validCupSize(f, n) != nil {
				t.Errorf("%s %d should be allowed", f, n)
			}
		}
	}
	for f, ns := range bad {
		for _, n := range ns {
			if validCupSize(f, n) == nil {
				t.Errorf("%s %d should be refused", f, n)
			}
		}
	}
}
