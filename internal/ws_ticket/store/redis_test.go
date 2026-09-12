package store

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/rueidis"
	"github.com/webmafia/tumladan/internal/model"
)

func TestRedisStoreTicketLifecycle(t *testing.T) {
	client := redisTestClient(t)
	store := NewRedisStore(client, time.Minute, redisTestPrefix(t))
	session := model.AuthSession{
		SessionID: "session-1",
		Actor: model.Actor{
			ID:          "actor-1",
			Type:        model.ActorTypeUser,
			DisplayName: "Player",
		},
	}

	ticket, err := store.Create(session)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	reserved, err := store.Reserve(ticket)
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if reserved.SessionID != session.SessionID || reserved.Actor.ID != session.Actor.ID {
		t.Fatalf("Reserve() = %+v, want %+v", reserved, session)
	}
	if _, err := store.Reserve(ticket); !errors.Is(err, ErrTicketInUse) {
		t.Fatalf("second Reserve() error = %v, want %v", err, ErrTicketInUse)
	}

	store.Release(ticket)
	if _, err := store.Reserve(ticket); err != nil {
		t.Fatalf("Reserve() after Release() error = %v", err)
	}
	if err := store.Commit(ticket); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if _, err := store.Resolve(ticket); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("Resolve() after Commit() error = %v, want %v", err, ErrTicketNotFound)
	}
}

func TestRedisStoreReserveIsAtomic(t *testing.T) {
	client := redisTestClient(t)
	store := NewRedisStore(client, time.Minute, redisTestPrefix(t))
	ticket, err := store.Create(model.AuthSession{SessionID: "session-atomic"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	var successes atomic.Int32
	var unexpected atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, reserveErr := store.Reserve(ticket)
			switch {
			case reserveErr == nil:
				successes.Add(1)
			case errors.Is(reserveErr, ErrTicketInUse):
			default:
				unexpected.Add(1)
			}
		}()
	}
	wg.Wait()

	if successes.Load() != 1 || unexpected.Load() != 0 {
		t.Fatalf("atomic reserve: successes=%d unexpected=%d", successes.Load(), unexpected.Load())
	}
}

func TestRedisStoreExpiresTicket(t *testing.T) {
	client := redisTestClient(t)
	store := NewRedisStore(client, 30*time.Millisecond, redisTestPrefix(t))
	ticket, err := store.Create(model.AuthSession{SessionID: "session-expiry"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	if _, err := store.Resolve(ticket); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("Resolve() expired error = %v, want %v", err, ErrTicketNotFound)
	}
}

func redisTestClient(t *testing.T) rueidis.Client {
	t.Helper()
	redisURL := os.Getenv("REDIS_TEST_URL")
	if redisURL == "" {
		t.Skip("REDIS_TEST_URL is not set")
	}
	options, err := rueidis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}
	client, err := rueidis.NewClient(options)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(client.Close)

	return client
}

func redisTestPrefix(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("test:%s:%d", t.Name(), time.Now().UnixNano())
}
