package user

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"keeper/pkg/auth"
	"keeper/pkg/render"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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

func (m *mockService) List(ctx context.Context, appID int, role int8, limit, offset int) ([]*User, error) {
	args := m.Called(ctx, appID, role, limit, offset)
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
	handler := NewUserHandler(svc)

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

func TestHandler_ListManagers_SysAdmin(t *testing.T) {
	svc := new(mockService)
	handler := NewUserHandler(svc)

	managers := []*User{{ID: 2, Role: RoleManager}}
	svc.On("List", mock.Anything, 0, RoleManager, 50, 0).Return(managers, nil)

	req, _ := http.NewRequest("GET", "/managers", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1, Role: auth.RoleSysAdmin})
	rr := httptest.NewRecorder()

	handler.ListManagers(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestHandler_ListManagers_NonSysAdmin_Forbidden(t *testing.T) {
	svc := new(mockService)
	handler := NewUserHandler(svc)

	req, _ := http.NewRequest("GET", "/managers", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Role: auth.RoleAdmin})
	rr := httptest.NewRecorder()

	handler.ListManagers(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestHandler_ListManagers_NoClaims_Unauthorized(t *testing.T) {
	svc := new(mockService)
	handler := NewUserHandler(svc)

	req, _ := http.NewRequest("GET", "/managers", nil)
	rr := httptest.NewRecorder()

	handler.ListManagers(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHandler_Authenticate(t *testing.T) {
	svc := new(mockService)
	handler := NewUserHandler(svc)

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
