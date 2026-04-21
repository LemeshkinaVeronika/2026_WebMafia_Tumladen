package dto

type Phase string

const (
	PhasePlaceTile   Phase = "place_tile"
	PhasePlaceMeeple Phase = "place_meeple"
	PhaseResolveTurn Phase = "resolve_turn"
	PhaseFinished    Phase = "finished"
)
