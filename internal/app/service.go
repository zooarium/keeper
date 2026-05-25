package app

import (
	"context"
	"fmt"
	"log/slog"
)

// AppRepository defines the data access contract for apps.
type AppRepository interface {
	Create(ctx context.Context, a App) (*App, error)
	GetByID(ctx context.Context, id int) (*App, error)
	List(ctx context.Context) ([]*App, error)
	Update(ctx context.Context, id int, a *App) (*App, error)
	Delete(ctx context.Context, id int) error
}

// AppService defines the business logic for apps.
type AppService interface {
	Create(ctx context.Context, req CreateAppRequest) (*App, error)
	GetByID(ctx context.Context, id int) (*App, error)
	List(ctx context.Context) ([]*App, error)
	Update(ctx context.Context, id int, req UpdateAppRequest) (*App, error)
	Delete(ctx context.Context, id int) error
}

type appService struct {
	repo AppRepository
}

// NewAppService creates a new app service.
func NewAppService(repo AppRepository) AppService {
	return &appService{repo: repo}
}

func (s *appService) Create(ctx context.Context, req CreateAppRequest) (*App, error) {
	slog.Info("creating app", "name", req.Name)

	status := int8(1)
	if req.Status != 0 {
		status = req.Status
	}

	created, err := s.repo.Create(ctx, App{
		Name:   req.Name,
		Status: status,
	})
	if err != nil {
		return nil, fmt.Errorf("repository create: %w", err)
	}

	slog.Info("app created successfully", "id", created.ID, "name", created.Name)
	return created, nil
}

func (s *appService) GetByID(ctx context.Context, id int) (*App, error) {
	slog.Info("getting app by id", "id", id)
	return s.repo.GetByID(ctx, id)
}

func (s *appService) List(ctx context.Context) ([]*App, error) {
	slog.Info("listing apps")
	return s.repo.List(ctx)
}

func (s *appService) Update(ctx context.Context, id int, req UpdateAppRequest) (*App, error) {
	slog.Info("updating app", "id", id)
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Status != nil {
		existing.Status = *req.Status
	}

	updated, err := s.repo.Update(ctx, id, existing)
	if err != nil {
		return nil, err
	}

	slog.Info("app updated successfully", "id", id)
	return updated, nil
}

func (s *appService) Delete(ctx context.Context, id int) error {
	slog.Info("deleting app", "id", id)
	err := s.repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	slog.Info("app deleted successfully", "id", id)
	return nil
}
