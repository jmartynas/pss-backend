package main

import (
	"database/sql"
	"log/slog"
	"net/http"
	"strings"

	"github.com/nats-io/nats.go"
)

type handler struct {
	db        *sql.DB
	nc        *nats.Conn
	jwtSecret []byte
	log       *slog.Logger
}

func (h *handler) routes(corsOrigins string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /login", h.login)

	mux.HandleFunc("GET /users", h.requirePerm(permManageUsers, h.listUsers))
	mux.HandleFunc("POST /users/{id}/block", h.requirePerm(permManageUsers, h.blockUser))
	mux.HandleFunc("POST /users/{id}/unblock", h.requirePerm(permManageUsers, h.unblockUser))

	mux.HandleFunc("GET /routes", h.requirePerm(permManageRoutes, h.listRoutes))
	mux.HandleFunc("DELETE /routes/{id}", h.requirePerm(permManageRoutes, h.deleteRoute))

	mux.HandleFunc("GET /admins", h.requirePerm(permManageAdmins, h.listAdmins))
	mux.HandleFunc("POST /admins", h.requirePerm(permManageAdmins, h.createAdmin))
	mux.HandleFunc("PATCH /admins/{id}/permissions", h.requirePerm(permManageAdmins, h.updateAdminPermissions))
	mux.HandleFunc("DELETE /admins/{id}", h.requirePerm(permManageAdmins, h.deleteAdmin))

	mux.HandleFunc("PATCH /me/password", h.requireAuth(h.changePassword))

	return corsMiddleware(corsOrigins)(mux)
}

func corsMiddleware(origins string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && (origins == "*" || strings.Contains(","+origins+",", ","+origin+",")) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
