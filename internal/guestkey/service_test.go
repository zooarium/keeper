package guestkey

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"keeper/pkg/auth"

	"github.com/stretchr/testify/assert"
)

type mockRepo struct {
	keys        map[string]*GuestKey
	created     *GuestKey
	userBelongs bool
}

func (m *mockRepo) Create(ctx context.Context, k GuestKey) (*GuestKey, error) {
	k.ID = 1
	m.created = &k
	return &k, nil
}

func (m *mockRepo) GetByID(ctx context.Context, id int) (*GuestKey, error) {
	for _, k := range m.keys {
		if k.ID == id {
			return k, nil
		}
	}
	return nil, errors.New("guest key not found")
}

func (m *mockRepo) GetActiveBySiteKey(ctx context.Context, siteKey string) (*GuestKey, error) {
	if k, ok := m.keys[siteKey]; ok && k.Status == 1 {
		return k, nil
	}
	return nil, errors.New("guest key not found")
}

func (m *mockRepo) List(ctx context.Context, appID, limit, offset int) ([]*GuestKey, error) {
	return nil, nil
}

func (m *mockRepo) Update(ctx context.Context, id int, k *GuestKey) (*GuestKey, error) {
	return k, nil
}

func (m *mockRepo) Delete(ctx context.Context, id int) error {
	return nil
}

func (m *mockRepo) UserBelongsTo(ctx context.Context, userID, appID, divisionID int) (bool, error) {
	return m.userBelongs, nil
}

func newService(repo *mockRepo) GuestKeyService {
	jwt := auth.NewJWTManager("guest-secret", 30*time.Minute)
	return NewGuestKeyService(repo, jwt, 30*time.Minute)
}

func TestCreateGeneratesSiteKey(t *testing.T) {
	repo := &mockRepo{userBelongs: true}
	svc := newService(repo)

	k, err := svc.Create(context.Background(), CreateGuestKeyRequest{
		AppID: 1, DivisionID: 2, UserID: 3, Name: "storefront",
	})

	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(k.SiteKey, "gk_"))
	assert.Len(t, k.SiteKey, 3+48)
	assert.Equal(t, int8(1), k.Status)
}

func TestCreateRejectsMismatchedGuestUser(t *testing.T) {
	repo := &mockRepo{userBelongs: false}
	svc := newService(repo)

	_, err := svc.Create(context.Background(), CreateGuestKeyRequest{
		AppID: 1, DivisionID: 2, UserID: 3, Name: "storefront",
	})

	assert.ErrorIs(t, err, ErrGuestUserMismatch)
}

func TestAuthenticateMintsGuestToken(t *testing.T) {
	repo := &mockRepo{keys: map[string]*GuestKey{
		"gk_valid": {ID: 7, AppID: 4, DivisionID: 5, UserID: 6, SiteKey: "gk_valid", Status: 1},
	}}
	svc := newService(repo)

	resp, err := svc.Authenticate(context.Background(), GuestAuthRequest{SiteKey: "gk_valid"})
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.True(t, resp.ExpiresAt.After(time.Now()))

	// Token verifies with the guest secret and carries tenant scope + guest role.
	claims, err := auth.NewJWTManager("guest-secret", 0).Verify(resp.Token)
	assert.NoError(t, err)
	assert.Equal(t, 4, claims.AppID)
	assert.Equal(t, 5, claims.DivisionID)
	assert.Equal(t, 6, claims.UserID)
	assert.True(t, claims.IsGuest())

	// And is useless against a different (primary) secret.
	_, err = auth.NewJWTManager("primary-secret", 0).Verify(resp.Token)
	assert.Error(t, err)
}

func TestAuthenticateRejectsUnknownOrInactiveKey(t *testing.T) {
	repo := &mockRepo{keys: map[string]*GuestKey{
		"gk_inactive": {ID: 8, SiteKey: "gk_inactive", Status: 0},
	}}
	svc := newService(repo)

	_, err := svc.Authenticate(context.Background(), GuestAuthRequest{SiteKey: "gk_unknown"})
	assert.ErrorIs(t, err, ErrInvalidSiteKey)

	_, err = svc.Authenticate(context.Background(), GuestAuthRequest{SiteKey: "gk_inactive"})
	assert.ErrorIs(t, err, ErrInvalidSiteKey)
}
