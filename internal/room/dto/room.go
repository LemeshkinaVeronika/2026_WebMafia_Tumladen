package dto

import "encoding/json"

type CreateRoomRequest struct {
	Name string `json:"name"`
}

type ActorRequest struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	DisplayName string `json:"displayName"`
}

type CreateRoomServiceRequest struct {
	Actor ActorRequest `json:"actor"`
	Name  string       `json:"name"`
}

type JoinRoomRequest struct {
	Actor  ActorRequest `json:"actor"`
	RoomID string       `json:"roomId"`
}

type LeaveRoomRequest struct {
	ActorID string `json:"actorId"`
	RoomID  string `json:"roomId"`
}

type UpdateRoomSettingsRequest struct {
	Name       string          `json:"name"`
	GameType   string          `json:"gameType"`
	MaxPlayers int             `json:"maxPlayers"`
	Settings   json.RawMessage `json:"settings"`
}

type UpdateRoomSettingsServiceRequest struct {
	ActorID    string          `json:"actorId"`
	RoomID     string          `json:"roomId"`
	Name       string          `json:"name"`
	GameType   string          `json:"gameType"`
	MaxPlayers int             `json:"maxPlayers"`
	Settings   json.RawMessage `json:"settings"`
}

type StartRoomRequest struct {
	ActorID string `json:"actorId"`
	RoomID  string `json:"roomId"`
}

type RoomResponse struct {
	ID           string                `json:"id"`
	Name         string                `json:"name"`
	IsPrivate    bool                  `json:"isPrivate"`
	InviteCode   *string               `json:"inviteCode,omitempty"`
	OwnerActorID string                `json:"ownerActorId"`
	Status       string                `json:"status"`
	GameType     string                `json:"gameType"`
	MaxPlayers   int                   `json:"maxPlayers"`
	Settings     json.RawMessage       `json:"settings,omitempty"`
	CanStart     bool                  `json:"canStart"`
	PlayersCount int                   `json:"playersCount"`
	Participants []ParticipantResponse `json:"participants,omitempty"`
	CreatedAt    string                `json:"createdAt"`
	UpdatedAt    string                `json:"updatedAt"`
}

type ParticipantResponse struct {
	ActorID     string `json:"actorId"`
	DisplayName string `json:"displayName"`
	JoinedAt    string `json:"joinedAt"`
}

type ListPublicRoomsResponse struct {
	Rooms []RoomResponse `json:"rooms"`
}

type GetRoomByInviteCodeResponse struct {
	Room RoomResponse `json:"room"`
}

type FinishRoomMatchRequest struct {
	ActorID   *string         `json:"actorId,omitempty"`
	RoomID    string          `json:"roomId"`
	Reason    string          `json:"reason"`
	GameState json.RawMessage `json:"gameState,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
}

type DeleteRoomRequest struct {
	ActorID string `json:"actorId"`
	RoomID  string `json:"roomId"`
}
type KickParticipantRequest struct {
	ActorID       string `json:"actorId"`
	RoomID        string `json:"roomId"`
	TargetActorID string `json:"targetActorId"`
}
