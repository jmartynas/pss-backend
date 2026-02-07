package middleware

import (
	"net"
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
