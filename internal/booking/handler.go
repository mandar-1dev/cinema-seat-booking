package booking

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/sikozonpc/cinema/internal/auth"
	"github.com/sikozonpc/cinema/internal/realtime"
	"github.com/sikozonpc/cinema/internal/utils"
)

type handler struct {
	svc                *Service
	persistentBookings PersistentBookingRepository
	realtimeHub        *realtime.Hub
}

type PersistentBookingRepository interface {
	Save(context.Context, Booking) error
	ListByUser(context.Context, string) ([]Booking, error)
}

func NewHandler(svc *Service, repositories ...PersistentBookingRepository) *handler {
	h := &handler{svc: svc}

	if len(repositories) > 0 {
		h.persistentBookings = repositories[0]
	}

	return h
}

func (h *handler) SetRealtimeHub(hub *realtime.Hub) {
	h.realtimeHub = hub
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

	if h.realtimeHub != nil {
		h.realtimeHub.Broadcast(realtime.Event{
			Type:    "seat_held",
			MovieID: session.MovieID,
			SeatID:  session.SeatID,
			Status:  "held",
		})
	}
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

	if h.persistentBookings != nil {
		if err := h.persistentBookings.Save(r.Context(), session); err != nil {
			utils.WriteError(
				w,
				http.StatusInternalServerError,
				"booking confirmed but could not persist permanently",
			)
			return
		}
	}

	if h.realtimeHub != nil {
		h.realtimeHub.Broadcast(realtime.Event{
			Type:    "seat_confirmed",
			MovieID: session.MovieID,
			SeatID:  session.SeatID,
			Status:  "booked",
		})
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

	if h.realtimeHub != nil {
		h.realtimeHub.Broadcast(realtime.Event{
			Type: "availability_changed",
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) ListMyBookings(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())

	if claims == nil {
		utils.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	if h.persistentBookings == nil {
		utils.WriteError(w, http.StatusServiceUnavailable, "persistent bookings are unavailable")
		return
	}

	bookings, err := h.persistentBookings.ListByUser(
		r.Context(),
		claims.UserID,
	)

	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "could not load booking history")
		return
	}

	utils.WriteJSON(w, http.StatusOK, bookings)
}

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
