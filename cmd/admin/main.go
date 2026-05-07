package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/nats-io/nats.go"
)

func main() {
	if err := run(); err != nil {
		slog.Default().Error("fatal", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	db, err := sql.Open("mysql", cfg.mysqlDSN)
	if err != nil {
		return fmt.Errorf("mysql open: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("mysql ping: %w", err)
	}

	log.Info("mysql connected")

	nc, err := nats.Connect(cfg.natsURL,
		nats.Name("pss-admin"),
		nats.MaxReconnects(-1),
		nats.RetryOnFailedConnect(true),
	)

	if err != nil {
		return fmt.Errorf("nats: %w", err)
	}
	defer nc.Drain()

	log.Info("nats connected")

	h := &handler{db: db, nc: nc, jwtSecret: cfg.jwtSecret, log: log}

	if err := h.seedAdmin(cfg.seedEmail, cfg.seedPassword); err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}

	srv := &http.Server{
		Addr:         ":" + cfg.port,
		Handler:      h.routes(cfg.corsOrigins),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	srvErr := make(chan error, 1)
	go func() { srvErr <- srv.ListenAndServe() }()
	log.Info("admin server starting", slog.String("addr", srv.Addr))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Info("signal received", slog.String("signal", sig.String()))
	case err := <-srvErr:
		return fmt.Errorf("server: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	if err := <-srvErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server: %w", err)
	}

	log.Info("admin server stopped")
	return nil
}
