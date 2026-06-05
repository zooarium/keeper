package user

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"keeper/pkg/render"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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
