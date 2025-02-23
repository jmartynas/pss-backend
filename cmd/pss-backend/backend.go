package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bxcodec/dbresolver/v2"
	"github.com/gorilla/sessions"
	"github.com/jmartynas/pss-backend/db"
	"github.com/jmartynas/pss-backend/structs"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

const sessionKey = "session"

type contextKey string

const (
	emailKey contextKey = "email"
)

type backend struct {
	dbc   dbresolver.DB
	store *sessions.CookieStore
	oauth *oauth2.Config
	log   logrus.FieldLogger
}

func NewBackend(
	dbc dbresolver.DB,
	store *sessions.CookieStore,
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
		mux.HandleFunc("/google/login", b.googleLogin)
		mux.HandleFunc("/google/callback", b.googleCallback)
	}
	mux.HandleFunc("/login", b.login)
	mux.HandleFunc("/logout", b.logout)

	// ROUTE
	mux.Handle("POST /route/create", b.auth(b.createRoute))
	mux.HandleFunc("GET /route/list", b.listRoutes)

	return mux
}

func (b *backend) auth(fn http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := b.store.Get(r, sessionKey)

		authenticated, ok := session.Values["authenticated"].(bool)
		if !ok || !authenticated {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		email, _ := session.Values["email"].(string)
		ctx := context.WithValue(r.Context(), emailKey, email)
		fn.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (b *backend) logout(w http.ResponseWriter, r *http.Request) {
	session, _ := b.store.Get(r, string(config.SessionKey))
	delete(session.Values, "authenticated")
	session.Options.MaxAge = -1
	if err := session.Save(r, w); err != nil {
		b.log.WithError(err).Error("could not remove session")
		http.Error(w, "could not remove token", http.StatusInternalServerError)
	}
}

func (b *backend) login(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")

	hashedPassword, err := db.SelectPassword(r.Context(), b.dbc, email)
	switch {
	case err == nil: // OK
	case hashedPassword == nil:
		http.Error(w, "user does not exist", http.StatusUnauthorized)
	case errors.Is(err, r.Context().Err()):
		http.Error(w, "selecting password", http.StatusInternalServerError)
		return
	default:
		b.log.WithError(err).Error("selecting password")
		http.Error(w, "selecting password", http.StatusUnauthorized)
		return
	}

	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
	switch {
	case err == nil: // OK
	case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	default:
		b.log.WithError(err).Error("comparing password hashes")
		http.Error(w, "comparing password hashes", http.StatusInternalServerError)
		return
	}

	session, err := b.store.Get(r, sessionKey)
	if err != nil {
		http.Error(w, "could not get session store", http.StatusInternalServerError)
		return
	}

	session.Values["authenticated"] = true
	session.Values["oauth_provider"] = "custom"
	session.Values["email"] = email

	if err := session.Save(r, w); err != nil {
		http.Error(w, "could not save token", http.StatusInternalServerError)
	}
}

func (b *backend) googleLogin(w http.ResponseWriter, r *http.Request) {
	url := b.oauth.AuthCodeURL("", oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusFound)
}

func (b *backend) googleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "getting code", http.StatusBadRequest)
		return
	}

	token, err := b.oauth.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "exchanging token", http.StatusInternalServerError)
		return
	}

	client := b.oauth.Client(r.Context(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		http.Error(w, "failed to get user info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var userInfo structs.User
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		http.Error(w, "failed to unmarshal user info", http.StatusInternalServerError)
		return
	}
	if userInfo.Email == "" || userInfo.ID == "" || userInfo.Name == "" {
		http.Error(w, "invalid user info", http.StatusBadRequest)
		return
	}

	user, err := db.SelectUser(r.Context(), b.dbc, userInfo.Email)
	switch {
	case err == nil: // OK
	case errors.Is(err, db.ErrNotFound):
		err = db.InsertUser(r.Context(), b.dbc, user)
		switch {
		case err == nil: // OK
		case errors.Is(err, r.Context().Err()):
			http.Error(w, "creating user", http.StatusInternalServerError)
			return
		default:
			b.log.WithError(err).Error("creating google oauth user")
			http.Error(w, "creating google oauth user", http.StatusInternalServerError)
			return
		}
	case errors.Is(err, r.Context().Err()):
		http.Error(w, "getting user", http.StatusInternalServerError)
		return
	default:
		b.log.WithError(err).Error("getting user by email")
		http.Error(w, "getting user by email", http.StatusInternalServerError)
		return
	}

	session, err := b.store.Get(r, sessionKey)
	if err != nil {
		http.Error(w, "could not get session", http.StatusInternalServerError)
		return
	}

	session.Values["authenticated"] = true
	session.Values["oauth_provider"] = "google"
	session.Values["email"] = userInfo.Email

	if err := session.Save(r, w); err != nil {
		http.Error(w, "could not save session token", http.StatusInternalServerError)
		return
	}
}

func (b *backend) listRoutes(w http.ResponseWriter, r *http.Request) {
}

func (b *backend) createRoute(w http.ResponseWriter, r *http.Request) {
}
