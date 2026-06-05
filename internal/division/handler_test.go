package division

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
	handler := NewDivisionHandler(svc)

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
	handler := NewDivisionHandler(svc)

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
	handler := NewDivisionHandler(svc)

	req, _ := http.NewRequest("POST", "/divisions", bytes.NewBufferString("not-json"))
	rr := httptest.NewRecorder()

	handler.CreateDivision(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestDivisionHandler_List_Response(t *testing.T) {
	svc := new(mockDivisionService)
	handler := NewDivisionHandler(svc)

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
