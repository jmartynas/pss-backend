package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/go-sql-driver/mysql"
	mailjet "github.com/mailjet/mailjet-apiv3-go/v4"
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
		nats.Name("pss-mailer"),
		nats.MaxReconnects(-1),
		nats.RetryOnFailedConnect(true),
	)
	if err != nil {
		return fmt.Errorf("nats: %w", err)
	}
	defer nc.Drain()
	log.Info("nats connected")

	w := &worker{
		db:        db,
		mj:        mailjet.NewMailjetClient(cfg.mailjetAPIKey, cfg.mailjetSecretKey),
		fromEmail: cfg.fromEmail,
		fromName:  cfg.fromName,
		log:       log,
		trigger:   make(chan struct{}, 1),
	}

	sub, err := nc.Subscribe("email", func(_ *nats.Msg) {
		select {
		case w.trigger <- struct{}{}:
		default:
		}
	})
	if err != nil {
		return fmt.Errorf("nats subscribe: %w", err)
	}
	defer sub.Unsubscribe()
	log.Info("subscribed to nats subject")

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		w.run(ctx)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info("signal received", slog.String("signal", sig.String()))

	cancel()
	<-done
	log.Info("mailer stopped")
	return nil
}
