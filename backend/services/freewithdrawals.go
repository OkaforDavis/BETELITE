package services

import "context"

// EliteFreeWithdrawals is how many withdrawals an Elite member can make each
// plan month without a transfer fee. CrestArena pays the provider's fee for
// these, so the cap stops tiny repeated cash-outs from costing more than the
// plan brings in.
const EliteFreeWithdrawals = 5

// FreeWithdrawalsLeft is how many fee-free withdrawals the player has left
// this plan month (0 for anyone not on Elite). Failed withdrawals don't count.
func FreeWithdrawalsLeft(ctx context.Context, q querier, uid string) (int, error) {
	m, err := ActiveMembership(ctx, q, uid)
	if err != nil || m == nil || m.Tier != "elite" {
		return 0, err
	}
	var used int
	err = q.QueryRow(ctx, `SELECT COUNT(*) FROM withdrawals WHERE user_id = $1 AND fee_waived > 0
		AND status <> 'failed' AND created_at >= $2`, uid, m.StartsAt).Scan(&used)
	return max(EliteFreeWithdrawals-used, 0), err
}
