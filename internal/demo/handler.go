package demo

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/sikozonpc/cinema/internal/booking"
	"github.com/sikozonpc/cinema/internal/utils"
	qrcode "github.com/skip2/go-qrcode"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	var req CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid demo checkout request")
		return
	}
	pass, err := h.service.Complete(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCheckout):
			utils.WriteError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrNotOwner), errors.Is(err, booking.ErrNotSessionOwner):
			utils.WriteError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, booking.ErrSessionNotFound):
			utils.WriteError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, booking.ErrSeatAlreadyBooked):
			utils.WriteError(w, http.StatusConflict, err.Error())
		default:
			utils.WriteError(w, http.StatusInternalServerError, "could not save demo payment and pass")
		}
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{"ticket": pass, "demo_only": true})
}

func (h *Handler) DownloadQR(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("ticketID")
	if _, err := h.service.Verify(r.Context(), id); err != nil {
		writeDemoVerifyError(w, err)
		return
	}
	png, err := qrcode.Encode(id, qrcode.Medium, 320)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "could not generate demo QR")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", "inline; filename=\"demo-ticket.png\"")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Ticket-Mode", "demo-only")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TicketID string `json:"ticket_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TicketID == "" {
		utils.WriteError(w, http.StatusBadRequest, "ticket_id is required")
		return
	}
	pass, err := h.service.Verify(r.Context(), req.TicketID)
	if err != nil {
		writeDemoVerifyError(w, err)
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"status":          "demo_valid",
		"demo_only":       true,
		"valid_for_entry": false,
		"ticket": map[string]any{
			"id": pass.ID, "booking_id": pass.BookingID, "movie_title": pass.MovieTitle,
			"seat_id": pass.SeatID, "customer_name": pass.CustomerName, "status": pass.Status,
		},
	})
}

func writeDemoVerifyError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrInvalidPass) {
		utils.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	utils.WriteError(w, http.StatusInternalServerError, "could not verify demo pass")
}
