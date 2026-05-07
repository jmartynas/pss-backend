package config

import (
	"errors"
	"testing"

	"github.com/jmartynas/pss-backend/internal/auth"
)

func TestMySQLConfig_DSN(t *testing.T) {
	tests := []struct {
		name string
		cfg  MySQLConfig
		want string
	}{
		{
			name: "empty host returns empty",
			cfg:  MySQLConfig{Host: ""},
			want: "",
		},
		{
			name: "host and credentials build DSN",
			cfg: MySQLConfig{
				Host:     "localhost",
				Port:     3306,
				User:     "u",
				Password: "p",
				Database: "db",
			},
			want: "u:p@tcp(localhost:3306)/db?parseTime=true&multiStatements=true",
		},
		{
			name: "raw DSN used when set",
			cfg:  MySQLConfig{RawDSN: "user:pass@tcp(host:3306)/db"},
			want: "user:pass@tcp(host:3306)/db?multiStatements=true",
		},
		{
			name: "raw DSN with query keeps multiStatements",
			cfg:  MySQLConfig{RawDSN: "user:pass@tcp(host:3306)/db?parseTime=true"},
			want: "user:pass@tcp(host:3306)/db?parseTime=true&multiStatements=true",
		},
		{
			name: "raw DSN with multiStatements unchanged",
			cfg:  MySQLConfig{RawDSN: "user:pass@tcp(host:3306)/db?multiStatements=true"},
			want: "user:pass@tcp(host:3306)/db?multiStatements=true",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.DSN()
			if got != tt.want {
				t.Errorf("DSN() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	validMySQL := MySQLConfig{Host: "localhost", User: "u", Database: "d"}

	t.Run("require MySQL missing", func(t *testing.T) {
		cfg := &Config{MySQL: MySQLConfig{}, LogLevel: "info"}
		err := cfg.Validate(true, false)
		if !errors.Is(err, ErrMySQLRequired) {
			t.Errorf("Validate(requireMySQL=true) = %v, want ErrMySQLRequired", err)
		}
	})
	t.Run("require MySQL ok", func(t *testing.T) {
		cfg := &Config{MySQL: validMySQL, LogLevel: "info"}
		if err := cfg.Validate(true, false); err != nil {
			t.Errorf("Validate(requireMySQL=true) = %v", err)
		}
	})
	t.Run("invalid log level", func(t *testing.T) {
		cfg := &Config{MySQL: validMySQL, LogLevel: "invalid"}
		err := cfg.Validate(true, false)
		if !errors.Is(err, ErrInvalidLogLevel) {
			t.Errorf("Validate(invalid log) = %v, want ErrInvalidLogLevel", err)
		}
	})
	t.Run("valid log levels", func(t *testing.T) {
		for _, level := range []string{"debug", "info", "warn", "error"} {
			cfg := &Config{MySQL: validMySQL, LogLevel: level}
			if err := cfg.Validate(true, false); err != nil {
				t.Errorf("Validate(logLevel=%q) = %v", level, err)
			}
		}
	})
	t.Run("require OAuth incomplete", func(t *testing.T) {
		cfg := &Config{MySQL: validMySQL, LogLevel: "info", OAuth: OAuthConfig{}}
		err := cfg.Validate(false, true)
		if !errors.Is(err, ErrOAuthIncomplete) {
			t.Errorf("Validate(requireOAuth, empty) = %v, want ErrOAuthIncomplete", err)
		}
	})
	t.Run("require OAuth short secret", func(t *testing.T) {
		cfg := &Config{
			MySQL: validMySQL, LogLevel: "info",
			OAuth: OAuthConfig{
				BaseURL:   "https://a.com",
				JWTSecret: "short",
				Providers: map[string]auth.ProviderConfig{"google": {ClientID: "a", ClientSecret: "b"}},
			},
		}
		err := cfg.Validate(false, true)
		if !errors.Is(err, ErrJWTSecretLength) {
			t.Errorf("Validate(short JWT secret) = %v, want ErrJWTSecretLength", err)
		}
	})
	t.Run("require OAuth ok", func(t *testing.T) {
		cfg := &Config{
			MySQL: validMySQL, LogLevel: "info",
			OAuth: OAuthConfig{
				BaseURL:   "https://a.com",
				JWTSecret: "this-secret-is-at-least-32-characters-long",
				Providers: map[string]auth.ProviderConfig{"google": {ClientID: "a", ClientSecret: "b"}},
			},
		}
		if err := cfg.Validate(false, true); err != nil {
			t.Errorf("Validate(requireOAuth, valid) = %v", err)
		}
	})
}
