package db

import (
	"context"
	"fmt"
	"log/slog"

	"keeper/ent"
	"keeper/ent/app"
	"keeper/pkg/config"

	"golang.org/x/crypto/bcrypt"
)

const systemAppName = "System"

// Seed ensures one System app, one System division, and one sysadmin user exist.
// Safe to call on every startup — skips if System app already present.
func Seed(ctx context.Context, client *ent.Client, cfg config.SeedConfig) error {
	exists, err := client.App.Query().Where(app.NameEQ(systemAppName)).Exist(ctx)
	if err != nil {
		return fmt.Errorf("seed: check system app: %w", err)
	}
	if exists {
		slog.Info("seed: system app already present, skipping")
		return nil
	}

	slog.Info("seed: creating system app, division and sysadmin user")

	systemApp, err := client.App.Create().
		SetName(systemAppName).
		SetCurrency("INR").
		SetStatus(1).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("seed: create system app: %w", err)
	}

	systemDiv, err := client.Division.Create().
		SetAppID(systemApp.ID).
		SetName(systemAppName).
		SetPath("").
		SetDepth(0).
		SetStatus(1).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("seed: create system division: %w", err)
	}

	finalPath := fmt.Sprintf("/%d/", systemDiv.ID)
	systemDiv, err = client.Division.UpdateOneID(systemDiv.ID).
		SetPath(finalPath).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("seed: set division path: %w", err)
	}

	if cfg.AdminPassword == "admin" {
		slog.Warn("seed: using default sysadmin password — change SEED_ADMIN_PASSWORD in config")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("seed: hash sysadmin password: %w", err)
	}

	_, err = client.User.Create().
		SetAppID(systemApp.ID).
		SetDivisionID(systemDiv.ID).
		SetFirstname("System").
		SetLastname("Admin").
		SetEmail(cfg.AdminEmail).
		SetPassword(string(hashed)).
		SetRole(1).
		SetStatus(1).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("seed: create sysadmin user: %w", err)
	}

	slog.Info("seed: done", "app_id", systemApp.ID, "division_id", systemDiv.ID, "admin_email", cfg.AdminEmail)
	return nil
}
