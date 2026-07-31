package booking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const defaultHoldTTL = 2 * time.Minute

// RedisStore implements session-based seat booking backed by Redis.
//
// Key design:
//
//	seat:{movieID}:{seatID}   -> booking JSON  (TTL = held, no TTL = confirmed)
//	session:{sessionID}       -> seat key       (reverse lookup, same TTL as the hold)
//
// A seat's booking status is derived from whether its key exists and
// whether it has a TTL, rather than stored as an explicit field that could
// drift out of sync. An abandoned hold cleans itself up for free when its
// TTL expires — no background sweep job required.
type RedisStore struct {
	rdb *redis.Client
}

func NewRedisStore(rdb *redis.Client) *RedisStore {
	return &RedisStore{rdb: rdb}
}

func seatKey(movieID, seatID string) string {
	return fmt.Sprintf("seat:%s:%s", movieID, seatID)
}

func sessionKey(id string) string {
	return fmt.Sprintf("session:%s", id)
}

func (s *RedisStore) Book(b Booking) (Booking, error) {
	return s.hold(b)
}

func (s *RedisStore) hold(b Booking) (Booking, error) {
	ctx := context.Background()
	key := seatKey(b.MovieID, b.SeatID)

	b.ID = uuid.NewString()
	b.Status = "held"
	b.ExpiresAt = time.Now().Add(defaultHoldTTL)

	val, err := json.Marshal(b)
	if err != nil {
		return Booking{}, err
	}

	// Mode "NX" = SET only if the key doesn't already exist. Redis executes
	// this as a single atomic operation, so even if 100,000 requests hit
	// this line in the same instant, only one can ever win the key —
	// that's what guarantees exactly one booking per seat.
	res := s.rdb.SetArgs(ctx, key, val, redis.SetArgs{
		Mode: "NX",
		TTL:  defaultHoldTTL,
	})
	switch {
	case errors.Is(res.Err(), redis.Nil):
		return Booking{}, ErrSeatAlreadyBooked
	case res.Err() != nil:
		return Booking{}, res.Err()
	}

	if err := s.rdb.Set(ctx, sessionKey(b.ID), key, defaultHoldTTL).Err(); err != nil {
		// We couldn't record the reverse lookup, so this session could
		// never be confirmed or released later. Roll back the seat claim
		// rather than leave an orphaned, unmanageable hold.
		s.rdb.Del(ctx, key)
		return Booking{}, err
	}

	return b, nil
}

func (s *RedisStore) ListBookings(movieID string) []Booking {
	ctx := context.Background()
	pattern := fmt.Sprintf("seat:%s:*", movieID)

	var result []Booking
	iter := s.rdb.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		val, err := s.rdb.Get(ctx, iter.Val()).Result()
		if err != nil {
			continue
		}
		b, err := parseBooking(val)
		if err != nil {
			continue
		}
		result = append(result, b)
	}
	return result
}

func parseBooking(val string) (Booking, error) {
	var b Booking
	if err := json.Unmarshal([]byte(val), &b); err != nil {
		return Booking{}, err
	}
	return b, nil
}

// Confirm finalizes a held session into a permanent booking — but only
// for the user who created it. Without this ownership check, anyone who
// learns a sessionID could confirm or cancel someone else's booking.
func (s *RedisStore) Confirm(ctx context.Context, sessionID string, userID string) (Booking, error) {
	b, sk, err := s.getSession(ctx, sessionID)
	if err != nil {
		return Booking{}, err
	}
	if b.UserID != userID {
		return Booking{}, ErrNotSessionOwner
	}

	b.Status = "confirmed"
	b.ExpiresAt = time.Time{}

	val, err := json.Marshal(b)
	if err != nil {
		return Booking{}, err
	}

	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, sk, val, 0)                // TTL 0 = persists forever = permanently booked
	pipe.Persist(ctx, sessionKey(sessionID)) // strip TTL from the reverse-lookup key too
	if _, err := pipe.Exec(ctx); err != nil {
		return Booking{}, err
	}

	return b, nil
}

// Release cancels a held session immediately, freeing the seat without
// waiting for the TTL to expire. Only the owning user can do this.
func (s *RedisStore) Release(ctx context.Context, sessionID string, userID string) error {
	b, sk, err := s.getSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if b.UserID != userID {
		return ErrNotSessionOwner
	}
	return s.rdb.Del(ctx, sk, sessionKey(sessionID)).Err()
}

func (s *RedisStore) getSession(ctx context.Context, sessionID string) (Booking, string, error) {
	sk, err := s.rdb.Get(ctx, sessionKey(sessionID)).Result()
	if errors.Is(err, redis.Nil) {
		return Booking{}, "", ErrSessionNotFound
	}
	if err != nil {
		return Booking{}, "", err
	}

	val, err := s.rdb.Get(ctx, sk).Result()
	if errors.Is(err, redis.Nil) {
		return Booking{}, "", ErrSessionNotFound
	}
	if err != nil {
		return Booking{}, "", err
	}

	b, err := parseBooking(val)
	if err != nil {
		return Booking{}, "", err
	}
	return b, sk, nil
}
