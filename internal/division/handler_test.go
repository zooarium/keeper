package division

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

// testPolicyStore mirrors falcon's exported policy: sysadmin sudo bypasses
// everything; admin holds division CRUD scoped to its own tenant (Scope "own").
var testPolicyStore = policy.NewStoreFromPolicies(policy.Compile([]policy.Row{
	{Role: "sysadmin", IsSudo: true},
	{Role: "admin", Resource: strPtr("division"), Action: strPtr("create"), Scope: strPtr("own")},
	{Role: "admin", Resource: strPtr("division"), Action: strPtr("read"), Scope: strPtr("own")},
	{Role: "admin", Resource: strPtr("division"), Action: strPtr("update"), Scope: strPtr("own")},
	{Role: "admin", Resource: strPtr("division"), Action: strPtr("move"), Scope: strPtr("own")},
	{Role: "admin", Resource: strPtr("division"), Action: strPtr("delete"), Scope: strPtr("own")},
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

type mockDivisionService struct {
	mock.Mock
}

func (m *mockDivisionService) Create(ctx context.Context, req CreateDivisionRequest) (*Division, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Division), args.Error(1)
}

func (m *mockDivisionService) GetByID(ctx context.Context, appID, id int) (*Division, error) {
	args := m.Called(ctx, appID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Division), args.Error(1)
}

func (m *mockDivisionService) List(ctx context.Context, appID int, parentID *int, limit, offset int) ([]*Division, error) {
	args := m.Called(ctx, appID, parentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Division), args.Error(1)
}

func (m *mockDivisionService) Descendants(ctx context.Context, appID, id int) ([]*Division, error) {
	args := m.Called(ctx, appID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Division), args.Error(1)
}

func (m *mockDivisionService) Update(ctx context.Context, appID, id int, req UpdateDivisionRequest) (*Division, error) {
	args := m.Called(ctx, appID, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Division), args.Error(1)
}

func (m *mockDivisionService) Move(ctx context.Context, appID, id int, req MoveDivisionRequest) (*Division, error) {
	args := m.Called(ctx, appID, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Division), args.Error(1)
}

func (m *mockDivisionService) Delete(ctx context.Context, appID, id int) error {
	args := m.Called(ctx, appID, id)
	return args.Error(0)
}

func TestDivisionHandler_Create(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	reqBody := CreateDivisionRequest{
		AppID: 1,
		Name:  "Root",
	}

	expected := &Division{
		ID:    1,
		AppID: 1,
		Name:  "Root",
		Path:  "/1/",
		Depth: 0,
	}

	svc.On("Create", mock.Anything, reqBody).Return(expected, nil)

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/divisions", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.CreateDivision(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestDivisionHandler_List(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	expected := []*Division{
		{ID: 1, AppID: 1, Name: "Root 1", Path: "/1/", Depth: 0},
		{ID: 2, AppID: 1, Name: "Root 2", Path: "/2/", Depth: 0},
	}

	svc.On("List", mock.Anything, 1, (*int)(nil), mock.Anything, mock.Anything).Return(expected, nil)

	req, _ := http.NewRequest("GET", "/divisions", nil)
	rr := httptest.NewRecorder()

	handler.ListDivisions(rr, req)

	// No JWT in context → 401
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestDivisionHandler_Create_InvalidBody(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	req, _ := http.NewRequest("POST", "/divisions", bytes.NewBufferString("not-json"))
	rr := httptest.NewRecorder()

	handler.CreateDivision(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestDivisionHandler_List_Response(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	expected := []*Division{
		{ID: 1, AppID: 1, Name: "Root", Path: "/1/", Depth: 0, Status: 1},
	}

	svc.On("List", mock.Anything, mock.Anything, (*int)(nil), mock.Anything, mock.Anything).Return(expected, nil)

	req, _ := http.NewRequest("GET", "/divisions", nil)
	rr := httptest.NewRecorder()

	handler.ListDivisions(rr, req)

	var resp render.Response
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)
}

func TestDivisionHandler_CreateDivision_Admin_CanCreate(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	reqBody := CreateDivisionRequest{AppID: 1, Name: "Root"}
	expected := &Division{ID: 1, AppID: 1, Name: "Root", Path: "/1/"}
	svc.On("Create", mock.Anything, reqBody).Return(expected, nil)

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/divisions", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	rr := httptest.NewRecorder()

	handler.CreateDivision(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	svc.AssertExpectations(t)
}

func TestDivisionHandler_CreateDivision_NoPermission_Forbidden(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	reqBody := CreateDivisionRequest{AppID: 1, Name: "Root"}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/divisions", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 5, Roles: []auth.RoleAssignment{{Name: "unknown_role"}}})
	rr := httptest.NewRecorder()

	handler.CreateDivision(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestDivisionHandler_CreateDivision_OwnScope_CrossTenant_Forbidden(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	reqBody := CreateDivisionRequest{AppID: 2, Name: "Root"}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/divisions", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	rr := httptest.NewRecorder()

	handler.CreateDivision(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestDivisionHandler_ListDivisions_OwnScope_ScopedToCallerApp(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	expected := []*Division{{ID: 1, AppID: 1}}
	svc.On("List", mock.Anything, 1, (*int)(nil), 50, 0).Return(expected, nil)

	req, _ := http.NewRequest("GET", "/divisions", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	rr := httptest.NewRecorder()

	handler.ListDivisions(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestDivisionHandler_ListDivisions_AnyScope_Unrestricted(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	expected := []*Division{{ID: 1, AppID: 1}, {ID: 2, AppID: 2}}
	svc.On("List", mock.Anything, 0, (*int)(nil), 50, 0).Return(expected, nil)

	req, _ := http.NewRequest("GET", "/divisions", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1, Roles: []auth.RoleAssignment{{Name: "sysadmin"}}})
	rr := httptest.NewRecorder()

	handler.ListDivisions(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestDivisionHandler_ListDivisions_NoPermission_Forbidden(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	req, _ := http.NewRequest("GET", "/divisions", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 5, Roles: []auth.RoleAssignment{{Name: "unknown_role"}}})
	rr := httptest.NewRecorder()

	handler.ListDivisions(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestDivisionHandler_GetDivisionByID_OwnScope_ScopedToCallerApp(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	expected := &Division{ID: 4, AppID: 1}
	svc.On("GetByID", mock.Anything, 1, 4).Return(expected, nil)

	req, _ := http.NewRequest("GET", "/divisions/4", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.GetDivisionByID(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestDivisionHandler_GetDivisionByID_AnyScope_Unrestricted(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	expected := &Division{ID: 4, AppID: 2}
	svc.On("GetByID", mock.Anything, 0, 4).Return(expected, nil)

	req, _ := http.NewRequest("GET", "/divisions/4", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1, Roles: []auth.RoleAssignment{{Name: "sysadmin"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.GetDivisionByID(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestDivisionHandler_GetDivisionByID_NoPermission_Forbidden(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	req, _ := http.NewRequest("GET", "/divisions/4", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 5, Roles: []auth.RoleAssignment{{Name: "unknown_role"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.GetDivisionByID(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything, mock.Anything)
}

func TestDivisionHandler_GetDescendants_OwnScope_ScopedToCallerApp(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	expected := []*Division{{ID: 5, AppID: 1}}
	svc.On("Descendants", mock.Anything, 1, 4).Return(expected, nil)

	req, _ := http.NewRequest("GET", "/divisions/4/descendants", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.GetDescendants(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestDivisionHandler_GetDescendants_AnyScope_Unrestricted(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	expected := []*Division{{ID: 5, AppID: 2}}
	svc.On("Descendants", mock.Anything, 0, 4).Return(expected, nil)

	req, _ := http.NewRequest("GET", "/divisions/4/descendants", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1, Roles: []auth.RoleAssignment{{Name: "sysadmin"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.GetDescendants(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestDivisionHandler_GetDescendants_NoPermission_Forbidden(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	req, _ := http.NewRequest("GET", "/divisions/4/descendants", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 5, Roles: []auth.RoleAssignment{{Name: "unknown_role"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.GetDescendants(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Descendants", mock.Anything, mock.Anything, mock.Anything)
}

func TestDivisionHandler_UpdateDivision_Admin_CanUpdate(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	newName := "Renamed"
	reqBody := UpdateDivisionRequest{Name: &newName}
	updated := &Division{ID: 4, AppID: 1, Name: newName}
	svc.On("Update", mock.Anything, 1, 4, reqBody).Return(updated, nil)

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("PUT", "/divisions/4", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.UpdateDivision(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestDivisionHandler_UpdateDivision_NoPermission_Forbidden(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	newName := "Renamed"
	reqBody := UpdateDivisionRequest{Name: &newName}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("PUT", "/divisions/4", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 5, Roles: []auth.RoleAssignment{{Name: "unknown_role"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.UpdateDivision(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestDivisionHandler_MoveDivision_Admin_CanMove(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	newParentID := 2
	reqBody := MoveDivisionRequest{ParentID: &newParentID}
	moved := &Division{ID: 4, AppID: 1, ParentID: &newParentID}
	svc.On("Move", mock.Anything, 1, 4, reqBody).Return(moved, nil)

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("PUT", "/divisions/4/move", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.MoveDivision(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	svc.AssertExpectations(t)
}

func TestDivisionHandler_MoveDivision_NoPermission_Forbidden(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	newParentID := 2
	reqBody := MoveDivisionRequest{ParentID: &newParentID}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("PUT", "/divisions/4/move", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 5, Roles: []auth.RoleAssignment{{Name: "unknown_role"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.MoveDivision(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Move", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestDivisionHandler_DeleteDivision_Admin_CanDelete(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	svc.On("Delete", mock.Anything, 1, 4).Return(nil)

	req, _ := http.NewRequest("DELETE", "/divisions/4", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 3, Roles: []auth.RoleAssignment{{Name: "admin"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.DeleteDivision(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	svc.AssertExpectations(t)
}

func TestDivisionHandler_DeleteDivision_NoPermission_Forbidden(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc, testPolicyStore)

	req, _ := http.NewRequest("DELETE", "/divisions/4", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 5, Roles: []auth.RoleAssignment{{Name: "unknown_role"}}})
	req = withURLParam(req, "id", "4")
	rr := httptest.NewRecorder()

	handler.DeleteDivision(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
}
