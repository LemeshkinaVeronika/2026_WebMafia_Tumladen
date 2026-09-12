package service

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidislock"
)

type RedisRoomLocker struct {
	locker rueidislock.Locker
}

func NewRedisRoomLocker(redisURL, keyPrefix string) (*RedisRoomLocker, error) {
	options, err := rueidis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis lock URL: %w", err)
	}
	options.ClientName = "tumladan-match-locks"

	locker, err := rueidislock.NewLocker(rueidislock.LockerOption{
		ClientOption:   options,
		KeyPrefix:      keyPrefix + ":match_lock",
		KeyValidity:    10 * time.Second,
		ExtendInterval: 3 * time.Second,
		TryNextAfter:   25 * time.Millisecond,
		KeyMajority:    1,
		FallbackSETPX:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("create redis room locker: %w", err)
	}
	return &RedisRoomLocker{locker: locker}, nil
}

func (l *RedisRoomLocker) Lock(ctx context.Context, roomID string) (context.Context, func(), error) {
	lockedCtx, unlock, err := l.locker.WithContext(ctx, roomID)
	if err != nil {
		return ctx, func() {}, fmt.Errorf("lock room %s: %w", roomID, err)
	}
	return lockedCtx, unlock, nil
}

func (l *RedisRoomLocker) Close() {
	if l != nil && l.locker != nil {
		l.locker.Close()
	}
}
