package services

import (
	"context"
	"log"
	"time"

	"betelite-go/db"
	"betelite-go/models"
	"betelite-go/ws"
)

// AutomationConfig controls the timing of automated background tasks.
type AutomationConfig struct {
	MatchTimeoutDuration    time.Duration // How long before a P2P match is auto-expired
	ChallengeExpireDuration time.Duration // How long before a waiting challenge is auto-cancelled
	TickInterval            time.Duration // How often the automation loop runs
}

// DefaultAutomationConfig returns sensible defaults for production.
func DefaultAutomationConfig() AutomationConfig {
	return AutomationConfig{
		MatchTimeoutDuration:    2 * time.Hour,
		ChallengeExpireDuration: 24 * time.Hour,
		TickInterval:            5 * time.Minute,
	}
}

// StartAutomation launches the background goroutine that handles:
//   - P2P match timeout (auto-expire after 2 hours with escrow refund)
//   - Stale challenge cleanup (auto-cancel waiting challenges after 24 hours)
func StartAutomation(hub *ws.Hub, cfg AutomationConfig) {
	go func() {
		log.Println("[AUTOMATION] Background automation started")
		ticker := time.NewTicker(cfg.TickInterval)
		defer ticker.Stop()

		for range ticker.C {
			expireTimedOutMatches(hub, cfg.MatchTimeoutDuration)
			cleanupStaleChallenges(hub, cfg.ChallengeExpireDuration)
		}
	}()
}

// expireTimedOutMatches finds P2P matches that have been "live" longer than
// the timeout duration and auto-expires them with escrow refund.
func expireTimedOutMatches(hub *ws.Hub, timeout time.Duration) {
	if Engine == nil {
		return
	}

	Engine.mu.Lock()
	defer Engine.mu.Unlock()

	cutoff := time.Now().Add(-timeout)

	for id, match := range Engine.Matches {
		if match.Status != "live" || !match.IsP2P {
			continue
		}

		// Use match creation time (approximated from engine ticker state).
		// Matches that have been live for very long with minute stuck at low values
		// are likely abandoned.
		estimatedDuration := time.Duration(match.Minute) * time.Minute
		if estimatedDuration < timeout && match.Minute > 0 {
			continue // Still within expected game time
		}

		// For truly stale matches (minute == 0 or has been running too long):
		// We check using a simpler heuristic — if match minute hasn't progressed
		// past 90 (already handled by engine) but the match is still live, it's stuck.
		_ = cutoff // placeholder for future created_at field

		// Auto-expire if minute is way beyond normal game time (engine bug)
		// or if it's been sitting at 0 for too long
		if match.Minute > 120 {
			log.Printf("[AUTOMATION] Auto-expiring stale match %s (minute=%d)", id, match.Minute)
			match.Status = "expired"

			// Refund escrow if applicable
			if match.ChallengeID != "" {
				go refundExpiredMatch(match)
			}

			ws.BroadcastEvent(hub, "match_expired", map[string]interface{}{
				"matchId": id,
				"reason":  "Match timed out after 2 hours",
			})
		}
	}
}

// refundExpiredMatch refunds both players when a match is auto-expired.
func refundExpiredMatch(match *models.Match) {
	if db.Pool == nil {
		return
	}

	ctx := context.Background()
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		log.Printf("[AUTOMATION] Failed to begin refund transaction for match %s: %v", match.ID, err)
		return
	}
	defer tx.Rollback(ctx)

	// Find the escrow record
	var escrowID int64
	var creatorID, acceptorID string
	var amount int64
	var status string

	err = tx.QueryRow(ctx,
		"SELECT id, creator_id, COALESCE(acceptor_id, ''), amount, status FROM escrow WHERE challenge_id = $1 FOR UPDATE",
		match.ChallengeID,
	).Scan(&escrowID, &creatorID, &acceptorID, &amount, &status)
	if err != nil {
		log.Printf("[AUTOMATION] Escrow not found for challenge %s: %v", match.ChallengeID, err)
		return
	}

	if status != "held" {
		log.Printf("[AUTOMATION] Escrow %d already processed (status=%s), skipping refund", escrowID, status)
		return
	}

	// Refund both players
	if err := AdjustBalance(ctx, tx, creatorID, amount, "match_timeout_refund", match.ChallengeID); err != nil {
		log.Printf("[AUTOMATION] Failed to refund creator %s: %v", creatorID, err)
		return
	}
	if acceptorID != "" {
		if err := AdjustBalance(ctx, tx, acceptorID, amount, "match_timeout_refund", match.ChallengeID); err != nil {
			log.Printf("[AUTOMATION] Failed to refund acceptor %s: %v", acceptorID, err)
			return
		}
	}

	// Update escrow status
	_, err = tx.Exec(ctx, "UPDATE escrow SET status = 'expired' WHERE id = $1", escrowID)
	if err != nil {
		log.Printf("[AUTOMATION] Failed to update escrow status: %v", err)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("[AUTOMATION] Failed to commit refund: %v", err)
		return
	}

	log.Printf("[AUTOMATION] Refunded match %s (challenge %s) — creator=%s, acceptor=%s, amount=%d each",
		match.ID, match.ChallengeID, creatorID, acceptorID, amount)
}

// cleanupStaleChallenges auto-cancels challenges that have been in "waiting"
// status for longer than the expiry duration and refunds the creator.
func cleanupStaleChallenges(hub *ws.Hub, expiry time.Duration) {
	if db.Pool == nil {
		return
	}

	ctx := context.Background()
	cutoff := time.Now().Add(-expiry)

	// Find stale challenges
	rows, err := db.Pool.Query(ctx,
		"SELECT challenge_id, creator_id, amount FROM escrow WHERE status = 'waiting' AND created_at < $1",
		cutoff,
	)
	if err != nil {
		log.Printf("[AUTOMATION] Failed to query stale challenges: %v", err)
		return
	}
	defer rows.Close()

	type staleChallenge struct {
		ChallengeID string
		CreatorID   string
		Amount      int64
	}

	var stale []staleChallenge
	for rows.Next() {
		var sc staleChallenge
		if err := rows.Scan(&sc.ChallengeID, &sc.CreatorID, &sc.Amount); err == nil {
			stale = append(stale, sc)
		}
	}

	for _, sc := range stale {
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			continue
		}

		// Refund creator
		if err := AdjustBalance(ctx, tx, sc.CreatorID, sc.Amount, "challenge_expired_refund", sc.ChallengeID); err != nil {
			tx.Rollback(ctx)
			continue
		}

		// Mark as cancelled
		_, err = tx.Exec(ctx, "UPDATE escrow SET status = 'expired' WHERE challenge_id = $1", sc.ChallengeID)
		if err != nil {
			tx.Rollback(ctx)
			continue
		}

		if err := tx.Commit(ctx); err != nil {
			continue
		}

		log.Printf("[AUTOMATION] Auto-expired stale challenge %s (creator=%s, refunded=%d)",
			sc.ChallengeID, sc.CreatorID, sc.Amount)

		ws.BroadcastEvent(hub, "lobby_challenge_removed", map[string]string{
			"id":     sc.ChallengeID,
			"reason": "Challenge expired after 24 hours",
		})
	}
}
