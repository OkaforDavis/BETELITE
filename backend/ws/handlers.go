package ws

import (
	"encoding/json"
	"log"
	"time"
)

type WSMessage struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

func HandleMessage(hub *Hub, client *Client, msg WSMessage) {
	switch msg.Event {
	case "identify":
		var data struct {
			UID string `json:"uid"`
		}
		if err := json.Unmarshal(msg.Data, &data); err == nil && data.UID != "" {
			client.UID = data.UID
			hub.JoinRoom(client, "user:"+data.UID)
			hub.JoinRoom(client, "global")
			log.Printf("Client identified as %s", data.UID)
		}

	case "join_match":
		var data struct {
			MatchID string `json:"matchId"`
		}
		if err := json.Unmarshal(msg.Data, &data); err == nil && data.MatchID != "" {
			hub.JoinRoom(client, "match:"+data.MatchID)
			log.Printf("Client joined match room: %s", data.MatchID)
		}

	case "leave_match":
		var data struct {
			MatchID string `json:"matchId"`
		}
		if err := json.Unmarshal(msg.Data, &data); err == nil && data.MatchID != "" {
			hub.LeaveRoom(client, "match:"+data.MatchID)
		}

	case "chat_message", "reaction":
		// Rate Limit: 1 message per 0.5 seconds
		now := time.Now()
		if now.Sub(client.LastChat) < 500*time.Millisecond {
			// Drop message if sending too fast
			return
		}
		client.LastChat = now
		
		var data struct {
			MatchID string `json:"matchId"`
			Message string `json:"message"`
			Sender  string `json:"sender"`
			Emoji   string `json:"emoji"`
		}
		if err := json.Unmarshal(msg.Data, &data); err == nil && data.MatchID != "" {
			// Broadcast to the specified room. If matchId is 'global', broadcast to 'global' room.
			targetRoom := "match:" + data.MatchID
			if data.MatchID == "global" {
				targetRoom = "global"
			}
			
			out, _ := json.Marshal(WSMessage{
				Event: msg.Event,
				Data:  msg.Data,
			})
			hub.BroadcastToRoom(targetRoom, out)
		}

	case "stream_start", "stream_end", "stream_join":
		// Broadcast stream state to match room and all streams observers
		var data struct {
			MatchID string `json:"matchId"`
			RoomID  string `json:"roomId"` // frontend sometimes sends roomId instead
		}
		if err := json.Unmarshal(msg.Data, &data); err == nil {
			out, _ := json.Marshal(msg)
			if data.MatchID != "" {
				hub.BroadcastToRoom("match:"+data.MatchID, out)
			} else if data.RoomID != "" {
				// roomId is formatted as "stream:match_id", but we just broadcast to all for simplicity
				hub.BroadcastAll(out)
			}
		}

	case "ai_score_detected":
		// Log AI score detection, in a full app we'd trigger the game engine to confirm
		log.Printf("AI Score detected: %s", string(msg.Data))

	default:
		log.Printf("Unhandled websocket event: %s", msg.Event)
	}
}

// Helper to broadcast JSON events easily from HTTP handlers or engine
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
