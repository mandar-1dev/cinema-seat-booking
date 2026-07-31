package booking

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/sikozonpc/cinema/internal/utils"
)

type handler struct {
	svc *Service
}

func NewHandler(svc *Service) *handler {
	return &handler{svc}
}

type holdSeatRequest struct {
	UserID string `json:"user_id"`
}

type holdResponse struct {
	SessionID string `json:"session_id"`
	MovieID   string `json:"movie_id"`
	SeatID    string `json:"seat_id"`
	ExpiresAt string `json:"expires_at"`
}

func (h *handler) HoldSeat(w http.ResponseWriter, r *http.Request) {
	movieID := r.PathValue("movieID")
	seatID := r.PathValue("seatID")

	var req holdSeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" {
		utils.WriteError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	session, err := h.svc.Book(Booking{
		UserID:  req.UserID,
		SeatID:  seatID,
		MovieID: movieID,
	})
	if err != nil {
		writeBookingError(w, err)
		return
	}

	utils.WriteJSON(w, http.StatusCreated, holdResponse{
		SessionID: session.ID,
		MovieID:   session.MovieID,
		SeatID:    session.SeatID,
		ExpiresAt: session.ExpiresAt.Format(time.RFC3339),
	})
}

func (h *handler) ListSeats(w http.ResponseWriter, r *http.Request) {
	movieID := r.PathValue("movieID")
	bookings := h.svc.ListBookings(movieID)

	seats := make([]seatInfo, 0, len(bookings))
	for _, b := range bookings {
		seats = append(seats, seatInfo{
			SeatID:    b.SeatID,
			UserID:    b.UserID,
			Booked:    true,
			Confirmed: b.Status == "confirmed",
		})
	}

	utils.WriteJSON(w, http.StatusOK, seats)
}

type seatInfo struct {
	SeatID    string `json:"seat_id"`
	UserID    string `json:"user_id"`
	Booked    bool   `json:"booked"`
	Confirmed bool   `json:"confirmed"`
}

func (h *handler) ConfirmSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")

	var req holdSeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" {
		utils.WriteError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	session, err := h.svc.ConfirmSeat(r.Context(), sessionID, req.UserID)
	if err != nil {
		writeBookingError(w, err)
		return
	}

	utils.WriteJSON(w, http.StatusOK, sessionResponse{
		SessionID: session.ID,
		MovieID:   session.MovieID,
		SeatID:    session.SeatID,
		UserID:    session.UserID,
		Status:    session.Status,
	})
}

type sessionResponse struct {
	SessionID string `json:"session_id"`
	MovieID   string `json:"movie_id"`
	SeatID    string `json:"seat_id"`
	UserID    string `json:"user_id"`
	Status    string `json:"status"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

func (h *handler) ReleaseSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")

	var req holdSeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" {
		utils.WriteError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	if err := h.svc.ReleaseSeat(r.Context(), sessionID, req.UserID); err != nil {
		writeBookingError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// writeBookingError maps domain errors to HTTP status codes in one place
// so every handler doesn't repeat this switch.
func writeBookingError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrSeatAlreadyBooked):
		utils.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrNotSessionOwner):
		utils.WriteError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrSessionNotFound):
		utils.WriteError(w, http.StatusNotFound, err.Error())
	default:
		utils.WriteError(w, http.StatusInternalServerError, err.Error())
	}
}
