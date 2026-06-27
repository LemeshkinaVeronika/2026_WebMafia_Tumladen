package model

import "time"

const (
	AchievementFirstGameAny         = "first_game_any"
	AchievementCarcassonneFirstGame = "carcassonne_first_game"
	AchievementCarcassonneFirstWin  = "carcassonne_first_win"
)

type Achievement struct {
	Code        string
	Title       string
	Description string
	GameType    *string
	UnlockedAt  *time.Time
	MatchID     *string
}
