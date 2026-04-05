package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/webmafia/tumladan/internal/model"
	"github.com/webmafia/tumladan/internal/room/dto"
)

type repoStub struct {
	room                     *model.Room
	participants             []model.RoomParticipant
	updateSettingsCalled     bool
	startRoomWithMatchCalled bool
	updateSettingsRoomID     string
	updateSettingsGameType   string
	updateSettingsMaxPlayer  int
	startRoomWithMatchRoomID string
	startedMatch             *model.Match
	startedMatchPlayers      []model.MatchPlayer
}

func (r *repoStub) Create(context.Context, *model.Room) error                    { return nil }
func (r *repoStub) ListPublic(context.Context) ([]model.Room, error)             { return nil, nil }
func (r *repoStub) GetByInviteCode(context.Context, string) (*model.Room, error) { return nil, nil }
func (r *repoStub) GetByID(context.Context, string) (*model.Room, error)         { return r.room, nil }
func (r *repoStub) RemoveParticipant(context.Context, string, string) error      { return nil }
func (r *repoStub) ListParticipants(context.Context, string) ([]model.RoomParticipant, error) {
	return r.participants, nil
}
func (r *repoStub) JoinRoom(context.Context, string, string, string) (*model.Room, []model.RoomParticipant, error) {
	return r.room, r.participants, nil
}
func (r *repoStub) UpdateSettings(_ context.Context, roomID, gameType string, maxPlayers int, settings model.JSONB) (*model.Room, []model.RoomParticipant, error) {
	r.updateSettingsCalled = true
	r.updateSettingsRoomID = roomID
	r.updateSettingsGameType = gameType
	r.updateSettingsMaxPlayer = maxPlayers
	r.room.GameType = gameType
	r.room.MaxPlayers = maxPlayers
	r.room.Settings = settings
	return r.room, r.participants, nil
}
func (r *repoStub) UpdateStatus(context.Context, string, model.RoomStatus) error {
	return nil
}
func (r *repoStub) StartRoomWithMatch(_ context.Context, roomID string, match *model.Match, players []model.MatchPlayer) (*model.Room, []model.RoomParticipant, error) {
	r.startRoomWithMatchCalled = true
	r.startRoomWithMatchRoomID = roomID
	r.startedMatch = match
	r.startedMatchPlayers = players
	r.room.Status = model.RoomStatusPlaying
	return r.room, r.participants, nil
}
func (r *repoStub) FinishActiveMatch(context.Context, string, model.JSONB) (*model.Match, []model.MatchPlayer, error) {
	return nil, nil, nil
}
func (r *repoStub) AbandonActiveMatch(context.Context, string, string) (*model.Match, []model.MatchPlayer, error) {
	return nil, nil, nil
}
func (r *repoStub) DeleteRoom(context.Context, string) error { return nil }
func (r *repoStub) FindStaleEmptyWaitingRooms(context.Context, time.Time) ([]model.Room, error) {
	return nil, nil
}
func (r *repoStub) FindStaleEmptyPlayingRooms(context.Context, time.Time) ([]model.Room, error) {
	return nil, nil
}

func TestUpdateRoomSettingsRequiresOwner(t *testing.T) {
	repo := &repoStub{
		room: &model.Room{
			ID:           "room-1",
			OwnerActorID: "owner-1",
			Status:       model.RoomStatusWaiting,
			GameType:     "carcassonne",
			MaxPlayers:   2,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		},
	}

	svc := New(repo)

	_, err := svc.UpdateRoomSettings(context.Background(), dto.UpdateRoomSettingsServiceRequest{
		ActorID:    "guest-2",
		RoomID:     "room-1",
		GameType:   "carcassonne",
		MaxPlayers: 4,
	})
	if err != ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
	if repo.updateSettingsCalled {
		t.Fatal("expected UpdateSettings not to be called")
	}
}

func TestStartRoomRequiresOwner(t *testing.T) {
	repo := &repoStub{
		room: &model.Room{
			ID:           "room-1",
			OwnerActorID: "owner-1",
			Status:       model.RoomStatusWaiting,
			GameType:     "carcassonne",
			MaxPlayers:   4,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		},
		participants: []model.RoomParticipant{
			{ActorID: "owner-1", DisplayName: "Owner", JoinedAt: time.Now().UTC()},
			{ActorID: "guest-2", DisplayName: "Guest", JoinedAt: time.Now().UTC()},
		},
	}

	svc := New(repo)

	_, err := svc.StartRoom(context.Background(), dto.StartRoomRequest{
		ActorID: "guest-2",
		RoomID:  "room-1",
	})
	if err != ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
	if repo.startRoomWithMatchCalled {
		t.Fatal("expected StartRoomWithMatch not to be called")
	}
}

func TestStartRoomUpdatesStatusForOwner(t *testing.T) {
	repo := &repoStub{
		room: &model.Room{
			ID:           "room-1",
			Name:         "Test room",
			OwnerActorID: "owner-1",
			Status:       model.RoomStatusWaiting,
			GameType:     "carcassonne",
			MaxPlayers:   4,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		},
		participants: []model.RoomParticipant{
			{ActorID: "owner-1", DisplayName: "Owner", JoinedAt: time.Now().UTC()},
			{ActorID: "guest-2", DisplayName: "Guest", JoinedAt: time.Now().UTC()},
		},
	}

	svc := New(repo)

	resp, err := svc.StartRoom(context.Background(), dto.StartRoomRequest{
		ActorID: "owner-1",
		RoomID:  "room-1",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !repo.startRoomWithMatchCalled {
		t.Fatal("expected StartRoomWithMatch to be called")
	}
	if repo.startRoomWithMatchRoomID != "room-1" {
		t.Fatalf("expected room id room-1, got %s", repo.startRoomWithMatchRoomID)
	}
	if repo.startedMatch == nil {
		t.Fatal("expected match to be created")
	}
	if repo.startedMatch.Status != model.MatchStatusActive {
		t.Fatalf("expected match status %s, got %s", model.MatchStatusActive, repo.startedMatch.Status)
	}
	if len(repo.startedMatchPlayers) != len(repo.participants) {
		t.Fatalf("expected %d match players, got %d", len(repo.participants), len(repo.startedMatchPlayers))
	}
	var gameState map[string]any
	if err := json.Unmarshal(repo.startedMatch.GameState, &gameState); err != nil {
		t.Fatalf("expected valid game state json, got %v", err)
	}
	if resp.Status != string(model.RoomStatusPlaying) {
		t.Fatalf("expected response status %s, got %s", model.RoomStatusPlaying, resp.Status)
	}
}
