package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/centrifugal/centrifuge"
	"github.com/go-chi/chi/v5"
	matchDTO "github.com/webmafia/tumladan/internal/match/dto"
	matchService "github.com/webmafia/tumladan/internal/match/service"
	"github.com/webmafia/tumladan/internal/middleware"
	"github.com/webmafia/tumladan/internal/model"
	roomDTO "github.com/webmafia/tumladan/internal/room/dto"
	roomService "github.com/webmafia/tumladan/internal/room/service"
	wsticket "github.com/webmafia/tumladan/internal/ws_ticket/service"
	"github.com/webmafia/tumladan/pkg/logger"
)

const (
	roomHistorySize = 32
	roomHistoryTTL  = 3 * time.Minute
)

var disconnectKicked = centrifuge.Disconnect{
	Code:   4500,
	Reason: "kicked",
}

type Handler struct {
	roomService       *roomService.Service
	matchService      *matchService.Service
	ticketService     *wsticket.Service
	node              *centrifuge.Node
	registry          *Registry
	logger            logger.Logger
	websocketHandler  http.Handler
	httpStreamHandler http.Handler
	sseHandler        http.Handler
	emulationHandler  http.Handler
}

type MessageHandler struct {
	roomService  *roomService.Service
	matchService *matchService.Service
	node         *centrifuge.Node
	registry     *Registry
	logger       logger.Logger
}

type PublicMatchState struct {
	ID                  string                         `json:"id"`
	RoomID              string                         `json:"roomId"`
	GameType            string                         `json:"gameType"`
	Status              string                         `json:"status"`
	GameState           json.RawMessage                `json:"gameState"`
	Result              json.RawMessage                `json:"result,omitempty"`
	TerminationReason   *string                        `json:"terminationReason,omitempty"`
	TerminatedByActorID *string                        `json:"terminatedByActorId,omitempty"`
	TerminatedAt        *string                        `json:"terminatedAt,omitempty"`
	Players             []matchDTO.MatchPlayerResponse `json:"players"`
	CreatedAt           string                         `json:"createdAt"`
	UpdatedAt           string                         `json:"updatedAt"`
}

func NewHandler(
	roomService *roomService.Service,
	matchService *matchService.Service,
	ticketService *wsticket.Service,
	logger logger.Logger,
) (*Handler, error) {
	node, err := centrifuge.New(centrifuge.Config{})
	if err != nil {
		return nil, err
	}

	h := &Handler{
		roomService:   roomService,
		matchService:  matchService,
		ticketService: ticketService,
		node:          node,
		registry:      NewRegistry(),
		logger:        logger,
	}

	h.node.OnConnecting(h.onConnecting)
	h.node.OnConnect(h.onConnect)

	h.websocketHandler = h.wrapTransportHandler(centrifuge.NewWebsocketHandler(node, centrifuge.WebsocketConfig{}))
	h.httpStreamHandler = h.wrapTransportHandler(centrifuge.NewHTTPStreamHandler(node, centrifuge.HTTPStreamConfig{}))
	h.sseHandler = h.wrapTransportHandler(centrifuge.NewSSEHandler(node, centrifuge.SSEConfig{}))
	h.emulationHandler = h.wrapTransportHandler(centrifuge.NewEmulationHandler(node, centrifuge.EmulationConfig{}))

	return h, nil
}

func (h *Handler) Run() error {
	return h.node.Run()
}

func (h *Handler) Shutdown(ctx context.Context) error {
	return h.node.Shutdown(ctx)
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Handle("/ws", h.websocketHandler)
	r.Handle("/connection/websocket", h.websocketHandler)
	r.Handle("/connection/http_stream", h.httpStreamHandler)
	r.Handle("/connection/sse", h.sseHandler)
	r.Handle("/connection/emulation", h.emulationHandler)
}

func (h *Handler) wrapTransportHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, handshake, err := h.authenticate(r)
		if err != nil {
			http.Error(w, "invalid ticket", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), authSessionContextKey{}, session)
		ctx = context.WithValue(ctx, ticketHandshakeContextKey{}, handshake)
		ctx = centrifuge.SetCredentials(ctx, &centrifuge.Credentials{
			UserID: session.Actor.ID,
		})

		go func(ctx context.Context, handshake *ticketHandshake) {
			<-ctx.Done()
			handshake.Release(h.ticketService.Release)
		}(ctx, handshake)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) authenticate(r *http.Request) (model.AuthSession, *ticketHandshake, error) {
	ticketValue := r.URL.Query().Get("ticket")
	if ticketValue == "" {
		return model.AuthSession{}, nil, errors.New("missing ticket")
	}

	session, err := h.ticketService.Reserve(ticketValue)
	if err != nil {
		return model.AuthSession{}, nil, err
	}

	return session, newTicketHandshake(ticketValue), nil
}

func (h *Handler) onConnecting(ctx context.Context, _ centrifuge.ConnectEvent) (centrifuge.ConnectReply, error) {
	session, ok := authSessionFromContext(ctx)
	if !ok {
		return centrifuge.ConnectReply{}, centrifuge.ErrorUnauthorized
	}

	data, err := json.Marshal(map[string]any{
		"actorId":     session.Actor.ID,
		"displayName": session.Actor.DisplayName,
		"actorType":   session.Actor.Type,
	})
	if err != nil {
		return centrifuge.ConnectReply{}, err
	}

	return centrifuge.ConnectReply{
		Context: ctx,
		Credentials: &centrifuge.Credentials{
			UserID: session.Actor.ID,
		},
		Data: data,
	}, nil
}

func (h *Handler) onConnect(client *centrifuge.Client) {
	session, ok := authSessionFromContext(client.Context())
	if !ok {
		client.Disconnect()
		return
	}

	handshake, ok := ticketHandshakeFromContext(client.Context())
	if !ok || handshake.Ticket() == "" {
		client.Disconnect()
		return
	}

	if err := handshake.Commit(h.ticketService.Commit); err != nil {
		client.Disconnect()
		return
	}

	h.registry.Add(client, session.Actor)

	messageHandler := &MessageHandler{
		roomService:  h.roomService,
		matchService: h.matchService,
		node:         h.node,
		registry:     h.registry,
		logger:       h.logger,
	}

	client.OnSubscribe(func(e centrifuge.SubscribeEvent, cb centrifuge.SubscribeCallback) {
		cb(centrifuge.SubscribeReply{}, centrifuge.ErrorPermissionDenied)
	})

	client.OnMessage(func(e centrifuge.MessageEvent) {
		messageHandler.HandleMessage(client, e.Data)
	})

	client.OnDisconnect(func(e centrifuge.DisconnectEvent) {
		messageHandler.OnDisconnect(client, e)
	})
}

func (h *MessageHandler) HandleMessage(client *centrifuge.Client, message []byte) {
	locked := h.registry.WithClientLock(client.ID(), func(state *ConnectionState) {
		var msg ClientMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			h.sendServerMessage(client, ServerMessage{
				Type: "error",
				Payload: ErrorPayload{
					Message: "invalid message format",
				},
			})
			return
		}

		ctx := client.Context()

		switch msg.Type {
		case "join_room":
			h.handleJoinRoom(ctx, state, msg.Payload)
		case "update_room_settings":
			h.handleUpdateRoomSettings(ctx, state, msg.Payload)
		case "start_room":
			h.handleStartRoom(ctx, state, msg.Payload)
		case "match_action":
			h.handleMatchAction(ctx, state, msg.Payload)
		case "leave_match":
			h.handleLeaveMatch(ctx, state, msg.Payload)
		case "delete_room":
			h.handleDeleteRoom(ctx, state, msg.Payload)
		case "kick_participant":
			h.handleKickParticipant(ctx, state, msg.Payload)
		default:
			h.sendServerMessage(client, ServerMessage{
				Type: "error",
				Payload: ErrorPayload{
					Message: "unsupported message type",
				},
			})
		}
	})

	if !locked {
		h.loggerFromContext(client.Context()).With("clientID", client.ID()).Warnf("message ignored for unknown client")
	}
}

func (h *MessageHandler) OnDisconnect(client *centrifuge.Client, _ centrifuge.DisconnectEvent) {
	state, ok := h.registry.Remove(client.ID())
	if !ok {
		h.loggerFromContext(client.Context()).With("clientID", client.ID()).Warnf("disconnect for unknown client")
		return
	}

	if state.RoomID != "" && !h.registry.HasLocalClientForActor(state.RoomID, state.Actor.ID) {
		if err := h.roomService.MarkActorDisconnectedInActiveMatch(client.Context(), state.RoomID, state.Actor.ID); err != nil {
			h.loggerFromContext(client.Context()).With("roomID", state.RoomID, "actorID", state.Actor.ID, "error", err).Warnf("mark actor disconnected failed")
			return
		}

		h.broadcastActiveMatchStateIfExists(client.Context(), state.RoomID)
	}
}

func (h *MessageHandler) handleJoinRoom(ctx context.Context, state *ConnectionState, payload any) {
	if state == nil || state.Client == nil {
		return
	}

	client := state.Client
	raw, err := json.Marshal(payload)
	if err != nil {
		h.sendServerMessage(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid payload",
			},
		})
		return
	}

	var p JoinRoomPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.RoomID == "" {
		h.sendServerMessage(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid roomId",
			},
		})
		return
	}

	actor := state.Actor
	previousRoomID := state.RoomID
	leftPreviousRoom := false

	if previousRoomID == p.RoomID {
		if err := h.roomService.MarkActorReconnectedInActiveMatch(ctx, p.RoomID, actor.ID); err != nil {
			h.loggerFromContext(ctx).With("roomID", p.RoomID, "actorID", actor.ID, "error", err).Warnf("mark actor reconnected failed")
		} else {
			h.broadcastActiveMatchStateIfExists(ctx, p.RoomID)
		}

		roomState, err := h.roomService.GetRoomState(ctx, p.RoomID)
		if err != nil {
			h.sendServerMessage(client, ServerMessage{
				Type: "error",
				Payload: ErrorPayload{
					Message: "failed to load room",
				},
			})
			return
		}

		h.broadcastRoomState(ctx, p.RoomID, roomState)
		h.sendActiveMatchStateIfExists(ctx, state, p.RoomID)
		return
	}

	if previousRoomID != "" && previousRoomID != p.RoomID {
		previousRoomState, err := h.roomService.GetRoomState(ctx, previousRoomID)
		if err != nil {
			h.sendServerMessage(client, ServerMessage{
				Type: "error",
				Payload: ErrorPayload{
					Message: "failed to load previous room",
				},
			})
			return
		}

		if previousRoomState.Status == string(model.RoomStatusPlaying) {
			h.sendServerMessage(client, ServerMessage{
				Type: "error",
				Payload: ErrorPayload{
					Message: "cannot switch rooms during active match",
				},
			})
			return
		}

		if err := h.roomService.LeaveRoom(ctx, roomDTO.LeaveRoomRequest{
			ActorID: actor.ID,
			RoomID:  previousRoomID,
		}); err != nil {
			h.sendServerMessage(client, ServerMessage{
				Type: "error",
				Payload: ErrorPayload{
					Message: "failed to leave previous room",
				},
			})
			return
		}

		h.unsubscribeFromRoom(client, previousRoomID, actor.ID)
		h.registry.SetRoom(state, "")
		leftPreviousRoom = true

		previousRoomState, err = h.roomService.GetRoomState(ctx, previousRoomID)
		if err == nil {
			h.broadcastRoomState(ctx, previousRoomID, previousRoomState)
		}
	}

	roomState, err := h.roomService.JoinRoom(ctx, roomDTO.JoinRoomRequest{
		Actor: roomDTO.ActorRequest{
			ID:          actor.ID,
			Type:        string(actor.Type),
			DisplayName: actor.DisplayName,
		},
		RoomID: p.RoomID,
	})
	if err != nil {
		if leftPreviousRoom && previousRoomID != "" {
			rollbackRoomState, rollbackErr := h.roomService.JoinRoom(ctx, roomDTO.JoinRoomRequest{
				Actor: roomDTO.ActorRequest{
					ID:          actor.ID,
					Type:        string(actor.Type),
					DisplayName: actor.DisplayName,
				},
				RoomID: previousRoomID,
			})
			if rollbackErr == nil {
				if err := h.subscribeToRoom(client, previousRoomID, actor.ID); err == nil {
					h.registry.SetRoom(state, previousRoomID)
					h.broadcastRoomState(ctx, previousRoomID, rollbackRoomState)
				}
			}
		}

		message := "failed to join room"
		switch {
		case errors.Is(err, roomService.ErrRoomFull):
			message = "room is full"
		case errors.Is(err, roomService.ErrRoomNotFound):
			message = "room not found"
		case errors.Is(err, roomService.ErrRoomNotJoinable):
			message = "room is not joinable"
		}

		h.sendServerMessage(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: message,
			},
		})
		return
	}

	if err := h.subscribeToRoom(client, p.RoomID, actor.ID); err != nil {
		_ = h.roomService.LeaveRoom(ctx, roomDTO.LeaveRoomRequest{
			ActorID: actor.ID,
			RoomID:  p.RoomID,
		})

		if leftPreviousRoom && previousRoomID != "" {
			rollbackRoomState, rollbackErr := h.roomService.JoinRoom(ctx, roomDTO.JoinRoomRequest{
				Actor: roomDTO.ActorRequest{
					ID:          actor.ID,
					Type:        string(actor.Type),
					DisplayName: actor.DisplayName,
				},
				RoomID: previousRoomID,
			})
			if rollbackErr == nil && h.subscribeToRoom(client, previousRoomID, actor.ID) == nil {
				h.registry.SetRoom(state, previousRoomID)
				h.broadcastRoomState(ctx, previousRoomID, rollbackRoomState)
			}
		}

		h.sendServerMessage(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "failed to subscribe to room stream",
			},
		})
		return
	}

	h.registry.SetRoom(state, p.RoomID)
	if err := h.roomService.MarkActorReconnectedInActiveMatch(ctx, p.RoomID, actor.ID); err != nil {
		h.loggerFromContext(ctx).With("roomID", p.RoomID, "actorID", actor.ID, "error", err).Warnf("mark actor reconnected failed")
	} else {
		h.broadcastActiveMatchStateIfExists(ctx, p.RoomID)
	}

	h.broadcastRoomState(ctx, p.RoomID, roomState)
	h.sendActiveMatchStateIfExists(ctx, state, p.RoomID)
}

func (h *MessageHandler) handleUpdateRoomSettings(ctx context.Context, state *ConnectionState, payload any) {
	if state == nil || state.Client == nil {
		return
	}

	client := state.Client
	raw, err := json.Marshal(payload)
	if err != nil {
		h.sendServerMessage(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid payload",
			},
		})
		return
	}

	var p UpdateRoomSettingsPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.RoomID == "" {
		h.sendServerMessage(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid room settings payload",
			},
		})
		return
	}

	roomState, err := h.roomService.UpdateRoomSettings(
		ctx,
		roomDTO.UpdateRoomSettingsServiceRequest{
			ActorID:    state.Actor.ID,
			RoomID:     p.RoomID,
			Name:       p.Name,
			GameType:   p.GameType,
			MaxPlayers: p.MaxPlayers,
			Settings:   p.Settings,
		},
	)
	if err != nil {
		message := "failed to update room settings"
		switch {
		case errors.Is(err, roomService.ErrForbidden):
			message = "forbidden"
		case errors.Is(err, roomService.ErrRoomNotFound):
			message = "room not found"
		case errors.Is(err, roomService.ErrInvalidRoomName),
			errors.Is(err, roomService.ErrInvalidGameType),
			errors.Is(err, roomService.ErrInvalidMaxPlayers),
			errors.Is(err, roomService.ErrInvalidRoomSettings),
			errors.Is(err, roomService.ErrMaxPlayersLessThanParticipants),
			errors.Is(err, roomService.ErrRoomSettingsLocked):
			message = err.Error()
		}

		h.sendServerMessage(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: message,
			},
		})
		return
	}

	h.broadcastRoomState(ctx, p.RoomID, roomState)
}

func (h *MessageHandler) handleStartRoom(ctx context.Context, state *ConnectionState, payload any) {
	if state == nil || state.Client == nil {
		return
	}

	client := state.Client
	raw, err := json.Marshal(payload)
	if err != nil {
		h.sendServerMessage(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid payload",
			},
		})
		return
	}

	var p StartRoomPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.RoomID == "" {
		h.sendServerMessage(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid start room payload",
			},
		})
		return
	}

	roomState, err := h.roomService.StartRoom(ctx, roomDTO.StartRoomRequest{
		ActorID: state.Actor.ID,
		RoomID:  p.RoomID,
	})
	if err != nil {
		message := "failed to start room"
		switch {
		case errors.Is(err, roomService.ErrForbidden):
			message = "forbidden"
		case errors.Is(err, roomService.ErrRoomNotFound):
			message = "room not found"
		case errors.Is(err, roomService.ErrNotEnoughPlayers), errors.Is(err, roomService.ErrRoomNotReady):
			message = err.Error()
		}

		h.sendServerMessage(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: message,
			},
		})
		return
	}

	h.broadcastRoomState(ctx, p.RoomID, roomState)

	matchState, err := h.matchService.GetActiveByRoomID(ctx, p.RoomID)
	if err != nil {
		h.sendServerMessage(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "failed to load started match",
			},
		})
		return
	}

	if matchState.Status == string(model.MatchStatusFinished) {
		h.broadcastMatchFinished(ctx, p.RoomID)
		return
	}

	h.broadcastMatchState(ctx, p.RoomID, matchState)
}

func (h *MessageHandler) broadcastRoomState(ctx context.Context, roomID string, roomState any) {
	msg, err := json.Marshal(ServerMessage{
		Type:    "room_state",
		Payload: roomState,
	})
	if err != nil {
		h.loggerFromContext(ctx).With("roomID", roomID, "error", err).Warnf("marshal room state failed")
		return
	}

	h.publish(ctx, roomChannel(roomID), msg)
}

func (h *MessageHandler) sendMatchState(state *ConnectionState, matchState any) {
	if state == nil || state.Client == nil {
		return
	}

	client := state.Client
	h.sendServerMessage(client, ServerMessage{
		Type:    "match_state",
		Payload: h.publicMatchStatePayload(client.Context(), matchState),
	})
	h.sendServerMessage(client, ServerMessage{
		Type:    "match_private_state",
		Payload: privateMatchStatePayload(matchState, state.Actor.ID),
	})
}

func (h *MessageHandler) broadcastMatchState(ctx context.Context, roomID string, matchState any) {
	h.broadcastMatchEvent(ctx, roomID, "match_state", h.publicMatchStatePayload(ctx, matchState))
	h.broadcastPrivateMatchState(ctx, roomID, matchState)
}

func (h *MessageHandler) sendActiveMatchStateIfExists(ctx context.Context, state *ConnectionState, roomID string) {
	if state == nil || state.Client == nil {
		return
	}

	matchState, err := h.matchService.GetActiveByRoomID(ctx, roomID)
	if err != nil {
		if errors.Is(err, matchService.ErrMatchNotFound) {
			return
		}

		h.sendServerMessage(state.Client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "failed to load active match",
			},
		})
		return
	}

	h.sendMatchState(state, matchState)
}

func (h *MessageHandler) broadcastActiveMatchStateIfExists(ctx context.Context, roomID string) {
	matchState, err := h.matchService.GetActiveByRoomID(ctx, roomID)
	if err != nil {
		if errors.Is(err, matchService.ErrMatchNotFound) {
			return
		}

		h.loggerFromContext(ctx).With("roomID", roomID, "error", err).Warnf("failed to load active match for broadcast")
		return
	}

	h.broadcastMatchState(ctx, roomID, matchState)
}

func (h *MessageHandler) broadcastMatchEvent(ctx context.Context, roomID, eventType string, matchState any) {
	msg, err := json.Marshal(ServerMessage{
		Type:    eventType,
		Payload: matchState,
	})
	if err != nil {
		h.loggerFromContext(ctx).With("roomID", roomID, "eventType", eventType, "error", err).Warnf("marshal match event failed")
		return
	}

	h.publish(ctx, matchChannel(roomID), msg)
}

func (h *MessageHandler) broadcastPrivateMatchState(ctx context.Context, roomID string, matchState any) {
	for _, actorID := range actorIDsForMatchState(matchState) {
		msg, err := json.Marshal(ServerMessage{
			Type:    "match_private_state",
			Payload: privateMatchStatePayload(matchState, actorID),
		})
		if err != nil {
			h.loggerFromContext(ctx).With("roomID", roomID, "actorID", actorID, "error", err).Warnf("marshal private match state failed")
			continue
		}

		h.publish(ctx, roomPrivateChannel(roomID, actorID), msg)
	}
}

func (h *MessageHandler) handleMatchAction(ctx context.Context, state *ConnectionState, payload any) {
	if state == nil || state.Client == nil {
		return
	}

	client := state.Client
	raw, err := json.Marshal(payload)
	if err != nil {
		h.sendServerMessage(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid payload",
			},
		})
		return
	}

	var p MatchActionPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.RoomID == "" || p.Action == "" {
		h.sendServerMessage(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid match action payload",
			},
		})
		return
	}

	matchState, err := h.matchService.ApplyAction(ctx, matchDTO.ApplyMatchActionRequest{
		ActorID: state.Actor.ID,
		RoomID:  p.RoomID,
		Action:  p.Action,
		Payload: marshalRawMessage(p.Payload),
	})
	if err != nil {
		message := "failed to apply match action"
		switch {
		case errors.Is(err, matchService.ErrMatchNotFound):
			message = "match not found"
		case errors.Is(err, matchService.ErrMatchNotActive):
			message = "match is not active"
		case errors.Is(err, matchService.ErrInvalidMatchAction):
			message = "invalid match action"
		case errors.Is(err, matchService.ErrNotYourTurn):
			message = "not your turn"
		}

		h.sendServerMessage(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: message,
			},
		})
		return
	}

	h.broadcastMatchState(ctx, p.RoomID, matchState)
}

func marshalRawMessage(v any) json.RawMessage {
	if v == nil {
		return json.RawMessage([]byte(`null`))
	}

	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage([]byte(`null`))
	}

	return json.RawMessage(data)
}

func (h *MessageHandler) handleLeaveMatch(ctx context.Context, state *ConnectionState, payload any) {
	if state == nil || state.Client == nil {
		return
	}

	client := state.Client
	raw, err := json.Marshal(payload)
	if err != nil {
		h.sendServerMessage(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: "invalid payload"},
		})
		return
	}

	var p LeaveMatchPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.RoomID == "" {
		h.sendServerMessage(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: "invalid leave match payload"},
		})
		return
	}

	if err := h.roomService.LeaveActiveMatch(ctx, p.RoomID, state.Actor.ID); err != nil {
		message := "failed to leave match"
		switch {
		case errors.Is(err, roomService.ErrForbidden):
			message = "forbidden"
		case errors.Is(err, roomService.ErrRoomNotFound):
			message = "room not found"
		case errors.Is(err, roomService.ErrActiveMatchNotFound):
			message = "active match not found"
		}

		h.sendServerMessage(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: message},
		})
		return
	}

	h.broadcastMatchFinished(ctx, p.RoomID)
	h.disconnectLocalActorConnections(p.RoomID, state.Actor.ID)
}

func (h *MessageHandler) handleDeleteRoom(ctx context.Context, state *ConnectionState, payload any) {
	if state == nil || state.Client == nil {
		return
	}

	client := state.Client
	raw, err := json.Marshal(payload)
	if err != nil {
		h.sendServerMessage(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: "invalid payload"},
		})
		return
	}

	var p DeleteRoomPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.RoomID == "" {
		h.sendServerMessage(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: "invalid delete room payload"},
		})
		return
	}

	roomState, err := h.roomService.GetRoomState(ctx, p.RoomID)
	if err != nil {
		h.sendServerMessage(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: "room not found"},
		})
		return
	}

	if roomState.OwnerActorID != state.Actor.ID {
		h.sendServerMessage(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: "forbidden"},
		})
		return
	}

	if roomState.Status == string(model.RoomStatusPlaying) {
		if err := h.roomService.TerminateRoomMatch(ctx, p.RoomID, model.MatchTerminationReasonRoomDeleted); err == nil {
			h.broadcastMatchFinished(ctx, p.RoomID)
		}
	}

	if err := h.roomService.DeleteRoom(ctx, roomDTO.DeleteRoomRequest{
		ActorID: state.Actor.ID,
		RoomID:  p.RoomID,
	}); err != nil {
		message := "failed to delete room"
		switch {
		case errors.Is(err, roomService.ErrForbidden):
			message = "forbidden"
		case errors.Is(err, roomService.ErrRoomNotFound):
			message = "room not found"
		}

		h.sendServerMessage(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: message},
		})
		return
	}

	msg, err := json.Marshal(ServerMessage{
		Type: "room_deleted",
		Payload: map[string]string{
			"roomId": p.RoomID,
		},
	})
	if err != nil {
		h.loggerFromContext(ctx).With("roomID", p.RoomID, "error", err).Warnf("marshal room deleted failed")
		return
	}

	h.publish(ctx, roomChannel(p.RoomID), msg)
}

func (h *MessageHandler) handleKickParticipant(ctx context.Context, state *ConnectionState, payload any) {
	if state == nil || state.Client == nil {
		return
	}

	client := state.Client
	raw, err := json.Marshal(payload)
	if err != nil {
		h.sendServerMessage(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid payload",
			},
		})
		return
	}

	var p KickParticipantPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.RoomID == "" || p.TargetActorID == "" {
		h.sendServerMessage(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid kick participant payload",
			},
		})
		return
	}

	roomState, err := h.roomService.KickParticipant(ctx, roomDTO.KickParticipantRequest{
		ActorID:       state.Actor.ID,
		RoomID:        p.RoomID,
		TargetActorID: p.TargetActorID,
	})
	if err != nil {
		message := "failed to kick participant"
		switch {
		case errors.Is(err, roomService.ErrForbidden):
			message = "forbidden"
		case errors.Is(err, roomService.ErrRoomNotFound):
			message = "room not found"
		case errors.Is(err, roomService.ErrParticipantNotFound):
			message = "participant not found"
		case errors.Is(err, roomService.ErrCannotKickYourself):
			message = "cannot kick yourself"
		case errors.Is(err, roomService.ErrRoomModerationLocked):
			message = "room moderation is locked"
		}

		h.sendServerMessage(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: message,
			},
		})
		return
	}

	kickNotice, err := json.Marshal(ServerMessage{
		Type: "participant_kicked",
		Payload: ParticipantKickedPayload{
			Reason: "kicked_by_owner",
		},
	})
	if err != nil {
		h.loggerFromContext(ctx).With("roomID", p.RoomID, "actorID", p.TargetActorID, "error", err).Warnf("marshal participant kicked failed")
	} else {
		h.publishWithOptions(ctx, roomPrivateChannel(p.RoomID, p.TargetActorID), kickNotice, centrifuge.WithHistory(1, roomHistoryTTL))
	}

	h.disconnectLocalActorConnections(p.RoomID, p.TargetActorID)

	h.broadcastRoomState(ctx, p.RoomID, roomState)
}

func (h *MessageHandler) subscribeToRoom(client *centrifuge.Client, roomID, actorID string) error {
	if err := client.Subscribe(roomChannel(roomID), centrifuge.WithRecovery(true), centrifuge.WithPositioning(true)); err != nil {
		return err
	}
	if err := client.Subscribe(matchChannel(roomID), centrifuge.WithRecovery(true), centrifuge.WithPositioning(true)); err != nil {
		client.Unsubscribe(roomChannel(roomID))
		return err
	}
	if err := client.Subscribe(roomPrivateChannel(roomID, actorID), centrifuge.WithRecovery(true), centrifuge.WithPositioning(true)); err != nil {
		client.Unsubscribe(matchChannel(roomID))
		client.Unsubscribe(roomChannel(roomID))
		return err
	}
	return nil
}

func (h *MessageHandler) disconnectLocalActorConnections(roomID, actorID string) {
	for _, kickedClient := range h.registry.LocalClientsForActor(roomID, actorID) {
		kickedClient.Disconnect(disconnectKicked)
	}
}

func (h *MessageHandler) unsubscribeFromRoom(client *centrifuge.Client, roomID, actorID string) {
	client.Unsubscribe(roomChannel(roomID))
	client.Unsubscribe(matchChannel(roomID))
	client.Unsubscribe(roomPrivateChannel(roomID, actorID))
}

func roomChannel(roomID string) string {
	return "room:" + roomID
}

func matchChannel(roomID string) string {
	return "match:" + roomID
}

func roomPrivateChannel(roomID, actorID string) string {
	return "room_private:" + roomID + ":" + actorID
}

func actorIDsForMatchState(matchState any) []string {
	resp, ok := matchState.(*matchDTO.MatchResponse)
	if !ok {
		respValue, ok := matchState.(matchDTO.MatchResponse)
		if !ok {
			return nil
		}
		resp = &respValue
	}

	actorIDs := make([]string, 0, len(resp.Players))
	for _, player := range resp.Players {
		if player.ActorID == "" {
			continue
		}
		actorIDs = append(actorIDs, player.ActorID)
	}

	return actorIDs
}

func (h *MessageHandler) publicMatchStatePayload(ctx context.Context, matchState any) any {
	resp, ok := matchState.(*matchDTO.MatchResponse)
	if ok {
		return PublicMatchState{
			ID:                  resp.ID,
			RoomID:              resp.RoomID,
			GameType:            resp.GameType,
			Status:              resp.Status,
			GameState:           resp.GameState,
			Result:              resp.Result,
			TerminationReason:   resp.TerminationReason,
			TerminatedByActorID: resp.TerminatedByActorID,
			TerminatedAt:        resp.TerminatedAt,
			Players:             resp.Players,
			CreatedAt:           resp.CreatedAt,
			UpdatedAt:           resp.UpdatedAt,
		}
	}

	respValue, ok := matchState.(matchDTO.MatchResponse)
	if ok {
		return PublicMatchState{
			ID:                  respValue.ID,
			RoomID:              respValue.RoomID,
			GameType:            respValue.GameType,
			Status:              respValue.Status,
			GameState:           respValue.GameState,
			Result:              respValue.Result,
			TerminationReason:   respValue.TerminationReason,
			TerminatedByActorID: respValue.TerminatedByActorID,
			TerminatedAt:        respValue.TerminatedAt,
			Players:             respValue.Players,
			CreatedAt:           respValue.CreatedAt,
			UpdatedAt:           respValue.UpdatedAt,
		}
	}

	h.loggerFromContext(ctx).With("type", matchState).Warnf("unsupported match state payload type")
	return map[string]any{}
}

func privateMatchStatePayload(matchState any, actorID string) any {
	resp, ok := matchState.(*matchDTO.MatchResponse)
	if !ok {
		respValue, ok := matchState.(matchDTO.MatchResponse)
		if !ok {
			return map[string]any{}
		}
		resp = &respValue
	}

	isYourTurn := false

	var state struct {
		CurrentPlayerID string `json:"currentPlayerId"`
	}
	if len(resp.GameState) > 0 && json.Unmarshal(resp.GameState, &state) == nil {
		isYourTurn = state.CurrentPlayerID != "" && state.CurrentPlayerID == actorID
	}

	return struct {
		IsYourTurn bool `json:"isYourTurn"`
	}{
		IsYourTurn: isYourTurn,
	}
}

func (h *MessageHandler) broadcastMatchFinished(ctx context.Context, roomID string) {
	roomState, err := h.roomService.GetRoomState(ctx, roomID)
	if err == nil {
		h.broadcastRoomState(ctx, roomID, roomState)
	}

	matchState, err := h.matchService.GetLastByRoomID(ctx, roomID)
	if err == nil {
		h.broadcastMatchEvent(ctx, roomID, "match_finished", h.publicMatchStatePayload(ctx, matchState))
	}
}

func (h *Handler) NotifyMatchFinished(ctx context.Context, roomID string) {
	messageHandler := &MessageHandler{
		roomService:  h.roomService,
		matchService: h.matchService,
		node:         h.node,
		registry:     h.registry,
		logger:       h.logger,
	}

	messageHandler.broadcastMatchFinished(ctx, roomID)
}

func (h *MessageHandler) sendServerMessage(client *centrifuge.Client, msg ServerMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.loggerFromContext(client.Context()).With("type", msg.Type, "error", err).Warnf("marshal server message failed")
		return
	}

	if err := client.Send(data); err != nil {
		h.loggerFromContext(client.Context()).With("clientID", client.ID(), "type", msg.Type, "error", err).Warnf("send server message failed")
	}
}

func (h *MessageHandler) publish(ctx context.Context, channel string, data []byte) {
	h.publishWithOptions(ctx, channel, data, centrifuge.WithHistory(roomHistorySize, roomHistoryTTL))
}

func (h *MessageHandler) publishWithOptions(ctx context.Context, channel string, data []byte, opts ...centrifuge.PublishOption) {
	if _, err := h.node.Publish(channel, data, opts...); err != nil {
		h.loggerFromContext(ctx).With("channel", channel, "error", err).Warnf("publish failed")
	}
}

func (h *MessageHandler) loggerFromContext(ctx context.Context) logger.Logger {
	return middleware.LoggerFromContextOr(ctx, h.logger)
}
