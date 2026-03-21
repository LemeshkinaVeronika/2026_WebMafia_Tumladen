package ws

import (
	"net/http"

	"github.com/gorilla/websocket"
)

func NewUpgrader(config Config) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  config.ReadBufferSize,
		WriteBufferSize: config.WriteBufferSize,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
}
