package division

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"keeper/ent"
	entdivision "keeper/ent/division"
	"keeper/ent/predicate"
)

type divisionRepository struct {
	client *ent.Client
}

// NewDivisionRepository creates a new division repository.
func NewDivisionRepository(client *ent.Client) *divisionRepository {
	return &divisionRepository{client: client}
}

// Create inserts a new division. parentPath must be the parent's path ("/" for root).
// Path and depth are computed after the record is saved to obtain the new ID.
func (r *divisionRepository) Create(ctx context.Context, d Division, parentPath string) (*Division, error) {
	q := r.client.Division.Create().
		SetAppID(d.AppID).
		SetName(d.Name).
		SetPath("").
		SetDepth(0).
		SetStatus(d.Status)

	if d.ParentID != nil {
		q = q.SetParentID(*d.ParentID)
	}

	created, err := q.Save(ctx)
	if err != nil {
		slog.Error("database error: failed to create division", "name", d.Name, "error", err)
		return nil, err
	}

	finalPath := fmt.Sprintf("%s%d/", parentPath, created.ID)
	finalDepth := int8(strings.Count(finalPath, "/") - 2)

	updated, err := r.client.Division.UpdateOneID(created.ID).
		SetPath(finalPath).
		SetDepth(finalDepth).
		Save(ctx)
	if err != nil {
		slog.Error("database error: failed to set path for division", "id", created.ID, "error", err)
		_ = r.client.Division.DeleteOneID(created.ID).Exec(ctx)
		return nil, err
	}

	return r.mapToModel(updated), nil
}

// GetByID returns a division by ID scoped to an app. appID=0 bypasses the app filter.
func (r *divisionRepository) GetByID(ctx context.Context, appID, id int) (*Division, error) {
	q := r.client.Division.Query().Where(entdivision.IDEQ(id))
	if appID != 0 {
		q = q.Where(entdivision.AppIDEQ(appID))
	}
	d, err := q.Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.Warn("division not found in database", "id", id, "app_id", appID)
			return nil, fmt.Errorf("division not found: %w", err)
		}
		slog.Error("database error: failed to get division by id", "id", id, "error", err)
		return nil, err
	}
	return r.mapToModel(d), nil
}

// List returns divisions scoped to an app. appID=0 returns all apps. If parentID is nil, returns all divisions.
func (r *divisionRepository) List(ctx context.Context, appID int, parentID *int) ([]*Division, error) {
	q := r.client.Division.Query()
	if appID != 0 {
		q = q.Where(entdivision.AppIDEQ(appID))
	}
	if parentID != nil {
		q = q.Where(entdivision.ParentIDEQ(*parentID))
	}
	divisions, err := q.All(ctx)
	if err != nil {
		slog.Error("database error: failed to list divisions", "app_id", appID, "error", err)
		return nil, err
	}
	result := make([]*Division, len(divisions))
	for i, d := range divisions {
		result[i] = r.mapToModel(d)
	}
	return result, nil
}

// Descendants returns all descendants of the division identified by path. appID=0 bypasses the app filter.
func (r *divisionRepository) Descendants(ctx context.Context, appID int, path string) ([]*Division, error) {
	predicates := []predicate.Division{
		entdivision.PathHasPrefix(path),
		entdivision.PathNEQ(path),
	}
	if appID != 0 {
		predicates = append(predicates, entdivision.AppIDEQ(appID))
	}

	divisions, err := r.client.Division.Query().
		Where(predicates...).
		All(ctx)
	if err != nil {
		slog.Error("database error: failed to get descendants", "path", path, "error", err)
		return nil, err
	}
	result := make([]*Division, len(divisions))
	for i, d := range divisions {
		result[i] = r.mapToModel(d)
	}
	return result, nil
}

// Update updates name and status of a division. appID=0 bypasses the app filter.
func (r *divisionRepository) Update(ctx context.Context, appID, id int, d *Division) (*Division, error) {
	q := r.client.Division.Update().Where(entdivision.IDEQ(id))
	if appID != 0 {
		q = q.Where(entdivision.AppIDEQ(appID))
	}
	count, err := q.
		SetName(d.Name).
		SetStatus(d.Status).
		Save(ctx)
	if err != nil {
		slog.Error("database error: failed to update division", "id", id, "error", err)
		return nil, err
	}
	if count == 0 {
		slog.Warn("division not found for update", "id", id, "app_id", appID)
		return nil, fmt.Errorf("division not found")
	}
	return r.GetByID(ctx, appID, id)
}

// CascadeUpdatePath updates path and depth for a division and all its descendants.
// oldPath is the current path of the moved node; newPath is its new path.
func (r *divisionRepository) CascadeUpdatePath(ctx context.Context, id int, oldPath, newPath string) error {
	// Fetch all affected divisions (node + descendants)
	affected, err := r.client.Division.Query().
		Where(entdivision.PathHasPrefix(oldPath)).
		All(ctx)
	if err != nil {
		slog.Error("database error: failed to fetch divisions for path cascade", "old_path", oldPath, "error", err)
		return err
	}

	for _, d := range affected {
		updatedPath := newPath + d.Path[len(oldPath):]
		updatedDepth := int8(strings.Count(updatedPath, "/") - 2)
		_, err := r.client.Division.UpdateOneID(d.ID).
			SetPath(updatedPath).
			SetDepth(updatedDepth).
			Save(ctx)
		if err != nil {
			slog.Error("database error: failed to cascade update path", "id", d.ID, "error", err)
			return err
		}
	}
	return nil
}

// Move updates the parent_id of a division.
func (r *divisionRepository) Move(ctx context.Context, id int, newParentID *int) error {
	u := r.client.Division.UpdateOneID(id)
	if newParentID != nil {
		u = u.SetParentID(*newParentID)
	} else {
		u = u.ClearParentID()
	}
	_, err := u.Save(ctx)
	if err != nil {
		slog.Error("database error: failed to move division", "id", id, "error", err)
		return err
	}
	return nil
}

// CountChildren returns the number of direct children of a division.
func (r *divisionRepository) CountChildren(ctx context.Context, id int) (int, error) {
	return r.client.Division.Query().
		Where(entdivision.ParentIDEQ(id)).
		Count(ctx)
}

// CountUsers returns the number of users assigned to a division.
func (r *divisionRepository) CountUsers(ctx context.Context, id int) (int, error) {
	d, err := r.client.Division.Get(ctx, id)
	if err != nil {
		return 0, err
	}
	return r.client.Division.QueryUsers(d).Count(ctx)
}

// Delete removes a division scoped to an app. appID=0 bypasses the app filter.
func (r *divisionRepository) Delete(ctx context.Context, appID, id int) error {
	q := r.client.Division.Delete().Where(entdivision.IDEQ(id))
	if appID != 0 {
		q = q.Where(entdivision.AppIDEQ(appID))
	}
	count, err := q.Exec(ctx)
	if err != nil {
		slog.Error("database error: failed to delete division", "id", id, "error", err)
		return err
	}
	if count == 0 {
		slog.Warn("division not found for deletion", "id", id, "app_id", appID)
		return fmt.Errorf("division not found")
	}
	return nil
}

func (r *divisionRepository) mapToModel(d *ent.Division) *Division {
	result := &Division{
		ID:        d.ID,
		AppID:     d.AppID,
		ParentID:  d.ParentID,
		Name:      d.Name,
		Path:      d.Path,
		Depth:     d.Depth,
		Status:    d.Status,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
	return result
}
