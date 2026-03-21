package ws

import "time"

type Config struct {
	ReadBufferSize  int
	WriteBufferSize int
	SendBufferSize  int
	MaxMessageSize  int64
	WriteWait       time.Duration
	PongWait        time.Duration
	PingPeriod      time.Duration
	AllowedOrigins  []string
}

func NewDefaultConfig() Config {
	pongWait := 60 * time.Second

	return Config{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		SendBufferSize:  256,
		MaxMessageSize:  4096,
		WriteWait:       10 * time.Second,
		PongWait:        pongWait,
		PingPeriod:      pongWait * 9 / 10,
		AllowedOrigins:  nil,
	}
}
