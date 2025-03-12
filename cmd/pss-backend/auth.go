package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"

	"github.com/jmartynas/pss-backend/db"
	"github.com/jmartynas/pss-backend/structs"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

const sessionKey = "session"

type contextKey string

const (
	emailKey contextKey = "email"
)

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

func (b *backend) register(w http.ResponseWriter, r *http.Request) {
	user := &structs.User{}
	if err := json.NewDecoder(r.Body).Decode(user); err != nil {
		http.Error(w, "could not decode json", http.StatusBadRequest)
		return
	}

	if _, err := mail.ParseAddress(user.Email); err != nil {
		http.Error(w, "bad email address", http.StatusBadRequest)
		return
	}

	if user.Name == "" {
		http.Error(w, "name is empty", http.StatusBadRequest)
		return
	}
	if len(user.Name) > 255 {
		http.Error(w, "name is more than 255 characters long", http.StatusBadRequest)
		return
	}

	if len(*user.Password) < 8 {
		http.Error(w, "password is less than 8 characters long", http.StatusBadRequest)
		return
	}
	if len(*user.Password) > 255 {
		http.Error(w, "password is more than 255 characters long", http.StatusBadRequest)
		return
	}

	password, err := bcrypt.GenerateFromPassword([]byte(*user.Password), 12)
	if err != nil {
		http.Error(w, "could not hash password", http.StatusInternalServerError)
		return
	}
	pass := string(password)
	user.Password = &pass

	err = db.InsertUser(r.Context(), b.dbc, user)
	switch {
	case err == nil: // OK
	case errors.Is(err, r.Context().Err()):
		http.Error(w, "inserting user context error", http.StatusInternalServerError)
		return
	default:
		b.log.WithError(err).Error("inserting user")
		http.Error(w, "inserting user", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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
	var login struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&login); err != nil {
		http.Error(w, "could not decode json", http.StatusBadRequest)
		return
	}

	hashedPassword, err := db.SelectPassword(r.Context(), b.dbc, login.Email)
	switch {
	case err == nil: // OK
	case errors.Is(err, db.ErrNotFound):
		http.Error(w, "user does not exist", http.StatusUnauthorized)
		return
	case errors.Is(err, r.Context().Err()):
		http.Error(w, "selecting password", http.StatusInternalServerError)
		return
	default:
		b.log.WithError(err).Error("selecting password")
		http.Error(w, "selecting password", http.StatusUnauthorized)
		return
	}

	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(login.Password))
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
	session.Values["email"] = login.Email

	if err := session.Save(r, w); err != nil {
		fmt.Println(err)
		http.Error(w, "could not save token", http.StatusInternalServerError)
	}
}

func (b *backend) googleLogin(w http.ResponseWriter, r *http.Request) {
	url := b.oauth.AuthCodeURL(
		"",
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam(
			"scope",
			"https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile",
		),
	)
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
		http.Error(w, "failed to call google api", http.StatusInternalServerError)
		b.log.WithError(err).Error("calling google api")
		return
	}
	defer resp.Body.Close()

	userInfo := &structs.User{}
	if err := json.NewDecoder(resp.Body).Decode(userInfo); err != nil {
		http.Error(w, "failed to decode google user info", http.StatusInternalServerError)
		b.log.WithError(err).Error("decoding google api response")
		return
	}
	if userInfo.Email == "" || userInfo.Name == "" {
		http.Error(w, "invalid user info", http.StatusBadRequest)
		return
	}

	user, err := db.SelectUser(r.Context(), b.dbc, userInfo.Email)
	switch {
	case err == nil: // OK
	case errors.Is(err, db.ErrNotFound):
		fmt.Printf("%+v\n", userInfo)
		err = db.InsertUser(r.Context(), b.dbc, userInfo)
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
		user = userInfo
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
	session.Values["email"] = user.Email

	if err := session.Save(r, w); err != nil {
		http.Error(w, "could not save session token", http.StatusInternalServerError)
		return
	}
}
