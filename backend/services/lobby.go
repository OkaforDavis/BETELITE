package services

import (
	"context"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"
)

// One 1v1 at a time: a player with an unfinished 1v1 can't post or accept
// another challenge, and when one of their challenges is accepted, their other
// open challenges are closed (bounties refunded).

// UnfinishedP2PStatuses are the match states that still need the players.
const UnfinishedP2PStatuses = `('ready','submitted','disputed','review')`

// ActiveP2PMatch returns the id of the player's unfinished 1v1, or "".
func ActiveP2PMatch(ctx context.Context, q querier, uid string) (string, error) {
	var id string
	err := q.QueryRow(ctx, `SELECT id FROM matches WHERE kind = 'p2p' AND status IN `+UnfinishedP2PStatuses+`
		AND (home_id = $1 OR away_id = $1) ORDER BY created_at LIMIT 1`, uid).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// LockPlayers locks the users' rows in a fixed order, so two accepts that
// involve the same player run one after the other (and never deadlock).
func LockPlayers(ctx context.Context, tx pgx.Tx, ids ...string) error {
	sorted := append([]string{}, ids...)
	sort.Strings(sorted)
	for _, id := range sorted {
		if _, err := tx.Exec(ctx, "SELECT 1 FROM users WHERE id = $1 FOR UPDATE", id); err != nil {
			return err
		}
	}
	return nil
}

// CloseOpenChallenges cancels the player's other waiting challenges, refunding
// any money held for them, and returns their ids (for the live lobby).
func CloseOpenChallenges(ctx context.Context, tx pgx.Tx, uid, except string) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT challenge_id, amount, fee, kind FROM escrow
		WHERE creator_id = $1 AND status = 'waiting' AND challenge_id <> $2 FOR UPDATE`, uid, except)
	if err != nil {
		return nil, err
	}
	type open struct {
		id          string
		amount, fee int64
		kind        string
	}
	var list []open
	for rows.Next() {
		var o open
		if err := rows.Scan(&o.id, &o.amount, &o.fee, &o.kind); err != nil {
			rows.Close()
			return nil, err
		}
		list = append(list, o)
	}
	rows.Close()
	var ids []string
	for _, o := range list {
		switch {
		case o.kind == "bounty" && o.amount+o.fee > 0:
			err = AdjustBalance(ctx, tx, uid, o.amount+o.fee, "bounty_refund", o.id)
		case o.amount > 0:
			err = AdjustBalance(ctx, tx, uid, o.amount, "wager_refund", o.id)
		}
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, "UPDATE escrow SET status = 'cancelled' WHERE challenge_id = $1", o.id); err != nil {
			return nil, err
		}
		ids = append(ids, o.id)
	}
	return ids, nil
}
