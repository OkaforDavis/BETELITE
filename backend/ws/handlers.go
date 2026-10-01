package ws

import (
	"encoding/json"
	"log"
	"strings"
	"time"
	"unicode/utf8"
)

type WSMessage struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

// VerifyToken resolves a Firebase ID token to a UID; set at startup so this
// package doesn't depend on the auth middleware.
var VerifyToken func(token string) (string, error)

// DisplayName resolves a UID to the name shown in chat; set at startup.
var DisplayName func(uid string) string

func HandleMessage(hub *Hub, client *Client, msg WSMessage) {
	switch msg.Event {
	case "identify":
		// Clients prove who they are with their Firebase ID token; the UID is
		// never taken from the client directly.
		var data struct {
			Token string `json:"token"`
		}
		if json.Unmarshal(msg.Data, &data) != nil || data.Token == "" || VerifyToken == nil {
			return
		}
		uid, err := VerifyToken(data.Token)
		if err != nil {
			client.sendEvent("auth_error", map[string]string{"error": "invalid token"})
			return
		}
		client.UID = uid
		hub.JoinRoom(client, "user:"+uid)
		hub.JoinRoom(client, "global")
		client.sendEvent("identified", map[string]string{"uid": uid})

	case "join_room", "join_match":
		var data struct {
			Room    string `json:"room"`
			MatchID string `json:"matchId"`
		}
		if json.Unmarshal(msg.Data, &data) == nil {
			room := data.Room
			if room == "" && data.MatchID != "" {
				room = data.MatchID
			}
			if room != "" && len(room) <= 80 {
				hub.JoinRoom(client, "room:"+room)
			}
		}

	case "leave_room", "leave_match":
		var data struct {
			Room    string `json:"room"`
			MatchID string `json:"matchId"`
		}
		if json.Unmarshal(msg.Data, &data) == nil {
			room := data.Room
			if room == "" {
				room = data.MatchID
			}
			hub.LeaveRoom(client, "room:"+room)
		}

	case "chat_message", "reaction":
		if client.UID == "" {
			return // must identify first
		}
		now := time.Now()
		if now.Sub(client.LastChat) < 700*time.Millisecond {
			return
		}
		client.LastChat = now

		var data struct {
			Room    string `json:"room"`
			MatchID string `json:"matchId"`
			Message string `json:"message"`
			Emoji   string `json:"emoji"`
		}
		if json.Unmarshal(msg.Data, &data) != nil {
			return
		}
		room := data.Room
		if room == "" {
			room = data.MatchID
		}
		target := "room:" + room
		if room == "" || room == "global" {
			target = "global"
		}
		text := strings.TrimSpace(data.Message)
		if utf8.RuneCountInString(text) > 300 {
			text = string([]rune(text)[:300])
		}
		if msg.Event == "chat_message" && text == "" {
			return
		}
		emoji := data.Emoji
		if utf8.RuneCountInString(emoji) > 4 {
			emoji = ""
		}
		name := "Player"
		if DisplayName != nil {
			name = DisplayName(client.UID)
		}
		out, _ := json.Marshal(map[string]any{
			"event": msg.Event,
			"data": map[string]any{
				"room": room, "message": text, "emoji": emoji,
				"senderId": client.UID, "sender": name, "at": now.UnixMilli(),
			},
		})
		hub.BroadcastToRoom(target, out)

	case "ping":
		client.sendEvent("pong", nil)

	default:
		log.Printf("Unhandled websocket event: %s", msg.Event)
	}
}

func (c *Client) sendEvent(event string, data any) {
	b, _ := json.Marshal(data)
	msg, _ := json.Marshal(WSMessage{Event: event, Data: b})
	select {
	case c.Send <- msg:
	default:
	}
}

// BroadcastEvent sends a JSON event to every connected client.
func BroadcastEvent(hub *Hub, event string, data interface{}) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	msg, err := json.Marshal(WSMessage{Event: event, Data: dataBytes})
	if err != nil {
		return
	}
	hub.BroadcastAll(msg)
}

// SendEvent sends a JSON event to one user's connected devices.
func SendEvent(hub *Hub, uid, event string, data interface{}) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	msg, err := json.Marshal(WSMessage{Event: event, Data: dataBytes})
	if err != nil {
		return
	}
	hub.SendToUser(uid, msg)
}
