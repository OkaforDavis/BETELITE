package services

import (
	"context"
	"fmt"
	"log"

	"betelite-go/db"
	"betelite-go/models"
)

// HandleEscrowPayout handles the payout of an escrowed match using a PostgreSQL transaction.
// We use SELECT ... FOR UPDATE to prevent race conditions.
func HandleEscrowPayout(match *models.Match) error {
	ctx := context.Background()

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		log.Printf("[ERROR] Escrow Payout Tx Begin: %v", err)
		return err
	}
	defer tx.Rollback(ctx)

	// 1. Lock and fetch the escrow record
	var escrow models.Escrow
	err = tx.QueryRow(ctx, "SELECT id, creator_id, acceptor_id, amount, pool, status FROM escrow WHERE challenge_id = $1 FOR UPDATE", match.ChallengeID).
		Scan(&escrow.ID, &escrow.CreatorID, &escrow.AcceptorID, &escrow.Amount, &escrow.Pool, &escrow.Status)

	if err != nil {
		log.Printf("[ERROR] Fetching escrow for match %s: %v", match.ID, err)
		return err
	}

	if escrow.Status != "held" {
		log.Printf("[WARN] Escrow %d already processed (Status: %s)", escrow.ID, escrow.Status)
		return nil
	}

	// 2. Determine payout
	var winnerID string
	var loserID string

	if match.Result.Winner == "home" {
		winnerID = match.HomeID
		loserID = match.AwayID
	} else if match.Result.Winner == "away" {
		winnerID = match.AwayID
		loserID = match.HomeID
	}

	if match.Result.Winner == "draw" {
		err = AdjustBalance(ctx, tx, escrow.CreatorID, escrow.Amount, "wager_refund", match.ChallengeID)
		if err != nil {
			return err
		}
		err = AdjustBalance(ctx, tx, escrow.AcceptorID, escrow.Amount, "wager_refund", match.ChallengeID)
		if err != nil {
			return err
		}

		// Update escrow status
		_, err = tx.Exec(ctx, "UPDATE escrow SET status = 'refunded' WHERE id = $1", escrow.ID)
		if err != nil {
			return err
		}

	} else {
		// Calculate platform fee (e.g. 13.5%)
		feePercent := 0.135
		payout := int64(float64(escrow.Pool) * (1.0 - feePercent))

		metadata := fmt.Sprintf(`{"fee_deducted": %d, "loser_id": "%s"}`, escrow.Pool-payout, loserID)
		err = AdjustBalance(ctx, tx, winnerID, payout, "wager_win", match.ChallengeID, metadata)
		if err != nil {
			return err
		}

		// Update escrow status
		_, err = tx.Exec(ctx, "UPDATE escrow SET status = 'paid_out' WHERE id = $1", escrow.ID)
		if err != nil {
			return err
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		log.Printf("[ERROR] Committing escrow payout: %v", err)
		return err
	}

	log.Printf("[INFO] Escrow processed successfully for challenge %s", match.ChallengeID)
	return nil
}
