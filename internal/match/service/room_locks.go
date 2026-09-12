package service

import (
	"context"
	"sync"
)

type roomLocks struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newRoomLocks() *roomLocks {
	return &roomLocks{
		locks: make(map[string]*sync.Mutex),
	}
}

func (l *roomLocks) Lock(ctx context.Context, roomID string) (context.Context, func(), error) {
	l.mu.Lock()

	lock, ok := l.locks[roomID]
	if !ok {
		lock = &sync.Mutex{}
		l.locks[roomID] = lock
	}

	l.mu.Unlock()

	lock.Lock()

	return ctx, func() {
		lock.Unlock()
	}, nil
}
