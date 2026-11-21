package catalog

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoRepository struct {
	movies   *mongo.Collection
	theatres *mongo.Collection
	shows    *mongo.Collection
}

func NewMongoRepository(database *mongo.Database) *MongoRepository {
	return &MongoRepository{
		movies:   database.Collection("movies"),
		theatres: database.Collection("theatres"),
		shows:    database.Collection("shows"),
	}
}

func (r *MongoRepository) EnsureIndexes(ctx context.Context) error {
	indexes := []struct {
		collection *mongo.Collection
		models     []mongo.IndexModel
	}{
		{r.movies, []mongo.IndexModel{
			{Keys: bson.D{{Key: "status", Value: 1}, {Key: "release_date", Value: -1}}},
		}},
		{r.theatres, []mongo.IndexModel{
			{Keys: bson.D{{Key: "city", Value: 1}}},
		}},
		{r.shows, []mongo.IndexModel{
			{Keys: bson.D{{Key: "movie_id", Value: 1}, {Key: "show_date", Value: 1}}},
			{Keys: bson.D{{Key: "theatre_id", Value: 1}, {Key: "show_date", Value: 1}}},
		}},
	}
	for _, item := range indexes {
		if _, err := item.collection.Indexes().CreateMany(ctx, item.models); err != nil {
			return err
		}
	}
	return nil
}

func (r *MongoRepository) ListMovies(ctx context.Context) ([]Movie, error) {
	cursor, err := r.movies.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "release_date", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var result []Movie
	if err := cursor.All(ctx, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *MongoRepository) ListTheatres(ctx context.Context, city string) ([]Theatre, error) {
	filter := bson.M{}
	if city != "" {
		filter["city"] = city
	}
	cursor, err := r.theatres.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var result []Theatre
	if err := cursor.All(ctx, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *MongoRepository) ListShows(ctx context.Context, movieID, theatreID, date string) ([]Show, error) {
	filter := bson.M{}
	if movieID != "" {
		filter["movie_id"] = movieID
	}
	if theatreID != "" {
		filter["theatre_id"] = theatreID
	}
	if date != "" {
		filter["show_date"] = date
	}
	cursor, err := r.shows.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "start_time", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var result []Show
	if err := cursor.All(ctx, &result); err != nil {
		return nil, err
	}
	return result, nil
}
