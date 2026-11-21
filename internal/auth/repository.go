package auth

import (
	"context"
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrEmailExists = errors.New("email is already registered")

type MongoUserRepository struct {
	collection *mongo.Collection
}

func NewMongoUserRepository(database *mongo.Database) *MongoUserRepository {
	return &MongoUserRepository{collection: database.Collection("users")}
}

func (r *MongoUserRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

func (r *MongoUserRepository) Create(ctx context.Context, user User) (User, error) {
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))
	_, err := r.collection.InsertOne(ctx, user)
	if mongo.IsDuplicateKeyError(err) {
		return User{}, ErrEmailExists
	}
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (r *MongoUserRepository) FindByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := r.collection.FindOne(ctx, bson.M{"email": strings.ToLower(strings.TrimSpace(email))}).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return User{}, nil
	}
	return user, err
}

func (r *MongoUserRepository) FindByID(ctx context.Context, id string) (User, error) {
	var user User
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return User{}, nil
	}
	return user, err
}

func (r *MongoUserRepository) UpdateProfile(ctx context.Context, id string, profile Profile) error {
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"profile": profile}})
	return err
}
