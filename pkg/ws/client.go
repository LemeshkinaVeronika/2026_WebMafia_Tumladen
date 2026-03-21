package ws

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/webmafia/tumladan/internal/model"
)

type Handler interface {
	HandleMessage(ctx context.Context, client *Client, message []byte)
	OnDisconnect(ctx context.Context, client *Client)
}

type Client struct {
	mu      sync.RWMutex
	roomID  string
	actor   model.Actor
	hub     *Hub
	conn    *websocket.Conn
	send    chan []byte
	config  Config
	logger  *slog.Logger
	handler Handler
}

func NewClient(
	actor model.Actor,
	hub *Hub,
	conn *websocket.Conn,
	logger *slog.Logger,
	handler Handler,
	config Config,
) *Client {
	return &Client{
		actor:   actor,
		hub:     hub,
		conn:    conn,
		send:    make(chan []byte, config.SendBufferSize),
		logger:  logger,
		handler: handler,
		config:  config,
	}
}

func (c *Client) Actor() model.Actor {
	return c.actor
}

func (c *Client) ActorID() string {
	return c.actor.ID
}

func (c *Client) DisplayName() string {
	return c.actor.DisplayName
}

func (c *Client) RoomID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.roomID
}

func (c *Client) SetRoomID(roomID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.roomID = roomID
}

func (c *Client) SendMessage(data []byte) {
	select {
	case c.send <- data:
	default:
		if c.logger != nil {
			c.logger.Warn("client send channel full, dropping message", "actorID", c.ActorID())
		}
	}
}

func (c *Client) ReadPump(ctx context.Context) {
	defer func() {
		if c.RoomID() != "" {
			c.hub.Unregister(c)
		}

		if c.handler != nil {
			c.handler.OnDisconnect(ctx, c)
		}

		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(c.config.MaxMessageSize)

	if err := c.conn.SetReadDeadline(time.Now().Add(c.config.PongWait)); err != nil {
		if c.logger != nil {
			c.logger.Error("set read deadline failed", "actorID", c.ActorID(), "error", err)
		}
		return
	}

	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if c.logger != nil && websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Warn("unexpected websocket close", "actorID", c.ActorID(), "error", err)
			}
			return
		}

		if c.handler != nil {
			c.handler.HandleMessage(ctx, c, message)
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(c.config.PingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait)); err != nil {
				if c.logger != nil {
					c.logger.Error("set write deadline failed", "actorID", c.ActorID(), "error", err)
				}
				return
			}

			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				if c.logger != nil {
					c.logger.Error("write message failed", "actorID", c.ActorID(), "error", err)
				}
				return
			}

		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait)); err != nil {
				if c.logger != nil {
					c.logger.Error("set ping deadline failed", "actorID", c.ActorID(), "error", err)
				}
				return
			}

			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				if c.logger != nil {
					c.logger.Warn("write ping failed", "actorID", c.ActorID(), "error", err)
				}
				return
			}
		}
	}
}
