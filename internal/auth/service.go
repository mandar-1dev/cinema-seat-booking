package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrInvalidRegistration = errors.New("full_name, email and password are required")
	ErrWeakPassword        = errors.New("password must be at least 8 characters")
	ErrInvalidToken        = errors.New("invalid or expired token")
)

type Claims struct {
	UserID string `json:"uid"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type Repository interface {
	Create(context.Context, User) (User, error)
	FindByEmail(context.Context, string) (User, error)
	FindByID(context.Context, string) (User, error)
	UpdateProfile(context.Context, string, Profile) error
}

type Service struct {
	repo      Repository
	secret    []byte
	accessTTL time.Duration
}

func NewService(repo Repository, secret string) *Service {
	return &Service{
		repo:      repo,
		secret:    []byte(secret),
		accessTTL: 15 * time.Minute,
	}
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (TokenResponse, error) {
	req.FullName = strings.TrimSpace(req.FullName)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Password = req.Password
	if req.FullName == "" || req.Email == "" || req.Password == "" {
		return TokenResponse{}, ErrInvalidRegistration
	}
	if len(req.Password) < 8 {
		return TokenResponse{}, ErrWeakPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return TokenResponse{}, err
	}
	user := User{
		ID:               uuid.NewString(),
		FullName:         req.FullName,
		Email:            req.Email,
		Phone:            strings.TrimSpace(req.Phone),
		PasswordHash:     string(hash),
		Role:             "customer",
		MembershipLevel:  "Silver",
		AccountCreatedAt: time.Now().UTC(),
	}
	user, err = s.repo.Create(ctx, user)
	if err != nil {
		return TokenResponse{}, err
	}
	return s.issueTokens(user)
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (TokenResponse, error) {
	user, err := s.repo.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		return TokenResponse{}, err
	}
	if user.ID == "" || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return TokenResponse{}, ErrInvalidCredentials
	}
	return s.issueTokens(user)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenResponse, error) {
	claims, err := s.parseToken(refreshToken)
	if err != nil || claims.Subject != "refresh" {
		return TokenResponse{}, ErrInvalidToken
	}
	user, err := s.repo.FindByID(ctx, claims.UserID)
	if err != nil || user.ID == "" {
		return TokenResponse{}, ErrInvalidToken
	}
	return s.issueTokens(user)
}

func (s *Service) Profile(ctx context.Context, userID string) (User, error) {
	return s.repo.FindByID(ctx, userID)
}

func (s *Service) UpdateProfile(ctx context.Context, userID string, profile Profile) (User, error) {
	if err := s.repo.UpdateProfile(ctx, userID, profile); err != nil {
		return User{}, err
	}
	return s.repo.FindByID(ctx, userID)
}

func (s *Service) ValidateAccess(tokenString string) (*Claims, error) {
	claims, err := s.parseToken(tokenString)
	if err != nil || claims.Subject != "access" {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (s *Service) issueTokens(user User) (TokenResponse, error) {
	now := time.Now().UTC()
	accessClaims := Claims{
		UserID: user.ID,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "access",
			Issuer:    "cinema-booking",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
		},
	}
	access, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString(s.secret)
	if err != nil {
		return TokenResponse{}, err
	}

	refreshClaims := Claims{
		UserID: user.ID,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        randomTokenID(),
			Subject:   "refresh",
			Issuer:    "cinema-booking",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
		},
	}
	refresh, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(s.secret)
	if err != nil {
		return TokenResponse{}, err
	}

	return TokenResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(s.accessTTL.Seconds()),
		User:         user,
	}, nil
}

func (s *Service) parseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return s.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func randomTokenID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return uuid.NewString()
	}
	return hex.EncodeToString(b)
}
