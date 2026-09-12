package model

type UserGameStats struct {
	GameType   string
	Matches    int
	Wins       int
	Losses     int
	Draws      int
	TotalScore int
	BestScore  int
}
