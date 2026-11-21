package demo

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository interface {
	FindPaymentByBooking(context.Context, string) (Payment, error)
	SavePayment(context.Context, Payment) error
	FindPass(context.Context, string) (Pass, error)
	FindPassByBooking(context.Context, string) (Pass, error)
	SavePass(context.Context, Pass) error
}

type MongoRepository struct {
	payments *mongo.Collection
	passes   *mongo.Collection
}

func NewMongoRepository(database *mongo.Database) *MongoRepository {
	return &MongoRepository{
		payments: database.Collection("demo_payments"),
		passes:   database.Collection("demo_passes"),
	}
}

func (r *MongoRepository) EnsureIndexes(ctx context.Context) error {
	if _, err := r.payments.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "booking_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "owner_id", Value: 1}, {Key: "payment_timestamp", Value: -1}}},
	}); err != nil {
		return err
	}
	_, err := r.passes.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "booking_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "owner_id", Value: 1}}},
	})
	return err
}

func (r *MongoRepository) FindPaymentByBooking(ctx context.Context, bookingID string) (Payment, error) {
	var payment Payment
	err := r.payments.FindOne(ctx, bson.M{"booking_id": bookingID}).Decode(&payment)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Payment{}, nil
	}
	return payment, err
}

func (r *MongoRepository) SavePayment(ctx context.Context, payment Payment) error {
	_, err := r.payments.InsertOne(ctx, payment)
	return err
}

func (r *MongoRepository) FindPass(ctx context.Context, id string) (Pass, error) {
	var pass Pass
	err := r.passes.FindOne(ctx, bson.M{"_id": id}).Decode(&pass)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Pass{}, nil
	}
	return pass, err
}

func (r *MongoRepository) FindPassByBooking(ctx context.Context, bookingID string) (Pass, error) {
	var pass Pass
	err := r.passes.FindOne(ctx, bson.M{"booking_id": bookingID}).Decode(&pass)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Pass{}, nil
	}
	return pass, err
}

func (r *MongoRepository) SavePass(ctx context.Context, pass Pass) error {
	_, err := r.passes.InsertOne(ctx, pass)
	return err
}
