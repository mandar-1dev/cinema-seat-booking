package ticket

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoRepository struct {
	collection *mongo.Collection
}

func NewMongoRepository(database *mongo.Database) *MongoRepository {
	return &MongoRepository{collection: database.Collection("tickets")}
}

func (r *MongoRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "booking_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "user_id", Value: 1}}},
	})
	return err
}

func (r *MongoRepository) Save(ctx context.Context, ticket Ticket) error {
	_, err := r.collection.ReplaceOne(ctx, bson.M{"_id": ticket.ID}, ticket, options.Replace().SetUpsert(true))
	return err
}

func (r *MongoRepository) Find(ctx context.Context, id string) (Ticket, error) {
	var ticket Ticket
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&ticket)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Ticket{}, nil
	}
	return ticket, err
}

func (r *MongoRepository) FindByBooking(ctx context.Context, bookingID string) (Ticket, error) {
	var ticket Ticket
	err := r.collection.FindOne(ctx, bson.M{"booking_id": bookingID}).Decode(&ticket)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Ticket{}, nil
	}
	return ticket, err
}

func (r *MongoRepository) MarkUsed(ctx context.Context, id string) (Ticket, error) {
	ticket, err := r.Find(ctx, id)
	if err != nil || ticket.ID == "" {
		return Ticket{}, err
	}
	if ticket.Status == "used" {
		return ticket, ErrAlreadyUsed
	}
	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id, "status": "valid"},
		bson.M{"$set": bson.M{"status": "used", "used_at": time.Now().UTC()}},
	)
	if err != nil {
		return Ticket{}, err
	}
	if result.ModifiedCount == 0 {
		return Ticket{}, ErrAlreadyUsed
	}
	ticket.Status = "used"
	return ticket, nil
}
