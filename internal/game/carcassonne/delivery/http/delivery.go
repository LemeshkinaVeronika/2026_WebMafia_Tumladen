package http

import "encoding/json"

type IService interface {
	BuildTileCatalog() (json.RawMessage, error)
}

type Handler struct {
	service IService
}

func NewHandler(service IService) *Handler {
	return &Handler{service: service}
}
