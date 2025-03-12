package main

import (
	"net/http"

	"github.com/bxcodec/dbresolver/v2"
	"github.com/gorilla/sessions"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

type backend struct {
	dbc   dbresolver.DB
	store sessions.Store
	oauth *oauth2.Config
	log   logrus.FieldLogger
}

func NewBackend(
	dbc dbresolver.DB,
	store sessions.Store,
	oauth *oauth2.Config,
	log logrus.FieldLogger,
) *backend {
	return &backend{
		dbc:   dbc,
		store: store,
		oauth: oauth,
		log:   log.WithField("thread", "backend"),
	}
}

func (b *backend) Handlers() http.Handler {
	mux := http.NewServeMux()

	// LOGIN
	if b.oauth != nil {
		mux.HandleFunc("GET /google/login", b.googleLogin)
		mux.HandleFunc("/google/callback", b.googleCallback)
	}
	mux.HandleFunc("POST /register", b.register)
	mux.HandleFunc("POST /login", b.login)
	mux.HandleFunc("GET /logout", b.logout)

	// ROUTE
	mux.Handle("POST /route/create", b.auth(b.createRoute))
	mux.HandleFunc("GET /route/list", b.listRoutes)

	return mux
}

func (b *backend) listRoutes(w http.ResponseWriter, r *http.Request) {
}

func (b *backend) createRoute(w http.ResponseWriter, r *http.Request) {
}
