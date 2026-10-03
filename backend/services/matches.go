package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"betelite-go/db"
	"betelite-go/utils"
	"betelite-go/ws"
)

// Match statuses.
const (
	StatusReady     = "ready"     // both players known, waiting for a result
	StatusSubmitted = "submitted" // result uploaded, dispute window open
	StatusDisputed  = "disputed"  // opponent disputed, waiting for admin
	StatusReview    = "review"    // deadline missed / unclear, waiting for admin
	StatusConfirmed = "confirmed" // final, money/standings settled
	StatusVoid      = "void"      // cancelled, stakes refunded
)

// Tunables for the result flow.
const (
	DisputeWindow    = 15 * time.Minute
	P2PPlayWindow    = 3 * time.Hour
	MinOCRConfidence = 70
	P2PWinnerPercent = 80 // winner of a 1v1 receives this % of the pool
	MaxScreenshotLen = 8 << 20
)

// UserError is an error whose message is safe and useful to show the player.
type UserError struct {
	Status int
	Msg    string
}

func (e *UserError) Error() string { return e.Msg }

func userErr(status int, format string, a ...any) error {
	return &UserError{Status: status, Msg: fmt.Sprintf(format, a...)}
}

// MatchView is a match as returned to clients.
type MatchView struct {
	ID              string     `json:"id"`
	Kind            string     `json:"kind"` // p2p | tournament
	Game            string     `json:"game"`
	GameName        string     `json:"gameName"`
	HomeID          string     `json:"homeId"`
	HomeName        string     `json:"homeName"`
	HomeTag         string     `json:"homeTag"`
	HomeAvatar      string     `json:"homeAvatar"`
	AwayID          string     `json:"awayId"`
	AwayName        string     `json:"awayName"`
	AwayTag         string     `json:"awayTag"`
	AwayAvatar      string     `json:"awayAvatar"`
	TournamentID    string     `json:"tournamentId,omitempty"`
	TournamentName  string     `json:"tournamentName,omitempty"`
	Round           int        `json:"round,omitempty"`
	Status          string     `json:"status"`
	ScoreHome       *int       `json:"scoreHome"`
	ScoreAway       *int       `json:"scoreAway"`
	PensHome        *int       `json:"pensHome,omitempty"`
	PensAway        *int       `json:"pensAway,omitempty"`
	WinnerID        string     `json:"winnerId,omitempty"`
	SubmittedBy     string     `json:"submittedBy,omitempty"`
	DisputeDeadline *time.Time `json:"disputeDeadline,omitempty"`
	PlayDeadline    *time.Time `json:"playDeadline,omitempty"`
	DisputedBy      string     `json:"disputedBy,omitempty"`
	DisputeReason   string     `json:"disputeReason,omitempty"`
	InGameMatchID   string     `json:"inGameMatchId,omitempty"`
	Stake           int64      `json:"stake"`
	Pool            int64      `json:"pool"`
	Currency        string     `json:"currency"`
	CreatedAt       time.Time  `json:"createdAt"`
	SettledAt       *time.Time `json:"settledAt,omitempty"`
}

const matchSelect = `
SELECT m.id, m.kind, m.game, m.home_id, COALESCE(hu.username,''), COALESCE(hg.gamertag,''), COALESCE(hu.avatar_url,''),
       COALESCE(m.away_id,''), COALESCE(au.username,''), COALESCE(ag.gamertag,''), COALESCE(au.avatar_url,''),
       COALESCE(m.tournament_id,''), COALESCE(t.name,''), COALESCE(m.round,0), m.status,
       m.score_home, m.score_away, m.pens_home, m.pens_away, COALESCE(m.winner_id,''),
       COALESCE(m.submitted_by,''), m.dispute_deadline, m.play_deadline,
       COALESCE(m.disputed_by,''), COALESCE(m.dispute_reason,''), COALESCE(m.ingame_match_id,''),
       COALESCE(e.amount,0), COALESCE(e.pool,0), COALESCE(e.currency, t.currency, 'NGN'),
       m.created_at, m.settled_at
FROM matches m
LEFT JOIN users hu ON hu.id = m.home_id
LEFT JOIN users au ON au.id = m.away_id
LEFT JOIN game_profiles hg ON hg.user_id = m.home_id AND hg.game = m.game
LEFT JOIN game_profiles ag ON ag.user_id = m.away_id AND ag.game = m.game
LEFT JOIN tournaments t ON t.id = m.tournament_id
LEFT JOIN escrow e ON e.challenge_id = m.challenge_id`

func scanMatch(row pgx.Row) (*MatchView, error) {
	var m MatchView
	err := row.Scan(&m.ID, &m.Kind, &m.Game, &m.HomeID, &m.HomeName, &m.HomeTag, &m.HomeAvatar,
		&m.AwayID, &m.AwayName, &m.AwayTag, &m.AwayAvatar,
		&m.TournamentID, &m.TournamentName, &m.Round, &m.Status,
		&m.ScoreHome, &m.ScoreAway, &m.PensHome, &m.PensAway, &m.WinnerID,
		&m.SubmittedBy, &m.DisputeDeadline, &m.PlayDeadline,
		&m.DisputedBy, &m.DisputeReason, &m.InGameMatchID,
		&m.Stake, &m.Pool, &m.Currency, &m.CreatedAt, &m.SettledAt)
	if err != nil {
		return nil, err
	}
	if g := GameByID(m.Game); g != nil {
		m.GameName = g.Short
	}
	return &m, nil
}

func queryMatches(ctx context.Context, where string, args ...any) ([]*MatchView, error) {
	rows, err := db.Pool.Query(ctx, matchSelect+" "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*MatchView{}
	for rows.Next() {
		m, err := scanMatch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetMatch loads one match by id.
func GetMatch(ctx context.Context, id string) (*MatchView, error) {
	m, err := scanMatch(db.Pool.QueryRow(ctx, matchSelect+" WHERE m.id = $1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, userErr(404, "Match not found")
	}
	return m, err
}

// UserMatches lists a player's matches: active ones first, then recent history.
func UserMatches(ctx context.Context, uid string, limit int) ([]*MatchView, error) {
	return queryMatches(ctx, `WHERE (m.home_id = $1 OR m.away_id = $1)
		ORDER BY (m.status IN ('ready','submitted','disputed','review')) DESC, m.created_at DESC LIMIT $2`, uid, limit)
}

// CurrentMatch is the match the player is meant to play or act on right now:
// the oldest unfinished match they are part of (lowest tournament round first).
func CurrentMatch(ctx context.Context, uid string) (*MatchView, error) {
	ms, err := queryMatches(ctx, `WHERE (m.home_id = $1 OR m.away_id = $1)
		AND m.away_id IS NOT NULL AND m.status IN ('ready','submitted','disputed','review')
		ORDER BY m.created_at, COALESCE(m.round,0) LIMIT 1`, uid)
	if err != nil || len(ms) == 0 {
		return nil, err
	}
	return ms[0], nil
}

// LiveMatches lists unfinished matches for the public board.
func LiveMatches(ctx context.Context, limit int) ([]*MatchView, error) {
	return queryMatches(ctx, `WHERE m.status IN ('ready','submitted','disputed') AND m.away_id IS NOT NULL
		ORDER BY m.created_at DESC LIMIT $1`, limit)
}

// MatchesNeedingAdmin lists disputed and review matches, oldest first.
func MatchesNeedingAdmin(ctx context.Context) ([]*MatchView, error) {
	return queryMatches(ctx, `WHERE m.status IN ('disputed','review') ORDER BY m.created_at`)
}

// ExpectedFor returns a player's registered name/ID for a game (nil if not saved).
func ExpectedFor(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, uid, game string) (*ExpectedPlayer, error) {
	var p ExpectedPlayer
	err := q.QueryRow(ctx, "SELECT gamertag, COALESCE(ingame_id,'') FROM game_profiles WHERE user_id = $1 AND game = $2", uid, game).
		Scan(&p.Gamertag, &p.InGameID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &p, err
}

// RequireGameProfile returns a UserError if the player has not saved their
// in-game name for the game.
func RequireGameProfile(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, uid, game string) error {
	p, err := ExpectedFor(ctx, q, uid, game)
	if err != nil {
		return err
	}
	if p == nil {
		g := GameByID(game)
		name := game
		if g != nil {
			name = g.Short
		}
		return userErr(400, "Add your %s name in Profile → Game IDs before playing", name)
	}
	return nil
}

// CreateMatch inserts a ready match inside an existing transaction and locks
// both players' game profiles (names can't change once money is involved).
func CreateMatch(ctx context.Context, tx pgx.Tx, kind, game, homeID, awayID, challengeID, tournamentID string, round, slot int, playDeadline time.Time) (string, error) {
	id := utils.GenerateMatchID()
	var away, chal, tourn any
	if awayID != "" {
		away = awayID
	}
	if challengeID != "" {
		chal = challengeID
	}
	if tournamentID != "" {
		tourn = tournamentID
	}
	var rnd, slt any
	if kind == "tournament" {
		rnd, slt = round, slot
	}
	status := StatusReady
	if awayID == "" {
		status = StatusConfirmed // bye: auto-advance (tournament only)
	}
	_, err := tx.Exec(ctx, `INSERT INTO matches (id, kind, game, home_id, away_id, challenge_id, tournament_id, round, slot, status, play_deadline, winner_id, settled_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11, CASE WHEN $5::text IS NULL THEN $4 END, CASE WHEN $5::text IS NULL THEN NOW() END)`,
		id, kind, game, homeID, away, chal, tourn, rnd, slt, status, playDeadline)
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, "UPDATE game_profiles SET locked = TRUE WHERE game = $1 AND user_id IN ($2, $3)", game, homeID, awayID)
	return id, err
}

// ── Result submission ────────────────────────────────────────────────

// SubmitResult runs the screenshot through OCR, checks it against the match's
// registered players and opens the dispute window. Only participants of a
// ready match can submit; the screenshot and in-game match ID can only be used once.
func SubmitResult(ctx context.Context, matchID, uid string, img []byte) (*MatchView, error) {
	if len(img) == 0 {
		return nil, userErr(400, "Please attach a screenshot of the final result screen")
	}
	if len(img) > MaxScreenshotLen {
		return nil, userErr(400, "Screenshot is too large (max 8 MB)")
	}

	m, err := GetMatch(ctx, matchID)
	if err != nil {
		return nil, err
	}
	if uid != m.HomeID && uid != m.AwayID {
		return nil, userErr(403, "You are not a player in this match")
	}
	if m.Status != StatusReady {
		return nil, userErr(409, "A result has already been submitted for this match")
	}
	game := GameByID(m.Game)
	if game == nil {
		return nil, userErr(400, "This game is not supported for verified results")
	}

	sum := sha256.Sum256(img)
	hash := hex.EncodeToString(sum[:])
	var used bool
	db.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM matches WHERE screenshot_hash = $1)", hash).Scan(&used)
	if used {
		return nil, userErr(409, "This screenshot has already been used for another match")
	}

	home, err := ExpectedFor(ctx, db.Pool, m.HomeID, m.Game)
	if err != nil {
		return nil, err
	}
	away, err := ExpectedFor(ctx, db.Pool, m.AwayID, m.Game)
	if err != nil {
		return nil, err
	}
	if home == nil || away == nil {
		return nil, userErr(400, "Both players must save their %s name before a result can be verified", game.Short)
	}

	res, err := AnalyzeScreenshot(ctx, img, game, []ExpectedPlayer{*home, *away})
	if err != nil {
		log.Printf("[OCR] match %s: %v", matchID, err)
		return nil, userErr(502, "Our score reader is very busy right now. Nothing was lost: please tap Try again in a minute.")
	}
	ocrJSON, _ := json.Marshal(res)

	switch {
	case !res.IsVersusHuman:
		return nil, userErr(422, "This screenshot is not from a match against another player (%s). Upload your %s player-vs-player result.", nonEmpty(res.Mode, "vs AI / scenario"), game.Short)
	case !res.IsResultScreen:
		return nil, userErr(422, "This is not the final result screen. %s", nonEmpty(res.RejectReason, "Upload the full-time screen that shows both player names and the score."))
	case res.Confidence < MinOCRConfidence:
		return nil, userErr(422, "The screenshot is not clear enough to read the score. Upload a sharper, uncropped screenshot.")
	}

	hi, ai, err := MatchPlayers(res, *home, *away)
	if err != nil {
		return nil, userErr(422, "%s. Make sure the screenshot shows both players' names exactly as saved in your Game IDs.", capitalize(err.Error()))
	}
	hp, ap := res.Players[hi], res.Players[ai]

	winner := ""
	switch {
	case hp.Score > ap.Score:
		winner = m.HomeID
	case ap.Score > hp.Score:
		winner = m.AwayID
	case hp.PenaltyScore != nil && ap.PenaltyScore != nil && *hp.PenaltyScore != *ap.PenaltyScore:
		if *hp.PenaltyScore > *ap.PenaltyScore {
			winner = m.HomeID
		} else {
			winner = m.AwayID
		}
	}

	if winner == "" && m.Kind == "tournament" {
		var format string
		db.Pool.QueryRow(ctx, "SELECT format FROM tournaments WHERE id = $1", m.TournamentID).Scan(&format)
		if format == "knockout" {
			return nil, userErr(422, "Knockout matches need a winner. Upload the screen that shows the penalty shoot-out result.")
		}
	}

	var ingame any
	if res.InGameMatchID != "" {
		ingame = res.InGameMatchID
	}
	deadline := time.Now().Add(DisputeWindow)
	tag, err := db.Pool.Exec(ctx, `UPDATE matches SET status = 'submitted', score_home = $2, score_away = $3,
		pens_home = $4, pens_away = $5, winner_id = NULLIF($6,''), submitted_by = $7, submitted_at = NOW(),
		dispute_deadline = $8, ingame_match_id = $9, screenshot_hash = $10, ocr_result = $11::jsonb
		WHERE id = $1 AND status = 'ready'`,
		matchID, hp.Score, ap.Score, hp.PenaltyScore, ap.PenaltyScore, winner, uid, deadline, ingame, hash, string(ocrJSON))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, userErr(409, "This result (same in-game match or screenshot) has already been used for another match")
		}
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, userErr(409, "A result has already been submitted for this match")
	}
	if err := SaveEvidence(ctx, matchID, uid, "result", img); err != nil {
		log.Printf("[EVIDENCE] save result screenshot for %s: %v", matchID, err)
	}

	m, err = GetMatch(ctx, matchID)
	if err != nil {
		return nil, err
	}
	opponent := m.AwayID
	if uid == m.AwayID {
		opponent = m.HomeID
	}
	score := fmt.Sprintf("%s %d – %d %s", m.HomeTag, hp.Score, ap.Score, m.AwayTag)
	Notify(opponent, "result_submitted", "Result submitted — check it",
		score+". Confirm or dispute within 15 minutes, otherwise it becomes final.",
		"/match/"+m.ID, map[string]any{"matchId": m.ID})
	broadcastMatch(m)
	return m, nil
}

// ConfirmResult lets the opponent accept a submitted result early.
func ConfirmResult(ctx context.Context, matchID, uid string) (*MatchView, error) {
	m, err := GetMatch(ctx, matchID)
	if err != nil {
		return nil, err
	}
	if uid != m.HomeID && uid != m.AwayID {
		return nil, userErr(403, "You are not a player in this match")
	}
	if m.Status != StatusSubmitted {
		return nil, userErr(409, "There is no pending result to confirm")
	}
	if uid == m.SubmittedBy {
		return nil, userErr(400, "Your opponent needs to confirm the result you submitted")
	}
	if err := SettleMatch(ctx, matchID, uid); err != nil {
		return nil, err
	}
	return GetMatch(ctx, matchID)
}

// DisputeResult stops the automatic settlement and sends the match to an admin.
func DisputeResult(ctx context.Context, matchID, uid, reason string, evidence []byte) (*MatchView, error) {
	if len(reason) > 500 {
		reason = reason[:500]
	}
	if len(evidence) > MaxScreenshotLen {
		return nil, userErr(400, "Screenshot is too large (max 8 MB)")
	}
	if len(evidence) > 0 {
		if _, err := evidenceJPEG(evidence); err != nil { // reject unreadable files before disputing
			return nil, err
		}
	}
	tag, err := db.Pool.Exec(ctx, `UPDATE matches SET status = 'disputed', disputed_by = $2, dispute_reason = $3
		WHERE id = $1 AND status = 'submitted' AND submitted_by <> $2 AND (home_id = $2 OR away_id = $2)
		AND dispute_deadline > NOW()`, matchID, uid, reason)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, userErr(409, "This result can no longer be disputed")
	}
	if len(evidence) > 0 {
		if err := SaveEvidence(ctx, matchID, uid, "dispute", evidence); err != nil {
			log.Printf("[EVIDENCE] save dispute screenshot for %s: %v", matchID, err)
		}
	}
	m, err := GetMatch(ctx, matchID)
	if err != nil {
		return nil, err
	}
	Notify(m.SubmittedBy, "result_disputed", "Result disputed",
		"Your opponent disputed the result. An admin will review it and decide.", "/match/"+m.ID, map[string]any{"matchId": m.ID})
	notifyAdmins(ctx, "Match disputed", fmt.Sprintf("%s vs %s (%s) needs review.", m.HomeTag, m.AwayTag, m.GameName), m.ID)
	broadcastMatch(m)
	return m, nil
}

// ── Settlement ───────────────────────────────────────────────────────

// SettleMatch makes a submitted/disputed/review result final and pays out.
// It is idempotent: an already-confirmed match is left alone.
func SettleMatch(ctx context.Context, matchID, resolvedBy string) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var kind, status, homeID, game string
	var awayID, winnerID, challengeID, tournamentID *string
	var scoreHome, scoreAway *int
	err = tx.QueryRow(ctx, `SELECT kind, status, home_id, away_id, winner_id, challenge_id, tournament_id, score_home, score_away, game
		FROM matches WHERE id = $1 FOR UPDATE`, matchID).
		Scan(&kind, &status, &homeID, &awayID, &winnerID, &challengeID, &tournamentID, &scoreHome, &scoreAway, &game)
	if err != nil {
		return err
	}
	if status == StatusConfirmed || status == StatusVoid {
		return nil
	}
	if scoreHome == nil || scoreAway == nil || awayID == nil {
		return userErr(409, "This match has no result to settle")
	}

	if _, err := tx.Exec(ctx, "UPDATE matches SET status = 'confirmed', settled_at = NOW(), resolved_by = $2 WHERE id = $1", matchID, resolvedBy); err != nil {
		return err
	}

	winner := deref(winnerID)
	switch kind {
	case "p2p":
		if err := settleEscrow(ctx, tx, deref(challengeID), winner, homeID, *awayID); err != nil {
			return err
		}
	case "tournament":
		if err := onTournamentMatchSettled(ctx, tx, deref(tournamentID), matchID, homeID, *awayID, winner, *scoreHome, *scoreAway); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	m, err := GetMatch(ctx, matchID)
	if err == nil {
		announceResult(m)
		broadcastMatch(m)
	}
	// A first paid match unlocks invite rewards for either player.
	go func(a, b string) {
		RewardReferral(context.Background(), a)
		RewardReferral(context.Background(), b)
	}(homeID, *awayID)
	if kind == "tournament" {
		go AdvanceTournament(context.Background(), deref(tournamentID))
	}
	return nil
}

// settleEscrow pays the 1v1 winner P2PWinnerPercent of the pool, or refunds both on a draw.
func settleEscrow(ctx context.Context, tx pgx.Tx, challengeID, winner, homeID, awayID string) error {
	var escrowID int64
	var amount, pool int64
	var status string
	err := tx.QueryRow(ctx, "SELECT id, amount, pool, status FROM escrow WHERE challenge_id = $1 FOR UPDATE", challengeID).
		Scan(&escrowID, &amount, &pool, &status)
	if err != nil {
		return fmt.Errorf("escrow for %s: %w", challengeID, err)
	}
	if status != "held" {
		return nil
	}
	if winner == "" {
		if err := AdjustBalance(ctx, tx, homeID, amount, "wager_refund", challengeID); err != nil {
			return err
		}
		if err := AdjustBalance(ctx, tx, awayID, amount, "wager_refund", challengeID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "UPDATE escrow SET status = 'refunded' WHERE id = $1", escrowID)
		return err
	}
	payout := pool * P2PWinnerPercent / 100
	meta := fmt.Sprintf(`{"pool": %d, "fee": %d}`, pool, pool-payout)
	if err := AdjustBalance(ctx, tx, winner, payout, "wager_win", challengeID, meta); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "UPDATE escrow SET status = 'paid_out' WHERE id = $1", escrowID)
	return err
}

// VoidMatch cancels a match. 1v1 stakes are refunded; a tournament match is
// reset so the players can replay it.
func VoidMatch(ctx context.Context, matchID, by, reason string) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var kind, status, homeID, awayID string
	var challengeID, tournamentID *string
	if err := tx.QueryRow(ctx, "SELECT kind, status, home_id, COALESCE(away_id,''), challenge_id, tournament_id FROM matches WHERE id = $1 FOR UPDATE", matchID).
		Scan(&kind, &status, &homeID, &awayID, &challengeID, &tournamentID); err != nil {
		return err
	}
	if status == StatusConfirmed || status == StatusVoid {
		return userErr(409, "This match is already final")
	}

	if kind == "tournament" {
		var hours int
		tx.QueryRow(ctx, "SELECT round_hours FROM tournaments WHERE id = $1", deref(tournamentID)).Scan(&hours)
		if hours <= 0 {
			hours = 24
		}
		_, err = tx.Exec(ctx, `UPDATE matches SET status = 'ready', score_home = NULL, score_away = NULL, pens_home = NULL, pens_away = NULL,
			winner_id = NULL, submitted_by = NULL, submitted_at = NULL, dispute_deadline = NULL, disputed_by = NULL, dispute_reason = NULL,
			ingame_match_id = NULL, screenshot_hash = NULL, ocr_result = NULL, play_deadline = $2 WHERE id = $1`,
			matchID, time.Now().Add(time.Duration(hours)*time.Hour))
		if err != nil {
			return err
		}
	} else {
		var escrowID, amount int64
		var est string
		if err := tx.QueryRow(ctx, "SELECT id, amount, status FROM escrow WHERE challenge_id = $1 FOR UPDATE", deref(challengeID)).Scan(&escrowID, &amount, &est); err != nil {
			return err
		}
		if est == "held" {
			if err := AdjustBalance(ctx, tx, homeID, amount, "wager_refund", deref(challengeID)); err != nil {
				return err
			}
			if err := AdjustBalance(ctx, tx, awayID, amount, "wager_refund", deref(challengeID)); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, "UPDATE escrow SET status = 'refunded' WHERE id = $1", escrowID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, "UPDATE matches SET status = 'void', settled_at = NOW(), resolved_by = $2, dispute_reason = COALESCE(dispute_reason, $3) WHERE id = $1", matchID, by, reason); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	msg := "The match was cancelled and your stake refunded. " + reason
	if kind == "tournament" {
		msg = "The result was cleared. Please replay the match and submit a new screenshot. " + reason
	}
	for _, p := range []string{homeID, awayID} {
		Notify(p, "match_void", "Match reset", msg, "/match/"+matchID, map[string]any{"matchId": matchID})
	}
	if m, err := GetMatch(ctx, matchID); err == nil {
		broadcastMatch(m)
	}
	return nil
}

// AdminResolve sets the final score for a disputed/review match and settles it.
func AdminResolve(ctx context.Context, matchID, adminID string, scoreHome, scoreAway int, pensHome, pensAway *int) error {
	var homeID, awayID string
	if err := db.Pool.QueryRow(ctx, "SELECT home_id, COALESCE(away_id,'') FROM matches WHERE id = $1", matchID).Scan(&homeID, &awayID); err != nil {
		return userErr(404, "Match not found")
	}
	winner := ""
	switch {
	case scoreHome > scoreAway:
		winner = homeID
	case scoreAway > scoreHome:
		winner = awayID
	case pensHome != nil && pensAway != nil && *pensHome != *pensAway:
		if *pensHome > *pensAway {
			winner = homeID
		} else {
			winner = awayID
		}
	}
	tag, err := db.Pool.Exec(ctx, `UPDATE matches SET score_home = $2, score_away = $3, pens_home = $4, pens_away = $5, winner_id = NULLIF($6,'')
		WHERE id = $1 AND status IN ('submitted','disputed','review','ready')`, matchID, scoreHome, scoreAway, pensHome, pensAway, winner)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return userErr(409, "This match is already final")
	}
	return SettleMatch(ctx, matchID, adminID)
}

// ── Helpers ──────────────────────────────────────────────────────────

func announceResult(m *MatchView) {
	if m.ScoreHome == nil || m.ScoreAway == nil {
		return
	}
	score := fmt.Sprintf("%s %d – %d %s", m.HomeTag, *m.ScoreHome, *m.ScoreAway, m.AwayTag)
	for _, p := range []string{m.HomeID, m.AwayID} {
		title, body := "Match drawn", score+"."
		switch {
		case m.WinnerID == p:
			title = "You won! 🏆"
			if m.Kind == "p2p" {
				body = fmt.Sprintf("%s. %s winnings added to your wallet.", score, FormatMoney(m.Pool*P2PWinnerPercent/100, m.Currency))
			} else {
				body = score + ". You advance in " + m.TournamentName + "."
			}
		case m.WinnerID != "":
			title = "Match result confirmed"
		default:
			if m.Kind == "p2p" {
				body = score + ". Your stake has been refunded."
			}
		}
		Notify(p, "result_confirmed", title, body, "/match/"+m.ID, map[string]any{"matchId": m.ID})
	}
}

func broadcastMatch(m *MatchView) {
	if Hub != nil {
		ws.BroadcastEvent(Hub, "match_update", m)
	}
}

func notifyAdmins(ctx context.Context, title, message, matchID string) {
	rows, err := db.Pool.Query(ctx, "SELECT id FROM users WHERE is_admin")
	if err != nil {
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		Notify(id, "admin_review", title, message, "/admin", map[string]any{"matchId": matchID})
	}
}

// FormatMoney renders minor units (kobo/pesewas) as "₦1,500".
func FormatMoney(minor int64, currency string) string {
	sym := "₦"
	if currency == "GHS" {
		sym = "₵"
	}
	major := minor / 100
	s := fmt.Sprintf("%d", major)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if cents := minor % 100; cents != 0 {
		return fmt.Sprintf("%s%s.%02d", sym, s, cents)
	}
	return sym + s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// TournamentMatches lists a tournament's fixtures in round order.
func TournamentMatches(ctx context.Context, tournamentID string) ([]*MatchView, error) {
	return queryMatches(ctx, "WHERE m.tournament_id = $1 ORDER BY m.round, m.slot", tournamentID)
}
