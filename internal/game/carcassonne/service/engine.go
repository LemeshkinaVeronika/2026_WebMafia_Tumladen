package service

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) GameType() string {
	return "carcassonne"
}
