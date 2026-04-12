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

type sendToActorRequest struct {
	roomID  string
	actorID string
	data    []byte
}

type broadcastCustomRequest struct {
	roomID string
	build  func(*Client) []byte
}

type Hub struct {
	rooms           map[string]map[*Client]bool
	mu              sync.RWMutex
	broadcast       chan Message
	register        chan *Client
	unregister      chan *Client
	leave           chan leaveRequest
	disconnectActor chan disconnectActorRequest
	sendToActor     chan sendToActorRequest
	broadcastCustom chan broadcastCustomRequest
}

func NewHub() *Hub {
	return &Hub{
		broadcast:       make(chan Message),
		register:        make(chan *Client),
		unregister:      make(chan *Client),
		leave:           make(chan leaveRequest),
		disconnectActor: make(chan disconnectActorRequest),
		sendToActor:     make(chan sendToActorRequest),
		broadcastCustom: make(chan broadcastCustomRequest),
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

		case req := <-h.sendToActor:
			if req.roomID == "" || req.actorID == "" || len(req.data) == 0 {
				continue
			}

			if clients, ok := h.rooms[req.roomID]; ok {
				for client := range clients {
					if client.ActorID() != req.actorID {
						continue
					}
					client.SendMessage(req.data)
				}
			}

		case req := <-h.broadcastCustom:
			if req.roomID == "" || req.build == nil {
				continue
			}

			if clients, ok := h.rooms[req.roomID]; ok {
				for client := range clients {
					data := req.build(client)
					if len(data) == 0 {
						continue
					}

					select {
					case client.send <- data:
					default:
						delete(clients, client)
						close(client.send)

						if len(clients) == 0 {
							delete(h.rooms, req.roomID)
						}
					}
				}
			}

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

func (h *Hub) SendToActor(roomID, actorID string, data []byte) {
	h.sendToActor <- sendToActorRequest{
		roomID:  roomID,
		actorID: actorID,
		data:    data,
	}
}

func (h *Hub) BroadcastCustom(roomID string, build func(*Client) []byte) {
	h.broadcastCustom <- broadcastCustomRequest{
		roomID: roomID,
		build:  build,
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
