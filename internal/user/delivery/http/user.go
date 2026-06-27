package http

import (
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/webmafia/tumladan/internal/middleware"
	"github.com/webmafia/tumladan/internal/model"
	"github.com/webmafia/tumladan/internal/user/dto"
)

const maxAvatarSize = 5 << 20

type registerRequest struct {
	Nickname        string `json:"nickname"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"passwordConfirm"`
}

type loginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type logoutResponse struct {
	Status string `json:"status"`
}

type uploadAvatarResponse struct {
	URL string `json:"avatarUrl"`
}

type deleteAvatarResponse struct {
	Status string `json:"status"`
}

type updateProfileRequest struct {
	Nickname        string `json:"nickname"`
	Email           string `json:"email"`
	Password        string `json:"password,omitempty"`
	PasswordConfirm string `json:"passwordConfirm,omitempty"`
}

type publicProfileResponse struct {
	ID           string                   `json:"id"`
	Nickname     string                   `json:"nickname"`
	AvatarURL    string                   `json:"avatarUrl,omitempty"`
	Achievements []dto.Achievement        `json:"achievements"`
	MatchHistory []dto.MatchHistoryItem   `json:"matchHistory"`
	Stats        dto.UserGameStatsSummary `json:"stats"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	const op = "user.delivery.http.Register"
	log := middleware.LoggerFromContext(r.Context())

	req, file, header, err := h.parseRegisterRequest(r)
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if file != nil {
		defer file.Close()
	}

	var avatarSize int64
	var avatarType string
	if header != nil {
		avatarSize = header.Size
		avatarType = header.Header.Get("Content-Type")
		if err := h.validateAvatar(avatarType, header.Filename, avatarSize); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	resp, err := h.svc.Register(r.Context(), dto.RegisterRequest{
		Nickname:        req.Nickname,
		Email:           req.Email,
		Password:        req.Password,
		PasswordConfirm: req.PasswordConfirm,
		PreviousToken:   optionalBearerToken(r),
		Avatar:          file,
		AvatarFilename:  avatarFilename(header),
		AvatarSize:      avatarSize,
		AvatarType:      avatarType,
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if err := writeJSON(w, http.StatusCreated, resp); err != nil {
		log.Errorf("[%s]: encode response failed: %v", op, err)
	}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	const op = "user.delivery.http.Login"
	log := middleware.LoggerFromContext(r.Context())
	defer r.Body.Close()

	var req loginRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	resp, err := h.svc.Login(r.Context(), dto.LoginRequest{
		Identifier:    req.Identifier,
		Password:      req.Password,
		PreviousToken: optionalBearerToken(r),
	})
	if err != nil {
		log.With("identifier", req.Identifier, "passwordLength", len(req.Password), "error", err).Warnf("[%s]: login failed", op)
		handleServiceError(w, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, resp); err != nil {
		log.Errorf("[%s]: encode response failed: %v", op, err)
	}
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	const op = "user.delivery.http.Logout"
	log := middleware.LoggerFromContext(r.Context())

	if err := writeJSON(w, http.StatusOK, logoutResponse{Status: "ok"}); err != nil {
		log.Errorf("[%s]: encode response failed: %v", op, err)
	}
}

func (h *Handler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	const op = "user.delivery.http.UploadAvatar"
	log := middleware.LoggerFromContext(r.Context())

	actor, ok := userActorFromRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		http.Error(w, "avatar is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if err := h.validateAvatar(contentType, header.Filename, header.Size); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res, err := h.svc.UploadAvatar(r.Context(), dto.UploadAvatarRequest{
		UserID:      actor.ID,
		File:        file,
		Filename:    avatarFilename(header),
		Size:        header.Size,
		ContentType: contentType,
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, uploadAvatarResponse{URL: res.URL}); err != nil {
		log.Errorf("[%s]: encode response failed: %v", op, err)
	}
}

func (h *Handler) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	const op = "user.delivery.http.DeleteAvatar"
	log := middleware.LoggerFromContext(r.Context())

	actor, ok := userActorFromRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.svc.DeleteAvatar(r.Context(), dto.DeleteAvatarRequest{UserID: actor.ID}); err != nil {
		handleServiceError(w, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, deleteAvatarResponse{Status: "deleted"}); err != nil {
		log.Errorf("[%s]: encode response failed: %v", op, err)
	}
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	const op = "user.delivery.http.UpdateProfile"
	log := middleware.LoggerFromContext(r.Context())
	defer r.Body.Close()

	actor, ok := userActorFromRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req updateProfileRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	res, err := h.svc.UpdateProfile(r.Context(), dto.UpdateProfileRequest{
		UserID:          actor.ID,
		Nickname:        req.Nickname,
		Email:           req.Email,
		Password:        req.Password,
		PasswordConfirm: req.PasswordConfirm,
	})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, res); err != nil {
		log.Errorf("[%s]: encode response failed: %v", op, err)
	}
}

func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	const op = "user.delivery.http.GetProfile"
	log := middleware.LoggerFromContext(r.Context())

	actor, ok := userActorFromRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	res, err := h.svc.GetProfile(r.Context(), dto.GetProfileRequest{UserID: actor.ID})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	if err := writeJSON(w, http.StatusOK, res); err != nil {
		log.Errorf("[%s]: encode response failed: %v", op, err)
	}
}

func (h *Handler) GetPublicProfile(w http.ResponseWriter, r *http.Request) {
	const op = "user.delivery.http.GetPublicProfile"
	log := middleware.LoggerFromContext(r.Context())

	userID := chi.URLParam(r, "id")
	if userID == "" {
		http.Error(w, "user id is required", http.StatusBadRequest)
		return
	}

	res, err := h.svc.GetProfile(r.Context(), dto.GetProfileRequest{UserID: userID})
	if err != nil {
		handleServiceError(w, err)
		return
	}

	publicProfile := publicProfileResponse{
		ID:           res.ID,
		Nickname:     res.Nickname,
		AvatarURL:    res.AvatarURL,
		Achievements: res.Achievements,
		MatchHistory: res.MatchHistory,
		Stats:        res.Stats,
	}

	if err := writeJSON(w, http.StatusOK, publicProfile); err != nil {
		log.Errorf("[%s]: encode response failed: %v", op, err)
	}
}

func (h *Handler) parseRegisterRequest(r *http.Request) (registerRequest, multipart.File, *multipart.FileHeader, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(maxAvatarSize); err != nil {
			return registerRequest{}, nil, nil, err
		}

		req := registerRequest{
			Nickname:        r.FormValue("nickname"),
			Email:           r.FormValue("email"),
			Password:        r.FormValue("password"),
			PasswordConfirm: r.FormValue("passwordConfirm"),
		}

		file, header, err := r.FormFile("avatar")
		if err != nil {
			if err == http.ErrMissingFile {
				return req, nil, nil, nil
			}
			return registerRequest{}, nil, nil, err
		}

		return req, file, header, nil
	}

	defer r.Body.Close()
	var req registerRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		return registerRequest{}, nil, nil, err
	}

	return req, nil, nil, nil
}

func (h *Handler) validateAvatar(contentType, filename string, size int64) error {
	if size == 0 {
		return fmt.Errorf("empty file")
	}
	if size > maxAvatarSize {
		return fmt.Errorf("file too large")
	}
	if h.isAllowedAvatarContentType(contentType) {
		return nil
	}

	if h.isAllowedAvatarExtension(filename) {
		return nil
	}

	return fmt.Errorf("unsupported content type")
}

func (h *Handler) isAllowedAvatarContentType(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	for _, allowed := range h.allowedAvatarTypes {
		if contentType == strings.ToLower(strings.TrimSpace(allowed)) {
			return true
		}
	}
	return false
}

func (h *Handler) isAllowedAvatarExtension(filename string) bool {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp", ".svg", ".avif", ".heic", ".heif", ".ico":
		return true
	default:
		return false
	}
}

func avatarFilename(header *multipart.FileHeader) string {
	if header == nil {
		return ""
	}
	return header.Filename
}

func optionalBearerToken(r *http.Request) string {
	token, ok := middleware.BearerTokenFromRequest(r)
	if !ok {
		return ""
	}
	return token
}

func userActorFromRequest(r *http.Request) (model.Actor, bool) {
	actor, ok := middleware.ActorFromContext(r.Context())
	if !ok || actor.Type != model.ActorTypeUser || actor.ID == "" {
		return model.Actor{}, false
	}
	return actor, true
}
