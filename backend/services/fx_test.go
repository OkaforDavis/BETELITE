package services

import (
	"context"
	"testing"

	"betelite-go/config"
)

func withFX(t *testing.T, rate, margin float64) {
	t.Helper()
	old := config.Cfg
	config.Cfg.FXNGNPerGHS, config.Cfg.FXMarginPct = rate, margin
	t.Cleanup(func() { config.Cfg = old })
}

// The worked examples from the plan: ₵1 = ₦115, 3% margin.
func TestPayAmountAndConvertOut(t *testing.T) {
	withFX(t, 115, 3)
	ctx := context.Background()
	cases := []struct {
		name           string
		amount         int64
		from, to       string
		pay, converted int64
	}{
		{"₵100 wallet paid in naira", 100_00, "GHS", "NGN", 11_845_00, 11_155_00},
		{"₦11,500 wallet paid in cedis", 11_500_00, "NGN", "GHS", 103_00, 97_00},
		{"same currency", 3_600_00, "NGN", "NGN", 3_600_00, 3_600_00},
	}
	for _, c := range cases {
		pay, _, err := PayAmount(ctx, c.amount, c.from, c.to)
		if err != nil || pay != c.pay {
			t.Errorf("%s: PayAmount = %d (%v), want %d", c.name, pay, err, c.pay)
		}
		got, _, err := ConvertOut(ctx, c.amount, c.from, c.to)
		if err != nil || got != c.converted {
			t.Errorf("%s: ConvertOut = %d (%v), want %d", c.name, got, err, c.converted)
		}
	}
	// A round trip always loses value, so nobody can profit from converting back and forth.
	pay, _, _ := PayAmount(ctx, 100_00, "GHS", "NGN")
	back, _, _ := ConvertOut(ctx, pay, "NGN", "GHS")
	if back >= 100_00 {
		t.Errorf("round trip returned %d pesewas from ₵100", back)
	}
	if _, _, err := PayAmount(ctx, 100, "NGN", "USD"); err == nil {
		t.Error("USD should not be payable yet")
	}
}

func TestProvidersOrderAndFees(t *testing.T) {
	old := config.Cfg
	t.Cleanup(func() { config.Cfg = old })
	config.Cfg.PaymentProviders = []string{"flutterwave", "paystack"}
	config.Cfg.FlutterwaveSecretKey = "FLWSECK-test"
	config.Cfg.PaystackSecretKey = "sk_test"
	config.Cfg.PaystackSecretKeyGH = ""
	config.Cfg.FlutterwaveGHS = false

	if got := ProvidersFor("NGN"); len(got) != 2 || got[0] != ProviderFlutterwave {
		t.Errorf("NGN providers = %v, want flutterwave first then paystack", got)
	}
	if got := ProvidersFor("GHS"); len(got) != 0 {
		t.Errorf("GHS should be off until Flutterwave enables it, got %v", got)
	}
	config.Cfg.FlutterwaveGHS = true
	if got := ProvidersFor("GHS"); len(got) != 1 || got[0] != ProviderFlutterwave {
		t.Errorf("GHS providers = %v", got)
	}
	// Flutterwave Nigeria: 1.4% capped at ₦2,000; the grossed-up charge leaves the full amount.
	for _, net := range []int64{1_000_00, 3_600_00, 200_000_00} {
		g := GrossUpFor(ProviderFlutterwave, net, "NGN")
		if g-ruleFee(FeeRuleFor(ProviderFlutterwave, "NGN"), g) < net {
			t.Errorf("GrossUpFor(%d) = %d does not cover the fee", net, g)
		}
	}
	if TransferFeeFor(ProviderFlutterwave, 4_000_00, "NGN", "") != 10_75 || TransferFeeFor(ProviderFlutterwave, 200_00, "GHS", "mobile_money") != 3_00 {
		t.Error("Flutterwave transfer fees")
	}
}
