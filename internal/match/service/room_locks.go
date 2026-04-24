package service

import "sync"

type roomLocks struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newRoomLocks() *roomLocks {
	return &roomLocks{
		locks: make(map[string]*sync.Mutex),
	}
}

func (l *roomLocks) Lock(roomID string) func() {
	l.mu.Lock()

	lock, ok := l.locks[roomID]
	if !ok {
		lock = &sync.Mutex{}
		l.locks[roomID] = lock
	}

	l.mu.Unlock()

	lock.Lock()

	return func() {
		lock.Unlock()
	}
}
