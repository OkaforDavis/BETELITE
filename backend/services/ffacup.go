package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"betelite-go/db"
)

// Free-for-All cups ("ffa"): every player is in ONE game room (CODM private
// room, up to 8). There are no 1v1 matches. The cup's host (the player who
// hosts it, or an admin for CrestArena's own cups) posts the Room ID and
// password; posting the room starts the cup and alerts every player. After
// the game, the host uploads the final scoreboard and enters each player's
// score; the winner is paid like any other cup.

const FFAMaxPlayers = 8

// CupRoom is the game room of an FFA cup, shown to its players and host only.
type CupRoom struct {
	Code     string     `json:"code"`
	Password string     `json:"password,omitempty"`
	Note     string     `json:"note,omitempty"`
	PostedAt *time.Time `json:"postedAt,omitempty"`
}

type ffaCup struct {
	name, game, format, status, host, createdBy string
	settings                                    *MatchSettings
}

func loadFFACup(ctx context.Context, tx pgx.Tx, tid string) (*ffaCup, error) {
	var c ffaCup
	var settings []byte
	err := tx.QueryRow(ctx, `SELECT name, game, format, status, COALESCE(host_id,''), COALESCE(created_by,''), settings
		FROM tournaments WHERE id = $1 FOR UPDATE`, tid).Scan(&c.name, &c.game, &c.format, &c.status, &c.host, &c.createdBy, &settings)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, userErr(404, "Tournament not found")
	}
	if err != nil {
		return nil, err
	}
	if c.format != "ffa" {
		return nil, userErr(400, "This isn't a Free-for-All cup")
	}
	c.settings = ParseSettings(settings)
	return &c, nil
}

// CanManageCup: the hosting player of a host cup, or an admin for CrestArena's own cups
// (admins can step in on any cup).
func CanManageCup(hostID, uid string, admin bool) bool {
	return admin || (hostID != "" && hostID == uid)
}

func cupPlayers(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, tid string) ([]string, error) {
	rows, err := q.Query(ctx, "SELECT user_id FROM tournament_players WHERE tournament_id = $1", tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// PostCupRoom saves the room and, the first time, starts the cup (sign-up
// closes) and alerts every player with the loud match alert.
func PostCupRoom(ctx context.Context, tid, uid string, admin bool, code, pass, note string) (*CupRoom, error) {
	code = strings.TrimSpace(code)
	pass = strings.TrimSpace(pass)
	note = cleanReason(note)
	if !validRoomText(code, 3, 24) {
		return nil, userErr(400, "Type the Room ID exactly as the game shows it (letters and numbers)")
	}
	if pass != "" && !validRoomText(pass, 1, 24) {
		return nil, userErr(400, "Passwords can use letters and numbers only")
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	c, err := loadFFACup(ctx, tx, tid)
	if err != nil {
		return nil, err
	}
	if !CanManageCup(c.host, uid, admin) {
		return nil, userErr(403, "Only the cup's host can post the room")
	}
	if c.status != "open" && c.status != "active" {
		return nil, userErr(409, "This cup has finished")
	}
	players, err := cupPlayers(ctx, tx, tid)
	if err != nil {
		return nil, err
	}
	var oldCode string
	tx.QueryRow(ctx, "SELECT COALESCE(room_code,'') FROM tournaments WHERE id = $1", tid).Scan(&oldCode)
	starting := oldCode == "" // first post: the game is about to start
	if c.status == "open" && len(players) < 2 {
		return nil, userErr(409, "At least 2 players must join before you post the room")
	}
	if _, err := tx.Exec(ctx, `UPDATE tournaments SET room_code = $2, room_pass = NULLIF($3,''), room_note = NULLIF($4,''), room_at = NOW(),
		status = 'active', current_round = 1, started_at = COALESCE(started_at, NOW()) WHERE id = $1`, tid, code, pass, note); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	title, msg := c.name+": the room is open!", "Open the cup, copy the Room ID and password, and join in the game now."
	if !starting {
		title, msg = c.name+": room changed", "The host posted a new Room ID. Open the cup to see it."
	}
	for _, p := range players {
		// "match_ping" plays the loud match sound and stays on screen.
		Notify(p, "match_ping", title, msg, "/tournaments/"+tid, map[string]any{"tournamentId": tid})
	}
	now := time.Now()
	return &CupRoom{Code: code, Password: pass, Note: note, PostedAt: &now}, nil
}

// GetCupRoom returns the room to the cup's players and its host (nil if not posted).
func GetCupRoom(ctx context.Context, tid, uid string, admin bool) (*CupRoom, error) {
	var host, code, pass, note string
	var at *time.Time
	var joined bool
	err := db.Pool.QueryRow(ctx, `SELECT COALESCE(host_id,''), COALESCE(room_code,''), COALESCE(room_pass,''), COALESCE(room_note,''), room_at,
		EXISTS(SELECT 1 FROM tournament_players p WHERE p.tournament_id = t.id AND p.user_id = $2)
		FROM tournaments t WHERE id = $1`, tid, uid).Scan(&host, &code, &pass, &note, &at, &joined)
	if err != nil || code == "" {
		return nil, err
	}
	if !joined && !CanManageCup(host, uid, admin) {
		return nil, nil
	}
	return &CupRoom{Code: code, Password: pass, Note: note, PostedAt: at}, nil
}

// CupResult is what the host submits after the game: each player's score
// (kills) and the winner.
type CupResult struct {
	Scores map[string]int `json:"scores"`
	Winner string         `json:"winner"`
}

// SubmitCupResult records the scoreboard, ranks the players (winner first,
// then by score) and pays the prizes.
func SubmitCupResult(ctx context.Context, tid, uid string, admin bool, res CupResult, image []byte) error {
	if len(image) == 0 {
		return userErr(400, "Upload the final scoreboard screenshot")
	}
	img, err := evidenceJPEG(image)
	if err != nil {
		return err
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	c, err := loadFFACup(ctx, tx, tid)
	if err != nil {
		return err
	}
	if !CanManageCup(c.host, uid, admin) {
		return userErr(403, "Only the cup's host can submit the result")
	}
	if c.status != "active" {
		return userErr(409, "Post the room and play the game first")
	}
	players, err := cupPlayers(ctx, tx, tid)
	if err != nil {
		return err
	}
	isPlayer := map[string]bool{}
	for _, p := range players {
		isPlayer[p] = true
	}
	if !isPlayer[res.Winner] {
		return userErr(400, "Pick the winner from the cup's players")
	}
	limit := 0
	if c.settings != nil {
		limit = c.settings.ScoreLimit
	}
	for p, s := range res.Scores {
		if !isPlayer[p] {
			return userErr(400, "A score was sent for someone who isn't in this cup")
		}
		maxScore := 999
		if limit > 0 {
			maxScore = limit
		}
		if s < 0 || s > maxScore {
			return userErr(400, "Scores must be between 0 and %d", maxScore)
		}
	}
	top := 0
	for _, s := range res.Scores {
		top = max(top, s)
	}
	if res.Scores[res.Winner] < top {
		return userErr(400, "The winner must have the highest score. For a tie, play a tiebreak and pick its winner.")
	}
	order := append([]string(nil), players...)
	sort.SliceStable(order, func(i, j int) bool {
		a, b := order[i], order[j]
		if (a == res.Winner) != (b == res.Winner) {
			return a == res.Winner
		}
		return res.Scores[a] > res.Scores[b]
	})
	for _, p := range players {
		if _, err := tx.Exec(ctx, "UPDATE tournament_players SET score = $3 WHERE tournament_id = $1 AND user_id = $2", tid, p, res.Scores[p]); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO tournament_results (tournament_id, image, submitted_by) VALUES ($1,$2,$3)
		ON CONFLICT (tournament_id) DO UPDATE SET image = EXCLUDED.image, submitted_by = EXCLUDED.submitted_by, created_at = NOW()`, tid, img, uid); err != nil {
		return err
	}
	if err := finishTournament(ctx, tx, tid, order); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CupResultImage returns the final scoreboard to the cup's players, host and admins.
func CupResultImage(ctx context.Context, tid, uid string, admin bool) ([]byte, error) {
	var host string
	var joined bool
	var img []byte
	err := db.Pool.QueryRow(ctx, `SELECT COALESCE(t.host_id,''), EXISTS(SELECT 1 FROM tournament_players p WHERE p.tournament_id = t.id AND p.user_id = $2), r.image
		FROM tournaments t JOIN tournament_results r ON r.tournament_id = t.id WHERE t.id = $1`, tid, uid).Scan(&host, &joined, &img)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, userErr(404, "No result yet")
	}
	if err != nil {
		return nil, err
	}
	if !joined && !CanManageCup(host, uid, admin) {
		return nil, userErr(403, "Only the cup's players can see the scoreboard")
	}
	return img, nil
}

// remindFFACups alerts players 15 minutes before a Free-for-All cup starts,
// and the host when the start time comes and no room is posted yet. Each
// alert is sent once (the UPDATE claims it).
func remindFFACups(ctx context.Context) {
	for _, tid := range idsFrom(ctx, `UPDATE tournaments SET reminded_at = NOW()
		WHERE format = 'ffa' AND status = 'open' AND reminded_at IS NULL AND starts_at IS NOT NULL
		  AND starts_at BETWEEN NOW() AND NOW() + INTERVAL '15 minutes' RETURNING id`) {
		var name string
		db.Pool.QueryRow(ctx, "SELECT name FROM tournaments WHERE id = $1", tid).Scan(&name)
		players, _ := cupPlayers(ctx, db.Pool, tid)
		for _, p := range players {
			Notify(p, "tournament_start", name+" starts in 15 minutes", "Open the game and be ready. The host will post the Room ID here.", "/tournaments/"+tid, map[string]any{"tournamentId": tid})
		}
	}
	for _, tid := range idsFrom(ctx, `UPDATE tournaments SET host_alerted_at = NOW()
		WHERE format = 'ffa' AND status = 'open' AND host_alerted_at IS NULL AND room_code IS NULL
		  AND starts_at BETWEEN NOW() - INTERVAL '1 hour' AND NOW() RETURNING id`) {
		var name, who string
		db.Pool.QueryRow(ctx, "SELECT name, COALESCE(host_id, created_by, '') FROM tournaments WHERE id = $1", tid).Scan(&name, &who)
		players, _ := cupPlayers(ctx, db.Pool, tid)
		Notify(who, "match_ping", name+": post the room now",
			fmt.Sprintf("It's start time and %d players are waiting. Create the room in the game and post the Room ID.", len(players)),
			"/tournaments/"+tid, map[string]any{"tournamentId": tid})
	}
}
