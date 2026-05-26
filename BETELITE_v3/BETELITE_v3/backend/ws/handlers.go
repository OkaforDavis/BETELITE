package ws

import (
	"encoding/json"
	"log"
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

	case "chat_message":
		var data struct {
			MatchID string `json:"matchId"`
			Message string `json:"message"`
			Sender  string `json:"sender"`
		}
		if err := json.Unmarshal(msg.Data, &data); err == nil && data.MatchID != "" {
			// Re-serialize the message and broadcast to the match room
			out, _ := json.Marshal(WSMessage{
				Event: "chat_message",
				Data:  msg.Data,
			})
			hub.BroadcastToRoom("match:"+data.MatchID, out)
		}

	case "start_stream", "stop_stream", "stream_signal":
		// These events were used for WebRTC, if needed we can broadcast to the match room
		var data struct {
			MatchID string `json:"matchId"`
		}
		if err := json.Unmarshal(msg.Data, &data); err == nil && data.MatchID != "" {
			out, _ := json.Marshal(msg)
			hub.BroadcastToRoom("match:"+data.MatchID, out)
		}

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
