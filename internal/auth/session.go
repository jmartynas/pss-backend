package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/jmartynas/pss-backend/internal/errs"
)

const (
	CookieName    = "session"    
	RefreshCookie = "refresh_token"
	AccessTokenLifetimeSec = 15 * 60
	RefreshTokenMaxAgeSec  = 7 * 24 * 3600
	StateCookie            = "oauth_state"
	StateMaxAge            = 600
)

type Claims struct {
	jwt.RegisteredClaims
	SessionID uuid.UUID `json:"session_id"`
	UserID    uuid.UUID `json:"user_id"`
	Provider  string    `json:"provider"`
	Email     string    `json:"email"`
}

func SetSession(w http.ResponseWriter, secret, provider, sub, email string, secure bool) error {
	if secret == "" {
		return errs.ErrJWTSecretRequired
	}
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sub,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(RefreshTokenMaxAgeSec) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Provider: provider,
		Email:    email,
	}
	return setSessionCookie(w, secret, claims, secure, RefreshTokenMaxAgeSec)
}

func SetSessionWithID(w http.ResponseWriter, secret string, sessionID, userID uuid.UUID, provider, sub, email string, secure bool) error {
	if secret == "" {
		return errs.ErrJWTSecretRequired
	}
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sub,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(AccessTokenLifetimeSec) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		SessionID: sessionID,
		UserID:    userID,
		Provider:  provider,
		Email:     email,
	}
	return setSessionCookie(w, secret, claims, secure, AccessTokenLifetimeSec)
}

func setSessionCookie(w http.ResponseWriter, secret string, claims Claims, secure bool, maxAgeSec int) error {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return fmt.Errorf("sign session token: %w", err)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    signed,
		Path:     "/",
		MaxAge:   maxAgeSec,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func SetRefreshTokenCookie(w http.ResponseWriter, sessionID string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookie,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   RefreshTokenMaxAgeSec,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func GetRefreshToken(r *http.Request) (string, error) {
	c, err := r.Cookie(RefreshCookie)
	if err != nil || c == nil || c.Value == "" {
		return "", errs.ErrInvalidSession
	}
	return c.Value, nil
}

func ClearRefreshToken(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func GetSession(r *http.Request, secret string) (*Claims, error) {
	if secret == "" {
		return nil, errs.ErrJWTSecretRequired
	}
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return nil, errs.ErrInvalidSession
	}
	var claims Claims
	token, err := jwt.ParseWithClaims(c.Value, &claims, func(*jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid {
		return nil, errs.ErrInvalidSession
	}
	return &claims, nil
}

func ClearSession(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
