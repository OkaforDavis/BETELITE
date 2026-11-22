package services

import (
	"context"
	"errors"
	"log"
	"time"

	"betelite-go/db"
	"betelite-go/ws"
)

// AutomationConfig controls the timing of automated background tasks.
type AutomationConfig struct {
	ChallengeExpireDuration time.Duration // waiting challenges are cancelled and refunded after this
	TickInterval            time.Duration // how often the automation loop runs
}

// DefaultAutomationConfig returns sensible defaults for production.
func DefaultAutomationConfig() AutomationConfig {
	return AutomationConfig{
		ChallengeExpireDuration: 24 * time.Hour,
		TickInterval:            time.Minute,
	}
}

// StartAutomation launches the background loop that:
//   - settles results whose 15-minute dispute window has passed (pays out),
//   - refunds 1v1 matches nobody submitted a result for in time,
//   - sends overdue tournament matches to admin review,
//   - cancels and refunds challenges nobody accepted.
func StartAutomation(hub *ws.Hub, cfg AutomationConfig) {
	if db.Pool == nil {
		return
	}
	go func() {
		log.Println("[AUTOMATION] Background automation started")
		ticker := time.NewTicker(cfg.TickInterval)
		defer ticker.Stop()
		for {
			runAutomation(hub, cfg)
			<-ticker.C
		}
	}()
}

func runAutomation(hub *ws.Hub, cfg AutomationConfig) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[AUTOMATION] recovered from panic: %v", r)
		}
	}()
	ctx := context.Background()
	ReconcileDeposits(ctx)
	alertStuckWithdrawals(ctx)
	RemindExpiringMemberships(ctx)
	purgeOldEvidence(ctx)
	purgeSponsors(ctx)
	settleDueResults(ctx)
	handleMissedDeadlines(ctx)
	cleanupStaleChallenges(ctx, hub, cfg.ChallengeExpireDuration)
	ExpireUnfilledCups(ctx)
}

func idsFrom(ctx context.Context, sql string, args ...any) []string {
	rows, err := db.Pool.Query(ctx, sql, args...)
	if err != nil {
		log.Printf("[AUTOMATION] query failed: %v", err)
		return nil
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// settleDueResults makes undisputed results final once the window closes.
func settleDueResults(ctx context.Context) {
	for _, id := range idsFrom(ctx, "SELECT id FROM matches WHERE status = 'submitted' AND dispute_deadline <= NOW() LIMIT 100") {
		if err := SettleMatch(ctx, id, "auto"); err != nil {
			log.Printf("[AUTOMATION] settle %s: %v", id, err)
			var ue *UserError
			if errors.As(err, &ue) {
				db.Pool.Exec(ctx, "UPDATE matches SET status = 'review', dispute_reason = $2 WHERE id = $1 AND status = 'submitted'", id, ue.Msg)
				notifyAdmins(ctx, "Match needs review", ue.Msg, id)
			}
			continue
		}
		log.Printf("[AUTOMATION] settled match %s", id)
	}
}

// handleMissedDeadlines refunds unplayed 1v1s and flags overdue tournament matches.
func handleMissedDeadlines(ctx context.Context) {
	for _, id := range idsFrom(ctx, "SELECT id FROM matches WHERE kind = 'p2p' AND status = 'ready' AND play_deadline < NOW() LIMIT 100") {
		if err := VoidMatch(ctx, id, "auto", "No result was submitted in time."); err != nil {
			log.Printf("[AUTOMATION] void %s: %v", id, err)
		}
	}
	for _, id := range idsFrom(ctx, `UPDATE matches SET status = 'review', dispute_reason = 'Play deadline passed with no result'
		WHERE kind = 'tournament' AND status = 'ready' AND play_deadline < NOW() RETURNING id`) {
		notifyAdmins(ctx, "Tournament match overdue", "A tournament match passed its deadline without a result.", id)
	}
}

// cleanupStaleChallenges cancels challenges nobody accepted and refunds the creator.
func cleanupStaleChallenges(ctx context.Context, hub *ws.Hub, expiry time.Duration) {
	cutoff := time.Now().Add(-expiry)
	for _, cid := range idsFrom(ctx, "SELECT challenge_id FROM escrow WHERE status = 'waiting' AND created_at < $1 LIMIT 100", cutoff) {
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			continue
		}
		var creator string
		var amount, fee int64
		err = tx.QueryRow(ctx, "SELECT creator_id, amount, fee FROM escrow WHERE challenge_id = $1 AND status = 'waiting' FOR UPDATE", cid).Scan(&creator, &amount, &fee)
		if err == nil && amount+fee > 0 {
			// A bounty host gets the prize and the hosting fee back.
			err = AdjustBalance(ctx, tx, creator, amount+fee, "challenge_expired_refund", cid)
		}
		if err == nil {
			_, err = tx.Exec(ctx, "UPDATE escrow SET status = 'expired' WHERE challenge_id = $1", cid)
		}
		if err != nil || tx.Commit(ctx) != nil {
			tx.Rollback(ctx)
			continue
		}
		log.Printf("[AUTOMATION] expired challenge %s, refunded %d to %s", cid, amount, creator)
		ws.BroadcastEvent(hub, "lobby_challenge_removed", map[string]string{"id": cid, "reason": "expired"})
		msg := "Nobody accepted your challenge in 24 hours. Your stake has been refunded."
		if fee > 0 {
			msg = "Nobody accepted your bounty in 24 hours. The prize and the hosting fee are back in your wallet."
		} else if amount == 0 {
			msg = "Nobody accepted your challenge in 24 hours. Post a new one any time."
		}
		Notify(creator, "challenge_expired", "Challenge expired", msg, "/play", nil)
	}
}
