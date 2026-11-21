package ticket

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/sikozonpc/cinema/internal/auth"
	"github.com/sikozonpc/cinema/internal/utils"
	qrcode "github.com/skip2/go-qrcode"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) DownloadQR(w http.ResponseWriter, r *http.Request) {
	ticketID := r.PathValue("ticketID")
	claims := auth.ClaimsFromContext(r.Context())
	ticket, err := h.service.Verify(r.Context(), ticketID)
	if err != nil {
		writeError(w, err)
		return
	}
	if ticket.UserID != claims.UserID {
		utils.WriteError(w, http.StatusForbidden, "ticket belongs to another user")
		return
	}

	png, err := qrcode.Encode(ticketID, qrcode.Medium, 320)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "could not generate ticket QR")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", "inline; filename=\"ticket.png\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TicketID string `json:"ticket_id"`
	}
	if err := decodeJSON(r, &req); err != nil || req.TicketID == "" {
		utils.WriteError(w, http.StatusBadRequest, "ticket_id is required")
		return
	}
	ticket, err := h.service.Verify(r.Context(), req.TicketID)
	if err != nil {
		writeError(w, err)
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{"status": "valid", "ticket": ticket})
}

func (h *Handler) Use(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TicketID string `json:"ticket_id"`
	}
	if err := decodeJSON(r, &req); err != nil || req.TicketID == "" {
		utils.WriteError(w, http.StatusBadRequest, "ticket_id is required")
		return
	}
	ticket, err := h.service.Use(r.Context(), req.TicketID)
	if err != nil {
		writeError(w, err)
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{"status": "used", "ticket": ticket})
}

func decodeJSON(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidTicket):
		utils.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrAlreadyUsed):
		utils.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrExpired):
		utils.WriteError(w, http.StatusGone, err.Error())
	default:
		utils.WriteError(w, http.StatusInternalServerError, err.Error())
	}
}
