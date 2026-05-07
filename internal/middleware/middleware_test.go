package middleware

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseTrustedProxyCIDRs(t *testing.T) {
	tests := []struct {
		name    string
		csv     string
		wantErr bool
		wantLen int
	}{
		{"empty", "", false, 0},
		{"whitespace", "  ", false, 0},
		{"single valid", "127.0.0.0/8", false, 1},
		{"multiple valid", "127.0.0.0/8,10.0.0.0/8", false, 2},
		{"with spaces", " 127.0.0.0/8 , 10.0.0.0/8 ", false, 2},
		{"invalid CIDR", "not-a-cidr", true, 0},
		{"invalid in list", "127.0.0.0/8,invalid", true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTrustedProxyCIDRs(tt.csv)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTrustedProxyCIDRs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("ParseTrustedProxyCIDRs() len = %v, want %v", len(got), tt.wantLen)
			}
		})
	}
}

func TestParseTrustedProxyCIDRs_contains(t *testing.T) {
	nets, err := ParseTrustedProxyCIDRs("127.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 1 {
		t.Fatalf("expected 1 network, got %d", len(nets))
	}
	ip := net.ParseIP("127.0.0.1")
	if ip == nil {
		t.Fatal("parse IP")
	}
	if !nets[0].Contains(ip) {
		t.Error("expected 127.0.0.1 to be contained in 127.0.0.0/8")
	}
}

func TestRequestID_Generated(t *testing.T) {
	var capturedID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = GetRequestID(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if capturedID == "" {
		t.Error("expected request ID to be generated")
	}
}

func TestRequestID_Propagated(t *testing.T) {
	var capturedID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = GetRequestID(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "existing-id")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if capturedID != "existing-id" {
		t.Errorf("GetRequestID() = %q, want %q", capturedID, "existing-id")
	}
	if w.Header().Get("X-Request-ID") != "existing-id" {
		t.Errorf("response X-Request-ID = %q, want %q", w.Header().Get("X-Request-ID"), "existing-id")
	}
}

func TestGetRequestID_Empty(t *testing.T) {
	if got := GetRequestID(context.Background()); got != "" {
		t.Errorf("GetRequestID(empty ctx) = %q, want empty", got)
	}
}

func TestGetRealIP_Empty(t *testing.T) {
	if got := GetRealIP(context.Background()); got != "" {
		t.Errorf("GetRealIP(empty ctx) = %q, want empty", got)
	}
}

func TestRealIPWith_TrustedProxy_XRealIP(t *testing.T) {
	nets, _ := ParseTrustedProxyCIDRs("127.0.0.0/8")
	var capturedIP string
	handler := RealIPWith(nets)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedIP = GetRealIP(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Real-IP", "203.0.113.5")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if capturedIP != "203.0.113.5" {
		t.Errorf("GetRealIP() = %q, want %q", capturedIP, "203.0.113.5")
	}
}

func TestRealIPWith_TrustedProxy_XForwardedFor(t *testing.T) {
	nets, _ := ParseTrustedProxyCIDRs("127.0.0.0/8")
	var capturedIP string
	handler := RealIPWith(nets)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedIP = GetRealIP(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if capturedIP != "203.0.113.5" {
		t.Errorf("GetRealIP() = %q, want %q", capturedIP, "203.0.113.5")
	}
}

func TestRealIPWith_UntrustedProxy(t *testing.T) {
	nets, _ := ParseTrustedProxyCIDRs("10.0.0.0/8")
	var capturedIP string
	handler := RealIPWith(nets)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedIP = GetRealIP(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.1:1234"
	req.Header.Set("X-Real-IP", "1.2.3.4")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if capturedIP != "203.0.113.1" {
		t.Errorf("GetRealIP() = %q, want %q", capturedIP, "203.0.113.1")
	}
}

func TestNoCache_Headers(t *testing.T) {
	handler := NoCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	if w.Header().Get("Pragma") != "no-cache" {
		t.Errorf("Pragma = %q, want no-cache", w.Header().Get("Pragma"))
	}
}
