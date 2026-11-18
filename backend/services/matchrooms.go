package services

import (
	"context"
	"crypto/rand"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"betelite-go/db"
	"betelite-go/ws"
)

// RoomView is the in-game room for a match, shown only to its two players.
type RoomView struct {
	MatchID   string     `json:"matchId"`
	Mode      string     `json:"mode"`
	Code      string     `json:"code,omitempty"`
	Password  string     `json:"password,omitempty"`
	Link      string     `json:"link,omitempty"`
	HostID    string     `json:"hostId,omitempty"` // who hosts ("" = not decided yet)
	CanPost   bool       `json:"canPost"`          // this viewer may post / change the room
	PostedAt  *time.Time `json:"postedAt,omitempty"`
	MyGameID  string     `json:"myGameId,omitempty"`
	OppGameID string     `json:"oppGameId,omitempty"`
}

type roomRow struct {
	game, home, away, status string
	code, pass, link, by     string
	at                       *time.Time
	homeGameID, awayGameID   string
}

func loadRoom(ctx context.Context, matchID string) (*roomRow, error) {
	var r roomRow
	err := db.Pool.QueryRow(ctx, `SELECT m.game, m.home_id, COALESCE(m.away_id,''), m.status,
		COALESCE(m.room_code,''), COALESCE(m.room_pass,''), COALESCE(m.room_link,''), COALESCE(m.room_by,''), m.room_at,
		COALESCE(hg.ingame_id,''), COALESCE(ag.ingame_id,'')
		FROM matches m
		LEFT JOIN game_profiles hg ON hg.user_id = m.home_id AND hg.game = m.game
		LEFT JOIN game_profiles ag ON ag.user_id = m.away_id AND ag.game = m.game
		WHERE m.id = $1`, matchID).Scan(&r.game, &r.home, &r.away, &r.status,
		&r.code, &r.pass, &r.link, &r.by, &r.at, &r.homeGameID, &r.awayGameID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, userErr(404, "Match not found")
	}
	return &r, err
}

// GetRoom returns the room for one of the match's players (or an admin).
// For games where CrestArena makes the code (DLS), it is created on first view.
func GetRoom(ctx context.Context, matchID, uid string, admin bool) (*RoomView, error) {
	r, err := loadRoom(ctx, matchID)
	if err != nil {
		return nil, err
	}
	player := uid == r.home || uid == r.away
	if !player && !admin {
		return nil, userErr(403, "Only the two players can see the room")
	}
	g := GameByID(r.game)
	if g == nil {
		return nil, userErr(400, "This game isn't supported any more")
	}
	if g.Room.Mode == "shared" && r.code == "" && r.status == StatusReady {
		code := "CA" + randomCode(6)
		// Only the first view writes it, so both players always get the same code.
		db.Pool.Exec(ctx, "UPDATE matches SET room_code = $2, room_at = NOW() WHERE id = $1 AND room_code IS NULL", matchID, code)
		if r, err = loadRoom(ctx, matchID); err != nil {
			return nil, err
		}
	}
	v := &RoomView{MatchID: matchID, Mode: g.Room.Mode, Code: r.code, Password: r.pass, Link: r.link, PostedAt: r.at}
	if g.Room.Mode == "host" {
		v.HostID = r.by
		if v.HostID == "" && g.Room.Host == "home" {
			v.HostID = r.home
		}
		v.CanPost = player && r.status == StatusReady && r.away != "" &&
			(v.HostID == uid || (v.HostID == "" && g.Room.Host == "either"))
	}
	if uid == r.home {
		v.MyGameID, v.OppGameID = r.homeGameID, r.awayGameID
	} else if uid == r.away {
		v.MyGameID, v.OppGameID = r.awayGameID, r.homeGameID
	}
	return v, nil
}

// RoomInput is what the host posts: the room number/code, plus a password or
// the game's join link where the game uses them. An empty code clears the room
// (and, for "either" games, lets the other player host instead).
type RoomInput struct {
	Code     string `json:"code"`
	Password string `json:"password"`
	Link     string `json:"link"`
}

// SetRoom saves the room the host created and tells the other player.
func SetRoom(ctx context.Context, matchID, uid string, in RoomInput) (*RoomView, error) {
	r, err := loadRoom(ctx, matchID)
	if err != nil {
		return nil, err
	}
	if uid != r.home && uid != r.away {
		return nil, userErr(403, "Only the two players can post the room")
	}
	g := GameByID(r.game)
	if g == nil || g.Room.Mode != "host" {
		return nil, userErr(400, "This game doesn't need a room code. Use the code shown.")
	}
	if r.status != StatusReady || r.away == "" {
		return nil, userErr(409, "This match isn't waiting to be played")
	}
	if g.Room.Host == "home" && uid != r.home {
		return nil, userErr(403, "Your opponent creates the room for this game. Wait for them to post it.")
	}
	if r.by != "" && r.by != uid {
		return nil, userErr(409, "Your opponent is already hosting. Join their room.")
	}

	code := strings.TrimSpace(in.Code)
	if code == "" {
		if _, err := db.Pool.Exec(ctx, `UPDATE matches SET room_code = NULL, room_pass = NULL, room_link = NULL,
			room_by = NULL, room_at = NULL WHERE id = $1 AND room_by = $2`, matchID, uid); err != nil {
			return nil, err
		}
		pushRoom(matchID, r.home, r.away)
		return GetRoom(ctx, matchID, uid, false)
	}
	if !validRoomText(code, 3, 24) {
		return nil, userErr(400, "%s should be 3–24 letters or numbers, exactly as the game shows it", g.Room.CodeLabel)
	}
	pass := ""
	if g.Room.Password {
		pass = strings.TrimSpace(in.Password)
		if pass != "" && !validRoomText(pass, 1, 24) {
			return nil, userErr(400, "The password should be up to 24 letters or numbers")
		}
	}
	link := ""
	if g.Room.Link && strings.TrimSpace(in.Link) != "" {
		if link = cleanJoinLink(in.Link); link == "" {
			return nil, userErr(400, "That link doesn't look right. Paste the invite link from the game, starting with https://")
		}
	}

	// The WHERE clause makes "first to post hosts" safe when both post at once.
	tag, err := db.Pool.Exec(ctx, `UPDATE matches SET room_code = $3, room_pass = NULLIF($4,''), room_link = NULLIF($5,''),
		room_by = $2, room_at = NOW()
		WHERE id = $1 AND status = 'ready' AND (room_by IS NULL OR room_by = $2)`, matchID, uid, code, pass, link)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, userErr(409, "Your opponent is already hosting. Join their room.")
	}

	opp := r.away
	if uid == r.away {
		opp = r.home
	}
	if r.code == "" { // first post only, so editing a typo doesn't spam
		var name string
		db.Pool.QueryRow(ctx, "SELECT COALESCE(username,'Your opponent') FROM users WHERE id = $1", uid).Scan(&name)
		Notify(opp, "match_room", "Room ready", name+" created the "+g.Short+" room. Open the match to join.", "/match/"+matchID, map[string]any{"matchId": matchID})
	}
	pushRoom(matchID, r.home, r.away)
	return GetRoom(ctx, matchID, uid, false)
}

// pushRoom tells both players' open match pages to reload the room. Only the
// match ID is sent: the code and password are fetched by the players.
func pushRoom(matchID string, players ...string) {
	if Hub == nil {
		return
	}
	for _, p := range players {
		if p != "" {
			ws.SendEvent(Hub, p, "match_room", map[string]string{"matchId": matchID})
		}
	}
}

func validRoomText(s string, min, max int) bool {
	if n := len([]rune(s)); n < min || n > max {
		return false
	}
	for _, c := range s {
		ok := c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '-' || c == '_' || c == ' ' || c == '#'
		if !ok {
			return false
		}
	}
	return true
}

// cleanJoinLink accepts only plain https links (no scripts, no logins in the URL).
func cleanJoinLink(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) > 300 {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || !strings.Contains(u.Host, ".") {
		return ""
	}
	return u.String()
}

// randomCode makes an easy-to-type code without 0/O or 1/I mix-ups.
func randomCode(n int) string {
	const alphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	b := make([]byte, n)
	rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}
