package http

import (
	"context"

	"github.com/webmafia/tumladan/internal/guest/dto"
)

//go:generate mockgen -destination=../../../mocks/guest/service_mock.go -package=guest github.com/webmafia/tumladan/internal/guest/delivery/http IService
type IService interface {
	CreateGuestSession(ctx context.Context, req dto.CreateGuestSessionRequest) (*dto.CreateGuestSessionResponse, error)
}

type Handler struct {
	service IService
}

func NewHandler(service IService) *Handler {
	return &Handler{
		service: service,
	}
}
