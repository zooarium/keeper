package http

import (
	"time"

	_ "keeper/docs" // Import generated docs
	"keeper/internal/app"
	"keeper/internal/division"
	"keeper/internal/user"
	"keeper/pkg/auth"
	"keeper/pkg/config"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// NewRouter creates a new chi router with default middleware and application routes.
func NewRouter(userHandler *user.UserHandler, appHandler *app.AppHandler, divisionHandler *division.DivisionHandler, jwtManager *auth.JWTManager, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Add CORS middleware
	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any major browsers
	})
	r.Use(corsMiddleware.Handler)

	r.Use(httprate.LimitByIP(100, 1*time.Minute))

	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"), // The url pointing to API definition
	))

	r.Get("/health", HealthHandler)

	r.Mount("/users", userHandler.Routes(jwtManager))
	r.Mount("/apps", appHandler.Routes(jwtManager))
	r.Mount("/divisions", divisionHandler.Routes(jwtManager))

	return r
}
