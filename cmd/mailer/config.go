package main

import (
	"errors"
	"os"
)

type config struct {
	mysqlDSN         string
	natsURL          string
	mailjetAPIKey    string
	mailjetSecretKey string
	fromEmail        string
	fromName         string
}

func loadConfig() (*config, error) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		return nil, errors.New("MYSQL_DSN is required")
	}

	mjKey := os.Getenv("MAILJET_API_KEY")
	if mjKey == "" {
		return nil, errors.New("MAILJET_API_KEY is required")
	}

	mjSecret := os.Getenv("MAILJET_SECRET_KEY")
	if mjSecret == "" {
		return nil, errors.New("MAILJET_SECRET_KEY is required")
	}

	fromEmail := os.Getenv("FROM_EMAIL")
	if fromEmail == "" {
		return nil, errors.New("FROM_EMAIL is required")
	}

	return &config{
		mysqlDSN:         dsn,
		natsURL:          getEnv("NATS_URL", "nats://localhost:4222"),
		mailjetAPIKey:    mjKey,
		mailjetSecretKey: mjSecret,
		fromEmail:        fromEmail,
		fromName:         getEnv("FROM_NAME", "PSS"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
