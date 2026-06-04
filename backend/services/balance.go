package services

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// AdjustBalance adds or deducts from a user's balance and logs the transaction.
// Ensure this is called within a PostgreSQL transaction (pgx.Tx).
func AdjustBalance(ctx context.Context, tx pgx.Tx, userID string, amount int64, txType, refID string, metadata ...string) error {
	// Update user balance
	_, err := tx.Exec(ctx, "UPDATE users SET balance = balance + $1 WHERE id = $2", amount, userID)
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	// Insert transaction log
	var meta interface{}
	if len(metadata) > 0 {
		meta = metadata[0]
	}

	if meta != nil {
		_, err = tx.Exec(ctx, "INSERT INTO transactions (user_id, type, amount, ref_id, metadata) VALUES ($1, $2, $3, $4, $5)",
			userID, txType, amount, refID, meta)
	} else {
		_, err = tx.Exec(ctx, "INSERT INTO transactions (user_id, type, amount, ref_id) VALUES ($1, $2, $3, $4)",
			userID, txType, amount, refID)
	}

	if err != nil {
		return fmt.Errorf("failed to log transaction: %w", err)
	}

	return nil
}
