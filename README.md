# pss-backend

Production-ready Go HTTP server template with graceful shutdown, structured logging, and health checks.

## Features

- **Graceful shutdown** – Handles SIGINT/SIGTERM with configurable timeout
- **Structured logging** – JSON logs via `log/slog`, configurable level
- **Middleware** – Request ID, request logging, panic recovery, real IP, gzip
- **Health endpoint** – `/health` (liveness)
- **Configuration** – Environment-based (see `.env.example`)

## Quick start

```bash
# Install dependencies
go mod tidy

# Copy env template and edit with your values
cp .env.example .env

# Run with env from .env (Make loads .env only for the go process)
make run

# Or run without make (uses current shell env only)
go run ./cmd/server
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | HTTP listen port |
| `SERVER_READ_TIMEOUT` | 15 | Read timeout (seconds) |
| `SERVER_WRITE_TIMEOUT` | 15 | Write timeout (seconds) |
| `SERVER_IDLE_TIMEOUT` | 60 | Idle timeout (seconds) |
| `SERVER_SHUTDOWN_TIMEOUT` | 30 | Graceful shutdown timeout (seconds) |
| `TLS_CERT_FILE` | (none) | Path to TLS certificate file; HTTPS if both cert and key set |
| `TLS_KEY_FILE` | (none) | Path to TLS private key file |
| `TRUSTED_PROXY_CIDRS` | 127.0.0.0/8,10.0.0.0/8,... | Comma-separated CIDRs for trusted proxies (RealIP) |
| `MYSQL_DSN` | (none) | Full MySQL DSN; if set, other MYSQL_* are ignored |
| `MYSQL_HOST` | (none) | MySQL host (empty = no DB connection) |
| `MYSQL_PORT` | 3306 | MySQL port |
| `MYSQL_USER` | root | MySQL user |
| `MYSQL_PASSWORD` | (none) | MySQL password |
| `MYSQL_DATABASE` | pss | MySQL database name |
| `LOG_LEVEL` | info | Log level: debug, info, warn, error |
| `OAUTH_BASE_URL` | (none) | Public base URL for OAuth redirect_uri (e.g. https://api.example.com) |
| `OAUTH_JWT_SECRET` | (none) | Secret to sign session JWTs |
| `OAUTH_SUCCESS_URL` | / | Redirect after successful login |
| `OAUTH_<PROVIDER>_CLIENT_ID` | (none) | OAuth client ID for provider (google, github, microsoft) |
| `OAUTH_<PROVIDER>_CLIENT_SECRET` | (none) | OAuth client secret for provider |

## OAuth login

When `OAUTH_BASE_URL`, `OAUTH_JWT_SECRET`, and at least one provider’s `OAUTH_<NAME>_CLIENT_ID` and `OAUTH_<NAME>_CLIENT_SECRET` are set, the server exposes:

- `GET /auth/{provider}/login` – redirects to the provider’s consent screen (e.g. `/auth/google/login`)
- `GET /auth/{provider}/callback` – OAuth callback (set by redirect_uri)
- `GET /auth/logout` – clears the session cookie

**Adding a provider:** Edit `internal/auth/providers.go` and add an entry to `Registry` (AuthURL, TokenURL, UserInfoURL, Scopes). Set `OAUTH_<NAME>_CLIENT_ID` and `OAUTH_<NAME>_CLIENT_SECRET` (name lowercase, e.g. `OAUTH_GITHUB_CLIENT_ID`). No other code changes needed.

Built-in specs: **google**, **github**, **microsoft**.

## Endpoints

- `GET /health` – Liveness (returns 200)
- `GET /auth/{provider}/login` – Start OAuth login (when OAuth is configured)
- `GET /auth/{provider}/callback` – OAuth callback
- `GET /auth/logout` – Clear session

## Migrations

Migrations run automatically on startup using [golang-migrate](https://github.com/golang-migrate/migrate). Add pairs under `migrations/`: `NNNNNN_name.up.sql` and `NNNNNN_name.down.sql` (e.g. `000001_initial.up.sql`, `000001_initial.down.sql`). Only pending up migrations are applied; state is stored in `schema_migrations`.

## Project layout

```
cmd/server/           # Application entrypoint
  migrations/        # SQL migration files (embedded)
internal/
  auth/              # OAuth (provider registry, session, types)
  config/            # Configuration from env
  database/          # MySQL connection
  handlers/          # HTTP handlers (health, OAuth login/callback/logout)
  migrations/        # Migration executor
  middleware/        # HTTP middleware
  server/            # Server setup and routing
```

## Build for production

```bash
go build -o bin/server ./cmd/server
./bin/server
```
