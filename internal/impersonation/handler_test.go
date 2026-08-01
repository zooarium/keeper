package impersonation

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"keeper/internal/policy"
	"keeper/pkg/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func strPtr(s string) *string { return &s }

// handlerTestPolicyStore mirrors falcon's export shape: sysadmin sudo bypasses
// everything; support holds impersonation.read only (no create).
var handlerTestPolicyStore = policy.NewStoreFromPolicies(policy.Compile([]policy.Row{
	{Role: "sysadmin", IsSudo: true},
	{Role: "support", Resource: strPtr("impersonation"), Action: strPtr("read")},
}))

func withClaims(req *http.Request, claims *auth.UserClaims) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), auth.UserClaimsKey, claims))
}

type mockImpersonationService struct {
	mock.Mock
}

func (m *mockImpersonationService) Start(ctx context.Context, impersonatorUserID int, req StartImpersonationRequest) (*StartImpersonationResponse, error) {
	args := m.Called(ctx, impersonatorUserID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*StartImpersonationResponse), args.Error(1)
}

func (m *mockImpersonationService) Exchange(ctx context.Context, req ExchangeRequest) (*ExchangeResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ExchangeResponse), args.Error(1)
}

func (m *mockImpersonationService) Revoke(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *mockImpersonationService) RevokeByID(ctx context.Context, id int) (*ImpersonationSession, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ImpersonationSession), args.Error(1)
}

func (m *mockImpersonationService) GetByID(ctx context.Context, id int) (*ImpersonationSession, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ImpersonationSession), args.Error(1)
}

func (m *mockImpersonationService) List(ctx context.Context, appID, limit, offset int) ([]*ImpersonationSession, error) {
	args := m.Called(ctx, appID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ImpersonationSession), args.Error(1)
}

func (m *mockImpersonationService) IsSessionActive(ctx context.Context, sessionID string) (bool, error) {
	args := m.Called(ctx, sessionID)
	return args.Bool(0), args.Error(1)
}

func (m *mockImpersonationService) Services() []ServiceInfo {
	args := m.Called()
	return args.Get(0).([]ServiceInfo)
}

func TestListSessions_Unauthenticated(t *testing.T) {
	svc := new(mockImpersonationService)
	handler := NewImpersonationHandler(svc, handlerTestPolicyStore)

	req, _ := http.NewRequest("GET", "/impersonations", nil)
	rr := httptest.NewRecorder()

	handler.ListSessions(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	svc.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestListSessions_NoPermission(t *testing.T) {
	svc := new(mockImpersonationService)
	handler := NewImpersonationHandler(svc, handlerTestPolicyStore)

	req, _ := http.NewRequest("GET", "/impersonations", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1})
	rr := httptest.NewRecorder()

	handler.ListSessions(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "List", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestListSessions_ReadPermissionGranted(t *testing.T) {
	svc := new(mockImpersonationService)
	handler := NewImpersonationHandler(svc, handlerTestPolicyStore)

	svc.On("List", mock.Anything, 0, 50, 0).Return([]*ImpersonationSession{}, nil)

	req, _ := http.NewRequest("GET", "/impersonations", nil)
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1, Roles: []auth.RoleAssignment{{Name: "support"}}})
	rr := httptest.NewRecorder()

	handler.ListSessions(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestStart_RequiresCreatePermission(t *testing.T) {
	svc := new(mockImpersonationService)
	handler := NewImpersonationHandler(svc, handlerTestPolicyStore)

	body, _ := json.Marshal(StartImpersonationRequest{TargetUserID: 42, Audience: "squirrel"})
	req, _ := http.NewRequest("POST", "/impersonations", bytes.NewBuffer(body))
	// "support" role only grants impersonation.read, not create.
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1, Roles: []auth.RoleAssignment{{Name: "support"}}})
	rr := httptest.NewRecorder()

	handler.Start(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	svc.AssertNotCalled(t, "Start", mock.Anything, mock.Anything, mock.Anything)
}

func TestStart_SudoGranted(t *testing.T) {
	svc := new(mockImpersonationService)
	handler := NewImpersonationHandler(svc, handlerTestPolicyStore)

	expected := &StartImpersonationResponse{Code: "impc_x", SessionID: "imps_x", Audience: "squirrel"}
	svc.On("Start", mock.Anything, 1, StartImpersonationRequest{TargetUserID: 42, Audience: "squirrel"}).Return(expected, nil)

	body, _ := json.Marshal(StartImpersonationRequest{TargetUserID: 42, Audience: "squirrel"})
	req, _ := http.NewRequest("POST", "/impersonations", bytes.NewBuffer(body))
	req = withClaims(req, &auth.UserClaims{AppID: 1, UserID: 1, Roles: []auth.RoleAssignment{{Name: "sysadmin"}}})
	rr := httptest.NewRecorder()

	handler.Start(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}
