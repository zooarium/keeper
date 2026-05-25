package user

import (
	"context"
	"fmt"
	"log/slog"

	"keeper/ent"
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
	created, err := r.client.User.
		Create().
		SetAppID(u.AppID).
		SetFirstname(u.Firstname).
		SetLastname(u.Lastname).
		SetEmail(u.Email).
		SetPassword(u.Password).
		SetStatus(u.Status).
		Save(ctx)
	if err != nil {
		slog.Error("database error: failed to create user", "email", u.Email, "error", err)
		return nil, err
	}
	return r.GetByID(ctx, u.AppID, created.ID)
}

func (r *userRepository) GetByID(ctx context.Context, appID, id int) (*User, error) {
	u, err := r.client.User.Query().
		Where(user.IDEQ(id), user.AppIDEQ(appID)).
		WithApp().
		Only(ctx)
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

func (r *userRepository) List(ctx context.Context, appID int) ([]*User, error) {
	users, err := r.client.User.Query().
		Where(user.AppIDEQ(appID)).
		WithApp().
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
	count, err := r.client.User.Update().
		Where(user.IDEQ(id), user.AppIDEQ(appID)).
		SetAppID(u.AppID).
		SetFirstname(u.Firstname).
		SetLastname(u.Lastname).
		SetEmail(u.Email).
		SetPassword(u.Password).
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
	count, err := r.client.User.Delete().
		Where(user.IDEQ(id), user.AppIDEQ(appID)).
		Exec(ctx)
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
		ID:        u.ID,
		AppID:     u.AppID,
		Firstname: u.Firstname,
		Lastname:  u.Lastname,
		Email:     u.Email,
		Password:  u.Password,
		Status:    u.Status,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
	if u.Edges.App != nil {
		result.AppName = u.Edges.App.Name
		result.AppStatus = u.Edges.App.Status
	}
	return result
}
