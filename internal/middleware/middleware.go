package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/jmartynas/pss-backend/internal/auth"
	"github.com/jmartynas/pss-backend/internal/session"
	"github.com/jmartynas/pss-backend/internal/user"

	"github.com/google/uuid"
)

type contextKey string

const (
	RequestIDKey    contextKey = "request_id"
	RealIPKey       contextKey = "real_ip"
	SessionClaimsKey contextKey = "session_claims"
	UserKey         contextKey = "user"
)

type responseWriter struct {
	http.ResponseWriter
	status int
	written int64
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.written += int64(n)
	return n, err
}

func (w *responseWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}
		ctx := context.WithValue(r.Context(), RequestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

func GetRealIP(ctx context.Context) string {
	if ip, ok := ctx.Value(RealIPKey).(string); ok {
		return ip
	}
	return ""
}

func RequireAuth(db *sql.DB, jwtSecret string, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := auth.GetSession(r, jwtSecret)
			if claims == nil {
				if log != nil {
					log.WarnContext(r.Context(), "auth failed: missing or invalid session",
						slog.String("request_id", GetRequestID(r.Context())),
						slog.String("path", r.URL.Path),
						slog.Any("error", err))
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				return
			}
			if claims.SessionID != uuid.Nil && db != nil {
				row, err := session.GetByToken(r.Context(), db, claims.SessionID.String())
				if err != nil || row == nil {
					if log != nil {
						log.WarnContext(r.Context(), "auth failed: session not found or expired",
							slog.String("request_id", GetRequestID(r.Context())),
							slog.String("path", r.URL.Path),
							slog.Any("error", err))
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
					return
				}
			}
			ctx := context.WithValue(r.Context(), SessionClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetSessionClaims(ctx context.Context) *auth.Claims {
	if c, ok := ctx.Value(SessionClaimsKey).(*auth.Claims); ok {
		return c
	}
	return nil
}

// Authorize ensures the request has a valid session before allowing access.
// Validates the JWT cookie, verifies the session exists in the database when applicable,
// fetches the user from the database, and puts the user in context for downstream handlers.
func Authorize(db *sql.DB, jwtSecret string, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := auth.GetSession(r, jwtSecret)
			if claims == nil {
				if log != nil {
					log.WarnContext(r.Context(), "auth failed: missing or invalid session",
						slog.String("request_id", GetRequestID(r.Context())),
						slog.String("path", r.URL.Path),
						slog.Any("error", err))
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				return
			}
			if claims.SessionID != uuid.Nil && db != nil {
				row, err := session.GetByToken(r.Context(), db, claims.SessionID.String())
				if err != nil || row == nil {
					if log != nil {
						log.WarnContext(r.Context(), "auth failed: session not found or expired",
							slog.String("request_id", GetRequestID(r.Context())),
							slog.String("path", r.URL.Path),
							slog.Any("error", err))
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
					return
				}
			}
			if claims.UserID == uuid.Nil || db == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				return
			}
			u, err := user.GetByID(r.Context(), db, claims.UserID)
			if err != nil || u == nil {
				if log != nil {
					log.WarnContext(r.Context(), "auth failed: user not found",
						slog.String("request_id", GetRequestID(r.Context())),
						slog.String("path", r.URL.Path),
						slog.String("user_id", claims.UserID.String()),
						slog.Any("error", err))
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				return
			}
			ctx := context.WithValue(r.Context(), UserKey, u)
			ctx = context.WithValue(ctx, SessionClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUser returns the authenticated user from the request context.
// Returns nil if the request was not authorized or no user was set.
func GetUser(ctx context.Context) *user.User {
	if u, ok := ctx.Value(UserKey).(*user.User); ok {
		return u
	}
	return nil
}

func Logger(log *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(ww, r)
			log.InfoContext(r.Context(), "request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.Status()),
				slog.Duration("duration", time.Since(start)),
				slog.String("request_id", GetRequestID(r.Context())),
				slog.String("client_ip", GetRealIP(r.Context())),
			)
		})
	}
}

func Recoverer(log *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.ErrorContext(r.Context(), "panic recovered",
						slog.Any("error", err),
						slog.String("stack", string(debug.Stack())),
					)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func Timeout(seconds int) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), time.Duration(seconds)*time.Second)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RealIPWith(trustedNetworks []*net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ""

			remoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)
			if isTrustedProxy(remoteIP, trustedNetworks) {
				if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
					ip = strings.TrimSpace(xrip)
				} else if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
					ip = strings.TrimSpace(strings.Split(xff, ",")[0])
				}
			}

			if ip == "" {
				ip = remoteIP
			}

			ctx := context.WithValue(r.Context(), RealIPKey, ip)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ParseTrustedProxyCIDRs parses a comma-separated list of CIDRs. Returns an error if any entry is invalid.
func ParseTrustedProxyCIDRs(csv string) ([]*net.IPNet, error) {
	csv = strings.TrimSpace(csv)
	if csv == "" {
		return nil, nil
	}
	var out []*net.IPNet
	for _, s := range strings.Split(csv, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy CIDR %q: %w", s, err)
		}
		out = append(out, n)
	}
	return out, nil
}

func isTrustedProxy(remoteIP string, networks []*net.IPNet) bool {
	if len(networks) == 0 {
		return false
	}
	ip := net.ParseIP(remoteIP)
	if ip == nil {
		return false
	}
	for _, n := range networks {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func NoCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}
