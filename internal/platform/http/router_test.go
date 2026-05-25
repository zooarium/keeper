package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"keeper/internal/app"
	"keeper/internal/user"
	"keeper/pkg/auth"
	"keeper/pkg/config"

	"github.com/stretchr/testify/assert"
)

type mockUserService struct {
	user.UserService
}

func (m *mockUserService) List(ctx context.Context, appID int) ([]*user.User, error) {
	return []*user.User{}, nil
}

func (m *mockUserService) Authenticate(ctx context.Context, req user.AuthRequest) (*user.AuthResponse, error) {
	return &user.AuthResponse{}, nil
}

type mockAppService struct {
	app.AppService
}

func (m *mockAppService) List(ctx context.Context) ([]*app.App, error) {
	return []*app.App{}, nil
}

func TestRouterAuthentication(t *testing.T) {
	jwtManager := auth.NewJWTManager("secret", 1*time.Hour)
	// Use a mock service to avoid nil pointer dereference in handlers
	userSvc := &mockUserService{}
	userHandler := user.NewUserHandler(userSvc)

	appSvc := &mockAppService{}
	appHandler := app.NewAppHandler(appSvc)

	cfg := &config.Config{
		CORS: config.CORSConfig{
			AllowedOrigins: []string{"*"},
		},
	}
	router := NewRouter(userHandler, appHandler, jwtManager, cfg)

	tests := []struct {
		name           string
		method         string
		url            string
		wantStatusCode int
	}{
		{"Health public", "GET", "/health", http.StatusOK},
		{"Users Auth public", "POST", "/users/auth", http.StatusBadRequest}, // 400 because of empty body
		{"Users List protected", "GET", "/users", http.StatusUnauthorized},
		{"Users Create protected", "POST", "/users", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body *bytes.Buffer
			if tt.method == "POST" {
				body = bytes.NewBuffer([]byte("{}"))
			}

			var req *http.Request
			if body != nil {
				req, _ = http.NewRequest(tt.method, tt.url, body)
			} else {
				req, _ = http.NewRequest(tt.method, tt.url, nil)
			}

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			assert.Equal(t, tt.wantStatusCode, rr.Code)
		})
	}
}

func TestRouterAuthentication_ValidToken(t *testing.T) {
	jwtManager := auth.NewJWTManager("secret", 1*time.Hour)
	userSvc := &mockUserService{}
	userHandler := user.NewUserHandler(userSvc)

	appSvc := &mockAppService{}
	appHandler := app.NewAppHandler(appSvc)

	cfg := &config.Config{
		CORS: config.CORSConfig{
			AllowedOrigins: []string{"*"},
		},
	}
	router := NewRouter(userHandler, appHandler, jwtManager, cfg)

	token, _ := jwtManager.Generate(1, 1)

	req, _ := http.NewRequest("GET", "/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// If middleware works, it shouldn't be StatusUnauthorized (401).
	// It will be 500 because s.List is nil and it panics, and Recoverer catches it.
	assert.NotEqual(t, http.StatusUnauthorized, rr.Code)
}
