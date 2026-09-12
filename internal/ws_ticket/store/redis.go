package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/rueidis"
	"github.com/webmafia/tumladan/internal/model"
)

const redisOperationTimeout = 2 * time.Second

var (
	reserveTicketScript = rueidis.NewLuaScript(`
local raw = redis.call('GET', KEYS[1])
if not raw then return redis.error_reply('TICKET_NOT_FOUND') end
local ticket = cjson.decode(raw)
if ticket.reserved then return redis.error_reply('TICKET_IN_USE') end
ticket.reserved = true
redis.call('SET', KEYS[1], cjson.encode(ticket), 'KEEPTTL')
return raw
`)
	releaseTicketScript = rueidis.NewLuaScript(`
local raw = redis.call('GET', KEYS[1])
if not raw then return 0 end
local ticket = cjson.decode(raw)
ticket.reserved = false
redis.call('SET', KEYS[1], cjson.encode(ticket), 'KEEPTTL')
return 1
`)
	commitTicketScript = rueidis.NewLuaScript(`
local raw = redis.call('GET', KEYS[1])
if not raw then return redis.error_reply('TICKET_NOT_FOUND') end
local ticket = cjson.decode(raw)
if not ticket.reserved then return redis.error_reply('TICKET_NOT_RESERVED') end
redis.call('DEL', KEYS[1])
return 1
`)
)

type RedisStore struct {
	client rueidis.Client
	ttl    time.Duration
	prefix string
}

func NewRedisStore(client rueidis.Client, ttl time.Duration, prefix string) *RedisStore {
	return &RedisStore{
		client: client,
		ttl:    ttl,
		prefix: strings.TrimSuffix(prefix, ":"),
	}
}

func (s *RedisStore) Create(session model.AuthSession) (string, error) {
	for attempt := 0; attempt < 3; attempt++ {
		value, err := generateToken(32)
		if err != nil {
			return "", err
		}

		ticket := Ticket{
			Value:       value,
			SessionID:   session.SessionID,
			ActorID:     session.Actor.ID,
			ActorType:   string(session.Actor.Type),
			DisplayName: session.Actor.DisplayName,
			ExpiresAt:   time.Now().UTC().Add(s.ttl),
		}
		encoded, err := json.Marshal(ticket)
		if err != nil {
			return "", fmt.Errorf("marshal ws ticket: %w", err)
		}

		ctx, cancel := s.operationContext()
		result := s.client.Do(ctx, s.client.B().Set().Key(s.key(value)).Value(string(encoded)).Nx().Px(s.ttl).Build())
		cancel()
		if err := result.Error(); err == nil {
			return value, nil
		} else if !rueidis.IsRedisNil(err) {
			return "", fmt.Errorf("store ws ticket: %w", err)
		}
	}

	return "", errors.New("failed to generate unique ws ticket")
}

func (s *RedisStore) Reserve(value string) (model.AuthSession, error) {
	ctx, cancel := s.operationContext()
	defer cancel()

	raw, err := reserveTicketScript.Exec(ctx, s.client, []string{s.key(value)}, nil).ToString()
	if err != nil {
		return model.AuthSession{}, mapRedisTicketError(err)
	}
	return decodeTicketSession(raw)
}

func (s *RedisStore) Resolve(value string) (model.AuthSession, error) {
	ctx, cancel := s.operationContext()
	defer cancel()

	raw, err := s.client.Do(ctx, s.client.B().Get().Key(s.key(value)).Build()).ToString()
	if rueidis.IsRedisNil(err) {
		return model.AuthSession{}, ErrTicketNotFound
	}
	if err != nil {
		return model.AuthSession{}, fmt.Errorf("resolve ws ticket: %w", err)
	}
	return decodeTicketSession(raw)
}

func (s *RedisStore) Release(value string) {
	ctx, cancel := s.operationContext()
	defer cancel()
	_ = releaseTicketScript.Exec(ctx, s.client, []string{s.key(value)}, nil).Error()
}

func (s *RedisStore) Commit(value string) error {
	ctx, cancel := s.operationContext()
	defer cancel()

	if err := commitTicketScript.Exec(ctx, s.client, []string{s.key(value)}, nil).Error(); err != nil {
		return mapRedisTicketError(err)
	}
	return nil
}

func (s *RedisStore) key(value string) string {
	if s.prefix == "" {
		return "ws_ticket:" + value
	}
	return s.prefix + ":ws_ticket:" + value
}

func (s *RedisStore) operationContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), redisOperationTimeout)
}

func decodeTicketSession(raw string) (model.AuthSession, error) {
	var ticket Ticket
	if err := json.Unmarshal([]byte(raw), &ticket); err != nil {
		return model.AuthSession{}, fmt.Errorf("decode ws ticket: %w", err)
	}
	return model.AuthSession{
		SessionID: ticket.SessionID,
		Actor: model.Actor{
			ID:          ticket.ActorID,
			Type:        model.ActorType(ticket.ActorType),
			DisplayName: ticket.DisplayName,
		},
	}, nil
}

func mapRedisTicketError(err error) error {
	switch {
	case rueidis.IsRedisNil(err), strings.Contains(err.Error(), "TICKET_NOT_FOUND"):
		return ErrTicketNotFound
	case strings.Contains(err.Error(), "TICKET_IN_USE"):
		return ErrTicketInUse
	case strings.Contains(err.Error(), "TICKET_NOT_RESERVED"):
		return ErrTicketNotReserved
	default:
		return fmt.Errorf("redis ws ticket operation: %w", err)
	}
}
