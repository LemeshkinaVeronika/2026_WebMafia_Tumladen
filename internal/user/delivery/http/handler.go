package http

import (
	"context"

	"github.com/webmafia/tumladan/internal/user/dto"
)

//go:generate mockgen -destination=../../../mocks/user/service_mock.go -package=user github.com/webmafia/tumladan/internal/user/delivery/http IService,CSRFManager
type IService interface {
	Register(ctx context.Context, req dto.RegisterRequest) (*dto.RegisterResponse, error)
	Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error)
	UploadAvatar(ctx context.Context, req dto.UploadAvatarRequest) (*dto.UploadAvatarResponse, error)
	DeleteAvatar(ctx context.Context, req dto.DeleteAvatarRequest) error
	UpdateProfile(ctx context.Context, req dto.UpdateProfileRequest) (*dto.UpdateProfileResponse, error)
	GetProfile(ctx context.Context, req dto.GetProfileRequest) (*dto.GetProfileResponse, error)
}

type CSRFManager interface {
	Generate(userID, sessionID string) (string, error)
}

type Handler struct {
	svc                IService
	csrfManager        CSRFManager
	allowedAvatarTypes []string
}

func NewHandler(svc IService, csrfManager CSRFManager, allowedAvatarTypes []string) *Handler {
	return &Handler{
		svc:                svc,
		csrfManager:        csrfManager,
		allowedAvatarTypes: allowedAvatarTypes,
	}
}
