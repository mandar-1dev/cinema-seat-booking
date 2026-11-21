package demo

import "time"

type FoodItem struct {
	ID       string `json:"id" bson:"id"`
	Name     string `json:"name" bson:"name"`
	Quantity int    `json:"quantity" bson:"quantity"`
	UnitCost int    `json:"unit_cost" bson:"unit_cost"`
}

type Payment struct {
	ID           string     `json:"payment_id" bson:"_id"`
	BookingID    string     `json:"booking_id" bson:"booking_id"`
	OwnerID      string     `json:"-" bson:"owner_id"`
	CustomerName string     `json:"customer_name" bson:"customer_name"`
	MovieID      string     `json:"movie_id" bson:"movie_id"`
	MovieTitle   string     `json:"movie_title" bson:"movie_title"`
	SeatID       string     `json:"seat_id" bson:"seat_id"`
	ShowDate     string     `json:"show_date" bson:"show_date"`
	ShowTime     string     `json:"show_time" bson:"show_time"`
	Cinema       string     `json:"cinema" bson:"cinema"`
	Screen       string     `json:"screen" bson:"screen"`
	FoodItems    []FoodItem `json:"food_items" bson:"food_items"`
	Amount       int        `json:"amount" bson:"amount"`
	Currency     string     `json:"currency" bson:"currency"`
	Status       string     `json:"status" bson:"status"`
	Timestamp    time.Time  `json:"payment_timestamp" bson:"payment_timestamp"`
}

type Pass struct {
	ID           string     `json:"id" bson:"_id"`
	BookingID    string     `json:"booking_id" bson:"booking_id"`
	PaymentID    string     `json:"payment_reference" bson:"payment_reference"`
	OwnerID      string     `json:"-" bson:"owner_id"`
	CustomerName string     `json:"customer_name" bson:"customer_name"`
	MovieID      string     `json:"movie_id" bson:"movie_id"`
	MovieTitle   string     `json:"movie_title" bson:"movie_title"`
	SeatID       string     `json:"seat_id" bson:"seat_id"`
	ShowDate     string     `json:"show_date" bson:"show_date"`
	ShowTime     string     `json:"show_time" bson:"show_time"`
	Cinema       string     `json:"cinema" bson:"cinema"`
	Screen       string     `json:"screen" bson:"screen"`
	FoodItems    []FoodItem `json:"food_items" bson:"food_items"`
	Amount       int        `json:"amount" bson:"amount"`
	Currency     string     `json:"currency" bson:"currency"`
	Mode         string     `json:"mode" bson:"mode"`
	Status       string     `json:"status" bson:"status"`
	IssuedAt     time.Time  `json:"issued_at" bson:"issued_at"`
}

type CheckoutRequest struct {
	SessionID    string     `json:"session_id"`
	OwnerID      string     `json:"owner_id"`
	CustomerName string     `json:"customer_name"`
	MovieID      string     `json:"movie_id"`
	ShowDate     string     `json:"show_date"`
	FoodItems    []FoodItem `json:"food_items"`
}
