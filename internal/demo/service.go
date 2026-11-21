package demo

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sikozonpc/cinema/internal/booking"
)

var (
	ErrInvalidCheckout = errors.New("invalid demo checkout")
	ErrNotOwner        = errors.New("demo booking belongs to another browser")
	ErrInvalidPass     = errors.New("invalid demo pass")
)

type BookingConfirmer interface {
	ConfirmSeat(context.Context, string, string) (booking.Booking, error)
}

type BookingPersister interface {
	Save(context.Context, booking.Booking) error
}

type Service struct {
	bookings    BookingConfirmer
	persister   BookingPersister
	repository  Repository
	secret      []byte
	onConfirmed func(booking.Booking)
}

func NewService(bookings BookingConfirmer, persister BookingPersister, repository Repository, secret string) *Service {
	return &Service{
		bookings: bookings, persister: persister, repository: repository, secret: []byte(secret),
	}
}

func (s *Service) SetConfirmedCallback(callback func(booking.Booking)) {
	s.onConfirmed = callback
}

var foodMenu = map[string]FoodItem{
	"popcorn": {ID: "popcorn", Name: "Classic Popcorn", UnitCost: 320},
	"nachos":  {ID: "nachos", Name: "Loaded Nachos", UnitCost: 280},
	"cola":    {ID: "cola", Name: "Cold Drink", UnitCost: 180},
	"combo":   {ID: "combo", Name: "Movie Combo", UnitCost: 450},
}

var movieTitles = map[string]string{
	"inception": "Inception",
	"dune":      "Dune: Part Two",
}

func (s *Service) Complete(ctx context.Context, req CheckoutRequest) (Pass, error) {
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.OwnerID = strings.TrimSpace(req.OwnerID)
	req.CustomerName = strings.TrimSpace(req.CustomerName)
	req.MovieID = strings.TrimSpace(req.MovieID)
	req.ShowDate = strings.TrimSpace(req.ShowDate)
	if req.SessionID == "" || req.OwnerID == "" || len(req.OwnerID) > 100 ||
		req.CustomerName == "" || len([]rune(req.CustomerName)) > 80 ||
		req.ShowDate == "" || len(req.ShowDate) > 40 || movieTitles[req.MovieID] == "" {
		return Pass{}, ErrInvalidCheckout
	}

	pass, err := s.repository.FindPassByBooking(ctx, req.SessionID)
	if err != nil {
		return Pass{}, err
	}
	if pass.ID != "" {
		if pass.OwnerID != req.OwnerID {
			return Pass{}, ErrNotOwner
		}
		return pass, nil
	}

	payment, err := s.repository.FindPaymentByBooking(ctx, req.SessionID)
	if err != nil {
		return Pass{}, err
	}
	if payment.ID != "" {
		if payment.OwnerID != req.OwnerID {
			return Pass{}, ErrNotOwner
		}
	} else {
		foodItems, amount, err := normalizeFood(req.FoodItems)
		if err != nil {
			return Pass{}, err
		}
		bookingRecord, err := s.bookings.ConfirmSeat(ctx, req.SessionID, req.OwnerID)
		if err != nil {
			return Pass{}, err
		}
		if bookingRecord.MovieID != req.MovieID {
			return Pass{}, ErrInvalidCheckout
		}
		if err := s.persister.Save(ctx, bookingRecord); err != nil {
			return Pass{}, err
		}
		if s.onConfirmed != nil {
			s.onConfirmed(bookingRecord)
		}
		title, ok := movieTitles[bookingRecord.MovieID]
		if !ok {
			return Pass{}, ErrInvalidCheckout
		}
		payment = Payment{
			ID:           "DEMO-" + uuid.NewString(),
			BookingID:    bookingRecord.ID,
			OwnerID:      req.OwnerID,
			CustomerName: req.CustomerName,
			MovieID:      bookingRecord.MovieID,
			MovieTitle:   title,
			SeatID:       bookingRecord.SeatID,
			ShowDate:     req.ShowDate,
			ShowTime:     "7:30 PM",
			Cinema:       "Cinemora Central",
			Screen:       "Audi 1",
			FoodItems:    foodItems,
			Amount:       amount,
			Currency:     "INR",
			Status:       "simulated_success",
			Timestamp:    time.Now().UTC(),
		}
		if err := s.repository.SavePayment(ctx, payment); err != nil {
			// A retry after a concurrent checkout can safely reuse its payment record.
			saveErr := err
			payment, err = s.repository.FindPaymentByBooking(ctx, req.SessionID)
			if err != nil {
				return Pass{}, err
			}
			if payment.ID == "" {
				return Pass{}, saveErr
			}
			if payment.OwnerID != req.OwnerID {
				return Pass{}, ErrNotOwner
			}
		}
	}

	pass = Pass{
		ID:           s.signedID(payment.ID, payment.BookingID),
		BookingID:    payment.BookingID,
		PaymentID:    payment.ID,
		OwnerID:      payment.OwnerID,
		CustomerName: payment.CustomerName,
		MovieID:      payment.MovieID,
		MovieTitle:   payment.MovieTitle,
		SeatID:       payment.SeatID,
		ShowDate:     payment.ShowDate,
		ShowTime:     payment.ShowTime,
		Cinema:       payment.Cinema,
		Screen:       payment.Screen,
		FoodItems:    payment.FoodItems,
		Amount:       payment.Amount,
		Currency:     payment.Currency,
		Mode:         "demo",
		Status:       "demo_only",
		IssuedAt:     time.Now().UTC(),
	}
	if err := s.repository.SavePass(ctx, pass); err != nil {
		existing, findErr := s.repository.FindPassByBooking(ctx, payment.BookingID)
		if findErr != nil {
			return Pass{}, findErr
		}
		if existing.ID != "" && existing.OwnerID == req.OwnerID {
			return existing, nil
		}
		return Pass{}, err
	}
	return pass, nil
}

func normalizeFood(items []FoodItem) ([]FoodItem, int, error) {
	if len(items) > len(foodMenu) {
		return nil, 0, ErrInvalidCheckout
	}
	normalized := make([]FoodItem, 0, len(items))
	seen := make(map[string]bool, len(items))
	amount := 350 + 48
	for _, item := range items {
		menuItem, ok := foodMenu[item.ID]
		if !ok || seen[item.ID] || item.Quantity < 1 || item.Quantity > 10 {
			return nil, 0, ErrInvalidCheckout
		}
		seen[item.ID] = true
		menuItem.Quantity = item.Quantity
		amount += menuItem.UnitCost * item.Quantity
		normalized = append(normalized, menuItem)
	}
	return normalized, amount, nil
}

func (s *Service) Verify(ctx context.Context, id string) (Pass, error) {
	pass, err := s.repository.FindPass(ctx, id)
	if err != nil {
		return Pass{}, err
	}
	if pass.ID == "" {
		return Pass{}, ErrInvalidPass
	}
	parts := strings.Split(id, ".")
	if len(parts) != 2 || !hmac.Equal([]byte(s.signature(parts[0], pass.BookingID)), []byte(parts[1])) {
		return Pass{}, ErrInvalidPass
	}
	return pass, nil
}

func (s *Service) signedID(paymentID, bookingID string) string {
	id := strings.TrimPrefix(paymentID, "DEMO-")
	return fmt.Sprintf("%s.%s", id, s.signature(id, bookingID))
}

func (s *Service) signature(id, bookingID string) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(id + "|" + bookingID + "|demo"))
	return hex.EncodeToString(mac.Sum(nil))
}
