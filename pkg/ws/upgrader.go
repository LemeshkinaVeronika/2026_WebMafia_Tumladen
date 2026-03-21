package ws

import (
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

func NewUpgrader(config Config) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  config.ReadBufferSize,
		WriteBufferSize: config.WriteBufferSize,
		CheckOrigin: func(r *http.Request) bool {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin == "" {
				return true
			}

			for _, allowed := range config.AllowedOrigins {
				if origin == allowed {
					return true
				}
			}

			return false
		},
	}
}
