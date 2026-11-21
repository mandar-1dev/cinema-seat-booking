package catalog

import "time"

type Movie struct {
	ID          string   `json:"id" bson:"_id"`
	Name        string   `json:"name" bson:"name"`
	Description string   `json:"description" bson:"description"`
	Genres      []string `json:"genres" bson:"genres"`
	Language    string   `json:"language" bson:"language"`
	DurationMin int      `json:"duration_min" bson:"duration_min"`
	ReleaseDate string   `json:"release_date" bson:"release_date"`
	PosterURL   string   `json:"poster_url" bson:"poster_url"`
	TrailerURL  string   `json:"trailer_url" bson:"trailer_url"`
	Cast        []string `json:"cast" bson:"cast"`
	Rating      float64  `json:"rating" bson:"rating"`
	Status      string   `json:"status" bson:"status"`
}

type Theatre struct {
	ID            string   `json:"id" bson:"_id"`
	Name          string   `json:"name" bson:"name"`
	Address       string   `json:"address" bson:"address"`
	City          string   `json:"city" bson:"city"`
	Screens       []Screen `json:"screens" bson:"screens"`
	Facilities    []string `json:"facilities" bson:"facilities"`
	Accessibility []string `json:"accessibility" bson:"accessibility"`
}

type Screen struct {
	ID                string         `json:"id" bson:"id"`
	Name              string         `json:"name" bson:"name"`
	Capacity          int            `json:"capacity" bson:"capacity"`
	SeatConfiguration map[string]int `json:"seat_configuration" bson:"seat_configuration"`
}

type Show struct {
	ID               string             `json:"id" bson:"_id"`
	MovieID          string             `json:"movie_id" bson:"movie_id"`
	TheatreID        string             `json:"theatre_id" bson:"theatre_id"`
	ScreenID         string             `json:"screen_id" bson:"screen_id"`
	ShowDate         string             `json:"show_date" bson:"show_date"`
	StartTime        string             `json:"start_time" bson:"start_time"`
	EndTime          string             `json:"end_time" bson:"end_time"`
	Pricing          map[string]float64 `json:"pricing" bson:"pricing"`
	SeatAvailability map[string]string  `json:"seat_availability" bson:"seat_availability"`
	Status           string             `json:"status" bson:"status"`
	CreatedAt        time.Time          `json:"created_at" bson:"created_at"`
}
