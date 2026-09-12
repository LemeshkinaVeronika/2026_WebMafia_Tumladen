package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestRedisRoomLockerSerializesReplicas(t *testing.T) {
	redisURL := os.Getenv("REDIS_TEST_URL")
	if redisURL == "" {
		t.Skip("REDIS_TEST_URL is not set")
	}

	prefix := fmt.Sprintf("test:locks:%d", time.Now().UnixNano())
	first, err := NewRedisRoomLocker(redisURL, prefix)
	if err != nil {
		t.Fatalf("NewRedisRoomLocker(first) error = %v", err)
	}
	defer first.Close()
	second, err := NewRedisRoomLocker(redisURL, prefix)
	if err != nil {
		t.Fatalf("NewRedisRoomLocker(second) error = %v", err)
	}
	defer second.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, unlockFirst, err := first.Lock(ctx, "room-1")
	if err != nil {
		t.Fatalf("first Lock() error = %v", err)
	}

	acquired := make(chan struct{})
	go func() {
		_, unlockSecond, lockErr := second.Lock(ctx, "room-1")
		if lockErr == nil {
			unlockSecond()
			close(acquired)
		}
	}()

	select {
	case <-acquired:
		t.Fatal("second replica acquired the room lock before release")
	case <-time.After(75 * time.Millisecond):
	}

	unlockFirst()
	select {
	case <-acquired:
	case <-ctx.Done():
		t.Fatalf("second replica did not acquire released lock: %v", ctx.Err())
	}
}
