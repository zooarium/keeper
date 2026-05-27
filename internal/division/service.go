package division

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// DivisionRepository defines the data access contract for divisions.
type DivisionRepository interface {
	Create(ctx context.Context, d Division, parentPath string) (*Division, error)
	GetByID(ctx context.Context, appID, id int) (*Division, error)
	List(ctx context.Context, appID int, parentID *int) ([]*Division, error)
	Descendants(ctx context.Context, appID int, path string) ([]*Division, error)
	Update(ctx context.Context, appID, id int, d *Division) (*Division, error)
	CascadeUpdatePath(ctx context.Context, id int, oldPath, newPath string) error
	Move(ctx context.Context, id int, newParentID *int) error
	CountChildren(ctx context.Context, id int) (int, error)
	CountUsers(ctx context.Context, id int) (int, error)
	Delete(ctx context.Context, appID, id int) error
}

// DivisionService defines the business logic for divisions.
type DivisionService interface {
	Create(ctx context.Context, req CreateDivisionRequest) (*Division, error)
	GetByID(ctx context.Context, appID, id int) (*Division, error)
	List(ctx context.Context, appID int, parentID *int) ([]*Division, error)
	Descendants(ctx context.Context, appID, id int) ([]*Division, error)
	Update(ctx context.Context, appID, id int, req UpdateDivisionRequest) (*Division, error)
	Move(ctx context.Context, appID, id int, req MoveDivisionRequest) (*Division, error)
	Delete(ctx context.Context, appID, id int) error
}

type divisionService struct {
	repo DivisionRepository
}

// NewDivisionService creates a new division service.
func NewDivisionService(repo DivisionRepository) DivisionService {
	return &divisionService{repo: repo}
}

func (s *divisionService) Create(ctx context.Context, req CreateDivisionRequest) (*Division, error) {
	slog.Info("creating division", "app_id", req.AppID, "name", req.Name, "parent_id", req.ParentID)

	parentPath := "/"
	if req.ParentID != nil {
		parent, err := s.repo.GetByID(ctx, req.AppID, *req.ParentID)
		if err != nil {
			slog.Warn("create division failed: parent not found", "parent_id", *req.ParentID, "app_id", req.AppID)
			return nil, fmt.Errorf("parent division not found")
		}
		if parent.Status != 1 {
			slog.Warn("create division failed: parent inactive", "parent_id", *req.ParentID)
			return nil, errors.New("parent division is inactive")
		}
		parentPath = parent.Path
	}

	created, err := s.repo.Create(ctx, Division{
		AppID:    req.AppID,
		ParentID: req.ParentID,
		Name:     req.Name,
		Status:   1,
	}, parentPath)
	if err != nil {
		slog.Error("failed to create division in repository", "name", req.Name, "error", err)
		return nil, fmt.Errorf("repository create: %w", err)
	}

	slog.Info("division created successfully", "id", created.ID, "name", created.Name, "path", created.Path)
	return created, nil
}

func (s *divisionService) GetByID(ctx context.Context, appID, id int) (*Division, error) {
	slog.Info("getting division by id", "id", id, "app_id", appID)
	return s.repo.GetByID(ctx, appID, id)
}

func (s *divisionService) List(ctx context.Context, appID int, parentID *int) ([]*Division, error) {
	slog.Info("listing divisions", "app_id", appID, "parent_id", parentID)
	return s.repo.List(ctx, appID, parentID)
}

func (s *divisionService) Descendants(ctx context.Context, appID, id int) ([]*Division, error) {
	slog.Info("getting descendants", "id", id, "app_id", appID)
	d, err := s.repo.GetByID(ctx, appID, id)
	if err != nil {
		return nil, err
	}
	return s.repo.Descendants(ctx, appID, d.Path)
}

func (s *divisionService) Update(ctx context.Context, appID, id int, req UpdateDivisionRequest) (*Division, error) {
	slog.Info("updating division", "id", id, "app_id", appID)
	existing, err := s.repo.GetByID(ctx, appID, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Status != nil {
		existing.Status = *req.Status
	}

	updated, err := s.repo.Update(ctx, appID, id, existing)
	if err != nil {
		slog.Error("failed to update division in repository", "id", id, "error", err)
		return nil, err
	}

	slog.Info("division updated successfully", "id", id)
	return updated, nil
}

func (s *divisionService) Move(ctx context.Context, appID, id int, req MoveDivisionRequest) (*Division, error) {
	slog.Info("moving division", "id", id, "app_id", appID, "new_parent_id", req.ParentID)

	d, err := s.repo.GetByID(ctx, appID, id)
	if err != nil {
		return nil, err
	}

	newParentPath := "/"
	if req.ParentID != nil {
		// Cannot move to self
		if *req.ParentID == id {
			return nil, errors.New("cannot move division to itself")
		}

		newParent, err := s.repo.GetByID(ctx, appID, *req.ParentID)
		if err != nil {
			slog.Warn("move division failed: new parent not found", "parent_id", *req.ParentID)
			return nil, fmt.Errorf("parent division not found")
		}

		// Cycle check: new parent must not be a descendant of the division being moved
		if len(newParent.Path) >= len(d.Path) && newParent.Path[:len(d.Path)] == d.Path {
			slog.Warn("move division failed: cycle detected", "id", id, "new_parent_id", *req.ParentID)
			return nil, errors.New("cannot move division to its own descendant")
		}

		newParentPath = newParent.Path
	}

	oldPath := d.Path
	newPath := fmt.Sprintf("%s%d/", newParentPath, id)

	if err := s.repo.CascadeUpdatePath(ctx, id, oldPath, newPath); err != nil {
		slog.Error("failed to cascade update paths", "id", id, "error", err)
		return nil, fmt.Errorf("path update failed: %w", err)
	}

	if err := s.repo.Move(ctx, id, req.ParentID); err != nil {
		slog.Error("failed to move division", "id", id, "error", err)
		return nil, fmt.Errorf("move failed: %w", err)
	}

	slog.Info("division moved successfully", "id", id, "old_path", oldPath, "new_path", newPath)
	return s.repo.GetByID(ctx, appID, id)
}

func (s *divisionService) Delete(ctx context.Context, appID, id int) error {
	slog.Info("deleting division", "id", id, "app_id", appID)

	children, err := s.repo.CountChildren(ctx, id)
	if err != nil {
		slog.Error("failed to count children for division", "id", id, "error", err)
		return err
	}
	if children > 0 {
		slog.Warn("delete division rejected: has children", "id", id, "children", children)
		return errors.New("division has children; remove them first")
	}

	users, err := s.repo.CountUsers(ctx, id)
	if err != nil {
		slog.Error("failed to count users for division", "id", id, "error", err)
		return err
	}
	if users > 0 {
		slog.Warn("delete division rejected: has users", "id", id, "users", users)
		return errors.New("division has users; reassign them first")
	}

	if err := s.repo.Delete(ctx, appID, id); err != nil {
		slog.Error("failed to delete division", "id", id, "error", err)
		return err
	}

	slog.Info("division deleted successfully", "id", id)
	return nil
}
