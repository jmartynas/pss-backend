package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	sq "github.com/Masterminds/squirrel"
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"golang.org/x/crypto/bcrypt"
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

var (
	jwtSecret []byte
	db        *sql.DB
	nc        *nats.Conn
	log       *slog.Logger
)

func main() {
	log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	dsn := mustEnv("MYSQL_DSN")
	jwtSecret = []byte(mustEnv("ADMIN_JWT_SECRET"))
	port := getEnv("ADMIN_PORT", "8001")
	corsOrigins := getEnv("ADMIN_CORS_ORIGINS", "http://localhost:3001")
	natsURL := getEnv("NATS_URL", "nats://localhost:4222")

	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Error("mysql open failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Error("mysql ping failed", slog.Any("error", err))
		os.Exit(1)
	}
	log.Info("mysql connected")

	nc, err = nats.Connect(natsURL, nats.Name("pss-admin"), nats.MaxReconnects(-1), nats.RetryOnFailedConnect(true))
	if err != nil {
		log.Error("nats connection failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer nc.Drain()
	log.Info("nats connected", slog.String("url", natsURL))

	if err := seedAdmin(); err != nil {
		log.Error("seed admin failed", slog.Any("error", err))
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /login", handleLogin)
	mux.HandleFunc("GET /users", requirePerm(permManageUsers, handleListUsers))
	mux.HandleFunc("POST /users/{id}/block", requirePerm(permManageUsers, handleBlockUser))
	mux.HandleFunc("POST /users/{id}/unblock", requirePerm(permManageUsers, handleUnblockUser))
	mux.HandleFunc("GET /routes", requirePerm(permManageRoutes, handleListRoutes))
	mux.HandleFunc("DELETE /routes/{id}", requirePerm(permManageRoutes, handleDeleteRoute))
	mux.HandleFunc("GET /admins", requirePerm(permManageAdmins, handleListAdmins))
	mux.HandleFunc("POST /admins", requirePerm(permManageAdmins, handleCreateAdmin))
	mux.HandleFunc("PATCH /admins/{id}/permissions", requirePerm(permManageAdmins, handleUpdateAdminPermissions))
	mux.HandleFunc("DELETE /admins/{id}", requirePerm(permManageAdmins, handleDeleteAdmin))
	mux.HandleFunc("PATCH /me/password", requireAuth(handleChangePassword))

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      corsMiddleware(corsOrigins)(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("admin server starting", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	log.Info("admin server stopped")
}

func seedAdmin() error {
	email := getEnv("SEED_ADMIN_EMAIL", "")
	password := getEnv("SEED_ADMIN_PASSWORD", "")
	if email == "" || password == "" {
		return nil
	}

	var count int
	if err := sq.Select("COUNT(*)").From("admins").RunWith(db).QueryRow().Scan(&count); err != nil {
		return fmt.Errorf("count admins: %w", err)
	}
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if _, err = sq.Insert("admins").
		Columns("id", "email", "password", "status", "permissions").
		Values(uuid.New().String(), email, string(hash), 0, 7).
		RunWith(db).Exec(); err != nil {
		return fmt.Errorf("insert seed admin: %w", err)
	}
	log.Info("seed admin created", slog.String("email", email))
	return nil
}

// ── Auth middleware ────────────────────────────────────────────────────────────

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return requirePerm(0, next)
}

func requirePerm(perm uint8, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		var claims adminClaims
		token, err := jwt.ParseWithClaims(h[7:], &claims, func(*jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		}, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil || !token.Valid {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		if perm != 0 && claims.Permissions&perm == 0 {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		ctx := context.WithValue(r.Context(), adminCtxKey, &ctxAdmin{id: claims.AdminID, permissions: claims.Permissions})
		next(w, r.WithContext(ctx))
	}
}

func currentAdmin(r *http.Request) *ctxAdmin {
	a, _ := r.Context().Value(adminCtxKey).(*ctxAdmin)
	return a
}

// ── Login ─────────────────────────────────────────────────────────────────────

func handleChangePassword(w http.ResponseWriter, r *http.Request) {
	a := currentAdmin(r)
	if a == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Current == "" || req.New == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "current_password and new_password are required"})
		return
	}

	var hash string
	if err := sq.Select("password").From("admins").Where(sq.Eq{"id": a.id}).
		RunWith(db).QueryRowContext(r.Context()).Scan(&hash); err != nil {
		log.Error("fetch admin password", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Current)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "incorrect current password"})
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.New), bcrypt.DefaultCost)
	if err != nil {
		log.Error("hash new password", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if _, err = sq.Update("admins").Set("password", string(newHash)).Where(sq.Eq{"id": a.id}).
		RunWith(db).ExecContext(r.Context()); err != nil {
		log.Error("update password", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	var id, hash string
	var status, permissions uint8
	err := sq.Select("id", "password", "status", "permissions").
		From("admins").
		Where(sq.Eq{"email": req.Email, "deleted_at": nil}).
		Where(sq.Gt{"permissions": 0}).
		RunWith(db).QueryRowContext(r.Context()).Scan(&id, &hash, &status, &permissions)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	if err != nil {
		log.Error("query admin", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	claims := adminClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   id,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		AdminID:     id,
		Permissions: permissions,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(jwtSecret)
	if err != nil {
		log.Error("sign token", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":       signed,
		"permissions": permissions,
		"admin_id":    id,
	})
}

// ── Users ─────────────────────────────────────────────────────────────────────

func handleListUsers(w http.ResponseWriter, r *http.Request) {
	type userRow struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		Status    string `json:"status"`
		Provider  string `json:"provider"`
		CreatedAt string `json:"created_at"`
	}

	rows, err := sq.Select("id", "email", "COALESCE(name, '')", "status", "provider", "created_at").
		From("users").
		OrderBy("created_at DESC").
		Limit(1000).
		RunWith(db).QueryContext(r.Context())
	if err != nil {
		log.Error("list users", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	defer rows.Close()

	users := []userRow{}
	for rows.Next() {
		var u userRow
		var createdAt time.Time
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Status, &u.Provider, &createdAt); err != nil {
			log.Error("scan user", slog.Any("error", err))
			continue
		}
		u.CreatedAt = createdAt.Format(time.RFC3339)
		users = append(users, u)
	}
	writeJSON(w, http.StatusOK, users)
}

func handleBlockUser(w http.ResponseWriter, r *http.Request) {
	setUserStatus(w, r, "blocked")
}

func handleUnblockUser(w http.ResponseWriter, r *http.Request) {
	setUserStatus(w, r, "active")
}

func setUserStatus(w http.ResponseWriter, r *http.Request, status string) {
	id := r.PathValue("id")
	if status == "blocked" {
		var current string
		err := sq.Select("status").From("users").Where(sq.Eq{"id": id}).
			RunWith(db).QueryRowContext(r.Context()).Scan(&current)
		if err != nil || current == "inactive" {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "cannot block inactive user"})
			return
		}
	}
	if _, err := sq.Update("users").
		Set("status", status).
		Where(sq.Eq{"id": id}).
		RunWith(db).ExecContext(r.Context()); err != nil {
		log.Error("set user status", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if status == "blocked" {
		cancelDriverRoutes(r.Context(), id)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

// cancelDriverRoutes cancels all active routes where the user is the driver.
func cancelDriverRoutes(ctx context.Context, userID string) {
	rows, err := sq.Select("r.id").
		From("routes r").
		Join("participants p ON p.route_id = r.id").
		Where(sq.Eq{"p.user_id": userID, "p.status": "driver", "p.deleted_at": nil, "r.deleted_at": nil}).
		RunWith(db).QueryContext(ctx)
	if err != nil {
		log.Error("find driver routes", slog.Any("error", err))
		return
	}
	defer rows.Close()

	var routeIDs []string
	for rows.Next() {
		var rid string
		if err := rows.Scan(&rid); err == nil {
			routeIDs = append(routeIDs, rid)
		}
	}
	rows.Close()

	for _, rid := range routeIDs {
		if err := cancelRoute(ctx, rid); err != nil {
			log.Error("cancel driver route", slog.String("route_id", rid), slog.Any("error", err))
		}
	}
}

// ── Routes ────────────────────────────────────────────────────────────────────

func handleListRoutes(w http.ResponseWriter, r *http.Request) {
	type routeRow struct {
		ID        string  `json:"id"`
		From      string  `json:"from"`
		To        string  `json:"to"`
		LeavingAt *string `json:"leaving_at"`
		CreatedAt string  `json:"created_at"`
		Creator   string  `json:"creator"`
		Deleted   bool    `json:"deleted"`
	}

	rows, err := sq.Select(
		"r.id",
		"COALESCE(r.start_formatted_address, '')",
		"COALESCE(r.end_formatted_address, '')",
		"r.leaving_at",
		"r.created_at",
		"COALESCE(u.name, u.email)",
		"r.deleted_at IS NOT NULL",
	).From("routes r").
		Join("users u ON u.id = r.creator_user_id").
		Where(sq.Eq{"r.deleted_at": nil}).
		OrderBy("r.created_at DESC").
		Limit(1000).
		RunWith(db).QueryContext(r.Context())
	if err != nil {
		log.Error("list routes", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	defer rows.Close()

	routes := []routeRow{}
	for rows.Next() {
		var rt routeRow
		var leavingAt sql.NullTime
		var createdAt time.Time
		if err := rows.Scan(&rt.ID, &rt.From, &rt.To, &leavingAt, &createdAt, &rt.Creator, &rt.Deleted); err != nil {
			log.Error("scan route", slog.Any("error", err))
			continue
		}
		rt.CreatedAt = createdAt.Format(time.RFC3339)
		if leavingAt.Valid {
			s := leavingAt.Time.Format(time.RFC3339)
			rt.LeavingAt = &s
		}
		routes = append(routes, rt)
	}
	writeJSON(w, http.StatusOK, routes)
}

func handleDeleteRoute(w http.ResponseWriter, r *http.Request) {
	if err := cancelRoute(r.Context(), r.PathValue("id")); err != nil {
		log.Error("delete route", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// cancelRoute soft-deletes a route and performs full cleanup within a transaction.
func cancelRoute(ctx context.Context, routeID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := sq.Update("routes").
		Set("deleted_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": routeID, "deleted_at": nil}).
		RunWith(tx).ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("soft-delete route: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil // already deleted
	}

	emailLogID, err := cancelRouteCleanup(ctx, tx, routeID)
	if err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	if nc != nil {
		payload := fmt.Sprintf(`{"id":%q,"type":"route_cancelled"}`, emailLogID)
		nc.Publish("email", []byte(payload)) //nolint:errcheck
	}
	return nil
}

// cancelRouteCleanup deletes all related data for a cancelled route within an existing
// transaction and returns the email_log ID for NATS notification.
func cancelRouteCleanup(ctx context.Context, tx *sql.Tx, routeID string) (string, error) {
	pRows, err := sq.Select("id", "user_id", "status").
		From("participants").
		Where(sq.Eq{"route_id": routeID}).
		RunWith(tx).QueryContext(ctx)
	if err != nil {
		return "", fmt.Errorf("fetch participants: %w", err)
	}
	var allParticipantIDs []string
	var driverParticipantID, driverUserID string
	var passengerUserIDs []string
	for pRows.Next() {
		var pid, uid, status string
		if err := pRows.Scan(&pid, &uid, &status); err != nil {
			pRows.Close()
			return "", fmt.Errorf("scan participant: %w", err)
		}
		allParticipantIDs = append(allParticipantIDs, pid)
		if status == "driver" {
			driverParticipantID, driverUserID = pid, uid
		} else {
			passengerUserIDs = append(passengerUserIDs, uid)
		}
	}
	pRows.Close()
	if err := pRows.Err(); err != nil {
		return "", fmt.Errorf("iter participants: %w", err)
	}

	// Delete existing requests (request_stops cascade).
	if len(allParticipantIDs) > 0 {
		if _, err = sq.Delete("requests").
			Where(sq.Eq{"participant_id": allParticipantIDs}).
			RunWith(tx).ExecContext(ctx); err != nil {
			return "", fmt.Errorf("delete requests: %w", err)
		}
	}

	// Insert cancellation request + email_log for notification.
	emailLogID := uuid.New().String()
	if driverParticipantID != "" {
		requestID := uuid.New().String()
		if _, err = sq.Insert("requests").
			Columns("id", "participant_id").
			Values(requestID, driverParticipantID).
			RunWith(tx).ExecContext(ctx); err != nil {
			return "", fmt.Errorf("insert request: %w", err)
		}
		if _, err = sq.Insert("email_logs").
			Columns("id", "request_id", "type", "status").
			Values(emailLogID, requestID, "route_cancelled", "created").
			RunWith(tx).ExecContext(ctx); err != nil {
			return "", fmt.Errorf("insert email_log: %w", err)
		}
	}

	// Delete route group messages.
	if _, err = sq.Delete("route_messages").
		Where(sq.Eq{"route_id": routeID}).
		RunWith(tx).ExecContext(ctx); err != nil {
		return "", fmt.Errorf("delete route_messages: %w", err)
	}

	// Delete private chats between driver and passengers (private_messages cascade).
	if driverUserID != "" && len(passengerUserIDs) > 0 {
		if _, err = sq.Delete("private_chats").
			Where(sq.Or{
				sq.And{sq.Eq{"user1_id": driverUserID}, sq.Eq{"user2_id": passengerUserIDs}},
				sq.And{sq.Eq{"user2_id": driverUserID}, sq.Eq{"user1_id": passengerUserIDs}},
			}).
			RunWith(tx).ExecContext(ctx); err != nil {
			return "", fmt.Errorf("delete private_chats: %w", err)
		}
	}

	// Delete route stops.
	if _, err = sq.Delete("route_stops").
		Where(sq.Eq{"route_id": routeID}).
		RunWith(tx).ExecContext(ctx); err != nil {
		return "", fmt.Errorf("delete route_stops: %w", err)
	}

	// Soft-delete all participants.
	if len(allParticipantIDs) > 0 {
		if _, err = sq.Update("participants").
			Set("deleted_at", sq.Expr("NOW()")).
			Where(sq.Eq{"route_id": routeID, "deleted_at": nil}).
			RunWith(tx).ExecContext(ctx); err != nil {
			return "", fmt.Errorf("soft-delete participants: %w", err)
		}
	}

	return emailLogID, nil
}

// ── Admins ────────────────────────────────────────────────────────────────────

func handleCreateAdmin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		Permissions uint8  `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password are required"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Error("hash password", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	id := uuid.New().String()
	if _, err = sq.Insert("admins").
		Columns("id", "email", "password", "status", "permissions").
		Values(id, req.Email, string(hash), 0, req.Permissions).
		RunWith(db).ExecContext(r.Context()); err != nil {
		log.Error("create admin", slog.Any("error", err))
		writeJSON(w, http.StatusConflict, map[string]string{"error": "email already exists"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "email": req.Email, "permissions": req.Permissions})
}

func handleListAdmins(w http.ResponseWriter, r *http.Request) {
	type adminRow struct {
		ID          string `json:"id"`
		Email       string `json:"email"`
		Permissions uint8  `json:"permissions"`
		CreatedAt   string `json:"created_at"`
	}

	rows, err := sq.Select("id", "email", "permissions", "created_at").
		From("admins").
		Where(sq.Eq{"deleted_at": nil}).
		OrderBy("created_at ASC").
		RunWith(db).QueryContext(r.Context())
	if err != nil {
		log.Error("list admins", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	defer rows.Close()

	admins := []adminRow{}
	for rows.Next() {
		var a adminRow
		var createdAt time.Time
		if err := rows.Scan(&a.ID, &a.Email, &a.Permissions, &createdAt); err != nil {
			log.Error("scan admin", slog.Any("error", err))
			continue
		}
		a.CreatedAt = createdAt.Format(time.RFC3339)
		admins = append(admins, a)
	}
	writeJSON(w, http.StatusOK, admins)
}

func handleDeleteAdmin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if a := currentAdmin(r); a != nil && a.id == id {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cannot delete own account"})
		return
	}
	if _, err := sq.Update("admins").
		Set("deleted_at", time.Now()).
		Where(sq.Eq{"id": id, "deleted_at": nil}).
		RunWith(db).ExecContext(r.Context()); err != nil {
		log.Error("delete admin", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func handleUpdateAdminPermissions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if a := currentAdmin(r); a != nil && a.id == id {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cannot change own permissions"})
		return
	}

	var req struct {
		Permissions uint8 `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	if _, err := sq.Update("admins").
		Set("permissions", req.Permissions).
		Where(sq.Eq{"id": id}).
		RunWith(db).ExecContext(r.Context()); err != nil {
		log.Error("update admin permissions", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]uint8{"permissions": req.Permissions})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
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

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "required env var %s is not set\n", key)
		os.Exit(1)
	}
	return v
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
