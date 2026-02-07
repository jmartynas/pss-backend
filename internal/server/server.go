package server

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jmartynas/pss-backend/internal/config"
	"github.com/jmartynas/pss-backend/internal/handler"
	"github.com/jmartynas/pss-backend/internal/middleware"
	"github.com/jmartynas/pss-backend/internal/repository"
	"github.com/jmartynas/pss-backend/internal/service"
)

const oauthUserInfoTimeout = 10 * time.Second

type Server struct {
	httpServer *http.Server
	log        *slog.Logger
	tlsCert    string
	tlsKey     string
}

func New(cfg *config.Config, log *slog.Logger, db *sql.DB) *Server {
	// Repositories
	sessionRepo := repository.NewSessionRepository(db)
	userRepo := repository.NewUserRepository(db)
	routeRepo := repository.NewRouteRepository(db)
	appRepo := repository.NewApplicationRepository(db)
	reviewRepo := repository.NewReviewRepository(db)
	vehicleRepo := repository.NewVehicleRepository(db)

	// Services
	routeSvc := service.NewRouteService(routeRepo, reviewRepo)
	appSvc := service.NewApplicationService(appRepo, routeRepo)
	userSvc := service.NewUserService(userRepo, reviewRepo)

	// Handlers
	routeH := handler.NewRouteHandler(routeSvc, log)
	appH := handler.NewApplicationHandler(appSvc, log)
	secure := cfg.Server.TLSCertFile != "" && cfg.Server.TLSKeyFile != ""
	userH := handler.NewUserHandler(userSvc, sessionRepo, secure, log)
	vehicleH := handler.NewVehicleHandler(vehicleRepo, log)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("GET /ready", handler.Ready(db))

	// Public routes
	mux.HandleFunc("GET /routes/{id}", routeH.GetRoute)
	mux.HandleFunc("POST /routes/search", routeH.SearchRoutes)
	mux.HandleFunc("GET /users/{id}", userH.GetUser)

	if cfg.OAuth.BaseURL != "" && cfg.OAuth.JWTSecret != "" && len(cfg.OAuth.Providers) > 0 {
		auth := func(h http.Handler) http.Handler {
			return middleware.Authorize(sessionRepo, userRepo, cfg.OAuth.JWTSecret, log)(h)
		}

		// Route management
		mux.Handle("POST /routes", auth(http.HandlerFunc(routeH.CreateRoute)))
		mux.Handle("PATCH /routes/{id}", auth(http.HandlerFunc(routeH.UpdateRoute)))
		mux.Handle("DELETE /routes/{id}", auth(http.HandlerFunc(routeH.DeleteRoute)))
		mux.Handle("GET /routes/my", auth(http.HandlerFunc(routeH.GetMyRoutes)))
		mux.Handle("GET /routes/participated", auth(http.HandlerFunc(routeH.GetMyParticipatedRoutes)))

		// Application management
		mux.Handle("POST /routes/{id}/applications", auth(http.HandlerFunc(appH.Apply)))
		mux.Handle("GET /routes/{id}/applications", auth(http.HandlerFunc(appH.ListByRoute)))
		mux.Handle("GET /routes/{id}/applications/my", auth(http.HandlerFunc(appH.GetMyForRoute)))
		mux.Handle("PATCH /routes/{id}/applications/{appId}", auth(http.HandlerFunc(appH.ReviewApplication)))
		mux.Handle("PATCH /routes/{id}/applications/{appId}/stops", auth(http.HandlerFunc(appH.UpdateMyStops)))
		mux.Handle("POST /routes/{id}/applications/{appId}/stop-change", auth(http.HandlerFunc(appH.RequestStopChange)))
		mux.Handle("PATCH /routes/{id}/applications/{appId}/stop-change", auth(http.HandlerFunc(appH.ReviewStopChange)))
		mux.Handle("DELETE /routes/{id}/applications/{appId}/stop-change", auth(http.HandlerFunc(appH.CancelStopChange)))
		mux.Handle("DELETE /routes/{id}/applications/{appId}", auth(http.HandlerFunc(appH.Cancel)))
		mux.Handle("GET /applications/my", auth(http.HandlerFunc(appH.GetMyApplications)))

		// User profile
		mux.Handle("GET /users/me", auth(http.HandlerFunc(userH.GetMe)))
		mux.Handle("PATCH /users/me", auth(http.HandlerFunc(userH.UpdateMe)))
		mux.Handle("POST /users/me/disable", auth(http.HandlerFunc(userH.DisableMe)))

		// Vehicles
		mux.Handle("GET /vehicles/my", auth(http.HandlerFunc(vehicleH.ListMy)))
		mux.Handle("POST /vehicles", auth(http.HandlerFunc(vehicleH.Create)))
		mux.Handle("PATCH /vehicles/{id}", auth(http.HandlerFunc(vehicleH.Update)))
		mux.Handle("DELETE /vehicles/{id}", auth(http.HandlerFunc(vehicleH.Delete)))

		// OAuth
		authH := &handler.AuthHandler{
			BaseURL:    cfg.OAuth.BaseURL,
			JWTSecret:  cfg.OAuth.JWTSecret,
			Providers:  cfg.OAuth.Providers,
			SuccessURL: cfg.OAuth.SuccessURL,
			Users:      userRepo,
			Sessions:   sessionRepo,
			Log:        log,
			Secure:     secure,
			HTTPClient: &http.Client{Timeout: oauthUserInfoTimeout},
		}
		mux.HandleFunc("GET /auth/{provider}/login", authH.Login)
		mux.HandleFunc("GET /auth/{provider}/callback", authH.Callback)
		mux.HandleFunc("POST /auth/refresh", authH.Refresh)
		mux.HandleFunc("GET /auth/logout", authH.Logout)
	}

	trustedProxyNetworks, err := middleware.ParseTrustedProxyCIDRs(cfg.Server.TrustedProxyCIDRs)
	if err != nil {
		log.Warn("invalid trusted proxy CIDRs, real IP will use connection remote addr", "error", err, "value", cfg.Server.TrustedProxyCIDRs)
		trustedProxyNetworks = nil
	}

	h := middleware.NoCache(mux)
	h = middleware.Recoverer(log)(h)
	h = middleware.Logger(log)(h)
	h = middleware.RequestID(h)
	h = middleware.RealIPWith(trustedProxyNetworks)(h)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      h,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	return &Server{
		httpServer: srv,
		log:        log,
		tlsCert:    cfg.Server.TLSCertFile,
		tlsKey:     cfg.Server.TLSKeyFile,
	}
}

func (s *Server) Start() error {
	if s.tlsCert != "" && s.tlsKey != "" {
		s.log.Info("server starting (HTTPS)", slog.String("addr", s.httpServer.Addr))
		return s.httpServer.ListenAndServeTLS(s.tlsCert, s.tlsKey)
	}
	s.log.Info("server starting", slog.String("addr", s.httpServer.Addr))
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("server shutting down")
	return s.httpServer.Shutdown(ctx)
}
