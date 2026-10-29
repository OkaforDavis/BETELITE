package services

import (
	"betelite-go/config"
)

// Payment providers, tried in the order of PAYMENT_PROVIDERS (default
// flutterwave, then paystack as the fallback).

const (
	ProviderFlutterwave = "flutterwave"
	ProviderPaystack    = "paystack"
)

// providerSupports reports whether a provider is configured for a currency.
func providerSupports(provider, currency string) bool {
	switch provider {
	case ProviderFlutterwave:
		if !FlutterwaveReady() {
			return false
		}
		return currency == "NGN" || (currency == "GHS" && config.Cfg.FlutterwaveGHS)
	case ProviderPaystack:
		return PaystackSecret(currency) != "" && (currency == "NGN" || (currency == "GHS" && config.Cfg.PaystackSecretKeyGH != ""))
	}
	return false
}

// ProvidersFor lists the configured providers for a currency, preferred first.
func ProvidersFor(currency string) []string {
	var out []string
	for _, p := range config.Cfg.PaymentProviders {
		if providerSupports(p, currency) {
			out = append(out, p)
		}
	}
	return out
}

// PayCurrencies are the currencies players can pay in right now.
func PayCurrencies() []string {
	var out []string
	for _, c := range []string{"NGN", "GHS"} {
		if len(ProvidersFor(c)) > 0 {
			out = append(out, c)
		}
	}
	return out
}

// Collection fees per provider (minor units). Paystack: paystack.com/pricing.
// Flutterwave: local Nigerian payments 1.4% capped at ₦2,000; Ghana mobile
// money and cards about 2% (check the Flutterwave dashboard pricing page).
var providerCollectionFees = map[string]map[string]FeeRule{
	ProviderPaystack:    CollectionFees,
	ProviderFlutterwave: {"NGN": {Percent: 1.4, Cap: 2_000_00}, "GHS": {Percent: 2}},
}

// FeeRuleFor returns a provider's collection fee for a currency.
func FeeRuleFor(provider, currency string) FeeRule {
	return providerCollectionFees[provider][currency]
}

// GrossUpFor is the smallest charge that leaves net after the provider's fee.
func GrossUpFor(provider string, net int64, currency string) int64 {
	rule := FeeRuleFor(provider, currency)
	fee := func(g int64) int64 { return ruleFee(rule, g) }
	if net <= 0 {
		return 0
	}
	gross := net
	for i := 0; i < 64 && gross-fee(gross) < net; i++ {
		gross += max(net-(gross-fee(gross)), 1)
	}
	return gross
}

func ruleFee(r FeeRule, gross int64) int64 {
	fee := (gross*int64(r.Percent*100) + 9_999) / 10_000
	if r.Flat > 0 && gross >= r.FlatFrom {
		fee += r.Flat
	}
	if r.Cap > 0 && fee > r.Cap {
		fee = r.Cap
	}
	return fee
}

// TransferFeeFor is the provider's fee for sending a withdrawal.
// Flutterwave (Nigeria): ₦10.75 up to ₦5,000, ₦26.88 up to ₦50,000, ₦53.75 above;
// Ghana mobile money about 1.5% (minimum ₵1).
func TransferFeeFor(provider string, amount int64, currency, bankType string) int64 {
	if provider != ProviderFlutterwave {
		return TransferFee(amount, currency, bankType)
	}
	if currency == "GHS" {
		return max(amount*15/1000, 1_00)
	}
	switch {
	case amount <= 5_000_00:
		return 10_75
	case amount <= 50_000_00:
		return 26_88
	default:
		return 53_75
	}
}

// PayoutProvider is the provider used for withdrawals in a currency.
func PayoutProvider(currency string) string {
	if list := ProvidersFor(currency); len(list) > 0 {
		return list[0]
	}
	return ProviderPaystack
}
