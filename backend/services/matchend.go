package services

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"betelite-go/db"
)

// Ending an unplayed 1v1 (posted or accepted by mistake, can't play now…).
//   - Friendly (no money): either player ends it straight away.
//   - Bounty / stake: the other player must agree. If they don't answer within
//     EndRequestTimeout, it ends by itself. If they decline, the match goes on.
//
// Ending works only before a result is uploaded; after that, use Dispute.
// Tournament matches can't be ended by players (support/admin handles them).
// Ending refunds exactly like a match that was never played (VoidMatch).
const EndRequestTimeout = 15 * time.Minute

type endMatch struct {
	kind, status, home, away, challenge string
	reqBy                               string
	amount                              int64
}

func loadEndMatch(ctx context.Context, tx pgx.Tx, matchID string) (*endMatch, error) {
	var m endMatch
	err := tx.QueryRow(ctx, `SELECT m.kind, m.status, m.home_id, COALESCE(m.away_id,''), COALESCE(m.challenge_id,''),
		COALESCE(m.end_requested_by,''), COALESCE(e.amount,0)
		FROM matches m LEFT JOIN escrow e ON e.challenge_id = m.challenge_id
		WHERE m.id = $1 FOR UPDATE OF m`, matchID).
		Scan(&m.kind, &m.status, &m.home, &m.away, &m.challenge, &m.reqBy, &m.amount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, userErr(404, "Match not found")
	}
	return &m, err
}

func (m *endMatch) check(uid string) error {
	if uid != m.home && uid != m.away {
		return userErr(403, "Only the two players can end this match")
	}
	if m.kind != "p2p" {
		return userErr(400, "Tournament matches can't be ended. Contact support if there's a problem.")
	}
	if m.status != StatusReady || m.away == "" {
		return userErr(409, "This match can't be ended now. If a result was uploaded and it's wrong, dispute it.")
	}
	return nil
}

func cleanReason(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > 120 {
		s = string([]rune(s)[:120])
	}
	return s
}

// RequestEnd ends a friendly at once, or asks the other player to agree.
func RequestEnd(ctx context.Context, matchID, uid, reason string) (*MatchView, error) {
	reason = cleanReason(reason)
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	m, err := loadEndMatch(ctx, tx, matchID)
	if err != nil {
		return nil, err
	}
	if err := m.check(uid); err != nil {
		return nil, err
	}
	name := Username(ctx, uid)
	opp := m.home
	if uid == m.home {
		opp = m.away
	}

	// Friendly, or the other player already asked: end it now.
	if m.amount == 0 || (m.reqBy != "" && m.reqBy != uid) {
		tx.Rollback(ctx)
		why := name + " ended the match."
		if m.reqBy != "" && m.reqBy != uid {
			why = "Both players agreed to end the match."
		} else if reason != "" {
			why = name + " ended the match: “" + reason + "”."
		}
		return endNow(ctx, matchID, uid, why)
	}
	if m.reqBy == uid {
		return nil, userErr(409, "You already asked. Waiting for your opponent to answer.")
	}
	if _, err := tx.Exec(ctx, "UPDATE matches SET end_requested_by = $2, end_requested_at = NOW(), end_reason = $3 WHERE id = $1",
		matchID, uid, reason); err != nil {
		return nil, err
	}
	if _, err := saveChat(ctx, tx, matchID, uid, "end_request", reason); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	msg := name + " wants to end your match"
	if reason != "" {
		msg += ": “" + reason + "”"
	}
	Notify(opp, "match_end_request", "End the match?", msg+". Agree or keep playing. If you don't answer in 15 minutes, it ends by itself.",
		"/match/"+matchID, map[string]any{"matchId": matchID})
	return afterEndChange(ctx, matchID)
}

// RespondEnd: the other player agrees or declines; the asker can take it back.
func RespondEnd(ctx context.Context, matchID, uid string, agree bool) (*MatchView, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	m, err := loadEndMatch(ctx, tx, matchID)
	if err != nil {
		return nil, err
	}
	if err := m.check(uid); err != nil {
		return nil, err
	}
	if m.reqBy == "" {
		return nil, userErr(409, "Nobody has asked to end this match")
	}
	if agree {
		if m.reqBy == uid {
			return nil, userErr(400, "Your opponent has to agree, not you")
		}
		tx.Rollback(ctx)
		return endNow(ctx, matchID, uid, "Both players agreed to end the match.")
	}
	if _, err := tx.Exec(ctx, "UPDATE matches SET end_requested_by = NULL, end_requested_at = NULL, end_reason = NULL WHERE id = $1", matchID); err != nil {
		return nil, err
	}
	kind := "end_declined"
	if m.reqBy == uid {
		kind = "end_withdrawn"
	}
	if _, err := saveChat(ctx, tx, matchID, uid, kind, ""); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if kind == "end_declined" {
		Notify(m.reqBy, "match_end_declined", "Match goes on", Username(ctx, uid)+" wants to play. The match is still on.",
			"/match/"+matchID, map[string]any{"matchId": matchID})
	}
	return afterEndChange(ctx, matchID)
}

func endNow(ctx context.Context, matchID, by, reason string) (*MatchView, error) {
	if err := voidMatch(ctx, matchID, by, reason, StatusReady); err != nil {
		return nil, err
	}
	return GetMatch(ctx, matchID)
}

// afterEndChange sends the updated match (and the chat line) to both players.
func afterEndChange(ctx context.Context, matchID string) (*MatchView, error) {
	mv, err := GetMatch(ctx, matchID)
	if err != nil {
		return nil, err
	}
	broadcastMatch(mv)
	if rows, err := db.Pool.Query(ctx, `SELECT id, user_id, kind, body, created_at FROM match_messages
		WHERE match_id = $1 ORDER BY created_at DESC LIMIT 1`, matchID); err == nil {
		for rows.Next() {
			var c ChatMessage
			if rows.Scan(&c.ID, &c.UserID, &c.Kind, &c.Body, &c.CreatedAt) == nil {
				pushChat(matchID, &c, mv.HomeID, mv.AwayID)
			}
		}
		rows.Close()
	}
	return mv, nil
}

// endUnansweredRequests ends 1v1s whose end request got no answer in time.
func endUnansweredRequests(ctx context.Context) {
	for _, id := range idsFrom(ctx, `SELECT id FROM matches WHERE kind = 'p2p' AND status = 'ready'
		AND end_requested_by IS NOT NULL AND end_requested_at < NOW() - make_interval(secs => $1) LIMIT 100`, EndRequestTimeout.Seconds()) {
		var by string
		db.Pool.QueryRow(ctx, "SELECT COALESCE(end_requested_by,'') FROM matches WHERE id = $1", id).Scan(&by)
		if err := voidMatch(ctx, id, "auto", Username(ctx, by)+" asked to end the match and there was no answer in 15 minutes.", StatusReady); err != nil {
			log.Printf("[AUTOMATION] end %s: %v", id, err)
		}
	}
}

// EndUnansweredRequestsForTest runs the automation step (tests only).
func EndUnansweredRequestsForTest(ctx context.Context) { endUnansweredRequests(ctx) }
