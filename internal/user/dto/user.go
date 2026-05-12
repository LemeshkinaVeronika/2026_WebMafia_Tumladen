package dto

import "io"

type RegisterRequest struct {
	Nickname        string
	Email           string
	Password        string
	PasswordConfirm string
	Avatar          io.Reader
	AvatarFilename  string
	AvatarSize      int64
	AvatarType      string
}
type RegisterResponse struct {
	Actor ActorResponse `json:"actor"`
	Token string        `json:"token"`
}

type LoginRequest struct {
	Identifier string
	Password   string
}

type LoginResponse struct {
	Actor ActorResponse `json:"actor"`
	Token string        `json:"token"`
}

type ActorResponse struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email,omitempty"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
}

type UploadAvatarRequest struct {
	UserID      string
	File        io.Reader
	Filename    string
	Size        int64
	ContentType string
}
type UploadAvatarResponse struct {
	URL string `json:"avatar_url"`
}

type DeleteAvatarRequest struct {
	UserID string
}

type UpdateProfileRequest struct {
	UserID          string
	Nickname        string
	Email           string
	Password        string
	PasswordConfirm string
}

type UpdateProfileResponse struct {
	ID           string               `json:"id"`
	Nickname     string               `json:"nickname"`
	Email        string               `json:"email"`
	AvatarURL    string               `json:"avatarUrl,omitempty"`
	MatchHistory []MatchHistoryItem   `json:"matchHistory"`
	Stats        UserGameStatsSummary `json:"stats"`
}

type GetProfileRequest struct {
	UserID string
}
type GetProfileResponse struct {
	ID           string               `json:"id"`
	Nickname     string               `json:"nickname"`
	Email        string               `json:"email"`
	AvatarURL    string               `json:"avatarUrl,omitempty"`
	MatchHistory []MatchHistoryItem   `json:"matchHistory"`
	Stats        UserGameStatsSummary `json:"stats"`
}

type MatchHistoryItem struct {
	ID                  string                    `json:"id"`
	RoomID              string                    `json:"roomId"`
	GameType            string                    `json:"gameType"`
	Status              string                    `json:"status"`
	TerminationReason   *string                   `json:"terminationReason,omitempty"`
	TerminatedByActorID *string                   `json:"terminatedByActorId,omitempty"`
	TerminatedAt        *string                   `json:"terminatedAt,omitempty"`
	Result              MatchResultSummary        `json:"result"`
	Players             []MatchHistoryPlayerScore `json:"players"`
	CreatedAt           string                    `json:"createdAt"`
	UpdatedAt           string                    `json:"updatedAt"`
}

type MatchResultSummary struct {
	HasResult bool     `json:"hasResult"`
	Winners   []string `json:"winners"`
}

type MatchHistoryPlayerScore struct {
	ActorID     string `json:"actorId"`
	ActorType   string `json:"actorType"`
	DisplayName string `json:"displayName"`
	Seat        int    `json:"seat"`
	Score       int    `json:"score"`
	Rank        int    `json:"rank"`
	IsWinner    bool   `json:"isWinner"`
}

type UserGameStatsSummary struct {
	Overall UserGameStats   `json:"overall"`
	ByGame  []UserGameStats `json:"byGame"`
}

type UserGameStats struct {
	GameType     string  `json:"gameType,omitempty"`
	Matches      int     `json:"matches"`
	Wins         int     `json:"wins"`
	Draws        int     `json:"draws"`
	Losses       int     `json:"losses"`
	WinRate      float64 `json:"winRate"`
	TotalScore   int     `json:"totalScore"`
	AverageScore float64 `json:"averageScore"`
	BestScore    int     `json:"bestScore"`
}
