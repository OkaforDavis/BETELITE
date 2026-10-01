package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"betelite-go/db"
)

// RefundWithdrawal returns held withdrawal money to the wallet and marks the
// request rejected/failed. fromStatus guards against double refunds.
func RefundWithdrawal(ctx context.Context, id, reason, fromStatus string) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var uid, currency string
	var amount int64
	newStatus := "failed"
	if fromStatus == "pending" {
		newStatus = "rejected"
	}
	err = tx.QueryRow(ctx, `UPDATE withdrawals SET status = $3, note = $2, updated_at = NOW()
		WHERE id = $1 AND status = $4 RETURNING user_id, amount, currency`, id, reason, newStatus, fromStatus).Scan(&uid, &amount, &currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // already handled
	}
	if err != nil {
		return err
	}
	if err := AdjustBalance(ctx, tx, uid, amount, "withdrawal_refund", id); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	Notify(uid, "withdrawal", "Withdrawal not completed",
		fmt.Sprintf("%s was returned to your wallet. %s", FormatMoney(amount, currency), reason), "/wallet", nil)
	return nil
}

// ApproveWithdrawal sends a pending withdrawal through Paystack.
func ApproveWithdrawal(ctx context.Context, id, adminID string) error {
	var uid, currency, bankCode, account, name, bankType, status string
	var amount int64
	err := db.Pool.QueryRow(ctx, `SELECT user_id, currency, bank_code, account_number, COALESCE(account_name,''), COALESCE(note,''), status, amount
		FROM withdrawals WHERE id = $1`, id).Scan(&uid, &currency, &bankCode, &account, &name, &bankType, &status, &amount)
	if errors.Is(err, pgx.ErrNoRows) {
		return userErr(404, "Withdrawal not found")
	}
	if err != nil {
		return err
	}
	// Claim it first so two admins can't pay the same request twice.
	tag, err := db.Pool.Exec(ctx, "UPDATE withdrawals SET status = 'processing', reviewed_by = $2, updated_at = NOW() WHERE id = $1 AND status = 'pending'", id, adminID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return userErr(409, "This withdrawal is already being processed")
	}
	recipient, transfer, err := PaystackPayout(ctx, currency, bankType, name, account, bankCode, amount, id, "CrestArena withdrawal")
	if err != nil {
		// Put it back so the admin can retry or reject.
		db.Pool.Exec(ctx, "UPDATE withdrawals SET status = 'pending', updated_at = NOW() WHERE id = $1 AND status = 'processing'", id)
		return userErr(502, "Paystack could not start the transfer: %v", err)
	}
	_, err = db.Pool.Exec(ctx, "UPDATE withdrawals SET recipient_code = $2, transfer_code = $3, updated_at = NOW() WHERE id = $1", id, recipient, transfer)
	return err
}

// NotifyAdmins sends a notification to every admin.
func NotifyAdmins(ctx context.Context, title, message string) {
	notifyAdmins(ctx, title, message, "")
}
