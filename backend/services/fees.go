package services

// Payment fees are passed on to the player so CrestArena doesn't lose money
// on them: a checkout adds Paystack's collection fee on top of the amount,
// and a withdrawal deducts Paystack's transfer fee from what is sent.
// Rates from paystack.com/pricing (October 2026); amounts in minor units.

// FeeRule describes Paystack's collection fee for one currency.
type FeeRule struct {
	Percent  float64 `json:"percent"`  // e.g. 1.5 for 1.5%
	Flat     int64   `json:"flat"`     // added when the charge is at least FlatFrom
	FlatFrom int64   `json:"flatFrom"` // 0 = always
	Cap      int64   `json:"cap"`      // 0 = no cap
}

var CollectionFees = map[string]FeeRule{
	"NGN": {Percent: 1.5, Flat: 100_00, FlatFrom: 2_500_00, Cap: 2_000_00},
	"GHS": {Percent: 1.95},
}

// GatewayFee is what Paystack keeps from a charge of gross.
func GatewayFee(gross int64, currency string) int64 {
	r, ok := CollectionFees[currency]
	if !ok {
		return 0
	}
	fee := (gross*int64(r.Percent*100) + 9_999) / 10_000 // round up
	if r.Flat > 0 && gross >= r.FlatFrom {
		fee += r.Flat
	}
	if r.Cap > 0 && fee > r.Cap {
		fee = r.Cap
	}
	return fee
}

// GrossUp is the smallest charge that leaves net after Paystack's fee.
func GrossUp(net int64, currency string) int64 {
	if net <= 0 {
		return 0
	}
	gross := net
	for i := 0; i < 64 && gross-GatewayFee(gross, currency) < net; i++ {
		short := net - (gross - GatewayFee(gross, currency))
		gross += max(short, 1)
	}
	return gross
}

// TransferFee is Paystack's fee for sending a withdrawal. Ghana charges
// differently for mobile money wallets and bank accounts.
func TransferFee(amount int64, currency, bankType string) int64 {
	switch currency {
	case "GHS":
		if bankType == "mobile_money" {
			return 1_00
		}
		return 8_00
	default:
		switch {
		case amount <= 5_000_00:
			return 10_00
		case amount <= 50_000_00:
			return 25_00
		default:
			return 50_00
		}
	}
}
