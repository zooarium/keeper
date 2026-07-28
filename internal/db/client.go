package db

import (
	"context"
	"fmt"
	"log/slog"

	"keeper/ent"
	"keeper/ent/migrate"
	"keeper/pkg/config"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// NewClient creates a new ent.Client based on the configured database driver.
// Supported drivers: "sqlite3" (default) and "postgres". The returned
// *entsql.Driver is the same connection used by the client; keep it around
// for readiness pings (ent.Client exposes no driver accessor of its own).
func NewClient(cfg config.DatabaseConfig) (*ent.Client, *entsql.Driver, error) {
	switch cfg.Driver {
	case "postgres":
		return newPostgresClient(cfg.DSN)
	case "sqlite3", "":
		return NewSQLiteClient(cfg.Path)
	default:
		return nil, nil, fmt.Errorf("unsupported database driver: %q", cfg.Driver)
	}
}

// NewSQLiteClient creates a new ent.Client for SQLite.
func NewSQLiteClient(path string) (*ent.Client, *entsql.Driver, error) {
	slog.Info("opening sqlite connection", "path", path)
	drv, err := entsql.Open(dialect.SQLite, fmt.Sprintf("file:%s?cache=shared&_fk=1&_journal_mode=WAL&_busy_timeout=5000", path))
	if err != nil {
		slog.Error("failed to open sqlite connection", "path", path, "error", err)
		return nil, nil, fmt.Errorf("failed opening connection to sqlite: %w", err)
	}
	client, err := migrateClient(ent.NewClient(ent.Driver(drv)))
	if err != nil {
		return nil, nil, err
	}
	return client, drv, nil
}

// newPostgresClient creates a new ent.Client for PostgreSQL using the provided DSN.
func newPostgresClient(dsn string) (*ent.Client, *entsql.Driver, error) {
	if dsn == "" {
		return nil, nil, fmt.Errorf("postgres driver selected but DB.DSN is empty")
	}
	slog.Info("opening postgres connection")
	drv, err := entsql.Open(dialect.Postgres, dsn)
	if err != nil {
		slog.Error("failed to open postgres connection", "error", err)
		return nil, nil, fmt.Errorf("failed opening connection to postgres: %w", err)
	}
	client, err := migrateClient(ent.NewClient(ent.Driver(drv)))
	if err != nil {
		return nil, nil, err
	}
	return client, drv, nil
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

// Ping verifies the database connection is alive, for use by readiness
// checks. Works uniformly across sqlite3/postgres since it's a plain
// connection ping, not a query.
func Ping(ctx context.Context, drv *entsql.Driver) error {
	return drv.DB().PingContext(ctx)
}
