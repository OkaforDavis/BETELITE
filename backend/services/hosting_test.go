package services

import "testing"

func TestHostFee(t *testing.T) {
	cases := map[int64]int64{10_000_00: 2_000_00, 2_000_00: 400_00, 5_00: 1_00, 1: 1}
	for prize, want := range cases {
		if got := HostFee(prize); got != want {
			t.Errorf("HostFee(%d) = %d, want %d", prize, got, want)
		}
	}
}

func TestSplitPoolAddsUp(t *testing.T) {
	for _, split := range [][]float64{{100}, {70, 30}, {50, 30, 20}, {60, 40}} {
		for _, pool := range []int64{2_000_00, 10_001_00, 33_333_33} {
			parts := SplitPool(pool, split)
			var sum int64
			for _, p := range parts {
				sum += p
			}
			if sum != pool {
				t.Errorf("SplitPool(%d, %v) = %v sums to %d", pool, split, parts, sum)
			}
			if len(parts) > 1 && parts[0] < parts[1] {
				t.Errorf("SplitPool(%d, %v): 1st place %d below 2nd %d", pool, split, parts[0], parts[1])
			}
		}
	}
}

func TestValidateSplit(t *testing.T) {
	ok := [][]float64{{100}, {70, 30}, {50, 30, 20}}
	for _, s := range ok {
		if err := ValidateSplit(s, 3); err != nil {
			t.Errorf("ValidateSplit(%v) = %v, want nil", s, err)
		}
	}
	bad := [][]float64{{}, {90}, {50, 50, 10}, {33.3, 33.3, 33.4}, {100, 0}, {-10, 110}}
	for _, s := range bad {
		if err := ValidateSplit(s, 3); err == nil {
			t.Errorf("ValidateSplit(%v) = nil, want error", s)
		}
	}
	if err := ValidateSplit([]float64{50, 30, 20}, 2); err == nil {
		t.Error("knockout should allow at most 2 places")
	}
}
