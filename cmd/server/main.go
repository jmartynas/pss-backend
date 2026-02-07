package main

import (
	"context"
	"embed"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmartynas/pss-backend/internal/config"
	"github.com/jmartynas/pss-backend/internal/database"
	"github.com/jmartynas/pss-backend/internal/migrations"
	"github.com/jmartynas/pss-backend/internal/server"
)

//go:embed migrations
var migrationFS embed.FS

func main() {
	cfg := config.Load()
	log := newLogger(cfg.LogLevel)

	db, err := database.Open(cfg.MySQL)
	if err != nil {
		log.Error("mysql connection failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer db.Close()
	log.Info("mysql connected")

	if err := migrations.Run(db, migrationFS, "migrations", log); err != nil {
		log.Error("migrations failed", slog.Any("error", err))
		os.Exit(1)
	}

	srv := server.New(cfg, log, db)

	go func() {
		if err := srv.Start(); err != nil && err != context.Canceled {
			log.Error("server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown error", slog.Any("error", err))
		os.Exit(1)
	}
	log.Info("server stopped")
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
