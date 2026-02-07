package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jmartynas/pss-backend/internal/auth"
)

type Config struct {
	Server   ServerConfig
	MySQL    MySQLConfig
	OAuth    OAuthConfig
	LogLevel string
}

type OAuthConfig struct {
	BaseURL    string
	JWTSecret  string
	SuccessURL string
	Providers  map[string]auth.ProviderConfig
}

type MySQLConfig struct {
	RawDSN   string
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

type ServerConfig struct {
	Port              int
	ReadTimeout       int
	WriteTimeout      int
	IdleTimeout       int
	ShutdownTimeout   int
	TLSCertFile       string
	TLSKeyFile        string
	TrustedProxyCIDRs string
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            getEnvInt("PORT", 8080),
			ReadTimeout:     getEnvInt("SERVER_READ_TIMEOUT", 15),
			WriteTimeout:    getEnvInt("SERVER_WRITE_TIMEOUT", 15),
			IdleTimeout:     getEnvInt("SERVER_IDLE_TIMEOUT", 60),
			ShutdownTimeout: getEnvInt("SERVER_SHUTDOWN_TIMEOUT", 30),
			TLSCertFile:       getEnv("TLS_CERT_FILE", ""),
			TLSKeyFile:        getEnv("TLS_KEY_FILE", ""),
			TrustedProxyCIDRs: getEnv("TRUSTED_PROXY_CIDRS", "127.0.0.0/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,::1/128,fc00::/7"),
		},
		MySQL: MySQLConfig{
			RawDSN:   getEnv("MYSQL_DSN", ""),
			Host:     getEnv("MYSQL_HOST", ""),
			Port:     getEnvInt("MYSQL_PORT", 3306),
			User:     getEnv("MYSQL_USER", "root"),
			Password: getEnv("MYSQL_PASSWORD", ""),
			Database: getEnv("MYSQL_DATABASE", "pss"),
		},
		OAuth:    loadOAuthConfig(),
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}
}

func (c MySQLConfig) DSN() string {
	if c.RawDSN != "" {
		if !strings.Contains(c.RawDSN, "multiStatements") {
			if strings.Contains(c.RawDSN, "?") {
				return c.RawDSN + "&multiStatements=true"
			}
			return c.RawDSN + "?multiStatements=true"
		}
		return c.RawDSN
	}
	if c.Host == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=true",
		c.User, c.Password, c.Host, c.Port, c.Database)
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func loadOAuthConfig() OAuthConfig {
	providers := make(map[string]auth.ProviderConfig)
	for name := range auth.Registry {
		id := getEnv("OAUTH_"+strings.ToUpper(name)+"_CLIENT_ID", "")
		secret := getEnv("OAUTH_"+strings.ToUpper(name)+"_CLIENT_SECRET", "")
		if id != "" && secret != "" {
			providers[strings.ToLower(name)] = auth.ProviderConfig{ClientID: id, ClientSecret: secret}
		}
	}
	return OAuthConfig{
		BaseURL:    getEnv("OAUTH_BASE_URL", ""),
		JWTSecret:  getEnv("OAUTH_JWT_SECRET", ""),
		SuccessURL: getEnv("OAUTH_SUCCESS_URL", "/"),
		Providers:  providers,
	}
}
