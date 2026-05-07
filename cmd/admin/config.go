package main

import (
	"errors"
	"os"
)

type config struct {
	mysqlDSN     string
	jwtSecret    []byte
	port         string
	corsOrigins  string
	natsURL      string
	seedEmail    string
	seedPassword string
}

func loadConfig() (*config, error) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		return nil, errors.New("MYSQL_DSN is required")
	}

	secret := os.Getenv("ADMIN_JWT_SECRET")
	if secret == "" {
		return nil, errors.New("ADMIN_JWT_SECRET is required")
	}

	if len(secret) < 32 {
		return nil, errors.New("ADMIN_JWT_SECRET must be at least 32 characters")
	}

	return &config{
		mysqlDSN:     dsn,
		jwtSecret:    []byte(secret),
		port:         getEnv("ADMIN_PORT", "8001"),
		corsOrigins:  getEnv("ADMIN_CORS_ORIGINS", "http://localhost:3001"),
		natsURL:      getEnv("NATS_URL", "nats://localhost:4222"),
		seedEmail:    getEnv("SEED_ADMIN_EMAIL", ""),
		seedPassword: getEnv("SEED_ADMIN_PASSWORD", ""),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
