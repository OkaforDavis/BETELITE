package services

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"betelite-go/db"
	"betelite-go/utils"
	"betelite-go/ws"
)

// Match chat: the two players of a match can message each other and send an
// "I'm ready" alert (a ping). Admins can read the chat when checking a dispute.
// Messages are kept as evidence and deleted 30 days after the match is final.
const (
	ChatMaxLen        = 300
	chatPerMinute     = 15               // messages per player per match per minute
	PingCooldown      = 60 * time.Second // between two alerts from the same player
	PingMaxPerMatch   = 10
	chatAfterFinal    = time.Hour       // "GG" time after the match is settled
	chatPushThrottle  = 2 * time.Minute // phone pushes for chat, per match
	chatHistoryLength = 200
)

var linkRe = regexp.MustCompile(`(?i)(https?://|www\.)`)

type ChatMessage struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Kind      string    `json:"kind"` // text | ping
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

type ChatView struct {
	Messages  []ChatMessage `json:"messages"`
	CanSend   bool          `json:"canSend"`
	CanPing   bool          `json:"canPing"`
	OppOnline bool          `json:"oppOnline"`
	PingWait  int           `json:"pingWait"` // seconds until the next alert is allowed
}

type chatMatch struct {
	home, away, status string
	settledAt          *time.Time
}

func loadChatMatch(ctx context.Context, q querier, matchID string) (*chatMatch, error) {
	var m chatMatch
	err := q.QueryRow(ctx, "SELECT home_id, COALESCE(away_id,''), status, settled_at FROM matches WHERE id = $1", matchID).
		Scan(&m.home, &m.away, &m.status, &m.settledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, userErr(404, "Match not found")
	}
	return &m, err
}

func (m *chatMatch) opponent(uid string) string {
	if uid == m.home {
		return m.away
	}
	return m.home
}

func (m *chatMatch) player(uid string) bool { return uid != "" && (uid == m.home || uid == m.away) }

// open: messages can be sent while the match is being played or checked, and
// for a short while after it is final.
func (m *chatMatch) open() bool {
	if m.away == "" {
		return false
	}
	switch m.status {
	case StatusReady, StatusSubmitted, "disputed", "review":
		return true
	}
	return m.settledAt != nil && time.Since(*m.settledAt) < chatAfterFinal
}

// GetChat returns the chat of a match to one of its players (or an admin).
func GetChat(ctx context.Context, matchID, uid string, admin bool) (*ChatView, error) {
	m, err := loadChatMatch(ctx, db.Pool, matchID)
	if err != nil {
		return nil, err
	}
	player := m.player(uid)
	if !player && !admin {
		return nil, userErr(403, "Only the two players can see this chat")
	}
	rows, err := db.Pool.Query(ctx, `SELECT id, user_id, kind, body, created_at FROM (
		SELECT * FROM match_messages WHERE match_id = $1 ORDER BY created_at DESC LIMIT $2) t
		ORDER BY created_at`, matchID, chatHistoryLength)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	v := &ChatView{Messages: []ChatMessage{}}
	for rows.Next() {
		var c ChatMessage
		if err := rows.Scan(&c.ID, &c.UserID, &c.Kind, &c.Body, &c.CreatedAt); err != nil {
			return nil, err
		}
		v.Messages = append(v.Messages, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if player {
		v.CanSend = m.open()
		v.CanPing = m.status == StatusReady && m.away != ""
		if Hub != nil {
			v.OppOnline = Hub.IsOnline(m.opponent(uid))
		}
		if v.CanPing {
			if wait, _, err := pingWait(ctx, db.Pool, matchID, uid); err == nil {
				v.PingWait = int(wait.Seconds())
			}
		}
	}
	return v, nil
}

// SendChat posts a text message from one player to the other.
func SendChat(ctx context.Context, matchID, uid, body string) (*ChatMessage, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, userErr(400, "Type a message first")
	}
	if utf8.RuneCountInString(body) > ChatMaxLen {
		return nil, userErr(400, "Messages can be up to %d characters", ChatMaxLen)
	}
	if linkRe.MatchString(body) {
		return nil, userErr(400, "Links can't be sent in chat. Put invite links in the \"Start the match\" card.")
	}
	m, err := loadChatMatch(ctx, db.Pool, matchID)
	if err != nil {
		return nil, err
	}
	if !m.player(uid) {
		return nil, userErr(403, "Only the two players can chat in this match")
	}
	if !m.open() {
		return nil, userErr(409, "This chat is closed because the match has finished")
	}
	var recent int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM match_messages
		WHERE match_id = $1 AND user_id = $2 AND created_at > NOW() - INTERVAL '1 minute'`, matchID, uid).Scan(&recent); err != nil {
		return nil, err
	}
	if recent >= chatPerMinute {
		return nil, userErr(429, "You're sending messages too fast. Wait a moment.")
	}
	msg, err := saveChat(ctx, db.Pool, matchID, uid, "text", body)
	if err != nil {
		return nil, err
	}
	opp := m.opponent(uid)
	pushChat(matchID, msg, m.home, m.away)
	// Phone push only when the opponent doesn't have the app open (no spam).
	if Hub == nil || !Hub.IsOnline(opp) {
		if chatPushAllowed(matchID + ":" + opp) {
			preview := body
			if utf8.RuneCountInString(preview) > 80 {
				preview = string([]rune(preview)[:80]) + "…"
			}
			go sendPush(opp, map[string]any{
				"title": Username(context.Background(), uid) + " (match chat)", "body": preview,
				"url": "/match/" + matchID + "?chat=1", "tag": "match_chat_" + matchID, "kind": "match_chat",
			})
		}
	}
	return msg, nil
}

// PingOpponent sends the "I'm ready to play" alert: a loud notification with
// the match sound, plus a line in the chat.
func PingOpponent(ctx context.Context, matchID, uid string) (*ChatMessage, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	// One alert at a time per player (two quick taps can't both pass the cooldown).
	if _, err := tx.Exec(ctx, "SELECT 1 FROM users WHERE id = $1 FOR UPDATE", uid); err != nil {
		return nil, err
	}
	m, err := loadChatMatch(ctx, tx, matchID)
	if err != nil {
		return nil, err
	}
	if !m.player(uid) {
		return nil, userErr(403, "Only the two players can send alerts in this match")
	}
	if m.status != StatusReady || m.away == "" {
		return nil, userErr(409, "Alerts are only for matches that haven't been played yet")
	}
	wait, count, err := pingWait(ctx, tx, matchID, uid)
	if err != nil {
		return nil, err
	}
	if count >= PingMaxPerMatch {
		return nil, userErr(429, "You've sent the most alerts for this match. Send a chat message instead.")
	}
	if wait > 0 {
		return nil, userErr(429, "You can alert again in %d seconds", int(wait.Seconds())+1)
	}
	msg, err := saveChat(ctx, tx, matchID, uid, "ping", "")
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	opp := m.opponent(uid)
	name := Username(ctx, uid)
	pushChat(matchID, msg, m.home, m.away)
	Notify(opp, "match_ping", name+" is ready to play!",
		name+" is waiting for you in your match. Open CrestArena and start now.",
		"/match/"+matchID, map[string]any{"matchId": matchID, "from": uid})
	return msg, nil
}

// pingWait returns how long until this player may alert again, and how many
// alerts they have sent in this match.
func pingWait(ctx context.Context, q querier, matchID, uid string) (time.Duration, int, error) {
	var count int
	var last *time.Time
	err := q.QueryRow(ctx, `SELECT COUNT(*), MAX(created_at) FROM match_messages
		WHERE match_id = $1 AND user_id = $2 AND kind = 'ping'`, matchID, uid).Scan(&count, &last)
	if err != nil || last == nil {
		return 0, count, err
	}
	wait := PingCooldown - time.Since(*last)
	if wait < 0 {
		wait = 0
	}
	return wait, count, nil
}

func saveChat(ctx context.Context, q querier, matchID, uid, kind, body string) (*ChatMessage, error) {
	msg := &ChatMessage{ID: "msg_" + utils.GenerateBaseID(), UserID: uid, Kind: kind, Body: body}
	err := q.QueryRow(ctx, `INSERT INTO match_messages (id, match_id, user_id, kind, body)
		VALUES ($1,$2,$3,$4,$5) RETURNING created_at`, msg.ID, matchID, uid, kind, body).Scan(&msg.CreatedAt)
	return msg, err
}

// pushChat sends the new message to both players' open apps.
func pushChat(matchID string, msg *ChatMessage, players ...string) {
	if Hub == nil {
		return
	}
	for _, p := range players {
		if p != "" {
			ws.SendEvent(Hub, p, "match_chat", map[string]any{"matchId": matchID, "message": msg})
		}
	}
}

var (
	chatPushMu   sync.Mutex
	chatPushLast = map[string]time.Time{}
)

func chatPushAllowed(key string) bool {
	chatPushMu.Lock()
	defer chatPushMu.Unlock()
	now := time.Now()
	if t, ok := chatPushLast[key]; ok && now.Sub(t) < chatPushThrottle {
		return false
	}
	chatPushLast[key] = now
	if len(chatPushLast) > 5000 { // keep the map small
		for k, t := range chatPushLast {
			if now.Sub(t) > chatPushThrottle {
				delete(chatPushLast, k)
			}
		}
	}
	return true
}

// purgeOldChat deletes chat 30 days after a match became final (same rule as
// result screenshots).
func purgeOldChat(ctx context.Context) {
	db.Pool.Exec(ctx, `DELETE FROM match_messages c USING matches m
		WHERE c.match_id = m.id AND m.status IN ('confirmed','void')
		  AND COALESCE(m.settled_at, c.created_at) < NOW() - make_interval(secs => $1)`, EvidenceRetention.Seconds())
}
