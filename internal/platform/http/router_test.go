package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"keeper/internal/app"
	"keeper/internal/division"
	"keeper/internal/guestkey"
	"keeper/internal/user"
	"keeper/pkg/auth"
	"keeper/pkg/config"

	"github.com/stretchr/testify/assert"
)

type mockUserService struct {
	user.UserService
}

func (m *mockUserService) List(ctx context.Context, appID, limit, offset int) ([]*user.User, error) {
	return []*user.User{}, nil
}

func (m *mockUserService) Authenticate(ctx context.Context, req user.AuthRequest) (*user.AuthResponse, error) {
	return &user.AuthResponse{}, nil
}

type mockAppService struct {
	app.AppService
}

func (m *mockAppService) List(ctx context.Context, limit, offset int) ([]*app.App, error) {
	return []*app.App{}, nil
}

type mockDivisionService struct {
	division.DivisionService
}

type mockGuestKeyService struct {
	guestkey.GuestKeyService
}

func (m *mockDivisionService) List(ctx context.Context, appID int, parentID *int, limit, offset int) ([]*division.Division, error) {
	return []*division.Division{}, nil
}

func TestRouterAuthentication(t *testing.T) {
	jwtManager := auth.NewJWTManager("secret", 1*time.Hour)

	userSvc := &mockUserService{}
	userHandler := user.NewUserHandler(userSvc)

	appSvc := &mockAppService{}
	appHandler := app.NewAppHandler(appSvc)

	divSvc := &mockDivisionService{}
	divHandler := division.NewDivisionHandler(divSvc)

	gkSvc := &mockGuestKeyService{}
	gkHandler := guestkey.NewGuestKeyHandler(gkSvc)

	cfg := &config.Config{
		CORS: config.CORSConfig{
			AllowedOrigins: []string{"*"},
		},
	}
	router := NewRouter(userHandler, appHandler, divHandler, gkHandler, jwtManager, cfg)

	tests := []struct {
		name           string
		method         string
		url            string
		wantStatusCode int
	}{
		{"Health public", "GET", "/health", http.StatusOK},
		{"Users Auth public", "POST", "/users/auth", http.StatusBadRequest},
		{"Users List protected", "GET", "/users", http.StatusUnauthorized},
		{"Users Create protected", "POST", "/users", http.StatusUnauthorized},
		{"Divisions List protected", "GET", "/divisions", http.StatusUnauthorized},
		{"Divisions Create protected", "POST", "/divisions", http.StatusUnauthorized},
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

	divSvc := &mockDivisionService{}
	divHandler := division.NewDivisionHandler(divSvc)

	gkSvc := &mockGuestKeyService{}
	gkHandler := guestkey.NewGuestKeyHandler(gkSvc)

	cfg := &config.Config{
		CORS: config.CORSConfig{
			AllowedOrigins: []string{"*"},
		},
	}
	router := NewRouter(userHandler, appHandler, divHandler, gkHandler, jwtManager, cfg)

	token, _ := jwtManager.Generate(1, 1, 1, 0)

	req, _ := http.NewRequest("GET", "/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.NotEqual(t, http.StatusUnauthorized, rr.Code)
}
