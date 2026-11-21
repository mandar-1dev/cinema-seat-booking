package ticket

import "time"

type Ticket struct {
	ID        string    `json:"id" bson:"_id"`
	BookingID string    `json:"booking_id" bson:"booking_id"`
	UserID    string    `json:"user_id" bson:"user_id"`
	MovieID   string    `json:"movie_id" bson:"movie_id"`
	SeatID    string    `json:"seat_id" bson:"seat_id"`
	Status    string    `json:"status" bson:"status"`
	IssuedAt  time.Time `json:"issued_at" bson:"issued_at"`
	UsedAt    time.Time `json:"used_at,omitempty" bson:"used_at,omitempty"`
}
