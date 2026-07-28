package user

import (
	"context"
	"fmt"
	"log/slog"

	"keeper/ent"
	entdivision "keeper/ent/division"
	"keeper/ent/user"
)

type userRepository struct {
	client *ent.Client
}

// NewUserRepository creates a new user repository.
func NewUserRepository(client *ent.Client) *userRepository {
	return &userRepository{client: client}
}

func (r *userRepository) Create(ctx context.Context, u User) (*User, error) {
	// Validate division belongs to the same app
	count, err := r.client.Division.Query().
		Where(entdivision.IDEQ(u.DivisionID), entdivision.AppIDEQ(u.AppID)).
		Count(ctx)
	if err != nil {
		slog.Error("database error: failed to validate division for user", "division_id", u.DivisionID, "error", err)
		return nil, err
	}
	if count == 0 {
		slog.Warn("create user rejected: division not found for app", "division_id", u.DivisionID, "app_id", u.AppID)
		return nil, fmt.Errorf("division not found for app")
	}

	created, err := r.client.User.
		Create().
		SetAppID(u.AppID).
		SetDivisionID(u.DivisionID).
		SetFirstname(u.Firstname).
		SetLastname(u.Lastname).
		SetEmail(u.Email).
		SetPassword(u.Password).
		SetRole(u.Role).
		SetStatus(u.Status).
		Save(ctx)
	if err != nil {
		slog.Error("database error: failed to create user", "email", u.Email, "error", err)
		return nil, err
	}
	return r.GetByID(ctx, u.AppID, created.ID)
}

func (r *userRepository) GetByID(ctx context.Context, appID, id int) (*User, error) {
	q := r.client.User.Query().Where(user.IDEQ(id))
	if appID != 0 {
		q = q.Where(user.AppIDEQ(appID))
	}
	u, err := q.WithApp().WithDivision().Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.Warn("user not found in database", "id", id, "app_id", appID)
			return nil, fmt.Errorf("user not found: %w", err)
		}
		slog.Error("database error: failed to get user by id", "id", id, "error", err)
		return nil, err
	}
	return r.mapToModel(u), nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	u, err := r.client.User.Query().
		Where(user.EmailEQ(email)).
		WithApp().
		WithDivision().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.Warn("user not found in database", "email", email)
			return nil, fmt.Errorf("user not found: %w", err)
		}
		slog.Error("database error: failed to get user by email", "email", email, "error", err)
		return nil, err
	}
	return r.mapToModel(u), nil
}

func (r *userRepository) List(ctx context.Context, appID int, role int8, limit, offset int) ([]*User, error) {
	q := r.client.User.Query()
	if appID != 0 {
		q = q.Where(user.AppIDEQ(appID))
	}
	if role >= 0 {
		q = q.Where(user.RoleEQ(role))
	}
	users, err := q.
		Order(ent.Asc(user.FieldID)).
		Limit(limit).
		Offset(offset).
		WithApp().
		WithDivision().
		All(ctx)
	if err != nil {
		slog.Error("database error: failed to list users", "error", err)
		return nil, err
	}
	result := make([]*User, len(users))
	for i, u := range users {
		result[i] = r.mapToModel(u)
	}
	return result, nil
}

func (r *userRepository) Update(ctx context.Context, appID, id int, u *User) (*User, error) {
	q := r.client.User.Update().Where(user.IDEQ(id))
	if appID != 0 {
		q = q.Where(user.AppIDEQ(appID))
	}
	count, err := q.
		SetAppID(u.AppID).
		SetDivisionID(u.DivisionID).
		SetFirstname(u.Firstname).
		SetLastname(u.Lastname).
		SetEmail(u.Email).
		SetPassword(u.Password).
		SetRole(u.Role).
		SetStatus(u.Status).
		Save(ctx)
	if err != nil {
		slog.Error("database error: failed to update user", "id", id, "error", err)
		return nil, err
	}
	if count == 0 {
		slog.Warn("user not found for update", "id", id, "app_id", appID)
		return nil, fmt.Errorf("user not found")
	}
	return r.GetByID(ctx, u.AppID, id)
}

func (r *userRepository) Delete(ctx context.Context, appID, id int) error {
	q := r.client.User.Delete().Where(user.IDEQ(id))
	if appID != 0 {
		q = q.Where(user.AppIDEQ(appID))
	}
	count, err := q.Exec(ctx)
	if err != nil {
		slog.Error("database error: failed to delete user", "id", id, "error", err)
		return err
	}
	if count == 0 {
		slog.Warn("user not found for deletion", "id", id, "app_id", appID)
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *userRepository) mapToModel(u *ent.User) *User {
	result := &User{
		ID:         u.ID,
		AppID:      u.AppID,
		DivisionID: u.DivisionID,
		Firstname:  u.Firstname,
		Lastname:   u.Lastname,
		Email:      u.Email,
		Password:   u.Password,
		Role:       u.Role,
		Status:     u.Status,
		CreatedAt:  u.CreatedAt,
		UpdatedAt:  u.UpdatedAt,
	}
	if u.Edges.App != nil {
		result.AppName = u.Edges.App.Name
		result.AppStatus = u.Edges.App.Status
	}
	if u.Edges.Division != nil {
		result.DivisionName = u.Edges.Division.Name
	}
	return result
}
