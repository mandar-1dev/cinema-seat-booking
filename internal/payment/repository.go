package payment

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoRepository struct {
	collection *mongo.Collection
}

func NewMongoRepository(database *mongo.Database) *MongoRepository {
	return &MongoRepository{collection: database.Collection("payments")}
}

func (r *MongoRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "booking_id", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true)},
		{Keys: bson.D{{Key: "transaction_reference", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true)},
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "payment_timestamp", Value: -1}}},
	})
	return err
}

func (r *MongoRepository) Save(ctx context.Context, record Record) error {
	_, err := r.collection.ReplaceOne(
		ctx,
		bson.M{"_id": record.ID},
		record,
		options.Replace().SetUpsert(true),
	)
	return err
}

func (r *MongoRepository) FindOrder(ctx context.Context, orderID, bookingID, userID string) (Record, error) {
	var record Record
	err := r.collection.FindOne(ctx, bson.M{
		"_id":        orderID,
		"booking_id": bookingID,
		"user_id":    userID,
		"provider":   "razorpay",
	}).Decode(&record)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Record{}, nil
	}
	return record, err
}

func (r *MongoRepository) UpdateOrder(ctx context.Context, orderID string, record Record) error {
	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": orderID, "booking_id": record.BookingID, "user_id": record.UserID},
		bson.M{"$set": bson.M{
			"transaction_reference": record.Transaction,
			"status":                record.Status,
			"refund_status":         record.RefundStatus,
			"payment_timestamp":     record.Timestamp,
			"currency":              record.Currency,
		}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("payment order %q was not found", orderID)
	}
	return nil
}
