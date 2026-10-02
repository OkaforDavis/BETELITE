package services

import (
	"context"
	"errors"
	"log"

	"github.com/jackc/pgx/v5"

	"betelite-go/db"
)

// CreditDeposit asks Paystack for the authoritative state of a deposit and
// credits the wallet exactly once when it succeeded. It is safe to call from
// the app, the webhook and the background reconciler at the same time.
//
// Paystack reports "abandoned" for a checkout the customer hasn't finished
// yet, so only a confirmed failure/reversal marks a deposit failed — and even
// then a later success is still credited (customers can retry a declined card
// on the same checkout).
func CreditDeposit(ctx context.Context, reference string) (string, error) {
	var uid, currency, status string
	var amount int64
	err := db.Pool.QueryRow(ctx, "SELECT user_id, currency, amount, status FROM deposits WHERE reference = $1", reference).
		Scan(&uid, &currency, &amount, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", &UserError{Status: 404, Msg: "Deposit not found"}
	}
	if err != nil {
		return "", err
	}
	if status == "paid" {
		return "paid", nil
	}

	txn, err := PaystackVerify(ctx, currency, reference)
	db.Pool.Exec(ctx, "UPDATE deposits SET checked_at = NOW(), gateway_status = $2 WHERE reference = $1", reference, txnStatus(txn, err))
	if err != nil {
		return "", err
	}
	switch txn.Status {
	case "success":
	case "failed", "reversed":
		db.Pool.Exec(ctx, "UPDATE deposits SET status = 'failed' WHERE reference = $1 AND status = 'pending'", reference)
		return "failed", nil
	default: // abandoned (not finished yet), ongoing, pending, processing, queued
		return status, nil
	}
	if txn.Currency != currency || txn.Amount < amount {
		log.Printf("[PAYMENTS] deposit %s mismatch: paid %d %s, expected %d %s", reference, txn.Amount, txn.Currency, amount, currency)
		NotifyAdmins(ctx, "Deposit needs review", "Deposit "+reference+" was paid with a different amount or currency.")
		return "", &UserError{Status: 409, Msg: "Payment amount mismatch. Please contact support."}
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, "UPDATE deposits SET status = 'paid', paid_at = NOW() WHERE reference = $1 AND status <> 'paid'", reference)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 0 {
		return "paid", nil // credited by a concurrent call
	}
	if err := AdjustBalance(ctx, tx, uid, amount, "deposit", reference); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	log.Printf("[PAYMENTS] credited deposit %s (%d %s) to %s", reference, amount, currency, uid)
	Notify(uid, "deposit", "Deposit received", FormatMoney(amount, currency)+" has been added to your wallet.", "/wallet", nil)
	return "paid", nil
}

func txnStatus(t *PaystackTxn, err error) string {
	if err != nil || t == nil {
		return "error"
	}
	return t.Status
}

// ReconcileDeposits re-checks unconfirmed deposits with Paystack so a paid
// deposit is credited even if the customer closed the app and the webhook
// never arrived. Fresh deposits are checked every minute, older ones every
// 15 minutes, for up to 7 days.
func ReconcileDeposits(ctx context.Context) {
	rows, err := db.Pool.Query(ctx, `SELECT reference FROM deposits
		WHERE status IN ('pending','failed') AND created_at > NOW() - INTERVAL '7 days'
		  AND created_at < NOW() - INTERVAL '30 seconds'
		  AND (checked_at IS NULL
		       OR (created_at > NOW() - INTERVAL '2 hours' AND checked_at < NOW() - INTERVAL '50 seconds')
		       OR checked_at < NOW() - INTERVAL '15 minutes')
		ORDER BY created_at DESC LIMIT 50`)
	if err != nil {
		log.Printf("[PAYMENTS] reconcile query: %v", err)
		return
	}
	var refs []string
	for rows.Next() {
		var r string
		if rows.Scan(&r) == nil {
			refs = append(refs, r)
		}
	}
	rows.Close()
	for _, ref := range refs {
		if _, err := CreditDeposit(ctx, ref); err != nil {
			log.Printf("[PAYMENTS] reconcile %s: %v", ref, err)
		}
	}
}
