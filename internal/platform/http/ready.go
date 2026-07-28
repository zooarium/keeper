package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"keeper/internal/db"
	"keeper/pkg/config"
	"keeper/pkg/render"

	entsql "entgo.io/ent/dialect/sql"
)

// falconTimeout falls back to a safe default when cfg.Falcon.Timeout is
// unset (e.g. a hand-built *config.Config in tests, bypassing Load()'s
// viper defaults).
func falconTimeout(cfg *config.Config) time.Duration {
	if cfg.Falcon.Timeout > 0 {
		return cfg.Falcon.Timeout
	}
	return 3 * time.Second
}

// ReadyHandler pings the database and falcon (login's role-resolution
// dependency, fail-closed) with a short timeout and reports whether the
// service is ready to receive traffic. Distinct from /health, which is a
// static liveness check and touches neither.
func ReadyHandler(drv *entsql.Driver, falconClient *http.Client, falconHealthURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.Ping(ctx, drv); err != nil {
			render.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "NOT_READY", "error": err.Error()})
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, falconHealthURL, nil)
		if err != nil {
			render.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "NOT_READY", "error": err.Error()})
			return
		}
		resp, err := falconClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			slog.Error("falcon not reachable", "url", falconHealthURL, "error", err)
			render.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "NOT_READY", "error": "falcon unreachable"})
			return
		}
		_ = resp.Body.Close()

		render.JSON(w, http.StatusOK, map[string]string{"status": "READY"})
	}
}
