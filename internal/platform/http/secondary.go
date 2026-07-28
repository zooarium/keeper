package http

import (
	"fmt"
	"net/http"
	"strings"

	"keeper/pkg/auth"
	"keeper/pkg/config"
	"keeper/pkg/httpclient"
	"keeper/pkg/render"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var allowedMethods = map[string]bool{
	http.MethodGet: true, http.MethodPost: true, http.MethodPut: true,
	http.MethodPatch: true, http.MethodDelete: true, http.MethodHead: true,
	http.MethodOptions: true,
}

// NewSecondaryRouter builds the router for one secondary listener. It reuses
// the same entity handlers (via the mount hook) but only exposes the routes
// allow-listed in the listener's ROUTES, with rate limiting driven by its
// config. Keeper bakes JWT auth into the entity routers themselves (with
// public exceptions such as POST /users/auth), so listeners always inherit
// that protection — AUTH: false is rejected at config load. A per-listener
// JWT_SECRET swaps the manager the entity routers verify with (e.g. the
// guest secret). Swagger is not exposed; /health and /metrics are.
func NewSecondaryRouter(cfg *config.Config, sec *config.SecondaryConfig, jwtManager *auth.JWTManager, mount func(r chi.Router, jm *auth.JWTManager), dbDriver *entsql.Driver) (*chi.Mux, error) {
	allow, err := allowRoutes(sec.Routes)
	if err != nil {
		return nil, err
	}

	jm := jwtManager
	if sec.JWTSecret != "" {
		jm = auth.NewJWTManager(sec.JWTSecret, 0)
	}

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(RequestLogger)
	r.Use(middleware.Recoverer)
	r.Use(MetricsMiddleware)

	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	})
	r.Use(corsMiddleware.Handler)

	r.Use(httprate.LimitByIP(sec.RateLimit.Requests, sec.RateLimit.Window))

	falconClient := httpclient.New(httpclient.Config{Timeout: falconTimeout(cfg), Name: "falcon-ready-" + sec.Name})

	r.Get("/health", HealthHandler)
	r.Get("/ready", ReadyHandler(dbDriver, falconClient, cfg.Falcon.BaseURL+"/health"))

	// Prometheus metrics endpoint, exempt from auth and the allow-list.
	r.Handle("/metrics", promhttp.Handler())

	r.Group(func(r chi.Router) {
		r.Use(allow)
		mount(r, jm)
	})

	return r, nil
}

// ValidateRoutes checks "METHOD /path" allow-list patterns without building
// a router. Used by the -check-config flag to vet config before deployment.
func ValidateRoutes(patterns []string) error {
	_, err := allowRoutes(patterns)
	return err
}

// allowRoutes returns a middleware rejecting any request that does not match
// one of the configured "METHOD /path" patterns (chi syntax, e.g.
// "GET /users/{id}"). Patterns are validated up front so a typo fails at
// startup instead of silently exposing or hiding routes.
func allowRoutes(patterns []string) (mw func(http.Handler) http.Handler, err error) {
	// chi panics on malformed path patterns (e.g. unbalanced braces);
	// surface those as config errors rather than crashing.
	defer func() {
		if r := recover(); r != nil {
			mw, err = nil, fmt.Errorf("invalid secondary route pattern: %v", r)
		}
	}()

	matcher := chi.NewMux()
	stub := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	for _, p := range patterns {
		method, path, ok := strings.Cut(p, " ")
		method = strings.ToUpper(strings.TrimSpace(method))
		path = strings.TrimSpace(path)
		if !ok || !allowedMethods[method] || !strings.HasPrefix(path, "/") {
			return nil, fmt.Errorf("invalid secondary route pattern %q, want \"METHOD /path\"", p)
		}
		matcher.Method(method, path, stub)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rctx := chi.NewRouteContext()
			if !matcher.Match(rctx, r.Method, r.URL.Path) {
				render.Error(w, http.StatusNotFound, "not found")
				return
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}
