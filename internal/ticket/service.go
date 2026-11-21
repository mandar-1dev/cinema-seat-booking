package ticket

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidTicket = errors.New("invalid ticket")
	ErrAlreadyUsed   = errors.New("ticket has already been used")
	ErrExpired       = errors.New("ticket is expired")
)

type Repository interface {
	Save(context.Context, Ticket) error
	Find(context.Context, string) (Ticket, error)
	FindByBooking(context.Context, string) (Ticket, error)
	MarkUsed(context.Context, string) (Ticket, error)
}

type Service struct {
	repository Repository
	secret     []byte
}

func NewService(repository Repository, secret string) *Service {
	return &Service{repository: repository, secret: []byte(secret)}
}

func (s *Service) Issue(ctx context.Context, bookingID, userID, movieID, seatID string) (Ticket, error) {
	existing, err := s.repository.FindByBooking(ctx, bookingID)
	if err != nil {
		return Ticket{}, err
	}
	if existing.ID != "" {
		return existing, nil
	}

	id := uuid.NewString()
	payload := id + "|" + bookingID
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(payload))
	signedID := id + "." + hex.EncodeToString(mac.Sum(nil))

	ticket := Ticket{
		ID:        signedID,
		BookingID: bookingID,
		UserID:    userID,
		MovieID:   movieID,
		SeatID:    seatID,
		Status:    "valid",
		IssuedAt:  time.Now().UTC(),
	}
	if err := s.repository.Save(ctx, ticket); err != nil {
		existing, findErr := s.repository.FindByBooking(ctx, bookingID)
		if findErr == nil && existing.ID != "" {
			return existing, nil
		}
		return Ticket{}, err
	}
	return ticket, nil
}

func (s *Service) Verify(ctx context.Context, signedID string) (Ticket, error) {
	ticket, err := s.repository.Find(ctx, signedID)
	if err != nil {
		return Ticket{}, err
	}
	if ticket.ID == "" {
		return Ticket{}, ErrInvalidTicket
	}
	parts := splitSignedID(signedID)
	if len(parts) != 2 {
		return Ticket{}, ErrInvalidTicket
	}
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(parts[0] + "|" + ticket.BookingID))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return Ticket{}, ErrInvalidTicket
	}
	return ticket, nil
}

func (s *Service) Use(ctx context.Context, signedID string) (Ticket, error) {
	ticket, err := s.Verify(ctx, signedID)
	if err != nil {
		return Ticket{}, err
	}
	if ticket.Status == "used" {
		return Ticket{}, ErrAlreadyUsed
	}
	return s.repository.MarkUsed(ctx, signedID)
}

func splitSignedID(id string) []string {
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == '.' {
			return []string{id[:i], id[i+1:]}
		}
	}
	return nil
}
