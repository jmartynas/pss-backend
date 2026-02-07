package errs

import (
	"database/sql"
	"errors"
)

var (
	ErrNotFound        = sql.ErrNoRows
	ErrForbidden       = errors.New("forbidden")
	ErrConflict        = errors.New("conflict")
	ErrRouteFull       = errors.New("route is full")
	ErrAlreadyApplied  = errors.New("already applied to this route")
	ErrRouteStarted    = errors.New("route has already started")

	ErrJWTSecretRequired = errors.New("auth: JWT secret is required")
	ErrDSNNotConfigured  = errors.New("mysql: DSN not configured (set MYSQL_DSN or MYSQL_HOST)")
	ErrInvalidSession    = errors.New("auth: invalid or missing session")
)
