package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

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
