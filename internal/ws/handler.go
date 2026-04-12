package ws

import (
	"context"
	"encoding/json"
	"errors"
	matchDTO "github.com/webmafia/tumladan/internal/match/dto"
	matchService "github.com/webmafia/tumladan/internal/match/service"
	roomDTO "github.com/webmafia/tumladan/internal/room/dto"
	wsticket "github.com/webmafia/tumladan/internal/ws_ticket/service"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/webmafia/tumladan/internal/model"
	roomService "github.com/webmafia/tumladan/internal/room/service"
	pkgws "github.com/webmafia/tumladan/pkg/ws"
)

type Handler struct {
	roomService   *roomService.Service
	matchService  *matchService.Service
	ticketService *wsticket.Service
	hub           *pkgws.Hub
	upgrader      websocket.Upgrader
	wsConfig      pkgws.Config
}

type MessageHandler struct {
	roomService  *roomService.Service
	matchService *matchService.Service
	hub          *pkgws.Hub
}

func NewHandler(
	roomService *roomService.Service,
	matchService *matchService.Service,
	ticketService *wsticket.Service,
	hub *pkgws.Hub,
	wsConfig pkgws.Config,
) *Handler {
	return &Handler{
		roomService:   roomService,
		matchService:  matchService,
		ticketService: ticketService,
		hub:           hub,
		upgrader:      pkgws.NewUpgrader(wsConfig),
		wsConfig:      wsConfig,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ticketValue := r.URL.Query().Get("ticket")
	if ticketValue == "" {
		http.Error(w, "missing ticket", http.StatusUnauthorized)
		return
	}

	authSession, err := h.ticketService.Reserve(ticketValue)
	if err != nil {
		http.Error(w, "invalid ticket", http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.ticketService.Release(ticketValue)
		return
	}

	if err := h.ticketService.Commit(ticketValue); err != nil {
		_ = conn.Close()
		return
	}

	messageHandler := &MessageHandler{
		roomService:  h.roomService,
		matchService: h.matchService,
		hub:          h.hub,
	}

	client := pkgws.NewClient(authSession.Actor, h.hub, conn, nil, messageHandler, h.wsConfig)

	go client.WritePump()
	client.ReadPump(r.Context())
}

func (h *MessageHandler) HandleMessage(ctx context.Context, client *pkgws.Client, message []byte) {
	var msg ClientMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		writeJSON(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid message format",
			},
		})
		return
	}

	switch msg.Type {
	case "join_room":
		h.handleJoinRoom(ctx, client, msg.Payload)

	case "update_room_settings":
		h.handleUpdateRoomSettings(ctx, client, msg.Payload)

	case "start_room":
		h.handleStartRoom(ctx, client, msg.Payload)

	case "match_action":
		h.handleMatchAction(ctx, client, msg.Payload)

	case "finish_room_match":
		h.handleFinishRoomMatch(ctx, client, msg.Payload)

	case "delete_room":
		h.handleDeleteRoom(ctx, client, msg.Payload)

	case "abandon_room_match":
		h.handleAbandonRoomMatch(ctx, client, msg.Payload)

	case "kick_participant":
		h.handleKickParticipant(ctx, client, msg.Payload)

	default:
		writeJSON(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "unsupported message type",
			},
		})
	}
}

func (h *MessageHandler) OnDisconnect(ctx context.Context, client *pkgws.Client) {
	roomID := client.RoomID()
	if roomID == "" {
		return
	}

	roomState, err := h.roomService.GetRoomState(ctx, roomID)
	if err != nil {
		return
	}

	if roomState.Status == string(model.RoomStatusPlaying) {
		if h.hub.IsRoomEmpty(roomID) {
			_ = h.roomService.MarkRoomEmpty(ctx, roomID)
		}
		return
	}

	_ = h.roomService.LeaveRoom(ctx, roomDTO.LeaveRoomRequest{
		ActorID: client.ActorID(),
		RoomID:  roomID,
	})

	roomState, err = h.roomService.GetRoomState(ctx, roomID)
	if err != nil {
		return
	}

	h.broadcastRoomState(roomID, roomState)
}

func (h *MessageHandler) handleJoinRoom(ctx context.Context, client *pkgws.Client, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		writeJSON(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid payload",
			},
		})
		return
	}

	var p JoinRoomPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.RoomID == "" {
		writeJSON(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid roomId",
			},
		})
		return
	}

	previousRoomID := client.RoomID()
	leftPreviousRoom := false

	if previousRoomID != "" && previousRoomID != p.RoomID {
		previousRoomState, err := h.roomService.GetRoomState(ctx, previousRoomID)
		if err != nil {
			writeJSON(client, ServerMessage{
				Type: "error",
				Payload: ErrorPayload{
					Message: "failed to load previous room",
				},
			})
			return
		}

		if previousRoomState.Status == string(model.RoomStatusPlaying) {
			writeJSON(client, ServerMessage{
				Type: "error",
				Payload: ErrorPayload{
					Message: "cannot switch rooms during active match",
				},
			})
			return
		}

		if err := h.roomService.LeaveRoom(ctx, roomDTO.LeaveRoomRequest{
			ActorID: client.ActorID(),
			RoomID:  previousRoomID,
		}); err != nil {
			writeJSON(client, ServerMessage{
				Type: "error",
				Payload: ErrorPayload{
					Message: "failed to leave previous room",
				},
			})
			return
		}

		leftPreviousRoom = true
		h.hub.Leave(client, previousRoomID)
		client.SetRoomID("")

		previousRoomState, err = h.roomService.GetRoomState(ctx, previousRoomID)
		if err == nil {
			h.broadcastRoomState(previousRoomID, previousRoomState)
		}
	}

	roomState, err := h.roomService.JoinRoom(ctx, roomDTO.JoinRoomRequest{
		Actor: roomDTO.ActorRequest{
			ID:          client.ActorID(),
			Type:        string(client.Actor().Type),
			DisplayName: client.Actor().DisplayName,
		},
		RoomID: p.RoomID,
	})
	if err != nil {
		if leftPreviousRoom {
			rollbackRoomState, rollbackErr := h.roomService.JoinRoom(ctx, roomDTO.JoinRoomRequest{
				Actor: roomDTO.ActorRequest{
					ID:          client.ActorID(),
					Type:        string(client.Actor().Type),
					DisplayName: client.Actor().DisplayName,
				},
				RoomID: previousRoomID,
			})
			if rollbackErr == nil {
				client.SetRoomID(previousRoomID)
				h.hub.Register(client)
				h.broadcastRoomState(previousRoomID, rollbackRoomState)
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

		writeJSON(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: message,
			},
		})
		return
	}

	client.SetRoomID(p.RoomID)
	h.hub.Register(client)

	h.broadcastRoomState(p.RoomID, roomState)
	h.sendActiveMatchStateIfExists(ctx, client, p.RoomID)
}

func writeJSON(client *pkgws.Client, msg ServerMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	client.SendMessage(data)
}

func (h *MessageHandler) handleUpdateRoomSettings(ctx context.Context, client *pkgws.Client, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		writeJSON(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid payload",
			},
		})
		return
	}

	var p UpdateRoomSettingsPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.RoomID == "" {
		writeJSON(client, ServerMessage{
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
			ActorID:    client.ActorID(),
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

		writeJSON(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: message,
			},
		})
		return
	}

	h.broadcastRoomState(p.RoomID, roomState)
}

func (h *MessageHandler) handleStartRoom(ctx context.Context, client *pkgws.Client, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		writeJSON(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid payload",
			},
		})
		return
	}

	var p StartRoomPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.RoomID == "" {
		writeJSON(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid start room payload",
			},
		})
		return
	}

	roomState, err := h.roomService.StartRoom(ctx, roomDTO.StartRoomRequest{
		ActorID: client.ActorID(),
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

		writeJSON(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: message,
			},
		})
		return
	}

	h.broadcastRoomState(p.RoomID, roomState)

	matchState, err := h.matchService.GetActiveByRoomID(ctx, p.RoomID)
	if err != nil {
		writeJSON(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "failed to load started match",
			},
		})
		return
	}

	h.broadcastMatchState(p.RoomID, matchState)
}

func (h *MessageHandler) broadcastRoomState(roomID string, roomState any) {
	msg, err := json.Marshal(ServerMessage{
		Type:    "room_state",
		Payload: roomState,
	})
	if err != nil {
		return
	}

	h.hub.BroadcastTo(roomID, msg)
}

func (h *MessageHandler) sendMatchState(client *pkgws.Client, matchState any) {
	msg, err := json.Marshal(ServerMessage{
		Type:    "match_state",
		Payload: matchStatePayloadForActor(matchState, client.ActorID()),
	})
	if err != nil {
		return
	}

	client.SendMessage(msg)
}

func (h *MessageHandler) broadcastMatchState(roomID string, matchState any) {
	h.hub.BroadcastCustom(roomID, func(client *pkgws.Client) []byte {
		msg, err := json.Marshal(ServerMessage{
			Type:    "match_state",
			Payload: matchStatePayloadForActor(matchState, client.ActorID()),
		})
		if err != nil {
			return nil
		}

		return msg
	})
}

func (h *MessageHandler) sendActiveMatchStateIfExists(ctx context.Context, client *pkgws.Client, roomID string) {
	matchState, err := h.matchService.GetActiveByRoomID(ctx, roomID)
	if err != nil {
		if errors.Is(err, matchService.ErrMatchNotFound) {
			return
		}

		writeJSON(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "failed to load active match",
			},
		})
		return
	}

	h.sendMatchState(client, matchState)
}

func (h *MessageHandler) broadcastMatchEvent(roomID, eventType string, matchState any) {
	msg, err := json.Marshal(ServerMessage{
		Type:    eventType,
		Payload: matchState,
	})
	if err != nil {
		return
	}

	h.hub.BroadcastTo(roomID, msg)
}

func (h *MessageHandler) handleMatchAction(ctx context.Context, client *pkgws.Client, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		writeJSON(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid payload",
			},
		})
		return
	}

	var p MatchActionPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.RoomID == "" || p.Action == "" {
		writeJSON(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid match action payload",
			},
		})
		return
	}

	matchState, err := h.matchService.ApplyAction(ctx, matchDTO.ApplyMatchActionRequest{
		ActorID: client.ActorID(),
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

		writeJSON(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: message,
			},
		})
		return
	}

	h.broadcastMatchState(p.RoomID, matchState)
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

func (h *MessageHandler) handleFinishRoomMatch(ctx context.Context, client *pkgws.Client, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		writeJSON(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: "invalid payload"},
		})
		return
	}

	var p FinishRoomMatchPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.RoomID == "" {
		writeJSON(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: "invalid finish room match payload"},
		})
		return
	}

	if err := h.roomService.FinishRoomMatch(ctx, roomDTO.FinishRoomMatchRequest{
		ActorID: client.ActorID(),
		RoomID:  p.RoomID,
	}); err != nil {
		message := "failed to finish match"
		switch {
		case errors.Is(err, roomService.ErrForbidden):
			message = "forbidden"
		case errors.Is(err, roomService.ErrRoomNotFound):
			message = "room not found"
		case errors.Is(err, roomService.ErrActiveMatchNotFound):
			message = "active match not found"
		}

		writeJSON(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: message},
		})
		return
	}

	roomState, err := h.roomService.GetRoomState(ctx, p.RoomID)
	if err == nil {
		h.broadcastRoomState(p.RoomID, roomState)
	}

	matchState, err := h.matchService.GetLastByRoomID(ctx, p.RoomID)
	if err == nil {
		h.broadcastMatchEvent(p.RoomID, "match_finished", matchState)
	}
}

func (h *MessageHandler) handleDeleteRoom(ctx context.Context, client *pkgws.Client, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		writeJSON(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: "invalid payload"},
		})
		return
	}

	var p DeleteRoomPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.RoomID == "" {
		writeJSON(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: "invalid delete room payload"},
		})
		return
	}

	roomState, err := h.roomService.GetRoomState(ctx, p.RoomID)
	if err != nil {
		writeJSON(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: "room not found"},
		})
		return
	}

	if roomState.OwnerActorID != client.ActorID() {
		writeJSON(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: "forbidden"},
		})
		return
	}

	if roomState.Status == string(model.RoomStatusPlaying) {
		if err := h.roomService.AbandonRoomMatch(ctx, p.RoomID, "room_deleted"); err == nil {
			matchState, matchErr := h.matchService.GetLastByRoomID(ctx, p.RoomID)
			if matchErr == nil {
				h.broadcastMatchEvent(p.RoomID, "match_abandoned", matchState)
			}
		}
	}

	if err := h.roomService.DeleteRoom(ctx, roomDTO.DeleteRoomRequest{
		ActorID: client.ActorID(),
		RoomID:  p.RoomID,
	}); err != nil {
		message := "failed to delete room"
		switch {
		case errors.Is(err, roomService.ErrForbidden):
			message = "forbidden"
		case errors.Is(err, roomService.ErrRoomNotFound):
			message = "room not found"
		}

		writeJSON(client, ServerMessage{
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
	if err == nil {
		h.hub.BroadcastTo(p.RoomID, msg)
	}
}

func (h *MessageHandler) handleAbandonRoomMatch(ctx context.Context, client *pkgws.Client, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		writeJSON(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: "invalid payload"},
		})
		return
	}

	var p AbandonRoomMatchPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.RoomID == "" {
		writeJSON(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: "invalid abandon room match payload"},
		})
		return
	}

	reason := p.Reason
	if reason == "" {
		reason = "abandoned_by_owner"
	}

	room, err := h.roomService.GetRoomState(ctx, p.RoomID)
	if err != nil {
		writeJSON(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: "room not found"},
		})
		return
	}

	if room.OwnerActorID != client.ActorID() {
		writeJSON(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: "forbidden"},
		})
		return
	}

	if err := h.roomService.AbandonRoomMatch(ctx, p.RoomID, reason); err != nil {
		message := "failed to abandon match"
		switch {
		case errors.Is(err, roomService.ErrRoomNotFound):
			message = "room not found"
		case errors.Is(err, roomService.ErrActiveMatchNotFound):
			message = "active match not found"
		}
		writeJSON(client, ServerMessage{
			Type:    "error",
			Payload: ErrorPayload{Message: message},
		})
		return
	}

	roomState, err := h.roomService.GetRoomState(ctx, p.RoomID)
	if err == nil {
		h.broadcastRoomState(p.RoomID, roomState)
	}

	matchState, err := h.matchService.GetLastByRoomID(ctx, p.RoomID)
	if err == nil {
		h.broadcastMatchEvent(p.RoomID, "match_abandoned", matchState)
	}
}

func (h *MessageHandler) handleKickParticipant(ctx context.Context, client *pkgws.Client, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		writeJSON(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid payload",
			},
		})
		return
	}

	var p KickParticipantPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.RoomID == "" || p.TargetActorID == "" {
		writeJSON(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "invalid kick participant payload",
			},
		})
		return
	}

	roomState, err := h.roomService.KickParticipant(ctx, roomDTO.KickParticipantRequest{
		ActorID:       client.ActorID(),
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

		writeJSON(client, ServerMessage{
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
	if err == nil {
		h.hub.SendToActor(p.RoomID, p.TargetActorID, kickNotice)
	}

	h.hub.DisconnectActor(p.RoomID, p.TargetActorID)
	h.broadcastRoomState(p.RoomID, roomState)
}

func matchStatePayloadForActor(matchState any, actorID string) any {
	resp, ok := matchState.(*matchDTO.MatchResponse)
	if ok {
		return enrichMatchResponseForActor(resp, actorID)
	}

	respValue, ok := matchState.(matchDTO.MatchResponse)
	if ok {
		return enrichMatchResponseForActor(&respValue, actorID)
	}

	return matchState
}

func enrichMatchResponseForActor(matchState *matchDTO.MatchResponse, actorID string) matchDTO.MatchResponse {
	resp := *matchState
	isYourTurn := false

	var state struct {
		CurrentPlayerID string `json:"currentPlayerId"`
	}
	if len(resp.GameState) > 0 && json.Unmarshal(resp.GameState, &state) == nil {
		isYourTurn = state.CurrentPlayerID != "" && state.CurrentPlayerID == actorID
	}

	resp.IsYourTurn = &isYourTurn
	return resp
}
