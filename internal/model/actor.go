package model

type ActorType string

const (
	ActorTypeGuest ActorType = "guest"
	ActorTypeUser  ActorType = "user"
	ActorTypeBot   ActorType = "bot"
)

type Actor struct {
	ID          string
	Type        ActorType
	DisplayName string
}
