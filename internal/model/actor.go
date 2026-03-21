package model

type ActorType string

const (
	ActorTypeGuest ActorType = "guest"
	ActorTypeUser  ActorType = "user"
)

type Actor struct {
	ID          string
	Type        ActorType
	DisplayName string
}
