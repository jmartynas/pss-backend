# ── Stage 1: build ────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Download dependencies first (cached layer)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a static binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /server ./cmd/server

# ── Stage 2: runtime ──────────────────────────────────────────────────────────
FROM alpine:3.21

# ca-certificates for TLS/OAuth outbound calls; tzdata for correct time zones
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /server /server

EXPOSE 8000

# Run as non-root
USER nobody

ENTRYPOINT ["/server"]
