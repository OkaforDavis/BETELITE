package services

import (
	"context"
	"errors"
	"fmt"

	"betelite-go/db"
	"betelite-go/utils"
)

// Tips: a player sends money from their wallet to a host or another player
// as a thank-you. Tips are free: the full amount reaches the other player
// (only a cross-currency tip is converted at the usual wallet rate). A tip
// buys nothing and wins nothing, so it isn't a stake.
//
// To stop tips being used to move money between accounts, a tip must be
// sent from a cup, match or stream both people are part of, and there are
// per-tip and daily limits.

var (
	MinTip      = map[string]int64{"NGN": 100_00, "GHS": 1_00}
	MaxTip      = map[string]int64{"NGN": 20_000_00, "GHS": 200_00}
	MaxTipDaily = map[string]int64{"NGN": 50_000_00, "GHS": 500_00}
	TipPresets  = map[string][]int64{"NGN": {100_00, 200_00, 500_00, 1_000_00}, "GHS": {1_00, 2_00, 5_00, 10_00}}
)

// TipRequest is what the app sends to tip someone.
type TipRequest struct {
	To      string `json:"to"`
	Amount  int64  `json:"amount"`  // in the sender's wallet currency, minor units
	Context string `json:"context"` // tournament | match | stream
	RefID   string `json:"refId"`   // the cup, match or stream id
}

// TipResult is what the sender sees after tipping.
type TipResult struct {
	ID         string `json:"id"`
	Sent       int64  `json:"sent"`
	Currency   string `json:"currency"`
	Received   int64  `json:"received"`
	ToCurrency string `json:"toCurrency"`
	ToName     string `json:"toName"`
}

// TipAllowed checks that the person being tipped belongs to the cup or match
// the tip is sent from. Streams are checked by the caller (they live in memory).
func TipAllowed(ctx context.Context, req TipRequest) error {
	var ok bool
	switch req.Context {
	case "tournament":
		db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tournaments t WHERE t.id = $1 AND
			(t.host_id = $2 OR EXISTS(SELECT 1 FROM tournament_players p WHERE p.tournament_id = t.id AND p.user_id = $2)))`, req.RefID, req.To).Scan(&ok)
	case "match":
		db.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM matches WHERE id = $1 AND (home_id = $2 OR away_id = $2))", req.RefID, req.To).Scan(&ok)
	default:
		return userErr(400, "Tips can be sent from a cup, a match or a live stream")
	}
	if !ok {
		return userErr(400, "You can only tip the host or players of this %s", req.Context)
	}
	return nil
}

// SendTip moves a tip from one wallet to another.
func SendTip(ctx context.Context, from string, req TipRequest) (*TipResult, error) {
	if req.To == "" || req.To == from {
		return nil, userErr(400, "You can't tip yourself")
	}
	if err := RequireMoneyAccess(ctx, from); err != nil {
		return nil, err
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var currency string
	if err := tx.QueryRow(ctx, "SELECT COALESCE(currency,'NGN') FROM users WHERE id = $1 FOR UPDATE", from).Scan(&currency); err != nil {
		return nil, err
	}
	var toCurrency string
	if err := tx.QueryRow(ctx, "SELECT COALESCE(currency,'NGN') FROM users WHERE id = $1 AND deleted_at IS NULL", req.To).Scan(&toCurrency); err != nil {
		return nil, userErr(404, "That player can't receive tips")
	}
	if req.Amount < MinTip[currency] || req.Amount > MaxTip[currency] {
		return nil, userErr(400, "A tip must be between %s and %s", FormatMoney(MinTip[currency], currency), FormatMoney(MaxTip[currency], currency))
	}
	var today int64
	tx.QueryRow(ctx, "SELECT COALESCE(SUM(amount),0) FROM tips WHERE from_id = $1 AND created_at > NOW() - INTERVAL '24 hours'", from).Scan(&today)
	if today+req.Amount > MaxTipDaily[currency] {
		left := max(MaxTipDaily[currency]-today, 0)
		return nil, userErr(400, "You can tip up to %s a day. You have %s left today.", FormatMoney(MaxTipDaily[currency], currency), FormatMoney(left, currency))
	}
	received, _, err := ConvertOut(ctx, req.Amount, currency, toCurrency)
	if err != nil {
		return nil, err
	}
	if received <= 0 {
		return nil, userErr(400, "That tip is too small to send to another currency")
	}

	id := "tip_" + utils.GenerateBaseID()
	if err := AdjustBalance(ctx, tx, from, -req.Amount, "tip_sent", id); err != nil {
		if errors.Is(err, ErrInsufficientFunds) {
			return nil, userErr(402, "Top up your wallet to send this tip")
		}
		return nil, err
	}
	if err := AdjustBalance(ctx, tx, req.To, received, "tip_received", id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO tips (id, from_id, to_id, amount, currency, received, received_currency, context, ref_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, from, req.To, req.Amount, currency, received, toCurrency, req.Context, req.RefID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	fromName := Username(ctx, from)
	url := "/wallet"
	switch req.Context {
	case "tournament":
		url = "/tournaments/" + req.RefID
	case "match":
		url = "/match/" + req.RefID
	}
	Notify(req.To, "tip", "You got a tip 🎁", fmt.Sprintf("%s tipped you %s. It's in your wallet.", fromName, FormatMoney(received, toCurrency)), url, nil)
	return &TipResult{ID: id, Sent: req.Amount, Currency: currency, Received: received, ToCurrency: toCurrency, ToName: Username(ctx, req.To)}, nil
}
