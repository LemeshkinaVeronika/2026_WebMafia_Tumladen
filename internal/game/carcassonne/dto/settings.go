package dto

type RoomSettings struct {
	TurnTimeSeconds int           `json:"turnTimeSeconds"`
	Bots            []BotSettings `json:"bots,omitempty"`
	Expansions      []string      `json:"expansions,omitempty"`
}

type BotSettings struct {
	Difficulty string `json:"difficulty"`
}
