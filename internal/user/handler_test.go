package user

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"keeper/internal/policy"
	"keeper/pkg/auth"
	"keeper/pkg/render"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// withURLParam attaches a chi URL param (e.g. "id") to the request context.
func withURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// testPolicyStore mirrors falcon's exported policy: sysadmin sudo bypasses
// everything; admin holds user CRUD scoped to its own tenant (Scope "own").
var testPolicyStore = policy.NewStoreFromPolicies(policy.Compile([]policy.Row{
	{Role: "sysadmin", IsSudo: true},
	{Role: "admin", Resource: strPtr("user"), Action: strPtr("create"), Scope: strPtr("own")},
	{Role: "admin", Resource: strPtr("user"), Action: strPtr("read"), Scope: strPtr("own")},
	{Role: "admin", Resource: strPtr("user"), Action: strPtr("update"), Scope: strPtr("own")},
	{Role: "admin", Resource: strPtr("user"), Action: strPtr("delete"), Scope: strPtr("own")},
}))

func strPtr(s string) *string { return &s }

// withClaims returns a request carrying the given user claims in context,
// mirroring how auth middleware injects them.
func withClaims(req *http.Request, claims *auth.UserClaims) *http.Request {
	ctx := context.WithValue(req.Context(), auth.UserClaimsKey, claims)
	return req.WithContext(ctx)
}

type mockService struct {
	mock.Mock
}

func (m *mockService) Create(ctx context.Context, req CreateUserRequest) (*User, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*User), args.Error(1)
}

func (m *mockService) GetByID(ctx context.Context, appID, id int) (*User, error) {
	args := m.Called(ctx, appID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*User), args.Error(1)
}

func (m *mockService) List(ctx context.Context, appID, limit, offset int) ([]*User, error) {
	args := m.Called(ctx, appID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*User), args.Error(1)
}

func (m *mockService) Update(ctx context.Context, appID, id int, req UpdateUserRequest) (*User, error) {
	args := m.Called(ctx, appID, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*User), args.Error(1)
}

func (m *mockService) Delete(ctx context.Context, appID, id int) error {
	args := m.Called(ctx, appID, id)
	return args.Error(0)
}

func (m *mockService) Authenticate(ctx context.Context, req AuthRequest) (*AuthResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*AuthResponse), args.Error(1)
}

func TestHandler_Create(t *testing.T) {
	svc := new(mockService)
	handler := NewUserHandler(svc, testPolicyStore)

	reqBody := CreateUserRequest{
		AppID:     1,
		Firstname: "Hiren",
		Lastname:  "Chhatbar",
		Email:     "hiren@example.com",
		Password:  "password123",
	}

	expectedUser := &User{
		ID:        1,
		AppID:     reqBody.AppID,
		Firstname: reqBody.Firstname,
		Lastname:  reqBody.Lastname,
		Email:     reqBody.Email,
		Status:    1,
	}

	svc.On("Create", mock.Anything, reqBody).Return(expectedUser, nil)

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/users", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.CreateUser(rr, req)

	// No claims in context → 401
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHandler_Authenticate(t *testing.T) {
	svc := new(mockService)
	handler := NewUserHandler(svc, testPolicyStore)

	reqBody := AuthRequest{
		Email:    "hiren@example.com",
		Password: "password123",
	}

	expectedResp := &AuthResponse{
		Token: "fake-jwt-token",
		User: User{
			ID:    1,
			Email: reqBody.Email,
		},
	}

	svc.On("Authenticate", mock.Anything, reqBody).Return(expectedResp, nil)

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/users/auth", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.AuthenticateUser(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp render.Response
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)

	dataMap := resp.Data.(map[string]interface{})
	assert.Equal(t, expectedResp.Token, dataMap["token"])
}

func TestHandler_CreateUser_Admin_CanCreateUser(t *testing.T) {
	svc := new(mockService)
	handler := NewUserHandler(svc, testPolicyStore)

	reqBody := CreateUserRequest{
		AppID:      1,
		DivisionID: 1,
		Firstname:  "New",
		Lastname:   "Hire",
		Email:      "new@example.com",
		Password:   "password123",
	}
	expectedUser := &User{ID: 2, AppID: 1}
	svc.On("Create", mock.Anything, reqBody).Return(expectedUser, nil)

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/users", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	rr := httptest.NewRecorder()

	handler.CreateUser(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	svc.AssertExpectations(t)
}

func TestHandler_CreateUser_NoPermission_Forbidden(t *testing.T) {
	svc := new(mockService)
	handler := NewUserHandler(svc, testPolicyStore)

	reqBody := CreateUserRequest{
		AppID:      1,
		DivisionID: 1,
		Firstname:  "New",
		Lastname:   "Hire",
		Email:      "unauthorized@example.com",
		Password:   "password123",
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/users", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 5, Roles: []auth.RoleAssignment{{Name: "unknown_role"}}})
	rr := httptest.NewRecorder()

	handler.CreateUser(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestHandler_UpdateUser_Admin_CanUpdateUser(t *testing.T) {
	svc := new(mockService)
	handler := NewUserHandler(svc, testPolicyStore)

	newFirstname := "Updated"
	reqBody := UpdateUserRequest{Firstname: &newFirstname}
	updatedUser := &User{ID: 4, AppID: 1, Firstname: newFirstname}
	svc.On("Update", mock.Anything, 1, 4, reqBody).Return(updatedUser, nil)

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("PUT", "/users/4", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.UpdateUser(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestHandler_UpdateUser_NoPermission_Forbidden(t *testing.T) {
	svc := new(mockService)
	handler := NewUserHandler(svc, testPolicyStore)

	newFirstname := "Updated"
	reqBody := UpdateUserRequest{Firstname: &newFirstname}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("PUT", "/users/4", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 5, Roles: []auth.RoleAssignment{{Name: "unknown_role"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.UpdateUser(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestHandler_DeleteUser_Admin_CanDeleteUser(t *testing.T) {
	svc := new(mockService)
	handler := NewUserHandler(svc, testPolicyStore)

	svc.On("Delete", mock.Anything, 1, 4).Return(nil)

	req, _ := http.NewRequest("DELETE", "/users/4", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.DeleteUser(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	svc.AssertExpectations(t)
}

func TestHandler_DeleteUser_NoPermission_Forbidden(t *testing.T) {
	svc := new(mockService)
	handler := NewUserHandler(svc, testPolicyStore)

	req, _ := http.NewRequest("DELETE", "/users/4", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 5, Roles: []auth.RoleAssignment{{Name: "unknown_role"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.DeleteUser(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
}

func TestHandler_ListUsers_OwnScope_ScopedToCallerApp(t *testing.T) {
	svc := new(mockService)
	handler := NewUserHandler(svc, testPolicyStore)

	expectedUsers := []*User{{ID: 1, AppID: 1}}
	svc.On("List", mock.Anything, 1, 50, 0).Return(expectedUsers, nil)

	req, _ := http.NewRequest("GET", "/users", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	rr := httptest.NewRecorder()

	handler.ListUsers(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestHandler_ListUsers_AnyScope_Unrestricted(t *testing.T) {
	svc := new(mockService)
	handler := NewUserHandler(svc, testPolicyStore)

	expectedUsers := []*User{{ID: 1, AppID: 1}, {ID: 2, AppID: 2}}
	svc.On("List", mock.Anything, 0, 50, 0).Return(expectedUsers, nil)

	req, _ := http.NewRequest("GET", "/users", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1, Roles: []auth.RoleAssignment{{Name: "sysadmin"}}})
	rr := httptest.NewRecorder()

	handler.ListUsers(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestHandler_ListUsers_NoPermission_Forbidden(t *testing.T) {
	svc := new(mockService)
	handler := NewUserHandler(svc, testPolicyStore)

	req, _ := http.NewRequest("GET", "/users", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 5, Roles: []auth.RoleAssignment{{Name: "unknown_role"}}})
	rr := httptest.NewRecorder()

	handler.ListUsers(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestHandler_GetUserByID_AnyScope_Unrestricted(t *testing.T) {
	svc := new(mockService)
	handler := NewUserHandler(svc, testPolicyStore)

	expectedUser := &User{ID: 4, AppID: 2}
	svc.On("GetByID", mock.Anything, 0, 4).Return(expectedUser, nil)

	req, _ := http.NewRequest("GET", "/users/4", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1, Roles: []auth.RoleAssignment{{Name: "sysadmin"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.GetUserByID(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestHandler_GetUserByID_NoPermission_Forbidden(t *testing.T) {
	svc := new(mockService)
	handler := NewUserHandler(svc, testPolicyStore)

	req, _ := http.NewRequest("GET", "/users/4", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 5, Roles: []auth.RoleAssignment{{Name: "unknown_role"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.GetUserByID(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything, mock.Anything)
}
