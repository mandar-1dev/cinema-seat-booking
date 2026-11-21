package payment

import "time"

type OrderRequest struct {
	BookingID string  `json:"booking_id"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
}

type Record struct {
	ID           string    `json:"id" bson:"_id"`
	BookingID    string    `json:"booking_id" bson:"booking_id"`
	UserID       string    `json:"user_id" bson:"user_id"`
	Provider     string    `json:"provider" bson:"provider"`
	Transaction  string    `json:"transaction_reference" bson:"transaction_reference"`
	Amount       float64   `json:"amount" bson:"amount"`
	Currency     string    `json:"currency" bson:"currency"`
	Status       string    `json:"status" bson:"status"`
	RefundStatus string    `json:"refund_status" bson:"refund_status"`
	Timestamp    time.Time `json:"payment_timestamp" bson:"payment_timestamp"`
}
