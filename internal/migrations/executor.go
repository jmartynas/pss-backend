package migrations

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func Run(db *sql.DB, fsys fs.FS, dir string, log *slog.Logger) error {
	source, err := iofs.New(fsys, dir)
	if err != nil {
		return err
	}

	driver, err := mysql.WithInstance(db, &mysql.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance("iofs", source, "mysql", driver)
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()

	if log != nil {
		m.Log = &migrateLogger{log: log}
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

type migrateLogger struct{ log *slog.Logger }

func (l *migrateLogger) Printf(format string, v ...interface{}) {
	l.log.Info(fmt.Sprintf(format, v...))
}

func (l *migrateLogger) Verbose() bool { return false }
