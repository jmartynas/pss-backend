# Load .env into environment for all targets (only affects commands run by make).
# Copy .env.example to .env and edit. Variables are exported for this make process only.
-include .env
export

.PHONY: run build tidy test up down logs db

# ── Local dev ──────────────────────────────────────────────────────────────────
run: build
	./bin/server

build:
	go build -o bin/server ./cmd/server

tidy:
	go mod tidy

test:
	go test ./...

# ── Docker Compose ─────────────────────────────────────────────────────────────
up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

# Start only the database (useful for local backend development)
db:
	docker compose up -d mysql
