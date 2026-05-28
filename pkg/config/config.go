package config

import (
	"fmt"
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
	Path string `mapstructure:"PATH"`
}

// LogConfig holds the logging-specific configuration.
type LogConfig struct {
	Dir string `mapstructure:"DIR"`
}

// AuthConfig holds the authentication-specific configuration.
type AuthConfig struct {
	JWTSecret string        `mapstructure:"JWT_SECRET"`
	JWTExpiry time.Duration `mapstructure:"JWT_EXPIRY"`
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
	v.SetDefault("DB.PATH", "keeper.db")
	v.SetDefault("LOG.DIR", "log")
	v.SetDefault("AUTH.JWT_SECRET", "a-very-secure-and-shared-secret-key")
	v.SetDefault("AUTH.JWT_EXPIRY", 24*time.Hour)
	v.SetDefault("SEED.ADMIN_EMAIL", "admin@admin.com")
	v.SetDefault("SEED.ADMIN_PASSWORD", "admin")
	v.SetDefault("CORS.ALLOWED_ORIGINS", []string{"*"})

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

	return &cfg, nil
}
