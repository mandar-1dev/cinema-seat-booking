package auth

import "time"

type User struct {
	ID               string    `json:"id" bson:"_id"`
	FullName         string    `json:"full_name" bson:"full_name"`
	Email            string    `json:"email" bson:"email"`
	Phone            string    `json:"phone" bson:"phone"`
	PasswordHash     string    `json:"-" bson:"password_hash"`
	Role             string    `json:"role" bson:"role"`
	Profile          Profile   `json:"profile" bson:"profile"`
	BookingHistory   []string  `json:"booking_history,omitempty" bson:"booking_history,omitempty"`
	LoyaltyPoints    int       `json:"loyalty_points" bson:"loyalty_points"`
	MembershipLevel  string    `json:"membership_level" bson:"membership_level"`
	AccountCreatedAt time.Time `json:"account_created_at" bson:"account_created_at"`
}

type Profile struct {
	AvatarURL string `json:"avatar_url" bson:"avatar_url"`
	City      string `json:"city" bson:"city"`
	Birthday  string `json:"birthday" bson:"birthday"`
}

type RegisterRequest struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	User         User   `json:"user"`
}
