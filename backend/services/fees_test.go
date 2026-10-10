package services

import "testing"

func TestGatewayFee(t *testing.T) {
	cases := []struct {
		gross int64
		cur   string
		want  int64
	}{
		{1_000_00, "NGN", 15_00},          // 1.5%, no flat fee under ₦2,500
		{3_000_00, "NGN", 45_00 + 100_00}, // 1.5% + ₦100
		{500_000_00, "NGN", 2_000_00},     // capped at ₦2,000
		{30_00, "GHS", 59},                // 1.95% of ₵30 = 58.5 pesewas, rounded up
	}
	for _, c := range cases {
		if got := GatewayFee(c.gross, c.cur); got != c.want {
			t.Errorf("GatewayFee(%d %s) = %d, want %d", c.gross, c.cur, got, c.want)
		}
	}
}

// After Paystack takes its fee from the grossed-up charge, at least the
// intended amount must remain, and charging one unit less must not be enough.
func TestGrossUpCoversFee(t *testing.T) {
	for _, cur := range []string{"NGN", "GHS"} {
		for _, net := range []int64{1, 10_00, 30_00, 2_400_00, 2_462_00, 3_000_00, 8_000_00, 140_000_00, 1_000_000_00} {
			g := GrossUp(net, cur)
			if g-GatewayFee(g, cur) < net {
				t.Errorf("%s: GrossUp(%d) = %d leaves %d", cur, net, g, g-GatewayFee(g, cur))
			}
			if g > net && (g-1)-GatewayFee(g-1, cur) >= net {
				t.Errorf("%s: GrossUp(%d) = %d is not the smallest charge", cur, net, g)
			}
		}
	}
}

func TestTransferFee(t *testing.T) {
	if TransferFee(5_000_00, "NGN", "") != 10_00 || TransferFee(5_000_01, "NGN", "") != 25_00 || TransferFee(60_000_00, "NGN", "") != 50_00 {
		t.Error("NGN transfer tiers")
	}
	if TransferFee(100_00, "GHS", "mobile_money") != 1_00 || TransferFee(100_00, "GHS", "ghipss") != 8_00 {
		t.Error("GHS transfer fees")
	}
}
