package dto

type CreateGuestSessionRequest struct {
	DisplayName   string `json:"displayName"`
	PreviousToken string `json:"-"`
}

type GuestActorResponse struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	DisplayName string `json:"displayName"`
}

type CreateGuestSessionResponse struct {
	Actor GuestActorResponse `json:"actor"`
	Token string             `json:"token"`
}
