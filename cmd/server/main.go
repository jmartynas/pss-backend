package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmartynas/pss-backend/internal/config"
	"github.com/jmartynas/pss-backend/internal/database"
	"github.com/jmartynas/pss-backend/internal/migrations"
	"github.com/jmartynas/pss-backend/internal/server"
	"github.com/nats-io/nats.go"
)

//go:embed migrations
var migrationFS embed.FS

func main() {
	if err := run(); err != nil {
		slog.Default().Error("fatal", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	if err := cfg.Validate(true, true); err != nil {
		return err
	}

	log := newLogger(cfg.LogLevel)

	db, err := database.Open(cfg.MySQL)
	if err != nil {
		return fmt.Errorf("mysql: %w", err)
	}
	defer db.Close()
	log.Info("mysql connected")

	if err := migrations.Run(db, migrationFS, "migrations", log); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}

	nc, err := nats.Connect(cfg.NatsURL,
		nats.Name("pss-backend"),
		nats.MaxReconnects(-1),
		nats.RetryOnFailedConnect(true),
	)
	if err != nil {
		return fmt.Errorf("nats: %w", err)
	}
	defer nc.Drain()
	log.Info("nats connected")

	srv := server.New(cfg, log, db, nc)

	srvErr := make(chan error, 1)
	go func() { srvErr <- srv.Start() }()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Info("signal received", slog.String("signal", sig.String()))
	case err := <-srvErr:
		return fmt.Errorf("server: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	if err := <-srvErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server: %w", err)
	}

	log.Info("server stopped")
	return nil
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
