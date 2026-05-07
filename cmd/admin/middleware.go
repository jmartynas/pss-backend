package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

const (
	permManageUsers  uint8 = 1
	permManageRoutes uint8 = 2
	permManageAdmins uint8 = 4
)

type adminClaims struct {
	jwt.RegisteredClaims
	AdminID     string `json:"admin_id"`
	Permissions uint8  `json:"permissions"`
}

type ctxAdmin struct {
	id          string
	permissions uint8
}

type contextKey string

const adminCtxKey contextKey = "admin"

func (h *handler) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return h.requirePerm(0, next)
}

func (h *handler) requirePerm(perm uint8, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hdr := r.Header.Get("Authorization")
		if !strings.HasPrefix(hdr, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		var claims adminClaims
		token, err := jwt.ParseWithClaims(hdr[7:], &claims, func(*jwt.Token) (interface{}, error) {
			return h.jwtSecret, nil
		}, jwt.WithValidMethods([]string{"HS256"}))

		if err != nil || !token.Valid {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		if perm != 0 && claims.Permissions&perm == 0 {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}

		ctx := context.WithValue(r.Context(), adminCtxKey, &ctxAdmin{
			id:          claims.AdminID,
			permissions: claims.Permissions,
		})

		next(w, r.WithContext(ctx))
	}
}

func currentAdmin(r *http.Request) *ctxAdmin {
	a, _ := r.Context().Value(adminCtxKey).(*ctxAdmin)
	return a
}
