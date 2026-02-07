package database

import (
	"database/sql"
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
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}
