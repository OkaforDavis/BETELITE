package services

import (
	"context"
	"fmt"
	"time"

	"betelite-go/db"
)

// A player's home country sets their wallet currency, prices and where
// withdrawals go. Travellers just pay in the local currency; someone who has
// actually moved (seen in another supported country for 30+ days) can move
// their wallet there, at most once every 90 days. The balance converts at the
// day's rate less the FX margin. Admins can also do it after a complaint.

const (
	HomeSwitchAfter   = 30 * 24 * time.Hour
	HomeSwitchEvery   = 90 * 24 * time.Hour
	homeSwitchMinDays = 30
)

// HomeSwitch describes whether a player can move their wallet now.
type HomeSwitch struct {
	SeenCountry string `json:"seenCountry,omitempty"`
	To          string `json:"to,omitempty"`       // target currency, when relevant
	Eligible    bool   `json:"eligible"`           // can switch right now
	DaysLeft    int    `json:"daysLeft,omitempty"` // days until eligible
	Reason      string `json:"reason,omitempty"`
}

// RecordSeenCountry remembers where a player is using the app from, and since when.
func RecordSeenCountry(ctx context.Context, uid, country string) {
	if db.Pool == nil || country == "" {
		return
	}
	db.Pool.Exec(ctx, `UPDATE users SET seen_country_since = CASE WHEN seen_country IS DISTINCT FROM $2 OR seen_country_since IS NULL THEN NOW() ELSE seen_country_since END,
		seen_country = $2 WHERE id = $1`, uid, country)
}

// HomeSwitchStatus reports whether the player can move their wallet to the
// country they have been using the app from.
func HomeSwitchStatus(ctx context.Context, uid string) HomeSwitch {
	var currency string
	var seen *string
	var since, changed *time.Time
	if err := db.Pool.QueryRow(ctx, "SELECT COALESCE(currency,'NGN'), seen_country, seen_country_since, currency_changed_at FROM users WHERE id = $1", uid).
		Scan(&currency, &seen, &since, &changed); err != nil || seen == nil {
		return HomeSwitch{}
	}
	to := CurrencyForCountry(*seen)
	hs := HomeSwitch{SeenCountry: *seen}
	if to == currency || (to != "NGN" && to != "GHS") {
		return hs
	}
	hs.To = to
	if changed != nil && time.Since(*changed) < HomeSwitchEvery {
		hs.DaysLeft = int((HomeSwitchEvery-time.Since(*changed)).Hours()/24) + 1
		hs.Reason = "You changed country recently"
		return hs
	}
	if since == nil || time.Since(*since) < HomeSwitchAfter {
		d := 0
		if since != nil {
			d = int(time.Since(*since).Hours() / 24)
		}
		hs.DaysLeft = max(homeSwitchMinDays-d, 1)
		hs.Reason = "You can move your wallet after 30 days in the new country"
		return hs
	}
	hs.Eligible = true
	return hs
}

// SwitchHomeCurrency moves a wallet to another currency. force skips the
// 30/90-day rules (admins, after a complaint).
func SwitchHomeCurrency(ctx context.Context, uid, to string, force bool) (int64, error) {
	if to != "NGN" && to != "GHS" {
		return 0, userErr(400, "Choose Nigeria (₦) or Ghana (₵)")
	}
	if !force {
		hs := HomeSwitchStatus(ctx, uid)
		if !hs.Eligible || hs.To != to {
			if hs.Reason != "" {
				return 0, userErr(403, "%s (%d days left). Contact support if you've moved.", hs.Reason, hs.DaysLeft)
			}
			return 0, userErr(403, "You can only move your wallet to the country you're living in")
		}
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var from string
	var balance int64
	if err := tx.QueryRow(ctx, "SELECT COALESCE(currency,'NGN'), balance FROM users WHERE id = $1 FOR UPDATE", uid).Scan(&from, &balance); err != nil {
		return 0, err
	}
	if from == to {
		return 0, userErr(409, "Your wallet is already in %s", to)
	}
	var pending int
	tx.QueryRow(ctx, `SELECT (SELECT COUNT(*) FROM withdrawals WHERE user_id = $1 AND status IN ('pending','processing'))
		+ (SELECT COUNT(*) FROM escrow WHERE (creator_id = $1 OR acceptor_id = $1) AND status IN ('waiting','held') AND amount > 0)`, uid).Scan(&pending)
	if pending > 0 {
		return 0, userErr(409, "Finish your pending withdrawal or match first")
	}
	newBalance, rate, err := ConvertOut(ctx, balance, from, to)
	if err != nil {
		return 0, err
	}
	country := map[string]string{"NGN": "NG", "GHS": "GH"}[to]
	if _, err := tx.Exec(ctx, "UPDATE users SET currency = $2, country = $3, balance = $4, currency_changed_at = NOW(), updated_at = NOW() WHERE id = $1",
		uid, to, country, newBalance); err != nil {
		return 0, err
	}
	meta := fmt.Sprintf(`{"from": %q, "fromAmount": %d, "to": %q, "rate": %.6f, "forced": %t}`, from, balance, to, rate, force)
	if _, err := tx.Exec(ctx, "INSERT INTO transactions (user_id, type, amount, ref_id, metadata) VALUES ($1,'currency_switch',$2,$3,$4::jsonb)",
		uid, newBalance, "home_"+to, meta); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	forgetTier(uid)
	Notify(uid, "account", "Wallet moved to "+map[string]string{"NGN": "Nigeria", "GHS": "Ghana"}[to],
		fmt.Sprintf("Your balance of %s is now %s. Prices and withdrawals use %s from now on.", FormatMoney(balance, from), FormatMoney(newBalance, to), to), "/wallet", nil)
	return newBalance, nil
}
