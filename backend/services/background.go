package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"betelite-go/db"
)

// Money work that runs after the request has answered (an automatic payout)
// is tracked here, so a redeploy waits for it instead of killing it halfway.
var background sync.WaitGroup

// Background runs fn in a goroutine that WaitBackground waits for.
func Background(fn func()) {
	background.Add(1)
	go func() {
		defer background.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[BACKGROUND] recovered from panic: %v", r)
			}
		}()
		fn()
	}()
}

// WaitBackground waits up to timeout for background work to finish. Called on
// shutdown, after the server has stopped taking new requests.
func WaitBackground(timeout time.Duration) {
	done := make(chan struct{})
	go func() { background.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(timeout):
		log.Println("[BACKGROUND] shutdown: some work was still running")
	}
}

// stuckAlerted remembers which stuck withdrawals admins were told about.
var stuckAlerted sync.Map

// alertStuckWithdrawals tells admins about withdrawals that were claimed for
// sending but never got a transfer code (e.g. the server stopped mid-payout).
// They are not retried by themselves: the provider may have sent the money,
// so an admin checks the provider dashboard by reference first.
func alertStuckWithdrawals(ctx context.Context) {
	rows, err := db.Pool.Query(ctx, `SELECT id, amount - fee, currency FROM withdrawals
		WHERE status = 'processing' AND COALESCE(transfer_code,'') = '' AND updated_at < NOW() - INTERVAL '30 minutes'`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id, currency string
		var amount int64
		if rows.Scan(&id, &amount, &currency) != nil {
			continue
		}
		if _, seen := stuckAlerted.LoadOrStore(id, true); seen {
			continue
		}
		notifyAdmins(ctx, "Withdrawal needs a check", fmt.Sprintf(
			"%s (%s) started sending but we never heard back. Look up the reference on the provider dashboard, then use Mark as paid or Refund.",
			FormatMoney(amount, currency), id), "")
	}
}
