package http

import (
	"time"

	_ "keeper/docs" // Import generated docs
	"keeper/internal/app"
	"keeper/internal/division"
	"keeper/internal/guestkey"
	"keeper/internal/impersonation"
	"keeper/internal/user"
	"keeper/pkg/auth"
	"keeper/pkg/config"
	"keeper/pkg/httpclient"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// NewRouter creates a new chi router with default middleware and application routes.
func NewRouter(userHandler *user.UserHandler, appHandler *app.AppHandler, divisionHandler *division.DivisionHandler, guestKeyHandler *guestkey.GuestKeyHandler, impersonationHandler *impersonation.ImpersonationHandler, jwtManager *auth.JWTManager, cfg *config.Config, dbDriver *entsql.Driver) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(RequestLogger)
	r.Use(middleware.Recoverer)
	r.Use(MetricsMiddleware)

	// The impersonation exchange/logout/status endpoints are called cross-origin
	// from each registered service UI, so those origins must be allowed even if
	// the global CORS list is later tightened from "*".
	allowedOrigins := cfg.CORS.AllowedOrigins
	for i := range cfg.Services {
		allowedOrigins = append(allowedOrigins, cfg.Services[i].UIOrigin)
	}

	// Add CORS middleware
	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
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

	falconClient := httpclient.New(httpclient.Config{Timeout: falconTimeout(cfg), Name: "falcon-ready"})

	r.Get("/health", HealthHandler)
	r.Get("/ready", ReadyHandler(dbDriver, falconClient, cfg.Falcon.BaseURL+"/health"))

	// Prometheus metrics endpoint (exempt from JWT auth).
	r.Handle("/metrics", promhttp.Handler())

	r.Mount("/users", userHandler.Routes(jwtManager))
	r.Mount("/managers", userHandler.ManagerRoutes(jwtManager))
	r.Mount("/apps", appHandler.Routes(jwtManager))
	r.Mount("/divisions", divisionHandler.Routes(jwtManager))
	r.Mount("/guest-keys", guestKeyHandler.Routes(jwtManager))
	r.Mount("/impersonations", impersonationHandler.Routes(jwtManager))

	return r
}
