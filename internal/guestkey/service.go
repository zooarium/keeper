package guestkey

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"keeper/pkg/auth"
)

// ErrInvalidSiteKey is returned when the site key is unknown or inactive.
var ErrInvalidSiteKey = errors.New("invalid site key")

// ErrGuestUserMismatch is returned when the designated guest user does not
// exist or does not belong to the requested app/division.
var ErrGuestUserMismatch = errors.New("guest user does not belong to the given app and division")

// GuestKeyRepository defines the data access contract for guest keys.
type GuestKeyRepository interface {
	Create(ctx context.Context, k GuestKey) (*GuestKey, error)
	GetByID(ctx context.Context, id int) (*GuestKey, error)
	GetActiveBySiteKey(ctx context.Context, siteKey string) (*GuestKey, error)
	List(ctx context.Context, appID, limit, offset int) ([]*GuestKey, error)
	Update(ctx context.Context, id int, k *GuestKey) (*GuestKey, error)
	Delete(ctx context.Context, id int) error
	UserBelongsTo(ctx context.Context, userID, appID, divisionID int) (bool, error)
}

// GuestKeyService defines the business logic for guest keys.
type GuestKeyService interface {
	Create(ctx context.Context, req CreateGuestKeyRequest) (*GuestKey, error)
	GetByID(ctx context.Context, id int) (*GuestKey, error)
	List(ctx context.Context, appID, limit, offset int) ([]*GuestKey, error)
	Update(ctx context.Context, id int, req UpdateGuestKeyRequest) (*GuestKey, error)
	Delete(ctx context.Context, id int) error
	Authenticate(ctx context.Context, req GuestAuthRequest) (*GuestAuthResponse, error)
}

type guestKeyService struct {
	repo        GuestKeyRepository
	guestJWT    *auth.JWTManager
	guestExpiry time.Duration
}

// NewGuestKeyService creates a new guest key service. guestJWT must be
// constructed with the dedicated guest secret (AUTH.GUEST_JWT_SECRET) so
// guest tokens are useless on surfaces verifying with the primary secret.
func NewGuestKeyService(repo GuestKeyRepository, guestJWT *auth.JWTManager, guestExpiry time.Duration) GuestKeyService {
	return &guestKeyService{repo: repo, guestJWT: guestJWT, guestExpiry: guestExpiry}
}

func (s *guestKeyService) Create(ctx context.Context, req CreateGuestKeyRequest) (*GuestKey, error) {
	slog.Info("creating guest key", "name", req.Name, "app_id", req.AppID)

	ok, err := s.repo.UserBelongsTo(ctx, req.UserID, req.AppID, req.DivisionID)
	if err != nil {
		return nil, fmt.Errorf("validate guest user: %w", err)
	}
	if !ok {
		return nil, ErrGuestUserMismatch
	}

	siteKey, err := generateSiteKey()
	if err != nil {
		return nil, fmt.Errorf("generate site key: %w", err)
	}

	status := int8(1)
	if req.Status != 0 {
		status = req.Status
	}

	created, err := s.repo.Create(ctx, GuestKey{
		AppID:      req.AppID,
		DivisionID: req.DivisionID,
		UserID:     req.UserID,
		Name:       req.Name,
		SiteKey:    siteKey,
		Status:     status,
	})
	if err != nil {
		return nil, fmt.Errorf("repository create: %w", err)
	}

	slog.Info("guest key created successfully", "id", created.ID, "app_id", created.AppID)
	return created, nil
}

func (s *guestKeyService) GetByID(ctx context.Context, id int) (*GuestKey, error) {
	slog.Info("getting guest key by id", "id", id)
	return s.repo.GetByID(ctx, id)
}

func (s *guestKeyService) List(ctx context.Context, appID, limit, offset int) ([]*GuestKey, error) {
	slog.Info("listing guest keys", "app_id", appID, "limit", limit, "offset", offset)
	return s.repo.List(ctx, appID, limit, offset)
}

func (s *guestKeyService) Update(ctx context.Context, id int, req UpdateGuestKeyRequest) (*GuestKey, error) {
	slog.Info("updating guest key", "id", id)
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

	slog.Info("guest key updated successfully", "id", id)
	return updated, nil
}

func (s *guestKeyService) Delete(ctx context.Context, id int) error {
	slog.Info("deleting guest key", "id", id)
	err := s.repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	slog.Info("guest key deleted successfully", "id", id)
	return nil
}

// Authenticate exchanges an active site key for a short-lived guest JWT
// carrying the key's tenant scope (app/division) and guest identity.
func (s *guestKeyService) Authenticate(ctx context.Context, req GuestAuthRequest) (*GuestAuthResponse, error) {
	k, err := s.repo.GetActiveBySiteKey(ctx, req.SiteKey)
	if err != nil {
		return nil, ErrInvalidSiteKey
	}

	token, err := s.guestJWT.Generate(k.AppID, k.UserID, k.DivisionID, auth.RoleGuest)
	if err != nil {
		slog.Error("failed to generate guest token", "guest_key_id", k.ID, "error", err)
		return nil, fmt.Errorf("generate guest token: %w", err)
	}

	slog.Info("guest token issued", "guest_key_id", k.ID, "app_id", k.AppID)
	return &GuestAuthResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(s.guestExpiry),
	}, nil
}

// generateSiteKey returns a publishable site key: "gk_" + 48 hex chars of
// crypto/rand entropy.
func generateSiteKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "gk_" + hex.EncodeToString(b), nil
}
