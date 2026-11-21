package config

import (
	"errors"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPPort          string
	RedisURL          string
	MongoDBURI        string
	MongoDBDatabase   string
	JWTSecret         string
	RazorpayKeyID     string
	RazorpayKeySecret string
}

func Load() (Config, error) {
	// .env is optional for process-managed deployments, but loading it locally
	// makes the documented setup work without a separate shell configuration.
	_ = godotenv.Load(".env")
	cfg := Config{
		HTTPPort:          envOrDefault("HTTP_PORT", "8080"),
		RedisURL:          envOrDefault("REDIS_URL", "localhost:6379"),
		MongoDBURI:        strings.TrimSpace(os.Getenv("MONGODB_URI")),
		MongoDBDatabase:   envOrDefault("MONGODB_DATABASE", "cinema_booking"),
		JWTSecret:         strings.TrimSpace(os.Getenv("JWT_SECRET")),
		RazorpayKeyID:     strings.TrimSpace(os.Getenv("RAZORPAY_KEY_ID")),
		RazorpayKeySecret: strings.TrimSpace(os.Getenv("RAZORPAY_KEY_SECRET")),
	}
	if strings.TrimSpace(cfg.HTTPPort) == "" {
		return Config{}, errors.New("HTTP_PORT cannot be empty")
	}
	if strings.TrimSpace(cfg.RedisURL) == "" {
		return Config{}, errors.New("REDIS_URL cannot be empty")
	}
	if strings.TrimSpace(cfg.MongoDBDatabase) == "" {
		return Config{}, errors.New("MONGODB_DATABASE cannot be empty")
	}
	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
