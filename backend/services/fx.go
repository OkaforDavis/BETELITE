package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"betelite-go/config"
)

// Exchange rates for cross-currency payments (a Ghana player paying in naira,
// or the reverse) and for moving a wallet to a new home country. The market
// mid-rate is fetched a few times a day; FX_NGN_PER_GHS can pin it instead.
// Every conversion includes FX_MARGIN_PCT (default 3%) in CrestArena's favour.

var fxHTTP = &http.Client{Timeout: 10 * time.Second}

// fxSourceURL is a variable so tests can point it at a fake server.
var fxSourceURL = "https://open.er-api.com/v6/latest/GHS"

var (
	fxMu      sync.Mutex
	fxRate    float64 // NGN per GHS
	fxFetched time.Time
)

const fxMaxAge = 6 * time.Hour

// NGNPerGHS returns the market mid-rate (naira per cedi).
func NGNPerGHS(ctx context.Context) (float64, error) {
	if config.Cfg.FXNGNPerGHS > 0 {
		return config.Cfg.FXNGNPerGHS, nil
	}
	fxMu.Lock()
	defer fxMu.Unlock()
	if fxRate > 0 && time.Since(fxFetched) < fxMaxAge {
		return fxRate, nil
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", fxSourceURL, nil)
	resp, err := fxHTTP.Do(req)
	if err != nil {
		if fxRate > 0 { // keep using the last good rate for up to a day
			if time.Since(fxFetched) < 24*time.Hour {
				return fxRate, nil
			}
		}
		return 0, fmt.Errorf("exchange rate unavailable: %w", err)
	}
	defer resp.Body.Close()
	var d struct {
		Result string             `json:"result"`
		Rates  map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil || d.Rates["NGN"] <= 0 {
		if fxRate > 0 && time.Since(fxFetched) < 24*time.Hour {
			return fxRate, nil
		}
		return 0, fmt.Errorf("exchange rate unavailable")
	}
	fxRate, fxFetched = d.Rates["NGN"], time.Now()
	return fxRate, nil
}

// midRate returns how many units of `to` one unit of `from` buys (NGN/GHS only).
func midRate(ctx context.Context, from, to string) (float64, error) {
	if from == to {
		return 1, nil
	}
	r, err := NGNPerGHS(ctx)
	if err != nil {
		return 0, err
	}
	switch {
	case from == "GHS" && to == "NGN":
		return r, nil
	case from == "NGN" && to == "GHS":
		return 1 / r, nil
	}
	return 0, userErr(400, "Payments in %s are not available yet", to)
}

// PayAmount is what a player must pay in payCurrency so that `amount` (in the
// wallet currency) is credited: the mid-rate plus the margin, rounded up.
func PayAmount(ctx context.Context, amount int64, walletCurrency, payCurrency string) (int64, float64, error) {
	if walletCurrency == payCurrency {
		return amount, 1, nil
	}
	r, err := midRate(ctx, walletCurrency, payCurrency)
	if err != nil {
		return 0, 0, err
	}
	rate := r * (1 + config.Cfg.FXMarginPct/100)
	return int64(math.Ceil(float64(amount)*rate - 1e-6)), rate, nil // tolerance absorbs float noise
}

// ConvertOut is what `amount` becomes when a wallet moves to another currency:
// the mid-rate less the margin, rounded down.
func ConvertOut(ctx context.Context, amount int64, from, to string) (int64, float64, error) {
	if from == to {
		return amount, 1, nil
	}
	r, err := midRate(ctx, from, to)
	if err != nil {
		return 0, 0, err
	}
	rate := r * (1 - config.Cfg.FXMarginPct/100)
	return int64(math.Floor(float64(amount)*rate + 1e-6)), rate, nil
}
