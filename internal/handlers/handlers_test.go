package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	Health(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Health status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	if body != `{"status":"ok"}` && body != "{\"status\":\"ok\"}\n" {
		t.Errorf("body = %q", body)
	}
}

func TestReady_NoDB(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	Ready(nil)(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Ready(nil) status = %d, want 503", rec.Code)
	}
}

func TestReady_WithDB(t *testing.T) {
	// Without a real DB we cannot test 200; we'd need an integration test or mock.
	// Passing a closed or invalid *sql.DB would panic on PingContext, so we only test nil.
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	var db *sql.DB
	Ready(db)(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Ready(nil db) status = %d, want 503", rec.Code)
	}
}
