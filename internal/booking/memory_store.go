package booking

import (
	"context"

	"github.com/google/uuid"
)

// MemoryStore is a plain reference implementation of BookingStore, useful
// for fast unit tests that don't need Redis running. It is NOT safe for
// concurrent use — see ConcurentStore for that.
type MemoryStore struct {
	bookings map[string]Booking // "movieID:seatID" -> booking
	sessions map[string]string  // sessionID -> "movieID:seatID"
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		bookings: map[string]Booking{},
		sessions: map[string]string{},
	}
}

func memSeatKey(movieID, seatID string) string {
	return movieID + ":" + seatID
}

func (s *MemoryStore) Book(b Booking) (Booking, error) {
	key := memSeatKey(b.MovieID, b.SeatID)
	if _, exists := s.bookings[key]; exists {
		return Booking{}, ErrSeatAlreadyBooked
	}

	b.ID = uuid.NewString()
	b.Status = "held"
	s.bookings[key] = b
	s.sessions[b.ID] = key
	return b, nil
}

func (s *MemoryStore) ListBookings(movieID string) []Booking {
	var result []Booking
	for _, b := range s.bookings {
		if b.MovieID == movieID {
			result = append(result, b)
		}
	}
	return result
}

func (s *MemoryStore) Confirm(ctx context.Context, sessionID, userID string) (Booking, error) {
	key, ok := s.sessions[sessionID]
	if !ok {
		return Booking{}, ErrSessionNotFound
	}
	b := s.bookings[key]
	if b.UserID != userID {
		return Booking{}, ErrNotSessionOwner
	}
	b.Status = "confirmed"
	s.bookings[key] = b
	return b, nil
}

func (s *MemoryStore) Release(ctx context.Context, sessionID, userID string) error {
	key, ok := s.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	if s.bookings[key].UserID != userID {
		return ErrNotSessionOwner
	}
	delete(s.bookings, key)
	delete(s.sessions, sessionID)
	return nil
}
