package app

import (
	"context"
	"fmt"
	"log/slog"

	"keeper/ent"
	entapp "keeper/ent/app"
)

type appRepository struct {
	client *ent.Client
}

// NewAppRepository creates a new app repository.
func NewAppRepository(client *ent.Client) *appRepository {
	return &appRepository{client: client}
}

func (r *appRepository) Create(ctx context.Context, a App) (*App, error) {
	created, err := r.client.App.
		Create().
		SetName(a.Name).
		SetStatus(a.Status).
		Save(ctx)
	if err != nil {
		slog.Error("database error: failed to create app", "name", a.Name, "error", err)
		return nil, err
	}
	return r.mapToModel(created), nil
}

func (r *appRepository) GetByID(ctx context.Context, id int) (*App, error) {
	a, err := r.client.App.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.Warn("app not found in database", "id", id)
			return nil, fmt.Errorf("app not found: %w", err)
		}
		slog.Error("database error: failed to get app by id", "id", id, "error", err)
		return nil, err
	}
	return r.mapToModel(a), nil
}

func (r *appRepository) List(ctx context.Context, limit, offset int) ([]*App, error) {
	apps, err := r.client.App.Query().
		Order(ent.Asc(entapp.FieldID)).
		Limit(limit).
		Offset(offset).
		All(ctx)
	if err != nil {
		slog.Error("database error: failed to list apps", "error", err)
		return nil, err
	}
	result := make([]*App, len(apps))
	for i, a := range apps {
		result[i] = r.mapToModel(a)
	}
	return result, nil
}

func (r *appRepository) Update(ctx context.Context, id int, a *App) (*App, error) {
	updated, err := r.client.App.UpdateOneID(id).
		SetName(a.Name).
		SetStatus(a.Status).
		Save(ctx)
	if err != nil {
		slog.Error("database error: failed to update app", "id", id, "error", err)
		return nil, err
	}
	return r.mapToModel(updated), nil
}

func (r *appRepository) Delete(ctx context.Context, id int) error {
	err := r.client.App.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.Warn("app not found for deletion", "id", id)
			return fmt.Errorf("app not found: %w", err)
		}
		slog.Error("database error: failed to delete app", "id", id, "error", err)
		return err
	}
	return nil
}

func (r *appRepository) mapToModel(a *ent.App) *App {
	return &App{
		ID:        a.ID,
		Name:      a.Name,
		Status:    a.Status,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}
