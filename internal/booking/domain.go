package booking

import (
	"context"
	"errors"
	"time"
)

var (
	ErrSeatAlreadyBooked = errors.New("seat is already taken")
	ErrSessionNotFound   = errors.New("booking session not found or expired")
	ErrNotSessionOwner   = errors.New("user does not own this session")
)

// Booking represents a seat reservation. Status is "held" while a user is
// mid-checkout, and becomes "confirmed" once they finalize it.
type Booking struct {
	ID        string
	MovieID   string
	SeatID    string
	UserID    string
	Status    string
	ExpiresAt time.Time
}

// BookingStore is implemented by RedisStore (production), MemoryStore
// (simple single-threaded tests), and ConcurentStore (concurrency tests
// without needing Redis running).
type BookingStore interface {
	Book(b Booking) (Booking, error)
	ListBookings(movieID string) []Booking

	Confirm(ctx context.Context, sessionID string, userID string) (Booking, error)
	Release(ctx context.Context, sessionID string, userID string) error
}
