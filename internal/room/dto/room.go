package dto

type CreateRoomRequest struct {
	Name string `json:"name"`
}

type UpdateRoomSettingsRequest struct {
	GameType   string `json:"gameType"`
	MaxPlayers int    `json:"maxPlayers"`
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
