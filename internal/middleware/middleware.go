package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/auth"
	"github.com/jmartynas/pss-backend/internal/domain"
)

type contextKey string

const (
	RequestIDKey     contextKey = "request_id"
	RealIPKey        contextKey = "real_ip"
	SessionClaimsKey contextKey = "session_claims"
	UserKey          contextKey = "user"
)

type responseWriter struct {
	http.ResponseWriter
	status  int
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

func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *responseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
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

func RequireAuth(sessions domain.SessionRepository, jwtSecret string, log *slog.Logger) func(http.Handler) http.Handler {
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
			if claims.SessionID != uuid.Nil && sessions != nil {
				if _, err := sessions.GetByToken(r.Context(), claims.SessionID.String()); err != nil {
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

func Authorize(sessions domain.SessionRepository, users domain.UserRepository, jwtSecret string, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			deny := func() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			}

			claims, err := auth.GetSession(r, jwtSecret)
			if claims == nil {
				if log != nil {
					log.WarnContext(r.Context(), "auth failed: missing or invalid session",
						slog.String("request_id", GetRequestID(r.Context())),
						slog.String("path", r.URL.Path),
						slog.Any("error", err))
				}
				deny()

				return
			}

			if claims.SessionID != uuid.Nil && sessions != nil {
				if _, err := sessions.GetByToken(r.Context(), claims.SessionID.String()); err != nil {
					if log != nil {
						log.WarnContext(r.Context(), "auth failed: session not found or expired",
							slog.String("request_id", GetRequestID(r.Context())),
							slog.String("path", r.URL.Path),
							slog.Any("error", err))
					}
					deny()

					return
				}
			}

			if claims.UserID == uuid.Nil || users == nil {
				deny()
				return
			}
			u, err := users.GetByID(r.Context(), claims.UserID)
			if err != nil {
				if log != nil {
					log.WarnContext(r.Context(), "auth failed: user not found",
						slog.String("request_id", GetRequestID(r.Context())),
						slog.String("path", r.URL.Path),
						slog.String("user_id", claims.UserID.String()),
						slog.Any("error", err))
				}
				deny()

				return
			}

			ctx := context.WithValue(r.Context(), UserKey, u)
			ctx = context.WithValue(ctx, SessionClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUser(ctx context.Context) *domain.User {
	if u, ok := ctx.Value(UserKey).(*domain.User); ok {
		return u
	}

	return nil
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

func RealIPWith(trustedProxies []*net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			if isTrustedProxy(ip, trustedProxies) {
				if h := r.Header.Get("X-Real-IP"); h != "" {
					ip = h
				} else if h := r.Header.Get("X-Forwarded-For"); h != "" {
					if idx := strings.Index(h, ","); idx >= 0 {
						ip = strings.TrimSpace(h[:idx])
					} else {
						ip = strings.TrimSpace(h)
					}
				}
			}
			ctx := context.WithValue(r.Context(), RealIPKey, ip)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetRealIP(ctx context.Context) string {
	if ip, ok := ctx.Value(RealIPKey).(string); ok {
		return ip
	}
	return ""
}

func NoCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}
