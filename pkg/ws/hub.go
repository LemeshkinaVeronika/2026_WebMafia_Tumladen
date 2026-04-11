package ws

import (
	"context"
	"sync"
)

type Message struct {
	RoomID string
	Data   []byte
}

type leaveRequest struct {
	client *Client
	roomID string
}

type disconnectActorRequest struct {
	roomID  string
	actorID string
}

type Hub struct {
	rooms           map[string]map[*Client]bool
	mu              sync.RWMutex
	broadcast       chan Message
	register        chan *Client
	unregister      chan *Client
	leave           chan leaveRequest
	disconnectActor chan disconnectActorRequest
}

func NewHub() *Hub {
	return &Hub{
		broadcast:       make(chan Message),
		register:        make(chan *Client),
		unregister:      make(chan *Client),
		leave:           make(chan leaveRequest),
		disconnectActor: make(chan disconnectActorRequest),
		rooms:           make(map[string]map[*Client]bool),
	}
}

func (h *Hub) Run(ctx context.Context) {
	defer func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		for _, clients := range h.rooms {
			for client := range clients {
				close(client.send)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case client := <-h.register:
			if client.RoomID() == "" {
				continue
			}

			h.mu.Lock()
			if h.rooms[client.RoomID()] == nil {
				h.rooms[client.RoomID()] = make(map[*Client]bool)
			}
			h.rooms[client.RoomID()][client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			roomID := client.RoomID()
			if roomID == "" {
				continue
			}

			h.mu.Lock()
			if clients, ok := h.rooms[roomID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					close(client.send)

					if len(clients) == 0 {
						delete(h.rooms, roomID)
					}
				}
			}
			h.mu.Unlock()

		case req := <-h.leave:
			if req.roomID == "" {
				continue
			}

			h.mu.Lock()
			if clients, ok := h.rooms[req.roomID]; ok {
				delete(clients, req.client)
				if len(clients) == 0 {
					delete(h.rooms, req.roomID)
				}
			}
			h.mu.Unlock()

		case req := <-h.disconnectActor:
			if req.roomID == "" || req.actorID == "" {
				continue
			}

			h.mu.Lock()
			if clients, ok := h.rooms[req.roomID]; ok {
				for client := range clients {
					if client.ActorID() != req.actorID {
						continue
					}

					delete(clients, client)
					close(client.send)
				}

				if len(clients) == 0 {
					delete(h.rooms, req.roomID)
				}
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.Lock()
			if clients, ok := h.rooms[message.RoomID]; ok {
				for client := range clients {
					select {
					case client.send <- message.Data:
					default:
						delete(clients, client)
						close(client.send)

						if len(clients) == 0 {
							delete(h.rooms, message.RoomID)
						}
					}
				}
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) Leave(client *Client, roomID string) {
	h.leave <- leaveRequest{
		client: client,
		roomID: roomID,
	}
}

func (h *Hub) DisconnectActor(roomID, actorID string) {
	h.disconnectActor <- disconnectActorRequest{
		roomID:  roomID,
		actorID: actorID,
	}
}

func (h *Hub) BroadcastTo(roomID string, data []byte) {
	h.broadcast <- Message{
		RoomID: roomID,
		Data:   data,
	}
}

func (h *Hub) RoomClientCount(roomID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, ok := h.rooms[roomID]
	if !ok {
		return 0
	}

	return len(clients)
}

func (h *Hub) IsRoomEmpty(roomID string) bool {
	return h.RoomClientCount(roomID) == 0
}
