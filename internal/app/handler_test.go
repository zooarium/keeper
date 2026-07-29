package app

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

func strPtr(s string) *string { return &s }

// testPolicyStore mirrors the "app" slice of rbac-plan.md's Worked Example:
// sysadmin sudo bypasses everything; admin/manager hold app.update on base
// fields only (permission 10), never status/manager_id (permissions 11/12).
var testPolicyStore = policy.NewStoreFromPolicies(policy.Compile([]policy.Row{
	{Role: "sysadmin", IsSudo: true},
	{Role: "admin", Resource: strPtr("app"), Action: strPtr("update")},
	{Role: "manager", Resource: strPtr("app"), Action: strPtr("update")},
}))

// withURLParam attaches a chi URL param (e.g. "id") to the request context.
func withURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

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

func (m *mockAppService) ListByManager(ctx context.Context, managerID, limit, offset int) ([]*App, error) {
	args := m.Called(ctx, managerID, limit, offset)
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

func (m *mockAppService) PublicBySiteKey(ctx context.Context, siteKey string) (*PublicApp, error) {
	args := m.Called(ctx, siteKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*PublicApp), args.Error(1)
}

func (m *mockAppService) PublicByID(ctx context.Context, id int) (*PublicApp, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*PublicApp), args.Error(1)
}

// withClaims returns a request carrying the given user claims in context,
// mirroring how auth middleware injects them.
func withClaims(req *http.Request, claims *auth.UserClaims) *http.Request {
	ctx := context.WithValue(req.Context(), auth.UserClaimsKey, claims)
	return req.WithContext(ctx)
}

func TestHandler_Create_SysAdmin(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc, testPolicyStore)

	reqBody := CreateAppRequest{
		Name:     "Test App",
		Currency: "INR",
	}

	expectedApp := &App{
		ID:     1,
		Name:   reqBody.Name,
		Status: 1,
	}

	svc.On("Create", mock.Anything, reqBody).Return(expectedApp, nil)

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/apps", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1, Role: auth.RoleSysAdmin, Roles: []auth.RoleAssignment{{Name: "sysadmin"}}})
	rr := httptest.NewRecorder()

	handler.CreateApp(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var resp render.Response
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)

	dataMap := resp.Data.(map[string]interface{})
	assert.Equal(t, expectedApp.Name, dataMap["name"])
}

func TestHandler_Create_TaxPercentOutOfRange(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc, testPolicyStore)

	body, _ := json.Marshal(CreateAppRequest{Name: "Test App", TaxPercent: 101})
	req, _ := http.NewRequest("POST", "/apps", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1, Role: auth.RoleSysAdmin, Roles: []auth.RoleAssignment{{Name: "sysadmin"}}})
	rr := httptest.NewRecorder()

	handler.CreateApp(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestHandler_Create_InvalidCurrency(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc, testPolicyStore)

	body, _ := json.Marshal(CreateAppRequest{Name: "Test App", Currency: "ZZZ"})
	req, _ := http.NewRequest("POST", "/apps", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1, Role: auth.RoleSysAdmin, Roles: []auth.RoleAssignment{{Name: "sysadmin"}}})
	rr := httptest.NewRecorder()

	handler.CreateApp(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestHandler_Create_NonSysAdmin_Forbidden(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc, testPolicyStore)

	body, _ := json.Marshal(CreateAppRequest{Name: "Test App"})
	req, _ := http.NewRequest("POST", "/apps", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1, Role: auth.RoleAdmin, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	rr := httptest.NewRecorder()

	handler.CreateApp(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestHandler_Create_NoClaims_Unauthorized(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc, testPolicyStore)

	body, _ := json.Marshal(CreateAppRequest{Name: "Test App"})
	req, _ := http.NewRequest("POST", "/apps", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.CreateApp(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestHandler_List_SysAdmin_All(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc, testPolicyStore)

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
	handler := NewAppHandler(svc, testPolicyStore)

	ownApp := &App{ID: 7, Name: "Own App", Status: 1}
	svc.On("GetByID", mock.Anything, 7).Return(ownApp, nil)

	req, _ := http.NewRequest("GET", "/apps", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 7, UserID: 2, Role: auth.RoleAdmin})
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

func TestHandler_List_Manager_AssignedAppsOnly(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc, testPolicyStore)

	managedApp := &App{ID: 3, Name: "Managed App", Status: 1}
	svc.On("ListByManager", mock.Anything, 9, 50, 0).Return([]*App{managedApp}, nil)

	req, _ := http.NewRequest("GET", "/apps", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 9, Role: auth.RoleManager})
	rr := httptest.NewRecorder()

	handler.ListApps(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertCalled(t, "ListByManager", mock.Anything, 9, 50, 0)
}

func TestHandler_GetAppByID_Manager_Assigned_Allowed(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc, testPolicyStore)

	managerID := 9
	expectedApp := &App{ID: 5, Name: "Managed", ManagerID: &managerID}
	svc.On("GetByID", mock.Anything, 5).Return(expectedApp, nil)

	req, _ := http.NewRequest("GET", "/apps/5", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 9, Role: auth.RoleManager})
	req = withURLParam(req, "id", "5")
	rr := httptest.NewRecorder()

	handler.GetAppByID(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandler_GetAppByID_Manager_NotAssigned_Forbidden(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc, testPolicyStore)

	otherManagerID := 42
	expectedApp := &App{ID: 5, Name: "Managed", ManagerID: &otherManagerID}
	svc.On("GetByID", mock.Anything, 5).Return(expectedApp, nil)

	req, _ := http.NewRequest("GET", "/apps/5", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 9, Role: auth.RoleManager})
	req = withURLParam(req, "id", "5")
	rr := httptest.NewRecorder()

	handler.GetAppByID(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestHandler_UpdateApp_Manager_CannotAssignManager(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc, testPolicyStore)

	managerID := 9
	expectedApp := &App{ID: 5, Name: "Managed", ManagerID: &managerID}
	svc.On("GetByID", mock.Anything, 5).Return(expectedApp, nil)

	newManager := 10
	body, _ := json.Marshal(UpdateAppRequest{ManagerID: &newManager})
	req, _ := http.NewRequest("PUT", "/apps/5", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 9, Role: auth.RoleManager, Roles: []auth.RoleAssignment{{Name: "manager"}}})
	req = withURLParam(req, "id", "5")
	rr := httptest.NewRecorder()

	handler.UpdateApp(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestHandler_UpdateApp_Admin_CannotAssignManager(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc, testPolicyStore)

	expectedApp := &App{ID: 1, Name: "Own"}
	svc.On("GetByID", mock.Anything, 1).Return(expectedApp, nil)

	newManager := 10
	body, _ := json.Marshal(UpdateAppRequest{ManagerID: &newManager})
	req, _ := http.NewRequest("PUT", "/apps/1", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Role: auth.RoleAdmin, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	req = withURLParam(req, "id", "1")
	rr := httptest.NewRecorder()

	handler.UpdateApp(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestHandler_UpdateApp_OwnApp_CannotChangeStatus(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc, testPolicyStore)

	expectedApp := &App{ID: 1, Name: "Own"}
	svc.On("GetByID", mock.Anything, 1).Return(expectedApp, nil)

	newStatus := int8(0)
	body, _ := json.Marshal(UpdateAppRequest{Status: &newStatus})
	req, _ := http.NewRequest("PUT", "/apps/1", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Role: auth.RoleAdmin, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	req = withURLParam(req, "id", "1")
	rr := httptest.NewRecorder()

	handler.UpdateApp(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestHandler_UpdateApp_Admin_CanUpdateBaseFields(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc, testPolicyStore)

	expectedApp := &App{ID: 1, Name: "Own"}
	svc.On("GetByID", mock.Anything, 1).Return(expectedApp, nil)

	newName := "Renamed"
	updated := &App{ID: 1, Name: newName}
	svc.On("Update", mock.Anything, 1, mock.Anything).Return(updated, nil)

	body, _ := json.Marshal(UpdateAppRequest{Name: &newName})
	req, _ := http.NewRequest("PUT", "/apps/1", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Role: auth.RoleAdmin, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	req = withURLParam(req, "id", "1")
	rr := httptest.NewRecorder()

	handler.UpdateApp(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestHandler_UpdateApp_Manager_CannotChangeStatus(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc, testPolicyStore)

	managerID := 9
	expectedApp := &App{ID: 5, Name: "Managed", ManagerID: &managerID}
	svc.On("GetByID", mock.Anything, 5).Return(expectedApp, nil)

	newStatus := int8(0)
	body, _ := json.Marshal(UpdateAppRequest{Status: &newStatus})
	req, _ := http.NewRequest("PUT", "/apps/5", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 9, Role: auth.RoleManager, Roles: []auth.RoleAssignment{{Name: "manager"}}})
	req = withURLParam(req, "id", "5")
	rr := httptest.NewRecorder()

	handler.UpdateApp(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

func TestHandler_UpdateApp_SysAdmin_CanChangeStatus(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc, testPolicyStore)

	expectedApp := &App{ID: 1, Name: "Any"}
	svc.On("GetByID", mock.Anything, 1).Return(expectedApp, nil)

	newStatus := int8(0)
	updated := &App{ID: 1, Name: "Any", Status: 0}
	svc.On("Update", mock.Anything, 1, mock.Anything).Return(updated, nil)

	body, _ := json.Marshal(UpdateAppRequest{Status: &newStatus})
	req, _ := http.NewRequest("PUT", "/apps/1", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1, Role: auth.RoleSysAdmin, Roles: []auth.RoleAssignment{{Name: "sysadmin"}}})
	req = withURLParam(req, "id", "1")
	rr := httptest.NewRecorder()

	handler.UpdateApp(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestHandler_List_NoClaims_Unauthorized(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc, testPolicyStore)

	req, _ := http.NewRequest("GET", "/apps", nil)
	rr := httptest.NewRecorder()

	handler.ListApps(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHandler_LookupApp_Success(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc, testPolicyStore)

	pub := &PublicApp{ID: 3, Name: "Public App", Tagline: "hi"}
	svc.On("PublicBySiteKey", mock.Anything, "gk_abc").Return(pub, nil)

	req, _ := http.NewRequest("GET", "/apps/lookup?site_key=gk_abc", nil)
	rr := httptest.NewRecorder()

	handler.LookupApp(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp render.Response
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "Public App", data["name"])
}

func TestHandler_LookupApp_MissingSiteKey(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc, testPolicyStore)

	req, _ := http.NewRequest("GET", "/apps/lookup", nil)
	rr := httptest.NewRecorder()

	handler.LookupApp(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	svc.AssertNotCalled(t, "PublicBySiteKey", mock.Anything, mock.Anything)
}

func TestHandler_LookupApp_NotFound(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc, testPolicyStore)

	svc.On("PublicBySiteKey", mock.Anything, "gk_bad").Return(nil, ErrAppNotPublic)

	req, _ := http.NewRequest("GET", "/apps/lookup?site_key=gk_bad", nil)
	rr := httptest.NewRecorder()

	handler.LookupApp(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}
