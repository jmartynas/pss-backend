package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/auth"
	"github.com/jmartynas/pss-backend/internal/session"
	"github.com/jmartynas/pss-backend/internal/user"

	"golang.org/x/oauth2"
)

type oauthUserInfo struct {
	ID             string `json:"id"`
	Sub            string `json:"sub"`
	Email          string `json:"email"`
	Login          string `json:"login"`
	Mail           string `json:"mail"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	EmailVerified  *bool  `json:"email_verified"`
	VerifiedEmail  *bool  `json:"verified_email"`
}

type AuthHandler struct {
	BaseURL    string
	JWTSecret  string
	Providers  map[string]auth.ProviderConfig
	SuccessURL string
	DB         *sql.DB
	Log        *slog.Logger
	Secure     bool
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(strings.TrimSpace(r.PathValue("provider")))
	if provider == "" {
		http.Error(w, "missing provider", http.StatusBadRequest)
		return
	}
	cfg, ok := h.Providers[provider]
	if !ok {
		http.Error(w, "unknown or disabled provider", http.StatusNotFound)
		return
	}
	spec, ok := auth.Registry[provider]
	if !ok {
		http.Error(w, "unknown provider", http.StatusNotFound)
		return
	}

	state, err := randomState()
	if err != nil {
		h.Log.Error("oauth state", slog.String("error", err.Error()))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.StateCookie,
		Value:    state + "|" + provider,
		Path:     "/",
		MaxAge:   auth.StateMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	redirectURI := h.BaseURL + "/auth/" + provider + "/callback"
	oauthCfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  redirectURI,
		Endpoint: oauth2.Endpoint{
			AuthURL:  spec.AuthURL,
			TokenURL: spec.TokenURL,
		},
		Scopes: spec.Scopes,
	}
	authURL := oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(strings.TrimSpace(r.PathValue("provider")))
	if provider == "" {
		http.Error(w, "missing provider", http.StatusBadRequest)
		return
	}
	cfg, ok := h.Providers[provider]
	if !ok {
		http.Error(w, "unknown or disabled provider", http.StatusNotFound)
		return
	}
	spec, ok := auth.Registry[provider]
	if !ok {
		http.Error(w, "unknown provider", http.StatusNotFound)
		return
	}

	stateCookieVal, _ := r.Cookie(auth.StateCookie)
	if stateCookieVal == nil || stateCookieVal.Value == "" {
		http.Error(w, "missing state", http.StatusBadRequest)
		return
	}
	parts := strings.SplitN(stateCookieVal.Value, "|", 2)
	if len(parts) != 2 || parts[1] != provider {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	if r.URL.Query().Get("state") != parts[0] {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: auth.StateCookie, Value: "", Path: "/", MaxAge: -1})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	redirectURI := h.BaseURL + "/auth/" + provider + "/callback"
	oauthCfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  redirectURI,
		Endpoint: oauth2.Endpoint{
			AuthURL:  spec.AuthURL,
			TokenURL: spec.TokenURL,
		},
		Scopes: spec.Scopes,
	}
	tok, err := oauthCfg.Exchange(context.Background(), code)
	if err != nil {
		h.Log.Error("oauth exchange", slog.String("provider", provider), slog.Any("error", err))
		http.Error(w, "exchange failed", http.StatusBadRequest)
		return
	}

	sub := ""
	email := ""
	name := ""
	emailVerified := false
	if spec.UserInfoURL != "" && tok.AccessToken != "" {
		var ver bool
		sub, email, name, EmailVerified, _ = h.fetchUserInfo(r.Context(), spec.UserInfoURL, tok.AccessToken, provider)
	}
	if sub == "" {
		sub = tok.AccessToken[:min(32, len(tok.AccessToken))]
	}
	if email != "" && !emailVerified {
		h.Log.Warn("login rejected: email not verified", slog.String("provider", provider), slog.String("email", email))
		http.Error(w, "Email address is not verified. Please verify your email with the provider and try again.", http.StatusForbidden)
		return
	}

	var userID uuid.UUID
	if h.DB != nil && email != "" {
		var errUpsert error
		userID, errUpsert = user.Upsert(r.Context(), h.DB, email, name, provider, sub)
		if errUpsert != nil {
			h.Log.Error("user upsert", slog.String("email", email), slog.Any("error", errUpsert))
		}
	}
	if userID == uuid.Nil {
		if err := auth.SetSession(w, h.JWTSecret, provider, sub, email, h.Secure); err != nil {
			h.Log.Error("session", slog.Any("error", err))
			http.Error(w, "session failed", http.StatusInternalServerError)
			return
		}
	} else {
		sessionID, err := session.Create(r.Context(), h.DB, userID, session.DefaultMaxAge)
		if err != nil {
			h.Log.Error("session create", slog.Any("error", err))
			http.Error(w, "session failed", http.StatusInternalServerError)
			return
		}
		if err := auth.SetSessionWithID(w, h.JWTSecret, sessionID, userID, provider, sub, email, h.Secure); err != nil {
			h.Log.Error("session", slog.Any("error", err))
			http.Error(w, "session failed", http.StatusInternalServerError)
			return
		}
		auth.SetRefreshTokenCookie(w, sessionID.String(), h.Secure)
	}

	redirectTo := h.SuccessURL
	if redirectTo == "" {
		redirectTo = "/"
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}

func (h *AuthHandler) fetchUserInfo(ctx context.Context, userInfoURL, accessToken, provider string) (sub, email, name string, verified bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoURL, nil)
	if err != nil {
		return "", "", "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	if provider == "github" {
		req.Header.Set("Accept", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", "", false, err
	}
	defer resp.Body.Close()
	var u oauthUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return "", "", "", false, err
	}
	sub = u.Sub
	if sub == "" {
		sub = u.ID
	}
	email = u.Email
	if email == "" {
		email = u.Mail
	}
	if email == "" && u.Login != "" {
		email = u.Login + "@users.noreply.github.com"
	}
	name = u.Name
	if name == "" {
		name = u.DisplayName
	}
	if name == "" && u.Login != "" {
		name = u.Login
	}
	verified = true
	if u.EmailVerified != nil && !*u.EmailVerified {
		verified = false
	}
	if u.VerifiedEmail != nil && !*u.VerifiedEmail {
		verified = false
	}
	return sub, email, name, verified, nil
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	if h.JWTSecret == "" || h.DB == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	refreshToken, err := auth.GetRefreshToken(r)
	if err != nil || refreshToken == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	row, err := session.GetByToken(r.Context(), h.DB, refreshToken)
	if err != nil {
		h.Log.Error("refresh get session", slog.Any("error", err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if row == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	u, err := user.GetByID(r.Context(), h.DB, row.UserID)
	if err != nil {
		h.Log.Error("refresh get user", slog.Any("error", err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	_ = session.ExtendExpiry(r.Context(), h.DB, refreshToken, session.DefaultMaxAge)
	sessionID, _ := uuid.Parse(refreshToken)
	if err := auth.SetSessionWithID(w, h.JWTSecret, sessionID, u.ID, u.Provider, u.ProviderSub, u.Email, h.Secure); err != nil {
		h.Log.Error("refresh session", slog.Any("error", err))
		http.Error(w, "session refresh failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if h.DB != nil {
		if ref, _ := auth.GetRefreshToken(r); ref != "" {
			_ = session.DeleteByToken(r.Context(), h.DB, ref)
		}
	}
	auth.ClearSession(w, h.Secure)
	auth.ClearRefreshToken(w, h.Secure)
	redirectTo := h.SuccessURL
	if redirectTo == "" {
		redirectTo = "/"
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}
