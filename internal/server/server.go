package server

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jmartynas/pss-backend/internal/config"
	"github.com/jmartynas/pss-backend/internal/handlers"
	"github.com/jmartynas/pss-backend/internal/middleware"
)

const oauthUserInfoTimeout = 10 * time.Second

type Server struct {
	httpServer *http.Server
	log        *slog.Logger
	tlsCert    string
	tlsKey     string
}

func New(cfg *config.Config, log *slog.Logger, db *sql.DB) *Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", handlers.Health)
	mux.HandleFunc("GET /ready", handlers.Ready(db))

	if cfg.OAuth.BaseURL != "" && cfg.OAuth.JWTSecret != "" && len(cfg.OAuth.Providers) > 0 {
		secure := cfg.Server.TLSCertFile != "" && cfg.Server.TLSKeyFile != ""
		authH := &handlers.AuthHandler{
			BaseURL:    cfg.OAuth.BaseURL,
			JWTSecret:  cfg.OAuth.JWTSecret,
			Providers:  cfg.OAuth.Providers,
			SuccessURL: cfg.OAuth.SuccessURL,
			DB:         db,
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
