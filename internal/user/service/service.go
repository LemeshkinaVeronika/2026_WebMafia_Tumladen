package service

import (
	"context"
	"io"

	"github.com/webmafia/tumladan/internal/model"
	jwtprovider "github.com/webmafia/tumladan/pkg/jwt"
)

//go:generate mockgen -destination=../../mocks/user/repository_mock.go -package=user github.com/webmafia/tumladan/internal/user/service IRepository
type IRepository interface {
	CreateUser(ctx context.Context, user model.User) error
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	GetUserByLogin(ctx context.Context, login string) (*model.User, error)
	GetUserByID(ctx context.Context, userID string) (*model.User, error)
	GetCurrentRoomByUserID(ctx context.Context, userID string) (*model.CurrentRoom, error)
	ListAchievementsByUserID(ctx context.Context, userID string) ([]model.Achievement, error)
	ListFinishedMatchesByUserID(ctx context.Context, userID string) ([]model.MatchWithPlayers, error)
	ListGameStatsByUserID(ctx context.Context, userID string) ([]model.UserGameStats, error)
	UpdateUserAvatar(ctx context.Context, userID string, avatarPath string) error
	UpdateUserProfile(ctx context.Context, user model.User) error
}

//go:generate mockgen -destination=../../mocks/user/storage_mock.go -package=user github.com/webmafia/tumladan/internal/user/service IStorage
type IStorage interface {
	UploadAvatar(ctx context.Context, file io.Reader, filename string, size int64, contentType string) (string, error)
	DeleteAvatar(ctx context.Context, objectName string) error
	GetAvatarURL(ctx context.Context, objectName string) (string, error)
}

type IGuestSessionCleaner interface {
	DeleteGuestSession(ctx context.Context, actorID string) error
}

type Service struct {
	repo                IRepository
	storage             IStorage
	tokenProvider       *jwtprovider.JWTProvider
	guestSessionCleaner IGuestSessionCleaner
}

func New(repo IRepository, storage IStorage, tokenProvider *jwtprovider.JWTProvider, guestSessionCleaner IGuestSessionCleaner) *Service {
	return &Service{
		repo:                repo,
		storage:             storage,
		tokenProvider:       tokenProvider,
		guestSessionCleaner: guestSessionCleaner,
	}
}
