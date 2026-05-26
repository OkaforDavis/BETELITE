package ws

import (
	"sync"
)

type Hub struct {
	// Registered clients.
	Clients map[*Client]bool

	// Rooms for multiplexing (e.g., "match:123", "user:uid")
	Rooms map[string]map[*Client]bool

	// Register requests from the clients.
	Register chan *Client

	// Unregister requests from clients.
	Unregister chan *Client

	// Room join/leave channels
	Join   chan *RoomAction
	Leave  chan *RoomAction

	mu sync.RWMutex
}

type RoomAction struct {
	Client *Client
	Room   string
}

func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[*Client]bool),
		Rooms:      make(map[string]map[*Client]bool),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Join:       make(chan *RoomAction),
		Leave:      make(chan *RoomAction),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client] = true
			h.mu.Unlock()
		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
				// Remove from all rooms
				for room, clients := range h.Rooms {
					if _, ok := clients[client]; ok {
						delete(clients, client)
						if len(clients) == 0 {
							delete(h.Rooms, room)
						}
					}
				}
			}
			h.mu.Unlock()
		case action := <-h.Join:
			h.mu.Lock()
			if h.Rooms[action.Room] == nil {
				h.Rooms[action.Room] = make(map[*Client]bool)
			}
			h.Rooms[action.Room][action.Client] = true
			h.mu.Unlock()
		case action := <-h.Leave:
			h.mu.Lock()
			if clients, ok := h.Rooms[action.Room]; ok {
				delete(clients, action.Client)
				if len(clients) == 0 {
					delete(h.Rooms, action.Room)
				}
			}
			h.mu.Unlock()
		}
	}
}

// JoinRoom adds a client to a room
func (h *Hub) JoinRoom(client *Client, room string) {
	h.Join <- &RoomAction{Client: client, Room: room}
}

// LeaveRoom removes a client from a room
func (h *Hub) LeaveRoom(client *Client, room string) {
	h.Leave <- &RoomAction{Client: client, Room: room}
}

// BroadcastToRoom sends a message to all clients in a specific room
func (h *Hub) BroadcastToRoom(room string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if clients, ok := h.Rooms[room]; ok {
		for client := range clients {
			select {
			case client.Send <- data:
			default:
				// If send buffer is full, remove client
				close(client.Send)
				delete(clients, client)
				delete(h.Clients, client)
			}
		}
	}
}

// BroadcastAll sends a message to all connected clients
func (h *Hub) BroadcastAll(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.Clients {
		select {
		case client.Send <- data:
		default:
			close(client.Send)
			delete(h.Clients, client)
		}
	}
}

// SendToUser sends a message to a specific user by UID (if they joined user:uid room)
func (h *Hub) SendToUser(uid string, data []byte) {
	h.BroadcastToRoom("user:"+uid, data)
}

func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.Clients)
}
