package db

import (
	"context"
	"fmt"
	"log/slog"

	"keeper/ent"
	"keeper/ent/migrate"
	"keeper/pkg/config"

	"entgo.io/ent/dialect"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// NewClient creates a new ent.Client based on the configured database driver.
// Supported drivers: "sqlite3" (default) and "postgres".
func NewClient(cfg config.DatabaseConfig) (*ent.Client, error) {
	switch cfg.Driver {
	case "postgres":
		return newPostgresClient(cfg.DSN)
	case "sqlite3", "":
		return NewSQLiteClient(cfg.Path)
	default:
		return nil, fmt.Errorf("unsupported database driver: %q", cfg.Driver)
	}
}

// NewSQLiteClient creates a new ent.Client for SQLite.
func NewSQLiteClient(path string) (*ent.Client, error) {
	slog.Info("opening sqlite connection", "path", path)
	client, err := ent.Open(dialect.SQLite, fmt.Sprintf("file:%s?cache=shared&_fk=1", path))
	if err != nil {
		slog.Error("failed to open sqlite connection", "path", path, "error", err)
		return nil, fmt.Errorf("failed opening connection to sqlite: %w", err)
	}
	return migrateClient(client)
}

// newPostgresClient creates a new ent.Client for PostgreSQL using the provided DSN.
func newPostgresClient(dsn string) (*ent.Client, error) {
	if dsn == "" {
		return nil, fmt.Errorf("postgres driver selected but DB.DSN is empty")
	}
	slog.Info("opening postgres connection")
	client, err := ent.Open(dialect.Postgres, dsn)
	if err != nil {
		slog.Error("failed to open postgres connection", "error", err)
		return nil, fmt.Errorf("failed opening connection to postgres: %w", err)
	}
	return migrateClient(client)
}

// migrateClient runs the ent auto-migration and returns the ready client.
func migrateClient(client *ent.Client) (*ent.Client, error) {
	slog.Info("running auto migration")
	if err := client.Schema.Create(context.Background(), migrate.WithGlobalUniqueID(true)); err != nil {
		slog.Error("failed to create schema resources", "error", err)
		if cerr := client.Close(); cerr != nil {
			slog.Error("failed to close client after schema creation failure", "error", cerr)
		}
		return nil, fmt.Errorf("failed creating schema resources: %w", err)
	}

	slog.Info("database initialization completed successfully")
	return client, nil
}
