package booking

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// ConcurentStore is a goroutine-safe in-memory store — same data shape as
// MemoryStore, but every method takes a lock. Useful for exercising the
// "exactly one booking wins" property in tests without needing Redis
// running (see the Redis-backed version of that test in service_test.go).
type ConcurentStore struct {
	mu       sync.RWMutex
	bookings map[string]Booking
	sessions map[string]string
}

func NewConcurentStore() *ConcurentStore {
	return &ConcurentStore{
		bookings: map[string]Booking{},
		sessions: map[string]string{},
	}
}

func (s *ConcurentStore) Book(b Booking) (Booking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

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

func (s *ConcurentStore) ListBookings(movieID string) []Booking {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Booking
	for _, b := range s.bookings {
		if b.MovieID == movieID {
			result = append(result, b)
		}
	}
	return result
}

func (s *ConcurentStore) Confirm(ctx context.Context, sessionID, userID string) (Booking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

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

func (s *ConcurentStore) Release(ctx context.Context, sessionID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

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
