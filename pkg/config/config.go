package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all the configuration for the application.
type Config struct {
	Environment string `mapstructure:"ENVIRONMENT"`
	Server      ServerConfig
	DB          DatabaseConfig `mapstructure:"DB"`
	Log         LogConfig
	Auth        AuthConfig
	Seed        SeedConfig
	CORS        CORSConfig
	Secondary   []SecondaryConfig `mapstructure:"SECONDARY"`
	Services    []ServiceEntry    `mapstructure:"SERVICES"`
	Falcon      FalconConfig      `mapstructure:"FALCON"`
}

// FalconConfig points login's role resolution at falcon's internal-s2s
// listener. Falcon's uptime is a hard dependency of login (fail-closed) —
// see internal/user.Service.Authenticate.
type FalconConfig struct {
	BaseURL string        `mapstructure:"BASE_URL"`
	Timeout time.Duration `mapstructure:"TIMEOUT"`
}

// ServiceEntry registers a downstream service that can be impersonated into.
// It drives three things at once: the redirect allow-list (keeper-ui may only
// hand off to a registered UIExchangeURL), the CORS allow-list for the public
// exchange endpoint (only registered UIOrigins may call it), and the audience
// stamped into the minted impersonation token (downstream rejects a token whose
// audience is not its own service key). Adding a new service is a single block
// here plus that UI implementing the shared exchange route.
type ServiceEntry struct {
	// Key uniquely identifies the service (e.g. "squirrel", "ant").
	Key string `mapstructure:"KEY"`
	// Audience stamped into the token's aud claim. Defaults to Key when empty.
	Audience string `mapstructure:"AUDIENCE"`
	// UIExchangeURL is the absolute URL of that service UI's exchange page,
	// opened with the one-time code in the fragment.
	UIExchangeURL string `mapstructure:"UI_EXCHANGE_URL"`
	// UIOrigin is the scheme://host[:port] of that service UI, used as the CORS
	// allow-list entry for the public exchange endpoint.
	UIOrigin string `mapstructure:"UI_ORIGIN"`
}

// SecondaryConfig drives one optional secondary listener: an additional HTTP
// server in the same process exposing only the allow-listed routes, with
// rate limiting configured independently of the primary server. Any number
// of listeners can be declared under SECONDARY. Identity always comes from
// JWT; JWT_SECRET (optional) makes the listener verify with a different
// signing key (e.g. the guest secret) instead of the primary AUTH.JWT_SECRET.
type SecondaryConfig struct {
	Name      string          `mapstructure:"NAME"`
	Enabled   bool            `mapstructure:"ENABLED"`
	Addr      string          `mapstructure:"ADDR"`
	JWTSecret string          `mapstructure:"JWT_SECRET"`
	RateLimit RateLimitConfig `mapstructure:"RATE_LIMIT"`
	Routes    []string        `mapstructure:"ROUTES"`
}

// RateLimitConfig holds rate limiter settings for a secondary listener.
type RateLimitConfig struct {
	Requests int           `mapstructure:"REQUESTS"`
	Window   time.Duration `mapstructure:"WINDOW"`
}

// CORSConfig holds the CORS-specific configuration.
type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"ALLOWED_ORIGINS"`
}

// ServerConfig holds the server-specific configuration.
type ServerConfig struct {
	Addr         string        `mapstructure:"ADDR"`
	Host         string        `mapstructure:"HOST"`
	ReadTimeout  time.Duration `mapstructure:"READ_TIMEOUT"`
	WriteTimeout time.Duration `mapstructure:"WRITE_TIMEOUT"`
	IdleTimeout  time.Duration `mapstructure:"IDLE_TIMEOUT"`
}

// DatabaseConfig holds the database-specific configuration.
type DatabaseConfig struct {
	Driver string `mapstructure:"DRIVER"`
	Path   string `mapstructure:"PATH"`
	DSN    string `mapstructure:"DSN"`
}

// LogConfig holds the logging-specific configuration.
type LogConfig struct {
	Dir   string `mapstructure:"DIR"`
	Level string `mapstructure:"LEVEL"`
}

// AuthConfig holds the authentication-specific configuration.
type AuthConfig struct {
	JWTSecret string        `mapstructure:"JWT_SECRET"`
	JWTExpiry time.Duration `mapstructure:"JWT_EXPIRY"`
	// Guest tokens are signed with a separate secret so they are
	// cryptographically useless on surfaces that verify with JWT_SECRET.
	GuestJWTSecret string        `mapstructure:"GUEST_JWT_SECRET"`
	GuestJWTExpiry time.Duration `mapstructure:"GUEST_JWT_EXPIRY"`
	// Impersonation tokens are signed with their own secret so they only verify
	// on surfaces explicitly configured with it (and never on primary/guest
	// surfaces). Kept deliberately short-lived.
	ImpersonationJWTSecret string        `mapstructure:"IMPERSONATION_JWT_SECRET"`
	ImpersonationJWTExpiry time.Duration `mapstructure:"IMPERSONATION_JWT_EXPIRY"`
}

// SeedConfig holds credentials for the bootstrapped sysadmin user.
type SeedConfig struct {
	AdminEmail    string `mapstructure:"ADMIN_EMAIL"`
	AdminPassword string `mapstructure:"ADMIN_PASSWORD"`
}

// Load loads the configuration from files and environment variables.
func Load() (*Config, error) {
	v := viper.New()

	// Default values
	v.SetDefault("ENVIRONMENT", "production")
	v.SetDefault("SERVER.ADDR", ":8080")
	v.SetDefault("SERVER.HOST", "localhost:8080")
	v.SetDefault("SERVER.READ_TIMEOUT", 5*time.Second)
	v.SetDefault("SERVER.WRITE_TIMEOUT", 10*time.Second)
	v.SetDefault("SERVER.IDLE_TIMEOUT", 120*time.Second)
	v.SetDefault("DB.DRIVER", "sqlite3")
	v.SetDefault("DB.PATH", "keeper.db")
	v.SetDefault("DB.DSN", "")
	v.SetDefault("LOG.DIR", "log")
	v.SetDefault("LOG.LEVEL", "info")
	v.SetDefault("AUTH.JWT_SECRET", "a-very-secure-and-shared-secret-key")
	v.SetDefault("AUTH.JWT_EXPIRY", 24*time.Hour)
	v.SetDefault("AUTH.GUEST_JWT_SECRET", "a-separate-guest-token-secret-key")
	v.SetDefault("AUTH.GUEST_JWT_EXPIRY", 30*time.Minute)
	v.SetDefault("AUTH.IMPERSONATION_JWT_SECRET", "a-separate-impersonation-token-secret-key")
	v.SetDefault("AUTH.IMPERSONATION_JWT_EXPIRY", 10*time.Minute)
	v.SetDefault("SEED.ADMIN_EMAIL", "admin@admin.com")
	v.SetDefault("SEED.ADMIN_PASSWORD", "admin")
	v.SetDefault("CORS.ALLOWED_ORIGINS", []string{"*"})
	v.SetDefault("FALCON.BASE_URL", "http://falcon:9091")
	v.SetDefault("FALCON.TIMEOUT", 3*time.Second)

	// Environment variables
	v.SetEnvPrefix("KEEPER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	if err := v.BindEnv("DB.PATH", "DB_PATH"); err != nil {
		return nil, fmt.Errorf("failed to bind env DB_PATH: %w", err)
	}
	v.AutomaticEnv()

	// Config file
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	// 1. Try to load base config.yaml
	v.SetConfigName("config")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read base config file: %w", err)
		}
	}

	// 2. Try to load environment-specific config (e.g. config.dev.yaml)
	env := v.GetString("ENVIRONMENT")
	if env != "" {
		v.SetConfigName(fmt.Sprintf("config.%s", env))
		if err := v.MergeInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, fmt.Errorf("failed to merge environment-specific config file: %w", err)
			}
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := normalizeSecondary(&cfg); err != nil {
		return nil, err
	}

	if err := normalizeServices(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// normalizeServices validates the impersonation service registry and applies
// per-entry defaults (viper defaults cannot reach into list elements). Keys and
// origins must be unique; exchange URL and origin are required and must be
// http(s); in production both must be https.
func normalizeServices(cfg *Config) error {
	prod := cfg.Environment == "production"
	seenKey := map[string]bool{}
	seenOrigin := map[string]bool{}
	for i := range cfg.Services {
		s := &cfg.Services[i]
		if s.Key == "" {
			return fmt.Errorf("SERVICES[%d]: KEY is required", i)
		}
		if seenKey[s.Key] {
			return fmt.Errorf("SERVICES[%d]: KEY %q already in use by another service", i, s.Key)
		}
		seenKey[s.Key] = true

		if s.Audience == "" {
			s.Audience = s.Key
		}
		if s.UIExchangeURL == "" {
			return fmt.Errorf("SERVICES[%d] (%s): UI_EXCHANGE_URL is required", i, s.Key)
		}
		if err := validateHTTPURL(s.UIExchangeURL, prod); err != nil {
			return fmt.Errorf("SERVICES[%d] (%s): UI_EXCHANGE_URL %v", i, s.Key, err)
		}
		if s.UIOrigin == "" {
			return fmt.Errorf("SERVICES[%d] (%s): UI_ORIGIN is required", i, s.Key)
		}
		if err := validateHTTPURL(s.UIOrigin, prod); err != nil {
			return fmt.Errorf("SERVICES[%d] (%s): UI_ORIGIN %v", i, s.Key, err)
		}
		if seenOrigin[s.UIOrigin] {
			return fmt.Errorf("SERVICES[%d] (%s): UI_ORIGIN %q already in use by another service", i, s.Key, s.UIOrigin)
		}
		seenOrigin[s.UIOrigin] = true
	}
	return nil
}

// validateHTTPURL checks that raw is an absolute http(s) URL. When prod is true
// only https is accepted — except for loopback hosts (localhost/127.0.0.1/::1),
// which may use http so local development works without TLS.
func validateHTTPURL(raw string, prod bool) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("is not a valid URL: %w", err)
	}
	if u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("must be an absolute http(s) URL, got %q", raw)
	}
	if prod && u.Scheme != "https" && !isLoopbackHost(u.Hostname()) {
		return fmt.Errorf("must be https in production, got %q", raw)
	}
	return nil
}

// isLoopbackHost reports whether host is a loopback address where plain http is
// acceptable even in production (local development).
func isLoopbackHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

// normalizeSecondary validates the secondary listener entries and applies
// per-entry defaults (viper defaults cannot reach into list elements).
func normalizeSecondary(cfg *Config) error {
	seen := map[string]bool{cfg.Server.Addr: true}
	for i := range cfg.Secondary {
		s := &cfg.Secondary[i]
		if !s.Enabled {
			continue
		}
		if s.Name == "" {
			s.Name = fmt.Sprintf("secondary-%d", i)
		}
		if s.Addr == "" {
			return fmt.Errorf("SECONDARY[%d] (%s): ADDR is required", i, s.Name)
		}
		if seen[s.Addr] {
			return fmt.Errorf("SECONDARY[%d] (%s): ADDR %q already in use by another listener", i, s.Name, s.Addr)
		}
		seen[s.Addr] = true
		if s.RateLimit.Requests <= 0 {
			s.RateLimit.Requests = 100
		}
		if s.RateLimit.Window <= 0 {
			s.RateLimit.Window = 1 * time.Minute
		}
	}
	return nil
}
