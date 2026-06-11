package guestkey

import (
	"context"
	"fmt"
	"log/slog"

	"keeper/ent"
	entguestkey "keeper/ent/guestkey"
	entuser "keeper/ent/user"
)

type guestKeyRepository struct {
	client *ent.Client
}

// NewGuestKeyRepository creates a new guest key repository.
func NewGuestKeyRepository(client *ent.Client) *guestKeyRepository {
	return &guestKeyRepository{client: client}
}

func (r *guestKeyRepository) Create(ctx context.Context, k GuestKey) (*GuestKey, error) {
	created, err := r.client.GuestKey.
		Create().
		SetAppID(k.AppID).
		SetDivisionID(k.DivisionID).
		SetUserID(k.UserID).
		SetName(k.Name).
		SetSiteKey(k.SiteKey).
		SetStatus(k.Status).
		Save(ctx)
	if err != nil {
		slog.Error("database error: failed to create guest key", "name", k.Name, "app_id", k.AppID, "error", err)
		return nil, err
	}
	return r.mapToModel(created), nil
}

func (r *guestKeyRepository) GetByID(ctx context.Context, id int) (*GuestKey, error) {
	k, err := r.client.GuestKey.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.Warn("guest key not found in database", "id", id)
			return nil, fmt.Errorf("guest key not found: %w", err)
		}
		slog.Error("database error: failed to get guest key by id", "id", id, "error", err)
		return nil, err
	}
	return r.mapToModel(k), nil
}

// GetActiveBySiteKey returns the guest key matching the site key with active
// status. Used by the public auth exchange.
func (r *guestKeyRepository) GetActiveBySiteKey(ctx context.Context, siteKey string) (*GuestKey, error) {
	k, err := r.client.GuestKey.Query().
		Where(entguestkey.SiteKey(siteKey), entguestkey.Status(1)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.Warn("active guest key not found for site key")
			return nil, fmt.Errorf("guest key not found: %w", err)
		}
		slog.Error("database error: failed to get guest key by site key", "error", err)
		return nil, err
	}
	return r.mapToModel(k), nil
}

// List returns guest keys; appID > 0 restricts to that app.
func (r *guestKeyRepository) List(ctx context.Context, appID, limit, offset int) ([]*GuestKey, error) {
	q := r.client.GuestKey.Query()
	if appID > 0 {
		q = q.Where(entguestkey.AppID(appID))
	}
	keys, err := q.
		Order(ent.Asc(entguestkey.FieldID)).
		Limit(limit).
		Offset(offset).
		All(ctx)
	if err != nil {
		slog.Error("database error: failed to list guest keys", "app_id", appID, "error", err)
		return nil, err
	}
	result := make([]*GuestKey, len(keys))
	for i, k := range keys {
		result[i] = r.mapToModel(k)
	}
	return result, nil
}

func (r *guestKeyRepository) Update(ctx context.Context, id int, k *GuestKey) (*GuestKey, error) {
	updated, err := r.client.GuestKey.UpdateOneID(id).
		SetName(k.Name).
		SetStatus(k.Status).
		Save(ctx)
	if err != nil {
		slog.Error("database error: failed to update guest key", "id", id, "error", err)
		return nil, err
	}
	return r.mapToModel(updated), nil
}

func (r *guestKeyRepository) Delete(ctx context.Context, id int) error {
	err := r.client.GuestKey.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.Warn("guest key not found for deletion", "id", id)
			return fmt.Errorf("guest key not found: %w", err)
		}
		slog.Error("database error: failed to delete guest key", "id", id, "error", err)
		return err
	}
	return nil
}

// UserBelongsTo reports whether the user exists with the given app and
// division — the guest identity must be a real, correctly scoped user.
func (r *guestKeyRepository) UserBelongsTo(ctx context.Context, userID, appID, divisionID int) (bool, error) {
	exists, err := r.client.User.Query().
		Where(entuser.ID(userID), entuser.AppID(appID), entuser.DivisionID(divisionID)).
		Exist(ctx)
	if err != nil {
		slog.Error("database error: failed to validate guest user", "user_id", userID, "error", err)
		return false, err
	}
	return exists, nil
}

func (r *guestKeyRepository) mapToModel(k *ent.GuestKey) *GuestKey {
	return &GuestKey{
		ID:         k.ID,
		AppID:      k.AppID,
		DivisionID: k.DivisionID,
		UserID:     k.UserID,
		Name:       k.Name,
		SiteKey:    k.SiteKey,
		Status:     k.Status,
		CreatedAt:  k.CreatedAt,
		UpdatedAt:  k.UpdatedAt,
	}
}
