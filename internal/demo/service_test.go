package demo

import (
	"context"
	"errors"
	"testing"

	"github.com/sikozonpc/cinema/internal/booking"
)

type testBookings struct {
	calls int
}

func (b *testBookings) ConfirmSeat(_ context.Context, sessionID, ownerID string) (booking.Booking, error) {
	b.calls++
	if sessionID != "booking-1" || ownerID != "owner-1" {
		return booking.Booking{}, booking.ErrNotSessionOwner
	}
	return booking.Booking{
		ID: sessionID, UserID: ownerID, MovieID: "inception", SeatID: "A1", Status: "confirmed",
	}, nil
}

type testBookingPersister struct {
	calls int
}

func (p *testBookingPersister) Save(_ context.Context, _ booking.Booking) error {
	p.calls++
	return nil
}

type testRepository struct {
	payment Payment
	pass    Pass
}

func (r *testRepository) FindPaymentByBooking(_ context.Context, bookingID string) (Payment, error) {
	if r.payment.BookingID == bookingID {
		return r.payment, nil
	}
	return Payment{}, nil
}

func (r *testRepository) SavePayment(_ context.Context, payment Payment) error {
	if r.payment.ID != "" {
		return errors.New("duplicate payment")
	}
	r.payment = payment
	return nil
}

func (r *testRepository) FindPass(_ context.Context, id string) (Pass, error) {
	if r.pass.ID == id {
		return r.pass, nil
	}
	return Pass{}, nil
}

func (r *testRepository) FindPassByBooking(_ context.Context, bookingID string) (Pass, error) {
	if r.pass.BookingID == bookingID {
		return r.pass, nil
	}
	return Pass{}, nil
}

func (r *testRepository) SavePass(_ context.Context, pass Pass) error {
	if r.pass.ID != "" {
		return errors.New("duplicate pass")
	}
	r.pass = pass
	return nil
}

func TestCompleteCreatesDemoPaymentAndVerifiablePass(t *testing.T) {
	bookings := &testBookings{}
	persister := &testBookingPersister{}
	repository := &testRepository{}
	service := NewService(bookings, persister, repository, "test-secret")
	request := CheckoutRequest{
		SessionID: "booking-1", OwnerID: "owner-1", CustomerName: "Ada Lovelace",
		MovieID: "inception", ShowDate: "21 Nov 2026",
		FoodItems: []FoodItem{{ID: "popcorn", Quantity: 1}},
	}

	pass, err := service.Complete(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if pass.ID == "" || pass.Mode != "demo" || pass.Status != "demo_only" {
		t.Fatalf("unexpected demo pass: %+v", pass)
	}
	if pass.Amount != 718 || repository.payment.Amount != 718 {
		t.Fatalf("expected server-calculated total 718, got pass=%d payment=%d", pass.Amount, repository.payment.Amount)
	}
	if repository.payment.Status != "simulated_success" || repository.payment.CustomerName != "Ada Lovelace" {
		t.Fatalf("unexpected persisted demo payment: %+v", repository.payment)
	}
	verified, err := service.Verify(context.Background(), pass.ID)
	if err != nil || verified.BookingID != request.SessionID {
		t.Fatalf("demo QR ID should verify against the saved pass: pass=%+v err=%v", verified, err)
	}
	if _, err := service.Verify(context.Background(), pass.ID+"tampered"); !errors.Is(err, ErrInvalidPass) {
		t.Fatalf("tampered QR should be rejected, got %v", err)
	}

	repeated, err := service.Complete(context.Background(), request)
	if err != nil || repeated.ID != pass.ID {
		t.Fatalf("repeat checkout should reuse the same pass: pass=%+v err=%v", repeated, err)
	}
	if bookings.calls != 1 || persister.calls != 1 {
		t.Fatalf("repeat checkout should not confirm the booking again: confirmations=%d persists=%d", bookings.calls, persister.calls)
	}
}

func TestCompleteRejectsAnotherOwner(t *testing.T) {
	repository := &testRepository{
		pass: Pass{ID: "existing", BookingID: "booking-1", OwnerID: "owner-1"},
	}
	service := NewService(&testBookings{}, &testBookingPersister{}, repository, "test-secret")
	_, err := service.Complete(context.Background(), CheckoutRequest{
		SessionID: "booking-1", OwnerID: "someone-else", CustomerName: "Ada",
		MovieID: "inception", ShowDate: "21 Nov 2026",
	})
	if !errors.Is(err, ErrNotOwner) {
		t.Fatalf("expected owner check to fail, got %v", err)
	}
}

func TestCompleteUsesServerFoodPrice(t *testing.T) {
	repository := &testRepository{}
	service := NewService(&testBookings{}, &testBookingPersister{}, repository, "test-secret")
	_, err := service.Complete(context.Background(), CheckoutRequest{
		SessionID: "booking-1", OwnerID: "owner-1", CustomerName: "Ada",
		MovieID: "inception", ShowDate: "21 Nov 2026",
		FoodItems: []FoodItem{{ID: "popcorn", Quantity: 1, UnitCost: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.payment.Amount != 718 || repository.payment.FoodItems[0].UnitCost != 320 {
		t.Fatalf("server should use its own catalog price, payment=%+v", repository.payment)
	}
}
