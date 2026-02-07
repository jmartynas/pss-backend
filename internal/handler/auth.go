package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/auth"
	"github.com/jmartynas/pss-backend/internal/domain"

	"golang.org/x/oauth2"
)

type oauthUserInfo struct {
	ID                any    `json:"id"` // string (Google/Microsoft) or number (GitHub)
	Sub               string `json:"sub"`
	Email             string `json:"email"`
	Login             string `json:"login"`
	Mail              string `json:"mail"`
	UserPrincipalName string `json:"userPrincipalName"` // Microsoft fallback email
	Name              string `json:"name"`
	DisplayName       string `json:"displayName"`
	GivenName         string `json:"given_name"`
	FamilyName        string `json:"family_name"`
	GivennameAlt      string `json:"givenname"`  // Microsoft OIDC
	FamilynameAlt     string `json:"familyname"` // Microsoft OIDC
	EmailVerified     *bool  `json:"email_verified"`
	VerifiedEmail     *bool  `json:"verified_email"`
}

// AuthHandler handles OAuth login, callback, refresh, and logout.
type AuthHandler struct {
	BaseURL    string
	JWTSecret  string
	Providers  map[string]auth.ProviderConfig
	SuccessURL string
	Users      domain.UserRepository
	Sessions   domain.SessionRepository
	Log        *slog.Logger
	Secure     bool
	HTTPClient *http.Client
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
		Secure:   h.Secure,
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
	http.SetCookie(w, &http.Cookie{Name: auth.StateCookie, Value: "", Path: "/", MaxAge: -1, Secure: h.Secure})

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

	sub, email, name, emailVerified := "", "", "", false
	if spec.UserInfoURL != "" && tok.AccessToken != "" {
		sub, email, name, emailVerified, _ = h.fetchUserInfo(r.Context(), spec.UserInfoURL, tok.AccessToken, provider)
	}
	if sub == "" {
		sub = tok.AccessToken[:min(32, len(tok.AccessToken))]
	}
	if email != "" && !emailVerified {
		h.Log.Warn("login rejected: email not verified", slog.String("provider", provider), slog.String("email", email))
		http.Error(w, "Email address is not verified. Please verify your email with the provider and try again.", http.StatusForbidden)
		return
	}
	if email == "" {
		http.Error(w, "verified email required for login", http.StatusForbidden)
		return
	}

	userID, err := h.Users.Upsert(r.Context(), email, name, provider, sub)
	if err != nil {
		h.Log.Error("user upsert", slog.String("email", email), slog.Any("error", err))
		http.Error(w, "login failed", http.StatusInternalServerError)
		return
	}

	u, err := h.Users.GetByID(r.Context(), userID)
	if err != nil {
		h.Log.Error("user fetch after upsert", slog.Any("error", err))
		http.Error(w, "login failed", http.StatusInternalServerError)
		return
	}
	if u.Status == "blocked" {
		h.Log.Warn("login rejected: account blocked", slog.String("user_id", userID.String()))
		loginBase := strings.TrimSuffix(h.SuccessURL, "/")
		http.Redirect(w, r, loginBase+"/login?error=blocked", http.StatusFound)
		return
	}

	sessionID, err := h.Sessions.Create(r.Context(), userID, domain.DefaultSessionMaxAge)
	if err != nil {
		h.Log.Error("session create", slog.Any("error", err))
		http.Error(w, "login failed", http.StatusInternalServerError)
		return
	}
	if err := auth.SetSessionWithID(w, h.JWTSecret, sessionID, userID, provider, sub, email, h.Secure); err != nil {
		h.Log.Error("session", slog.Any("error", err))
		http.Error(w, "session failed", http.StatusInternalServerError)
		return
	}
	auth.SetRefreshTokenCookie(w, sessionID.String(), h.Secure)

	redirectTo := h.SuccessURL
	if redirectTo == "" {
		redirectTo = "/"
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	if h.JWTSecret == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	refreshToken, err := auth.GetRefreshToken(r)
	if err != nil || refreshToken == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	sess, err := h.Sessions.GetByToken(r.Context(), refreshToken)
	if err != nil {
		h.Log.Error("refresh get session", slog.Any("error", err))
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	u, err := h.Users.GetByID(r.Context(), sess.UserID)
	if err != nil {
		h.Log.Error("refresh get user", slog.Any("error", err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if u.Status == "blocked" {
		_ = h.Sessions.DeleteByToken(r.Context(), refreshToken)
		http.Error(w, "account blocked", http.StatusForbidden)
		return
	}
	_ = h.Sessions.ExtendExpiry(r.Context(), refreshToken, domain.DefaultSessionMaxAge)
	sessionID, _ := uuid.Parse(refreshToken)
	if err := auth.SetSessionWithID(w, h.JWTSecret, sessionID, u.ID, u.Provider, u.ProviderSub, u.Email, h.Secure); err != nil {
		h.Log.Error("refresh session", slog.Any("error", err))
		http.Error(w, "session refresh failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if ref, _ := auth.GetRefreshToken(r); ref != "" {
		_ = h.Sessions.DeleteByToken(r.Context(), ref)
	}
	auth.ClearSession(w, h.Secure)
	auth.ClearRefreshToken(w, h.Secure)
	redirectTo := h.SuccessURL
	if redirectTo == "" {
		redirectTo = "/"
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
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
	client := h.HTTPClient
	if client == nil {
		client = &http.Client{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", false, err
	}
	defer resp.Body.Close()
	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return "", "", "", false, err
	}
	var u oauthUserInfo
	if err := json.Unmarshal(raw, &u); err != nil {
		h.Log.Warn("oauth userinfo decode", slog.String("provider", provider), slog.Any("error", err))
		// continue with partial decode
	}
	sub = u.Sub
	if sub == "" {
		switch v := u.ID.(type) {
		case string:
			sub = v
		case float64:
			sub = fmt.Sprintf("%.0f", v)
		}
	}
	email = u.Email
	if email == "" {
		email = u.Mail
	}
	if email == "" {
		email = u.UserPrincipalName // Microsoft personal accounts
	}
	name = u.Name
	if name == "" {
		name = u.DisplayName
	}
	if name == "" && u.GivenName != "" {
		name = strings.TrimSpace(u.GivenName + " " + u.FamilyName)
	}
	if name == "" && u.GivennameAlt != "" {
		name = strings.TrimSpace(u.GivennameAlt + " " + u.FamilynameAlt)
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

	// GitHub users with private emails: /user returns email=null.
	// Fall back to /user/emails to find the primary verified address.
	if provider == "github" && email == "" {
		email, verified = h.fetchGitHubPrimaryEmail(ctx, accessToken)
	}
	// Last resort: use the GitHub noreply address so login is never blocked
	// by a missing email field. This address is stable per GitHub user ID.
	if provider == "github" && email == "" && u.Login != "" {
		h.Log.Warn("github: could not resolve email, using noreply fallback", slog.String("login", u.Login))
		email = u.Login + "@users.noreply.github.com"
		verified = true
	}

	return sub, email, name, verified, nil
}

// fetchGitHubPrimaryEmail calls the GitHub /user/emails endpoint and returns
// the primary verified email address, or ("", false) if none is found.
func (h *AuthHandler) fetchGitHubPrimaryEmail(ctx context.Context, accessToken string) (string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		h.Log.Warn("github emails: build request", slog.Any("error", err))
		return "", false
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	client := h.HTTPClient
	if client == nil {
		client = &http.Client{}
	}
	resp, err := client.Do(req)
	if err != nil {
		h.Log.Warn("github emails: request failed", slog.Any("error", err))
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		h.Log.Warn("github emails: unexpected status", slog.Int("status", resp.StatusCode))
		return "", false
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		h.Log.Warn("github emails: decode failed", slog.Any("error", err))
		return "", false
	}
	// Prefer primary+verified; fall back to any verified email.
	fallback := ""
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, true
		}
		if e.Verified && fallback == "" {
			fallback = e.Email
		}
	}
	if fallback != "" {
		return fallback, true
	}
	return "", false
}
