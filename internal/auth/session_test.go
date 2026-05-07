package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jmartynas/pss-backend/internal/errs"
)

func TestGetSession_NoCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := GetSession(req, "secret")
	if err == nil {
		t.Fatal("expected error when no cookie")
	}
	if !errors.Is(err, errs.ErrInvalidSession) {
		t.Errorf("GetSession() err = %v, want ErrInvalidSession", err)
	}
}

func TestGetSession_EmptySecret(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: "any"})
	_, err := GetSession(req, "")
	if err == nil {
		t.Fatal("expected error when secret empty")
	}
	if !errors.Is(err, errs.ErrJWTSecretRequired) {
		t.Errorf("GetSession() err = %v, want ErrJWTSecretRequired", err)
	}
}

func TestGetSession_InvalidToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: "not.a.jwt"})
	_, err := GetSession(req, "secret")
	if !errors.Is(err, errs.ErrInvalidSession) {
		t.Errorf("GetSession() err = %v, want ErrInvalidSession", err)
	}
}

func TestGetSession_ExpiredToken(t *testing.T) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte("secret"))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: signed})
	_, err := GetSession(req, "secret")
	if !errors.Is(err, errs.ErrInvalidSession) {
		t.Errorf("GetSession() err = %v, want ErrInvalidSession", err)
	}
}

func TestGetSession_WrongSecret(t *testing.T) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte("correct-secret"))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: signed})
	_, err := GetSession(req, "wrong-secret")
	if !errors.Is(err, errs.ErrInvalidSession) {
		t.Errorf("GetSession() err = %v, want ErrInvalidSession", err)
	}
}

func TestSetSessionWithID_RoundTrip(t *testing.T) {
	secret := "test-secret-32-chars-minimum-len!"
	sessionID := uuid.New()
	userID := uuid.New()

	w := httptest.NewRecorder()
	err := SetSessionWithID(w, secret, sessionID, userID, "google", "sub123", "test@example.com", false)
	if err != nil {
		t.Fatalf("SetSessionWithID() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range w.Result().Cookies() {
		req.AddCookie(c)
	}

	claims, err := GetSession(req, secret)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if claims.SessionID != sessionID {
		t.Errorf("SessionID = %v, want %v", claims.SessionID, sessionID)
	}
	if claims.UserID != userID {
		t.Errorf("UserID = %v, want %v", claims.UserID, userID)
	}
	if claims.Provider != "google" {
		t.Errorf("Provider = %v, want google", claims.Provider)
	}
	if claims.Email != "test@example.com" {
		t.Errorf("Email = %v, want test@example.com", claims.Email)
	}
}

func TestGetRefreshToken_Missing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := GetRefreshToken(req)
	if !errors.Is(err, errs.ErrInvalidSession) {
		t.Errorf("GetRefreshToken() err = %v, want ErrInvalidSession", err)
	}
}

func TestGetRefreshToken_Present(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: RefreshCookie, Value: "token-value"})
	got, err := GetRefreshToken(req)
	if err != nil {
		t.Fatalf("GetRefreshToken() error = %v", err)
	}
	if got != "token-value" {
		t.Errorf("GetRefreshToken() = %q, want %q", got, "token-value")
	}
}

func TestClearSession_SetsMaxAgeNegative(t *testing.T) {
	w := httptest.NewRecorder()
	ClearSession(w, false)
	cookies := w.Result().Cookies()
	var found *http.Cookie
	for _, c := range cookies {
		if c.Name == CookieName {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("ClearSession() did not set cookie")
	}
	if found.MaxAge >= 0 {
		t.Errorf("ClearSession() MaxAge = %d, want negative", found.MaxAge)
	}
}
