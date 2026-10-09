package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"betelite-go/db"
)

// Default prize splits, in percent of total entry fees collected.
var (
	DefaultKnockoutSplit = []float64{70}
	DefaultLeagueSplit   = []float64{30, 10, 4.5, 4.5}
)

// DefaultSplit returns the default prize split for a tournament format.
func DefaultSplit(format string) []float64 {
	if format == "league" {
		return DefaultLeagueSplit
	}
	return DefaultKnockoutSplit
}

// Standing is one row of a tournament table.
type Standing struct {
	UserID       string `json:"userId"`
	Username     string `json:"username"`
	Gamertag     string `json:"gamertag"`
	AvatarURL    string `json:"avatarUrl"`
	Played       int    `json:"played"`
	Wins         int    `json:"wins"`
	Draws        int    `json:"draws"`
	Losses       int    `json:"losses"`
	GoalsFor     int    `json:"goalsFor"`
	GoalsAgainst int    `json:"goalsAgainst"`
	Points       int    `json:"points"`
	Eliminated   bool   `json:"eliminated"`
	Position     *int   `json:"position,omitempty"`
	Tier         string `json:"tier"` // membership badge
}

// Standings returns the table, sorted by points, goal difference, goals scored.
func Standings(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, tournamentID string) ([]Standing, error) {
	rows, err := q.Query(ctx, `
		SELECT tp.user_id, COALESCE(u.username,''), COALESCE(gp.gamertag,''), COALESCE(u.avatar_url,''),
		       tp.wins, tp.draws, tp.losses, tp.goals, tp.goals_against, tp.points, tp.eliminated, tp.final_position,
		       COALESCE((SELECT m.tier FROM memberships m WHERE m.user_id = tp.user_id AND m.starts_at <= NOW() AND m.ends_at > NOW() ORDER BY m.ends_at DESC LIMIT 1), 'free')
		FROM tournament_players tp
		JOIN tournaments t ON t.id = tp.tournament_id
		LEFT JOIN users u ON u.id = tp.user_id
		LEFT JOIN game_profiles gp ON gp.user_id = tp.user_id AND gp.game = t.game
		WHERE tp.tournament_id = $1`, tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Standing
	for rows.Next() {
		var s Standing
		if err := rows.Scan(&s.UserID, &s.Username, &s.Gamertag, &s.AvatarURL, &s.Wins, &s.Draws, &s.Losses,
			&s.GoalsFor, &s.GoalsAgainst, &s.Points, &s.Eliminated, &s.Position, &s.Tier); err != nil {
			return nil, err
		}
		s.Played = s.Wins + s.Draws + s.Losses
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Position != nil && b.Position != nil {
			return *a.Position < *b.Position
		}
		if a.Points != b.Points {
			return a.Points > b.Points
		}
		if gd1, gd2 := a.GoalsFor-a.GoalsAgainst, b.GoalsFor-b.GoalsAgainst; gd1 != gd2 {
			return gd1 > gd2
		}
		if a.GoalsFor != b.GoalsFor {
			return a.GoalsFor > b.GoalsFor
		}
		return a.Wins > b.Wins
	})
	return out, rows.Err()
}

// StartTournament seeds players randomly and creates fixtures. Called inside
// the join transaction when the last seat is taken.
func StartTournament(ctx context.Context, tx pgx.Tx, tournamentID string) error {
	var format, game string
	var hours int
	if err := tx.QueryRow(ctx, "SELECT format, game, round_hours FROM tournaments WHERE id = $1", tournamentID).Scan(&format, &game, &hours); err != nil {
		return err
	}
	if hours <= 0 {
		hours = 24
	}
	rows, err := tx.Query(ctx, "SELECT user_id FROM tournament_players WHERE tournament_id = $1", tournamentID)
	if err != nil {
		return err
	}
	var players []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			players = append(players, id)
		}
	}
	rows.Close()
	rand.Shuffle(len(players), func(i, j int) { players[i], players[j] = players[j], players[i] })

	now := time.Now()
	roundLen := time.Duration(hours) * time.Hour
	if format == "league" {
		for r, pairs := range roundRobin(players) {
			deadline := now.Add(time.Duration(r+1) * roundLen)
			for slot, p := range pairs {
				if _, err := CreateMatch(ctx, tx, "tournament", game, p[0], p[1], "", tournamentID, r+1, slot, deadline); err != nil {
					return err
				}
			}
		}
	} else {
		if err := createKnockoutRound(ctx, tx, tournamentID, game, 1, players, now.Add(roundLen)); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, "UPDATE tournaments SET status = 'active', current_round = 1, started_at = NOW() WHERE id = $1", tournamentID)
	return err
}

// roundRobin pairs every player with every other exactly once (circle method).
// A "" entry is a bye and its pairing is skipped.
func roundRobin(players []string) [][][2]string {
	p := append([]string{}, players...)
	if len(p)%2 == 1 {
		p = append(p, "")
	}
	n := len(p)
	var rounds [][][2]string
	for r := 0; r < n-1; r++ {
		var pairs [][2]string
		for i := 0; i < n/2; i++ {
			a, b := p[i], p[n-1-i]
			if a == "" || b == "" {
				continue
			}
			if r%2 == 1 { // alternate home/away
				a, b = b, a
			}
			pairs = append(pairs, [2]string{a, b})
		}
		rounds = append(rounds, pairs)
		// rotate all but the first
		last := p[n-1]
		copy(p[2:], p[1:n-1])
		p[1] = last
	}
	return rounds
}

// createKnockoutRound pairs players in order; an odd player out gets a bye.
func createKnockoutRound(ctx context.Context, tx pgx.Tx, tournamentID, game string, round int, players []string, deadline time.Time) error {
	for i := 0; i < len(players); i += 2 {
		away := ""
		if i+1 < len(players) {
			away = players[i+1]
		}
		if _, err := CreateMatch(ctx, tx, "tournament", game, players[i], away, "", tournamentID, round, i/2, deadline); err != nil {
			return err
		}
	}
	return nil
}

// onTournamentMatchSettled updates the table for a confirmed match.
func onTournamentMatchSettled(ctx context.Context, tx pgx.Tx, tournamentID, matchID, homeID, awayID, winner string, sh, sa int) error {
	var format string
	if err := tx.QueryRow(ctx, "SELECT format FROM tournaments WHERE id = $1", tournamentID).Scan(&format); err != nil {
		return err
	}
	if format == "knockout" && winner == "" {
		return userErr(409, "Knockout matches need a winner (penalties)")
	}
	update := func(uid string, gf, ga int, res string) error {
		w, d, l, pts := 0, 0, 0, 0
		switch res {
		case "w":
			w, pts = 1, 3
		case "d":
			d, pts = 1, 1
		default:
			l = 1
		}
		elim := format == "knockout" && res == "l"
		_, err := tx.Exec(ctx, `UPDATE tournament_players SET wins = wins + $3, draws = draws + $4, losses = losses + $5,
			goals = goals + $6, goals_against = goals_against + $7, points = points + $8, eliminated = eliminated OR $9
			WHERE tournament_id = $1 AND user_id = $2`, tournamentID, uid, w, d, l, gf, ga, pts, elim)
		return err
	}
	hr, ar := "d", "d"
	if winner == homeID {
		hr, ar = "w", "l"
	} else if winner == awayID {
		hr, ar = "l", "w"
	}
	if err := update(homeID, sh, sa, hr); err != nil {
		return err
	}
	return update(awayID, sa, sh, ar)
}

// AdvanceTournament moves a tournament forward once every match of the
// current round is final: next knockout round, or finish and pay prizes.
func AdvanceTournament(ctx context.Context, tournamentID string) {
	if tournamentID == "" || db.Pool == nil {
		return
	}
	if err := advanceTournament(ctx, tournamentID); err != nil {
		log.Printf("[TOURNAMENT] advance %s: %v", tournamentID, err)
	}
}

func advanceTournament(ctx context.Context, tournamentID string) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var format, game, status, name string
	var round, hours int
	if err := tx.QueryRow(ctx, "SELECT format, game, status, current_round, round_hours, name FROM tournaments WHERE id = $1 FOR UPDATE", tournamentID).
		Scan(&format, &game, &status, &round, &hours, &name); err != nil {
		return err
	}
	if status != "active" {
		return nil
	}

	var openThisRound, openTotal int
	tx.QueryRow(ctx, "SELECT COUNT(*) FILTER (WHERE round = $2), COUNT(*) FROM matches WHERE tournament_id = $1 AND status NOT IN ('confirmed','void')", tournamentID, round).
		Scan(&openThisRound, &openTotal)

	if format == "swiss" {
		if openThisRound > 0 {
			return tx.Commit(ctx)
		}
		if err := advanceSwiss(ctx, tx, tournamentID, game, name, round, hours); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	if format == "league" {
		if openTotal > 0 {
			// Track the earliest round that still has matches to play.
			_, err := tx.Exec(ctx, `UPDATE tournaments SET current_round = COALESCE((SELECT MIN(round) FROM matches
				WHERE tournament_id = $1 AND status NOT IN ('confirmed','void')), current_round) WHERE id = $1`, tournamentID)
			if err != nil {
				return err
			}
			return tx.Commit(ctx)
		}
		table, err := Standings(ctx, tx, tournamentID)
		if err != nil {
			return err
		}
		order := make([]string, len(table))
		for i, s := range table {
			order[i] = s.UserID
		}
		if err := finishTournament(ctx, tx, tournamentID, order); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	if openThisRound > 0 {
		return tx.Commit(ctx)
	}

	rows, err := tx.Query(ctx, "SELECT winner_id FROM matches WHERE tournament_id = $1 AND round = $2 AND winner_id IS NOT NULL ORDER BY slot", tournamentID, round)
	if err != nil {
		return err
	}
	var winners []string
	for rows.Next() {
		var w string
		if rows.Scan(&w) == nil {
			winners = append(winners, w)
		}
	}
	rows.Close()

	if len(winners) <= 1 {
		// Final played. Champion first, then everyone else by round reached.
		order := winners
		rest, err := tx.Query(ctx, `SELECT tp.user_id FROM tournament_players tp
			LEFT JOIN matches m ON m.tournament_id = tp.tournament_id AND (m.home_id = tp.user_id OR m.away_id = tp.user_id)
			WHERE tp.tournament_id = $1 GROUP BY tp.user_id ORDER BY MAX(COALESCE(m.round,0)) DESC, MAX(tp.goals) DESC`, tournamentID)
		if err != nil {
			return err
		}
		for rest.Next() {
			var id string
			if rest.Scan(&id) == nil && (len(order) == 0 || id != order[0]) {
				order = append(order, id)
			}
		}
		rest.Close()
		if err := finishTournament(ctx, tx, tournamentID, order); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	if hours <= 0 {
		hours = 24
	}
	next := round + 1
	if err := createKnockoutRound(ctx, tx, tournamentID, game, next, winners, time.Now().Add(time.Duration(hours)*time.Hour)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "UPDATE tournaments SET current_round = $2 WHERE id = $1", tournamentID, next); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	stage := roundName(len(winners))
	for _, w := range winners {
		Notify(w, "tournament_round", name+": "+stage, "Your next match is ready. Play it and upload the result before the deadline.", "/tournaments/"+tournamentID, map[string]any{"tournamentId": tournamentID})
	}
	// A bye in the new round is already final; keep going if that ended the round.
	return advanceIfByesOnly(ctx, tournamentID)
}

func advanceIfByesOnly(ctx context.Context, tournamentID string) error {
	var open int
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM matches m JOIN tournaments t ON t.id = m.tournament_id
		WHERE m.tournament_id = $1 AND m.round = t.current_round AND m.status NOT IN ('confirmed','void')`, tournamentID).Scan(&open)
	if open == 0 {
		return advanceTournament(ctx, tournamentID)
	}
	return nil
}

func roundName(players int) string {
	switch players {
	case 2:
		return "Final"
	case 3, 4:
		return "Semi-final"
	case 5, 6, 7, 8:
		return "Quarter-final"
	}
	return fmt.Sprintf("Round of %d", players)
}

// finishTournament records final positions, pays prizes and closes the tournament.
func finishTournament(ctx context.Context, tx pgx.Tx, tournamentID string, order []string) error {
	var entryFee, seed int64
	var splitJSON, fixedJSON []byte
	var format, name, currency string
	if err := tx.QueryRow(ctx, "SELECT entry_fee, prize_pool, prize_split, format, name, currency, prizes FROM tournaments WHERE id = $1", tournamentID).
		Scan(&entryFee, &seed, &splitJSON, &format, &name, &currency, &fixedJSON); err != nil {
		return err
	}
	if fixed := ParseFixedPrizes(fixedJSON); fixed != nil {
		return finishWithFixedPrizes(ctx, tx, tournamentID, name, order, fixed)
	}
	split := DefaultSplit(format)
	if len(splitJSON) > 0 {
		var s []float64
		if json.Unmarshal(splitJSON, &s) == nil && len(s) > 0 {
			split = s
		}
	}
	prizes := PrizeAmounts(entryFee, int64(len(order)), seed, split)

	for i, uid := range order {
		if _, err := tx.Exec(ctx, "UPDATE tournament_players SET final_position = $3 WHERE tournament_id = $1 AND user_id = $2", tournamentID, uid, i+1); err != nil {
			return err
		}
		if i < len(prizes) && prizes[i] > 0 {
			meta := fmt.Sprintf(`{"position": %d}`, i+1)
			if err := AdjustBalance(ctx, tx, uid, prizes[i], "tournament_prize", tournamentID, meta); err != nil {
				return err
			}
		}
	}
	winner := ""
	if len(order) > 0 {
		winner = order[0]
	}
	if _, err := tx.Exec(ctx, "UPDATE tournaments SET status = 'finished', winner_id = NULLIF($2,''), finished_at = NOW() WHERE id = $1", tournamentID, winner); err != nil {
		return err
	}

	go func() {
		for i, uid := range order {
			msg := fmt.Sprintf("%s has finished. You placed #%d.", name, i+1)
			title := "Tournament finished"
			if i < len(prizes) && prizes[i] > 0 {
				title = fmt.Sprintf("You placed #%d! 🏆", i+1)
				msg = fmt.Sprintf("%s prize from %s has been added to your wallet.", FormatMoney(prizes[i], currency), name)
			}
			Notify(uid, "tournament_finished", title, msg, "/tournaments/"+tournamentID, map[string]any{"tournamentId": tournamentID})
		}
	}()
	return nil
}

// PrizeAmounts turns a percentage split into amounts. Paid tournaments pay a
// percentage of total entry fees (the rest is the platform's share). Free
// tournaments distribute the admin-seeded pool in the same proportions.
func PrizeAmounts(entryFee, players, seed int64, split []float64) []int64 {
	out := make([]int64, len(split))
	if entryFee > 0 {
		total := entryFee * players
		for i, pct := range split {
			out[i] = int64(math.Floor(float64(total) * pct / 100))
		}
		return out
	}
	var sum float64
	for _, pct := range split {
		sum += pct
	}
	if seed <= 0 || sum <= 0 {
		return out
	}
	for i, pct := range split {
		out[i] = int64(math.Floor(float64(seed) * pct / sum))
	}
	return out
}

// SwissRounds is how many rounds a Swiss tournament plays: enough for one
// unbeaten leader (8 players → 3, 16 → 4, 32 → 5).
func SwissRounds(players int) int {
	r := 1
	for 1<<r < players {
		r++
	}
	return max(r, 2)
}

// advanceSwiss pairs the next Swiss round (players with similar points meet,
// never the same opponent twice) or finishes the tournament on the table.
func advanceSwiss(ctx context.Context, tx pgx.Tx, tournamentID, game, name string, round, hours int) error {
	table, err := Standings(ctx, tx, tournamentID)
	if err != nil {
		return err
	}
	if round >= SwissRounds(len(table)) {
		order := make([]string, len(table))
		for i, s := range table {
			order[i] = s.UserID
		}
		return finishTournament(ctx, tx, tournamentID, order)
	}

	met := map[[2]string]bool{}
	rows, err := tx.Query(ctx, "SELECT home_id, COALESCE(away_id,'') FROM matches WHERE tournament_id = $1", tournamentID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var a, b string
		if rows.Scan(&a, &b) == nil {
			met[[2]string{a, b}], met[[2]string{b, a}] = true, true
		}
	}
	rows.Close()

	ids := make([]string, len(table))
	for i, s := range table {
		ids[i] = s.UserID
	}
	pairs := swissPairs(ids, met)
	if hours <= 0 {
		hours = 24
	}
	next := round + 1
	deadline := time.Now().Add(time.Duration(hours) * time.Hour)
	for slot, p := range pairs {
		if _, err := CreateMatch(ctx, tx, "tournament", game, p[0], p[1], "", tournamentID, next, slot, deadline); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, "UPDATE tournaments SET current_round = $2 WHERE id = $1", tournamentID, next); err != nil {
		return err
	}
	go func() {
		for _, p := range pairs {
			for _, uid := range p {
				Notify(uid, "tournament_round", fmt.Sprintf("%s: round %d", name, next), "Your next match is ready. Play it and upload the result before the deadline.", "/tournaments/"+tournamentID, map[string]any{"tournamentId": tournamentID})
			}
		}
	}()
	return nil
}

// swissPairs pairs players in table order, each with the highest-placed
// opponent they haven't met yet. It backtracks when a greedy choice would
// leave the last players with only rematches, and allows a rematch only when
// no other pairing exists. The player count is always even.
func swissPairs(ids []string, met map[[2]string]bool) [][2]string {
	var try func(left []string, allowRematch bool) [][2]string
	try = func(left []string, allowRematch bool) [][2]string {
		if len(left) == 0 {
			return [][2]string{}
		}
		a := left[0]
		for i := 1; i < len(left); i++ {
			b := left[i]
			if met[[2]string{a, b}] && !allowRematch {
				continue
			}
			rest := append(append([]string{}, left[1:i]...), left[i+1:]...)
			if sub := try(rest, allowRematch); sub != nil {
				return append([][2]string{{a, b}}, sub...)
			}
		}
		return nil
	}
	if p := try(ids, false); p != nil {
		return p
	}
	return try(ids, true)
}

// FixedPrizes are set by an admin for each currency (minor units, 1st place
// first). Players from Ghana and Nigeria play together and each winner is paid
// in their own wallet currency.
type FixedPrizes map[string][]int64

// ParseFixedPrizes reads the tournaments.prizes column; nil when unset.
func ParseFixedPrizes(b []byte) FixedPrizes {
	if len(b) == 0 {
		return nil
	}
	var p FixedPrizes
	if json.Unmarshal(b, &p) != nil || len(p) == 0 {
		return nil
	}
	return p
}

// Places is the number of paid positions.
func (p FixedPrizes) Places() int {
	n := 0
	for _, list := range p {
		n = max(n, len(list))
	}
	return n
}

func finishWithFixedPrizes(ctx context.Context, tx pgx.Tx, tournamentID, name string, order []string, prizes FixedPrizes) error {
	type award struct {
		amount   int64
		currency string
	}
	awards := make([]award, len(order))
	for i, uid := range order {
		if _, err := tx.Exec(ctx, "UPDATE tournament_players SET final_position = $3 WHERE tournament_id = $1 AND user_id = $2", tournamentID, uid, i+1); err != nil {
			return err
		}
		var cur string
		tx.QueryRow(ctx, "SELECT COALESCE(currency,'NGN') FROM users WHERE id = $1", uid).Scan(&cur)
		list := prizes[cur]
		if i >= len(list) || list[i] <= 0 {
			continue
		}
		meta := fmt.Sprintf(`{"position": %d}`, i+1)
		if err := AdjustBalance(ctx, tx, uid, list[i], "tournament_prize", tournamentID, meta); err != nil {
			return err
		}
		awards[i] = award{list[i], cur}
	}
	winner := ""
	if len(order) > 0 {
		winner = order[0]
	}
	if _, err := tx.Exec(ctx, "UPDATE tournaments SET status = 'finished', winner_id = NULLIF($2,''), finished_at = NOW() WHERE id = $1", tournamentID, winner); err != nil {
		return err
	}
	go func() {
		for i, uid := range order {
			title, msg := "Tournament finished", fmt.Sprintf("%s has finished. You placed #%d.", name, i+1)
			if a := awards[i]; a.amount > 0 {
				title = fmt.Sprintf("You placed #%d! 🏆", i+1)
				msg = fmt.Sprintf("%s prize from %s has been added to your wallet.", FormatMoney(a.amount, a.currency), name)
			}
			Notify(uid, "tournament_finished", title, msg, "/tournaments/"+tournamentID, map[string]any{"tournamentId": tournamentID})
		}
	}()
	return nil
}
