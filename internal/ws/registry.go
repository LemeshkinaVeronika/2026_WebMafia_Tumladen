package ws

import (
	"sync"

	"github.com/centrifugal/centrifuge"
	"github.com/webmafia/tumladan/internal/model"
)

type ConnectionState struct {
	ProcessMu sync.Mutex
	Client    *centrifuge.Client
	Actor     model.Actor
	RoomID    string
}

type Registry struct {
	mu          sync.RWMutex
	connections map[string]*ConnectionState
	rooms       map[string]map[string]struct{}
}

func NewRegistry() *Registry {
	return &Registry{
		connections: make(map[string]*ConnectionState),
		rooms:       make(map[string]map[string]struct{}),
	}
}

func (r *Registry) Add(client *centrifuge.Client, actor model.Actor) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.connections[client.ID()] = &ConnectionState{
		Client: client,
		Actor:  actor,
	}
}

func (r *Registry) Get(clientID string) (*ConnectionState, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, ok := r.connections[clientID]
	return state, ok
}

func (r *Registry) SetRoom(state *ConnectionState, roomID string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if state == nil || state.Client == nil {
		return ""
	}
	clientID := state.Client.ID()

	current, ok := r.connections[clientID]
	if !ok || current != state {
		return ""
	}

	previousRoomID := state.RoomID
	if previousRoomID != "" {
		r.removeClientFromRoom(previousRoomID, clientID)
	}

	state.RoomID = roomID

	if roomID != "" {
		if r.rooms[roomID] == nil {
			r.rooms[roomID] = make(map[string]struct{})
		}
		r.rooms[roomID][clientID] = struct{}{}
	}

	return previousRoomID
}

func (r *Registry) Remove(clientID string) (*ConnectionState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.connections[clientID]
	if !ok {
		return nil, false
	}

	if state.RoomID != "" {
		r.removeClientFromRoom(state.RoomID, clientID)
	}

	delete(r.connections, clientID)
	return state, true
}

func (r *Registry) LocalClientsForActor(roomID, actorID string) []*centrifuge.Client {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clientIDs := r.rooms[roomID]
	clients := make([]*centrifuge.Client, 0, len(clientIDs))
	for clientID := range clientIDs {
		state, ok := r.connections[clientID]
		if !ok || state.Actor.ID != actorID || state.Client == nil {
			continue
		}
		clients = append(clients, state.Client)
	}

	return clients
}

func (r *Registry) HasLocalClientForActor(roomID, actorID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clientIDs := r.rooms[roomID]
	for clientID := range clientIDs {
		state, ok := r.connections[clientID]
		if ok && state.Actor.ID == actorID && state.Client != nil {
			return true
		}
	}

	return false
}

func (r *Registry) WithClientLock(clientID string, fn func(state *ConnectionState)) bool {
	r.mu.RLock()
	state, ok := r.connections[clientID]
	r.mu.RUnlock()
	if !ok {
		return false
	}

	state.ProcessMu.Lock()
	defer state.ProcessMu.Unlock()

	r.mu.RLock()
	current, ok := r.connections[clientID]
	r.mu.RUnlock()
	if !ok || current != state {
		return false
	}

	fn(state)
	return true
}

func (r *Registry) removeClientFromRoom(roomID, clientID string) {
	clients, ok := r.rooms[roomID]
	if !ok {
		return
	}

	delete(clients, clientID)
	if len(clients) == 0 {
		delete(r.rooms, roomID)
	}
}
