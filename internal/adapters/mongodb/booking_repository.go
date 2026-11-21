package mongodb

import (
	"context"
	"time"

	"github.com/sikozonpc/cinema/internal/booking"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type BookingRepository struct {
	collection *mongo.Collection
}

func NewBookingRepository(database *mongo.Database) *BookingRepository {
	return &BookingRepository{collection: database.Collection("bookings")}
}

func (r *BookingRepository) EnsureIndexes(ctx context.Context) error {
	models := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "_id", Value: -1}},
			Options: options.Index().SetName("booking_user_history"),
		},
		{
			Keys:    bson.D{{Key: "movie_id", Value: 1}, {Key: "seat_id", Value: 1}},
			Options: options.Index().SetName("booking_movie_seat"),
		},
	}
	_, err := r.collection.Indexes().CreateMany(ctx, models)
	return err
}

func (r *BookingRepository) Save(ctx context.Context, b booking.Booking) error {
	document := bson.M{
		"_id":               b.ID,
		"movie_id":          b.MovieID,
		"seat_id":           b.SeatID,
		"user_id":           b.UserID,
		"status":            b.Status,
		"booking_timestamp": time.Now().UTC(),
	}
	_, err := r.collection.ReplaceOne(
		ctx,
		bson.M{"_id": b.ID},
		document,
		options.Replace().SetUpsert(true),
	)
	return err
}

func (r *BookingRepository) ListByUser(ctx context.Context, userID string) ([]booking.Booking, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "booking_timestamp", Value: -1}})

	cursor, err := r.collection.Find(ctx, bson.M{"user_id": userID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var documents []booking.Booking
	if err := cursor.All(ctx, &documents); err != nil {
		return nil, err
	}
	return documents, nil
}
