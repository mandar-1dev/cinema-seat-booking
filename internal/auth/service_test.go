package auth

import (
	"context"
	"testing"
)

type memoryUserRepository struct {
	users map[string]User
}

func newMemoryUserRepository() *memoryUserRepository {
	return &memoryUserRepository{users: make(map[string]User)}
}

func (r *memoryUserRepository) Create(ctx context.Context, user User) (User, error) {
	for _, existing := range r.users {
		if existing.Email == user.Email {
			return User{}, ErrEmailExists
		}
	}
	r.users[user.ID] = user
	return user, nil
}

func (r *memoryUserRepository) FindByEmail(ctx context.Context, email string) (User, error) {
	for _, user := range r.users {
		if user.Email == email {
			return user, nil
		}
	}
	return User{}, nil
}

func (r *memoryUserRepository) FindByID(ctx context.Context, id string) (User, error) {
	return r.users[id], nil
}

func (r *memoryUserRepository) UpdateProfile(ctx context.Context, id string, profile Profile) error {
	user := r.users[id]
	user.Profile = profile
	r.users[id] = user
	return nil
}

func TestRegisterLoginAndRefresh(t *testing.T) {
	service := NewService(newMemoryUserRepository(), "test-secret-long-enough")
	ctx := context.Background()

	registered, err := service.Register(ctx, RegisterRequest{
		FullName: "Test User",
		Email:    "test@example.com",
		Phone:    "123",
		Password: "password123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if registered.AccessToken == "" || registered.RefreshToken == "" {
		t.Fatal("tokens were not issued")
	}

	loggedIn, err := service.Login(ctx, LoginRequest{
		Email:    "test@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err := service.Refresh(ctx, loggedIn.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.User.ID != registered.User.ID {
		t.Fatalf("expected user %q, got %q", registered.User.ID, refreshed.User.ID)
	}

	if _, err := service.Login(ctx, LoginRequest{
		Email:    "test@example.com",
		Password: "wrong-password",
	}); err != ErrInvalidCredentials {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}
