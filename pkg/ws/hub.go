package ws

import "context"

type Message struct {
	RoomID string
	Data   []byte
}

type Hub struct {
	rooms      map[string]map[*Client]bool
	broadcast  chan Message
	register   chan *Client
	unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		rooms:      make(map[string]map[*Client]bool),
	}
}

func (h *Hub) Run(ctx context.Context) {
	defer func() {
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

			if h.rooms[client.RoomID()] == nil {
				h.rooms[client.RoomID()] = make(map[*Client]bool)
			}
			h.rooms[client.RoomID()][client] = true

		case client := <-h.unregister:
			roomID := client.RoomID()
			if roomID == "" {
				continue
			}

			if clients, ok := h.rooms[roomID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					close(client.send)

					if len(clients) == 0 {
						delete(h.rooms, roomID)
					}
				}
			}

		case message := <-h.broadcast:
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
		}
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) BroadcastTo(roomID string, data []byte) {
	h.broadcast <- Message{
		RoomID: roomID,
		Data:   data,
	}
}
