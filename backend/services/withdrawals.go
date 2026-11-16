package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5"

	"betelite-go/config"
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
	var uid, currency, bankCode, account, name, bankType, status, provider string
	var amount, fee int64
	err := db.Pool.QueryRow(ctx, `SELECT user_id, currency, bank_code, account_number, COALESCE(account_name,''), COALESCE(note,''), status, amount, fee, provider
		FROM withdrawals WHERE id = $1`, id).Scan(&uid, &currency, &bankCode, &account, &name, &bankType, &status, &amount, &fee, &provider)
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
	var recipient, transfer string
	if provider == ProviderFlutterwave {
		transfer, err = FlutterwavePayout(ctx, currency, name, account, bankCode, amount-fee, id, "CrestArena withdrawal")
	} else {
		recipient, transfer, err = PaystackPayout(ctx, currency, bankType, name, account, bankCode, amount-fee, id, "CrestArena withdrawal")
	}
	if err != nil {
		// Put it back so the admin can retry or reject, and show the provider's
		// exact words: they say what to fix (balance, compliance, OTP, ...).
		db.Pool.Exec(ctx, "UPDATE withdrawals SET status = 'pending', updated_at = NOW() WHERE id = $1 AND status = 'processing'", id)
		log.Printf("[WITHDRAWALS] %s refused %s (%d %s): %v", provider, id, amount-fee, currency, err)
		hint := ""
		switch msg := strings.ToLower(err.Error()); {
		case strings.Contains(msg, "starter"):
			hint = " Paystack treats this business as a Starter business: check Settings → Compliance on the Paystack dashboard, and that PAYSTACK_SECRET_KEY belongs to the Registered business."
		case strings.Contains(msg, "balance"):
			hint = " Top up the provider balance (Paystack: Balance → Top up, or switch payouts to \"Settled to Balance\")."
		case strings.Contains(msg, "otp"):
			hint = " Turn off transfer OTP in the Paystack dashboard (Settings → Preferences)."
		}
		return userErr(502, "%s said: \"%v\".%s Or pay by bank transfer and use \"Mark as paid\".", capitalize(provider), err, hint)
	}
	_, err = db.Pool.Exec(ctx, "UPDATE withdrawals SET recipient_code = $2, transfer_code = $3, updated_at = NOW() WHERE id = $1", id, recipient, transfer)
	return err
}

// AutoApproveWithdrawal sends a fresh withdrawal straight away when it is
// within AUTO_PAYOUT_LIMIT for its currency. Anything the provider refuses
// stays pending for an admin, who is told why.
func AutoApproveWithdrawal(ctx context.Context, id string, amount int64, currency string) {
	limit := config.Cfg.AutoPayoutLimit[currency]
	if limit <= 0 || amount > limit {
		return
	}
	if err := ApproveWithdrawal(ctx, id, "auto"); err != nil {
		log.Printf("[WITHDRAWALS] auto-approve %s: %v", id, err)
		notifyAdmins(ctx, "Automatic payout failed", fmt.Sprintf("%s is waiting for you. %v", FormatMoney(amount, currency), err), "")
		return
	}
	log.Printf("[WITHDRAWALS] auto-approved %s (%d %s)", id, amount, currency)
}

// ApproveTransferRequest answers a Paystack transfer-approval call: the
// transfer is allowed only if it matches a withdrawal we are sending right
// now (same reference, amount, currency and account). Anything else, for
// example a transfer started with a stolen key, is refused.
func ApproveTransferRequest(ctx context.Context, reference string, amount int64, currency, accountNumber string) error {
	if reference == "" {
		return fmt.Errorf("no reference")
	}
	var status, cur, account string
	var total, fee int64
	err := db.Pool.QueryRow(ctx, "SELECT status, currency, account_number, amount, fee FROM withdrawals WHERE id = $1", reference).
		Scan(&status, &cur, &account, &total, &fee)
	if err != nil {
		return fmt.Errorf("unknown withdrawal %s", reference)
	}
	switch {
	case status != "processing":
		return fmt.Errorf("withdrawal %s is %s, not being sent", reference, status)
	case amount != 0 && amount != total-fee:
		return fmt.Errorf("amount %d does not match %d", amount, total-fee)
	case currency != "" && currency != cur:
		return fmt.Errorf("currency %s does not match %s", currency, cur)
	case accountNumber != "" && accountNumber != account:
		return fmt.Errorf("account does not match")
	}
	return nil
}

// NotifyAdmins sends a notification to every admin.
func NotifyAdmins(ctx context.Context, title, message string) {
	notifyAdmins(ctx, title, message, "")
}

// MarkWithdrawalPaid records a withdrawal the admin paid by hand (e.g. a bank
// transfer while Paystack payouts aren't available). The money was already
// held from the player's wallet when they requested it, so no balance moves.
func MarkWithdrawalPaid(ctx context.Context, id, adminID, note string) error {
	var uid, currency string
	var amount int64
	err := db.Pool.QueryRow(ctx, `UPDATE withdrawals SET status = 'paid', reviewed_by = $2, note = $3, updated_at = NOW()
		WHERE id = $1 AND status IN ('pending','processing') RETURNING user_id, amount - fee, currency`, id, adminID, "Paid manually: "+note).
		Scan(&uid, &amount, &currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return userErr(409, "This withdrawal is already completed or was rejected")
	}
	if err != nil {
		return err
	}
	Notify(uid, "withdrawal", "Withdrawal sent", FormatMoney(amount, currency)+" has been sent to your bank account.", "/wallet", nil)
	return nil
}
