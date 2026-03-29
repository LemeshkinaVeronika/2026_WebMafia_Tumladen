package ws

import (
	"context"
	"encoding/json"
	"errors"
	roomDTO "github.com/webmafia/tumladan/internal/room/dto"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/webmafia/tumladan/internal/model"
	roomService "github.com/webmafia/tumladan/internal/room/service"
	jwtprovider "github.com/webmafia/tumladan/pkg/jwt"
	pkgws "github.com/webmafia/tumladan/pkg/ws"
)

type Handler struct {
	roomService *roomService.Service
	jwtProvider *jwtprovider.JWTProvider
	hub         *pkgws.Hub
	upgrader    websocket.Upgrader
	wsConfig    pkgws.Config
}

type MessageHandler struct {
	roomService *roomService.Service
	hub         *pkgws.Hub
}

func NewHandler(
	roomService *roomService.Service,
	jwtProvider *jwtprovider.JWTProvider,
	hub *pkgws.Hub,
	wsConfig pkgws.Config,
) *Handler {
	return &Handler{
		roomService: roomService,
		jwtProvider: jwtProvider,
		hub:         hub,
		upgrader:    pkgws.NewUpgrader(wsConfig),
		wsConfig:    wsConfig,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	claims, err := h.jwtProvider.ParseToken(tokenStr)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	actorID := claims.ActorID
	if actorID == "" {
		actorID = claims.RegisteredClaims.Subject
	}
	if actorID == "" {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	actor := model.Actor{
		ID:          actorID,
		Type:        model.ActorType(claims.ActorType),
		DisplayName: claims.DisplayName,
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	messageHandler := &MessageHandler{
		roomService: h.roomService,
		hub:         h.hub,
	}

	client := pkgws.NewClient(
		actor,
		h.hub,
		conn,
		nil,
		messageHandler,
		h.wsConfig,
	)

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

	_ = h.roomService.LeaveRoom(ctx, roomDTO.LeaveRoomRequest{
		ActorID: client.ActorID(),
		RoomID:  roomID,
	})

	roomState, err := h.roomService.GetRoomState(ctx, roomID)
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
	if previousRoomID != "" && previousRoomID != p.RoomID {
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

		h.hub.Leave(client, previousRoomID)
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
		message := "failed to join room"
		if errors.Is(err, roomService.ErrRoomFull) {
			message = "room is full"
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
			GameType:   p.GameType,
			MaxPlayers: p.MaxPlayers,
		},
	)
	if err != nil {
		writeJSON(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "failed to update room settings",
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
		writeJSON(client, ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Message: "failed to start room",
			},
		})
		return
	}

	h.broadcastRoomState(p.RoomID, roomState)
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
