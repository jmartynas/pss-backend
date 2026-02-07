package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jmartynas/pss-backend/internal/config"
	"github.com/jmartynas/pss-backend/internal/errs"

	_ "github.com/go-sql-driver/mysql"
)

func Open(cfg config.MySQLConfig) (*sql.DB, error) {
	dsn := cfg.DSN()
	if dsn == "" {
		return nil, errs.ErrDSNNotConfigured
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	maxOpen := cfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 25
	}
	db.SetMaxOpenConns(maxOpen)
	maxIdle := cfg.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 5
	}
	db.SetMaxIdleConns(maxIdle)
	connLife := time.Duration(cfg.ConnMaxLifetimeSec) * time.Second
	if connLife <= 0 {
		connLife = 5 * time.Minute
	}
	db.SetConnMaxLifetime(connLife)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	return db, nil
}
