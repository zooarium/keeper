package app

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

type mockAppService struct {
	mock.Mock
}

func (m *mockAppService) Create(ctx context.Context, req CreateAppRequest) (*App, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*App), args.Error(1)
}

func (m *mockAppService) GetByID(ctx context.Context, id int) (*App, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*App), args.Error(1)
}

func (m *mockAppService) List(ctx context.Context, limit, offset int) ([]*App, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*App), args.Error(1)
}

func (m *mockAppService) Update(ctx context.Context, id int, req UpdateAppRequest) (*App, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*App), args.Error(1)
}

func (m *mockAppService) Delete(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// withClaims returns a request carrying the given user claims in context,
// mirroring how auth middleware injects them.
func withClaims(req *http.Request, claims *auth.UserClaims) *http.Request {
	ctx := context.WithValue(req.Context(), auth.UserClaimsKey, claims)
	return req.WithContext(ctx)
}

func TestHandler_Create_SysAdmin(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc)

	reqBody := CreateAppRequest{
		Name: "Test App",
	}

	expectedApp := &App{
		ID:     1,
		Name:   reqBody.Name,
		Status: 1,
	}

	svc.On("Create", mock.Anything, reqBody).Return(expectedApp, nil)

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/apps", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1, Role: auth.RoleSysAdmin})
	rr := httptest.NewRecorder()

	handler.CreateApp(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var resp render.Response
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)

	dataMap := resp.Data.(map[string]interface{})
	assert.Equal(t, expectedApp.Name, dataMap["name"])
}

func TestHandler_Create_NonSysAdmin_Forbidden(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc)

	body, _ := json.Marshal(CreateAppRequest{Name: "Test App"})
	req, _ := http.NewRequest("POST", "/apps", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1, Role: auth.RoleUser})
	rr := httptest.NewRecorder()

	handler.CreateApp(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestHandler_Create_NoClaims_Unauthorized(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc)

	body, _ := json.Marshal(CreateAppRequest{Name: "Test App"})
	req, _ := http.NewRequest("POST", "/apps", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.CreateApp(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestHandler_List_SysAdmin_All(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc)

	expectedApps := []*App{
		{ID: 1, Name: "App 1", Status: 1},
		{ID: 2, Name: "App 2", Status: 1},
	}

	svc.On("List", mock.Anything, mock.Anything, mock.Anything).Return(expectedApps, nil)

	req, _ := http.NewRequest("GET", "/apps", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1, Role: auth.RoleSysAdmin})
	rr := httptest.NewRecorder()

	handler.ListApps(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp render.Response
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)

	dataList := resp.Data.([]interface{})
	assert.Len(t, dataList, 2)
}

func TestHandler_List_NonSysAdmin_OwnAppOnly(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc)

	ownApp := &App{ID: 7, Name: "Own App", Status: 1}
	svc.On("GetByID", mock.Anything, 7).Return(ownApp, nil)

	req, _ := http.NewRequest("GET", "/apps", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 7, UserID: 2, Role: auth.RoleUser})
	rr := httptest.NewRecorder()

	handler.ListApps(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything)

	var resp render.Response
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)

	dataList := resp.Data.([]interface{})
	assert.Len(t, dataList, 1)
	assert.Equal(t, "Own App", dataList[0].(map[string]interface{})["name"])
}

func TestHandler_List_NoClaims_Unauthorized(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc)

	req, _ := http.NewRequest("GET", "/apps", nil)
	rr := httptest.NewRecorder()

	handler.ListApps(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
