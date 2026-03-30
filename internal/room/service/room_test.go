package service

import (
	"context"
	"testing"
	"time"

	"github.com/webmafia/tumladan/internal/model"
	"github.com/webmafia/tumladan/internal/room/dto"
)

type repoStub struct {
	room                    *model.Room
	participants            []model.RoomParticipantView
	updateSettingsCalled    bool
	updateStatusCalled      bool
	updateSettingsRoomID    string
	updateSettingsGameType  string
	updateSettingsMaxPlayer int
	updateStatusRoomID      string
	updateStatusValue       model.RoomStatus
}

func (r *repoStub) Create(context.Context, *model.Room) error                    { return nil }
func (r *repoStub) ListPublic(context.Context) ([]model.Room, error)             { return nil, nil }
func (r *repoStub) GetByInviteCode(context.Context, string) (*model.Room, error) { return nil, nil }
func (r *repoStub) GetByID(context.Context, string) (*model.Room, error)         { return r.room, nil }
func (r *repoStub) AddParticipant(context.Context, string, string, string) error { return nil }
func (r *repoStub) RemoveParticipant(context.Context, string, string) error      { return nil }
func (r *repoStub) ListParticipants(context.Context, string) ([]model.RoomParticipantView, error) {
	return r.participants, nil
}
func (r *repoStub) UpdateSettings(_ context.Context, roomID, gameType string, maxPlayers int) error {
	r.updateSettingsCalled = true
	r.updateSettingsRoomID = roomID
	r.updateSettingsGameType = gameType
	r.updateSettingsMaxPlayer = maxPlayers
	return nil
}
func (r *repoStub) UpdateStatus(_ context.Context, roomID string, status model.RoomStatus) error {
	r.updateStatusCalled = true
	r.updateStatusRoomID = roomID
	r.updateStatusValue = status
	return nil
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
		participants: []model.RoomParticipantView{
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
	if repo.updateStatusCalled {
		t.Fatal("expected UpdateStatus not to be called")
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
		participants: []model.RoomParticipantView{
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
	if !repo.updateStatusCalled {
		t.Fatal("expected UpdateStatus to be called")
	}
	if repo.updateStatusRoomID != "room-1" {
		t.Fatalf("expected room id room-1, got %s", repo.updateStatusRoomID)
	}
	if repo.updateStatusValue != model.RoomStatusPlaying {
		t.Fatalf("expected status %s, got %s", model.RoomStatusPlaying, repo.updateStatusValue)
	}
	if resp.Status != string(model.RoomStatusPlaying) {
		t.Fatalf("expected response status %s, got %s", model.RoomStatusPlaying, resp.Status)
	}
}
