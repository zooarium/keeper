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
		SetDomain(k.Domain).
		SetStatus(k.Status).
		Save(ctx)
	if err != nil {
		slog.Error("database error: failed to create guest key", "name", k.Name, "app_id", k.AppID, "error", err)
		return nil, err
	}
	return r.mapToModel(created), nil
}

// GetByID returns a guest key by ID scoped to an app. appID=0 bypasses the app filter.
func (r *guestKeyRepository) GetByID(ctx context.Context, appID, id int) (*GuestKey, error) {
	q := r.client.GuestKey.Query().Where(entguestkey.IDEQ(id))
	if appID != 0 {
		q = q.Where(entguestkey.AppID(appID))
	}
	k, err := q.Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.Warn("guest key not found in database", "id", id, "app_id", appID)
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

// GetActiveByDomain returns the active guest key registered for the given
// normalized domain. Used by the public site-key lookup. domain is unique, so
// at most one row matches.
func (r *guestKeyRepository) GetActiveByDomain(ctx context.Context, domain string) (*GuestKey, error) {
	k, err := r.client.GuestKey.Query().
		Where(entguestkey.Domain(domain), entguestkey.Status(1)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.Warn("active guest key not found for domain", "domain", domain)
			return nil, fmt.Errorf("guest key not found: %w", err)
		}
		slog.Error("database error: failed to get guest key by domain", "domain", domain, "error", err)
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

// Update updates name and status of a guest key. appID=0 bypasses the app filter.
func (r *guestKeyRepository) Update(ctx context.Context, appID, id int, k *GuestKey) (*GuestKey, error) {
	q := r.client.GuestKey.Update().Where(entguestkey.IDEQ(id))
	if appID != 0 {
		q = q.Where(entguestkey.AppID(appID))
	}
	count, err := q.
		SetName(k.Name).
		SetStatus(k.Status).
		Save(ctx)
	if err != nil {
		slog.Error("database error: failed to update guest key", "id", id, "error", err)
		return nil, err
	}
	if count == 0 {
		slog.Warn("guest key not found for update", "id", id, "app_id", appID)
		return nil, fmt.Errorf("guest key not found")
	}
	return r.GetByID(ctx, appID, id)
}

// Delete removes a guest key scoped to an app. appID=0 bypasses the app filter.
func (r *guestKeyRepository) Delete(ctx context.Context, appID, id int) error {
	q := r.client.GuestKey.Delete().Where(entguestkey.IDEQ(id))
	if appID != 0 {
		q = q.Where(entguestkey.AppID(appID))
	}
	count, err := q.Exec(ctx)
	if err != nil {
		slog.Error("database error: failed to delete guest key", "id", id, "error", err)
		return err
	}
	if count == 0 {
		slog.Warn("guest key not found for deletion", "id", id, "app_id", appID)
		return fmt.Errorf("guest key not found")
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
		Domain:     k.Domain,
		Status:     k.Status,
		CreatedAt:  k.CreatedAt,
		UpdatedAt:  k.UpdatedAt,
	}
}
