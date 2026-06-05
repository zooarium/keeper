package app

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

func TestHandler_Create(t *testing.T) {
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
	rr := httptest.NewRecorder()

	handler.CreateApp(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var resp render.Response
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)

	dataMap := resp.Data.(map[string]interface{})
	assert.Equal(t, expectedApp.Name, dataMap["name"])
}

func TestHandler_List(t *testing.T) {
	svc := new(mockAppService)
	handler := NewAppHandler(svc)

	expectedApps := []*App{
		{ID: 1, Name: "App 1", Status: 1},
		{ID: 2, Name: "App 2", Status: 1},
	}

	svc.On("List", mock.Anything, mock.Anything, mock.Anything).Return(expectedApps, nil)

	req, _ := http.NewRequest("GET", "/apps", nil)
	rr := httptest.NewRecorder()

	handler.ListApps(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp render.Response
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)

	dataList := resp.Data.([]interface{})
	assert.Len(t, dataList, 2)
}
