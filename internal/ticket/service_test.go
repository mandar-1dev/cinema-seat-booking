package ticket

import (
	"context"
	"testing"
)

type memoryRepository struct {
	byID      map[string]Ticket
	byBooking map[string]string
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		byID:      make(map[string]Ticket),
		byBooking: make(map[string]string),
	}
}

func (r *memoryRepository) Save(_ context.Context, ticket Ticket) error {
	r.byID[ticket.ID] = ticket
	r.byBooking[ticket.BookingID] = ticket.ID
	return nil
}

func (r *memoryRepository) Find(_ context.Context, id string) (Ticket, error) {
	return r.byID[id], nil
}

func (r *memoryRepository) FindByBooking(_ context.Context, bookingID string) (Ticket, error) {
	return r.byID[r.byBooking[bookingID]], nil
}

func (r *memoryRepository) MarkUsed(_ context.Context, id string) (Ticket, error) {
	ticket := r.byID[id]
	ticket.Status = "used"
	r.byID[id] = ticket
	return ticket, nil
}

func TestIssueIsIdempotentForBooking(t *testing.T) {
	ctx := context.Background()
	service := NewService(newMemoryRepository(), "test-secret")

	first, err := service.Issue(ctx, "booking-1", "user-1", "movie-1", "A1")
	if err != nil {
		t.Fatalf("issue first ticket: %v", err)
	}

	second, err := service.Issue(ctx, "booking-1", "user-1", "movie-1", "A1")
	if err != nil {
		t.Fatalf("issue duplicate ticket: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("duplicate issue returned a different ticket: got %q, want %q", second.ID, first.ID)
	}
	if _, err := service.Verify(ctx, second.ID); err != nil {
		t.Fatalf("verify issued ticket: %v", err)
	}
}
