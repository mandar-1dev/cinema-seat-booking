package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/sikozonpc/cinema/internal/utils"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	response, err := h.service.Register(r.Context(), req)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, ErrInvalidRegistration), errors.Is(err, ErrWeakPassword):
			status = http.StatusBadRequest
		case errors.Is(err, ErrEmailExists):
			status = http.StatusConflict
		}
		utils.WriteError(w, status, err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusCreated, response)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	response, err := h.service.Login(r.Context(), req)
	if err != nil {
		status := http.StatusUnauthorized
		if !errors.Is(err, ErrInvalidCredentials) {
			status = http.StatusInternalServerError
		}
		utils.WriteError(w, status, err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		utils.WriteError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}
	response, err := h.service.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	utils.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		utils.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	user, err := h.service.Profile(r.Context(), claims.UserID)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "could not load profile")
		return
	}
	utils.WriteJSON(w, http.StatusOK, user)
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		utils.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var profile Profile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := h.service.UpdateProfile(r.Context(), claims.UserID, profile)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "could not update profile")
		return
	}
	utils.WriteJSON(w, http.StatusOK, user)
}
