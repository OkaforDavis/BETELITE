package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ErrInsufficientFunds is returned when a debit would take a balance below zero.
var ErrInsufficientFunds = errors.New("insufficient funds")

// AdjustBalance adds (positive) or deducts (negative) from a user's balance and
// logs the transaction. It must run inside a PostgreSQL transaction; a debit
// that would overdraw the wallet fails with ErrInsufficientFunds.
func AdjustBalance(ctx context.Context, tx pgx.Tx, userID string, amount int64, txType, refID string, metadata ...string) error {
	tag, err := tx.Exec(ctx,
		"UPDATE users SET balance = balance + $1, updated_at = NOW() WHERE id = $2 AND balance + $1 >= 0",
		amount, userID)
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userID).Scan(&exists)
		if !exists {
			return fmt.Errorf("user %s not found", userID)
		}
		return ErrInsufficientFunds
	}

	var meta any
	if len(metadata) > 0 && metadata[0] != "" {
		meta = metadata[0]
	}
	_, err = tx.Exec(ctx,
		"INSERT INTO transactions (user_id, type, amount, ref_id, metadata) VALUES ($1, $2, $3, $4, $5::jsonb)",
		userID, txType, amount, refID, meta)
	if err != nil {
		return fmt.Errorf("failed to log transaction: %w", err)
	}
	return nil
}
