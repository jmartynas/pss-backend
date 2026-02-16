# Load .env into environment for all targets (only affects commands run by make).
# Copy .env.example to .env and edit. Variables are exported for this make process only.
-include .env
export

.PHONY: run build tidy test

run: build
	./bin/server

build:
	go build -o bin/server ./cmd/server

tidy:
	go mod tidy

test:
	go test ./...
