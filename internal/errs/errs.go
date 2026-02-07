package errs

import (
	"database/sql"
	"errors"
)

var (
	ErrNotFound = sql.ErrNoRows
	ErrJWTSecretRequired = errors.New("auth: JWT secret is required")
	ErrDSNNotConfigured = errors.New("mysql: DSN not configured (set MYSQL_DSN or MYSQL_HOST)")
	ErrInvalidSession = errors.New("auth: invalid or missing session")
)
