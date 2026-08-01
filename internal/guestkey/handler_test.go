package guestkey

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"keeper/internal/policy"
	"keeper/pkg/auth"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// testPolicyStore mirrors falcon's exported policy: sysadmin sudo bypasses
// everything; admin holds guestkey CRUD scoped to its own tenant (Scope "own").
var testPolicyStore = policy.NewStoreFromPolicies(policy.Compile([]policy.Row{
	{Role: "sysadmin", IsSudo: true},
	{Role: "admin", Resource: strPtr("guestkey"), Action: strPtr("create"), Scope: strPtr("own")},
	{Role: "admin", Resource: strPtr("guestkey"), Action: strPtr("read"), Scope: strPtr("own")},
	{Role: "admin", Resource: strPtr("guestkey"), Action: strPtr("update"), Scope: strPtr("own")},
	{Role: "admin", Resource: strPtr("guestkey"), Action: strPtr("delete"), Scope: strPtr("own")},
}))

func strPtr(s string) *string { return &s }

// withClaims returns a request carrying the given user claims in context,
// mirroring how auth middleware injects them.
func withClaims(req *http.Request, claims *auth.UserClaims) *http.Request {
	ctx := context.WithValue(req.Context(), auth.UserClaimsKey, claims)
	return req.WithContext(ctx)
}

// withURLParam attaches a chi URL param (e.g. "id") to the request context.
func withURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

type mockGuestKeyService struct {
	mock.Mock
}

func (m *mockGuestKeyService) Create(ctx context.Context, req CreateGuestKeyRequest) (*GuestKey, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*GuestKey), args.Error(1)
}

func (m *mockGuestKeyService) GetByID(ctx context.Context, appID, id int) (*GuestKey, error) {
	args := m.Called(ctx, appID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*GuestKey), args.Error(1)
}

func (m *mockGuestKeyService) List(ctx context.Context, appID, limit, offset int) ([]*GuestKey, error) {
	args := m.Called(ctx, appID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*GuestKey), args.Error(1)
}

func (m *mockGuestKeyService) Update(ctx context.Context, appID, id int, req UpdateGuestKeyRequest) (*GuestKey, error) {
	args := m.Called(ctx, appID, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*GuestKey), args.Error(1)
}

func (m *mockGuestKeyService) Delete(ctx context.Context, appID, id int) error {
	args := m.Called(ctx, appID, id)
	return args.Error(0)
}

func (m *mockGuestKeyService) Authenticate(ctx context.Context, req GuestAuthRequest) (*GuestAuthResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*GuestAuthResponse), args.Error(1)
}

func (m *mockGuestKeyService) LookupSiteKey(ctx context.Context, rawURL string) (*SiteKeyLookupResponse, error) {
	args := m.Called(ctx, rawURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SiteKeyLookupResponse), args.Error(1)
}

func (m *mockGuestKeyService) AppIDBySiteKey(ctx context.Context, siteKey string) (int, error) {
	args := m.Called(ctx, siteKey)
	return args.Int(0), args.Error(1)
}

func TestGuestKeyHandler_CreateGuestKey_NoClaims_Unauthorized(t *testing.T) {
	svc := new(mockGuestKeyService)
	handler := NewGuestKeyHandler(svc, testPolicyStore)

	reqBody := CreateGuestKeyRequest{AppID: 1, DivisionID: 1, UserID: 1, Name: "Storefront", Domain: "shop.acme.com"}
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/guest-keys", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.CreateGuestKey(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestGuestKeyHandler_CreateGuestKey_Admin_CanCreate(t *testing.T) {
	svc := new(mockGuestKeyService)
	handler := NewGuestKeyHandler(svc, testPolicyStore)

	reqBody := CreateGuestKeyRequest{AppID: 1, DivisionID: 1, UserID: 1, Name: "Storefront", Domain: "shop.acme.com"}
	expected := &GuestKey{ID: 1, AppID: 1, Name: "Storefront"}
	svc.On("Create", mock.Anything, reqBody).Return(expected, nil)

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/guest-keys", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	rr := httptest.NewRecorder()

	handler.CreateGuestKey(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	svc.AssertExpectations(t)
}

func TestGuestKeyHandler_CreateGuestKey_NoPermission_Forbidden(t *testing.T) {
	svc := new(mockGuestKeyService)
	handler := NewGuestKeyHandler(svc, testPolicyStore)

	reqBody := CreateGuestKeyRequest{AppID: 1, DivisionID: 1, UserID: 1, Name: "Storefront", Domain: "shop.acme.com"}
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/guest-keys", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 5, Roles: []auth.RoleAssignment{{Name: "unknown_role"}}})
	rr := httptest.NewRecorder()

	handler.CreateGuestKey(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestGuestKeyHandler_CreateGuestKey_OwnScope_CrossTenant_Forbidden(t *testing.T) {
	svc := new(mockGuestKeyService)
	handler := NewGuestKeyHandler(svc, testPolicyStore)

	reqBody := CreateGuestKeyRequest{AppID: 2, DivisionID: 1, UserID: 1, Name: "Storefront", Domain: "shop.acme.com"}
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/guest-keys", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	rr := httptest.NewRecorder()

	handler.CreateGuestKey(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestGuestKeyHandler_ListGuestKeys_OwnScope_ScopedToCallerApp(t *testing.T) {
	svc := new(mockGuestKeyService)
	handler := NewGuestKeyHandler(svc, testPolicyStore)

	expected := []*GuestKey{{ID: 1, AppID: 1}}
	svc.On("List", mock.Anything, 1, 50, 0).Return(expected, nil)

	req, _ := http.NewRequest("GET", "/guest-keys", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	rr := httptest.NewRecorder()

	handler.ListGuestKeys(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestGuestKeyHandler_ListGuestKeys_AnyScope_Unrestricted(t *testing.T) {
	svc := new(mockGuestKeyService)
	handler := NewGuestKeyHandler(svc, testPolicyStore)

	expected := []*GuestKey{{ID: 1, AppID: 1}, {ID: 2, AppID: 2}}
	svc.On("List", mock.Anything, 0, 50, 0).Return(expected, nil)

	req, _ := http.NewRequest("GET", "/guest-keys", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1, Roles: []auth.RoleAssignment{{Name: "sysadmin"}}})
	rr := httptest.NewRecorder()

	handler.ListGuestKeys(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestGuestKeyHandler_ListGuestKeys_NoPermission_Forbidden(t *testing.T) {
	svc := new(mockGuestKeyService)
	handler := NewGuestKeyHandler(svc, testPolicyStore)

	req, _ := http.NewRequest("GET", "/guest-keys", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 5, Roles: []auth.RoleAssignment{{Name: "unknown_role"}}})
	rr := httptest.NewRecorder()

	handler.ListGuestKeys(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestGuestKeyHandler_GetGuestKeyByID_OwnScope_ScopedToCallerApp(t *testing.T) {
	svc := new(mockGuestKeyService)
	handler := NewGuestKeyHandler(svc, testPolicyStore)

	expected := &GuestKey{ID: 4, AppID: 1}
	svc.On("GetByID", mock.Anything, 1, 4).Return(expected, nil)

	req, _ := http.NewRequest("GET", "/guest-keys/4", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.GetGuestKeyByID(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestGuestKeyHandler_GetGuestKeyByID_AnyScope_Unrestricted(t *testing.T) {
	svc := new(mockGuestKeyService)
	handler := NewGuestKeyHandler(svc, testPolicyStore)

	expected := &GuestKey{ID: 4, AppID: 2}
	svc.On("GetByID", mock.Anything, 0, 4).Return(expected, nil)

	req, _ := http.NewRequest("GET", "/guest-keys/4", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1, Roles: []auth.RoleAssignment{{Name: "sysadmin"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.GetGuestKeyByID(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestGuestKeyHandler_GetGuestKeyByID_NoPermission_Forbidden(t *testing.T) {
	svc := new(mockGuestKeyService)
	handler := NewGuestKeyHandler(svc, testPolicyStore)

	req, _ := http.NewRequest("GET", "/guest-keys/4", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 5, Roles: []auth.RoleAssignment{{Name: "unknown_role"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.GetGuestKeyByID(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything, mock.Anything)
}

func TestGuestKeyHandler_GetGuestKeyByID_OwnScope_CrossTenant_NotFound(t *testing.T) {
	svc := new(mockGuestKeyService)
	handler := NewGuestKeyHandler(svc, testPolicyStore)

	// scoped call filters by caller's app_id=1; repo/service reports not
	// found for the cross-tenant id rather than leaking existence via 403.
	svc.On("GetByID", mock.Anything, 1, 4).Return(nil, assert.AnError)

	req, _ := http.NewRequest("GET", "/guest-keys/4", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.GetGuestKeyByID(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	svc.AssertExpectations(t)
}

func TestGuestKeyHandler_UpdateGuestKey_Admin_CanUpdate(t *testing.T) {
	svc := new(mockGuestKeyService)
	handler := NewGuestKeyHandler(svc, testPolicyStore)

	newName := "Renamed"
	reqBody := UpdateGuestKeyRequest{Name: &newName}
	loaded := &GuestKey{ID: 4, AppID: 1}
	updated := &GuestKey{ID: 4, AppID: 1, Name: newName}
	svc.On("GetByID", mock.Anything, 1, 4).Return(loaded, nil)
	svc.On("Update", mock.Anything, 1, 4, reqBody).Return(updated, nil)

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("PUT", "/guest-keys/4", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.UpdateGuestKey(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestGuestKeyHandler_UpdateGuestKey_NoPermission_Forbidden(t *testing.T) {
	svc := new(mockGuestKeyService)
	handler := NewGuestKeyHandler(svc, testPolicyStore)

	newName := "Renamed"
	reqBody := UpdateGuestKeyRequest{Name: &newName}
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("PUT", "/guest-keys/4", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 5, Roles: []auth.RoleAssignment{{Name: "unknown_role"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.UpdateGuestKey(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestGuestKeyHandler_UpdateGuestKey_OwnScope_CrossTenant_NotFound(t *testing.T) {
	svc := new(mockGuestKeyService)
	handler := NewGuestKeyHandler(svc, testPolicyStore)

	svc.On("GetByID", mock.Anything, 1, 4).Return(nil, assert.AnError)

	newName := "Renamed"
	reqBody := UpdateGuestKeyRequest{Name: &newName}
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("PUT", "/guest-keys/4", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.UpdateGuestKey(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	svc.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestGuestKeyHandler_DeleteGuestKey_Admin_CanDelete(t *testing.T) {
	svc := new(mockGuestKeyService)
	handler := NewGuestKeyHandler(svc, testPolicyStore)

	loaded := &GuestKey{ID: 4, AppID: 1}
	svc.On("GetByID", mock.Anything, 1, 4).Return(loaded, nil)
	svc.On("Delete", mock.Anything, 1, 4).Return(nil)

	req, _ := http.NewRequest("DELETE", "/guest-keys/4", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.DeleteGuestKey(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestGuestKeyHandler_DeleteGuestKey_NoPermission_Forbidden(t *testing.T) {
	svc := new(mockGuestKeyService)
	handler := NewGuestKeyHandler(svc, testPolicyStore)

	req, _ := http.NewRequest("DELETE", "/guest-keys/4", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 5, Roles: []auth.RoleAssignment{{Name: "unknown_role"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.DeleteGuestKey(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
}

func TestGuestKeyHandler_DeleteGuestKey_OwnScope_CrossTenant_NotFound(t *testing.T) {
	svc := new(mockGuestKeyService)
	handler := NewGuestKeyHandler(svc, testPolicyStore)

	svc.On("GetByID", mock.Anything, 1, 4).Return(nil, assert.AnError)

	req, _ := http.NewRequest("DELETE", "/guest-keys/4", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.DeleteGuestKey(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	svc.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
}
