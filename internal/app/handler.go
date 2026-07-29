package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"keeper/internal/policy"
	"keeper/pkg/auth"
	"keeper/pkg/render"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"github.com/go-playground/validator/v10"
)

// AppHandler handles HTTP requests for apps.
type AppHandler struct {
	svc      AppService
	policy   *policy.Store
	validate *validator.Validate
}

// NewAppHandler creates a new app handler.
func NewAppHandler(svc AppService, policyStore *policy.Store) *AppHandler {
	v := validator.New()
	// httpurl accepts an empty value (optional field) or a valid http(s) URL.
	// Needed because validator's omitempty does not skip a non-nil *string
	// pointing to "" (e.g. clearing logo_url on update).
	_ = v.RegisterValidation("httpurl", func(fl validator.FieldLevel) bool {
		return isHTTPURL(fl.Field().String())
	})
	return &AppHandler{
		svc:      svc,
		policy:   policyStore,
		validate: v,
	}
}

// Routes returns the chi router for app endpoints.
func (h *AppHandler) Routes(jwtManager *auth.JWTManager) chi.Router {
	r := chi.NewRouter()

	// Public, hard rate-limited site-key lookup for public websites. Mirrors
	// the guest-keys/lookup containment: no auth, 10 req/min per IP.
	r.Group(func(r chi.Router) {
		r.Use(httprate.LimitByIP(10, 1*time.Minute))
		r.Get("/lookup", h.LookupApp)
		// Public-safe profile by app ID, for downstream services enriching
		// their responses (e.g. ant order details). Same projection and rate
		// containment as the site-key lookup.
		r.Get("/{id}/public", h.PublicAppByID)
	})

	// All routes are protected
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(jwtManager))

		r.Post("/", h.CreateApp)
		r.Get("/", h.ListApps)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.GetAppByID)
			r.Put("/", h.UpdateApp)
			r.Delete("/", h.DeleteApp)
		})
	})

	return r
}

// authorizeApp fetches the app and reports whether claims may access it:
// sudo (any app), the caller's own tenant app, or a manager assigned to it
// via app.manager_id (ownership fallback — outside Can(), Tier 3 territory).
func (h *AppHandler) authorizeApp(ctx context.Context, claims *auth.UserClaims, id int) (*App, bool) {
	a, err := h.svc.GetByID(ctx, id)
	if err != nil {
		return nil, false
	}
	if id == claims.AppID || policy.Can(ctx, h.policy, claims, id, "app", "read", "") {
		return a, true
	}
	return a, a.ManagerID != nil && *a.ManagerID == claims.UserID
}

// LookupApp godoc
// @Summary Look up public app profile by site key
// @Description Resolve the public-safe profile (name, tagline, logo, about, contact, currency) for the app bound to a publishable guest site key. Public (no auth) and hard rate-limited. Returns 404 for an unknown/inactive site key or an inactive app, without distinguishing between them.
// @Tags apps
// @Produce json
// @Param site_key query string true "Publishable guest site key (gk_...)"
// @Success 200 {object} render.Response{data=PublicApp}
// @Failure 400 {object} render.Response
// @Failure 404 {object} render.Response
// @Failure 429 {object} render.Response
// @Router /apps/lookup [get]
func (h *AppHandler) LookupApp(w http.ResponseWriter, r *http.Request) {
	siteKey := r.URL.Query().Get("site_key")
	if siteKey == "" {
		render.Error(w, http.StatusBadRequest, "site_key query param is required")
		return
	}

	a, err := h.svc.PublicBySiteKey(r.Context(), siteKey)
	if err != nil {
		if errors.Is(err, ErrAppNotPublic) {
			slog.Warn("public app lookup found no match", "remote_addr", r.RemoteAddr)
			render.Error(w, http.StatusNotFound, err.Error())
			return
		}
		render.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	render.JSON(w, http.StatusOK, a)
}

// PublicAppByID godoc
// @Summary Get public app profile by ID
// @Description Public-safe profile (name, tagline, logo, about, contact, currency) for an active app. Public (no auth) and hard rate-limited; used by downstream services to enrich their responses. Returns 404 for an unknown or inactive app.
// @Tags apps
// @Produce json
// @Param id path int true "App ID"
// @Success 200 {object} render.Response{data=PublicApp}
// @Failure 400 {object} render.Response
// @Failure 404 {object} render.Response
// @Failure 429 {object} render.Response
// @Router /apps/{id}/public [get]
func (h *AppHandler) PublicAppByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		render.Error(w, http.StatusBadRequest, "invalid app id")
		return
	}

	a, err := h.svc.PublicByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrAppNotPublic) {
			render.Error(w, http.StatusNotFound, err.Error())
			return
		}
		render.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	render.JSON(w, http.StatusOK, a)
}

// CreateApp godoc
// @Summary Create a new app
// @Description Create a new app with the provided details
// @Tags apps
// @Accept json
// @Produce json
// @Param app body CreateAppRequest true "App details"
// @Success 201 {object} render.Response{data=App}
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 403 {object} render.Response
// @Failure 500 {object} render.Response
// @Security Bearer
// @Router /apps [post]
func (h *AppHandler) CreateApp(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaimsFromContext(r.Context())
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !policy.Can(r.Context(), h.policy, claims, claims.AppID, "app", "create", "") {
		slog.Warn("create app rejected: caller lacks app.create permission", "app_id", claims.AppID, "user_id", claims.UserID)
		render.Error(w, http.StatusForbidden, "access denied")
		return
	}

	var req CreateAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("failed to decode create app request", "error", err)
		render.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		slog.Warn("invalid create app request", "error", err)
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	a, err := h.svc.Create(r.Context(), req)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	render.JSON(w, http.StatusCreated, a)
}

// ListApps godoc
// @Summary List all apps
// @Description Get a list of all registered apps
// @Tags apps
// @Produce json
// @Param limit query int false "Max results (default 50, max 500)"
// @Param offset query int false "Result offset (default 0)"
// @Success 200 {object} render.Response{data=[]App}
// @Failure 401 {object} render.Response
// @Failure 500 {object} render.Response
// @Security Bearer
// @Router /apps [get]
func (h *AppHandler) ListApps(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaimsFromContext(r.Context())
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Managers see the apps they've been assigned to manage.
	if claims.IsManager() {
		page := render.ParsePage(r)
		apps, err := h.svc.ListByManager(r.Context(), claims.UserID, page.Limit, page.Offset)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		render.JSON(w, http.StatusOK, apps)
		return
	}

	// Other non-sysadmins may only see their own app.
	if !claims.IsSysAdmin() {
		a, err := h.svc.GetByID(r.Context(), claims.AppID)
		if err != nil {
			render.JSON(w, http.StatusOK, []*App{})
			return
		}
		render.JSON(w, http.StatusOK, []*App{a})
		return
	}

	page := render.ParsePage(r)
	apps, err := h.svc.List(r.Context(), page.Limit, page.Offset)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	render.JSON(w, http.StatusOK, apps)
}

// GetAppByID godoc
// @Summary Get app by ID
// @Description Get a single app by its unique ID
// @Tags apps
// @Produce json
// @Param id path int true "App ID"
// @Success 200 {object} render.Response{data=App}
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 404 {object} render.Response
// @Security Bearer
// @Router /apps/{id} [get]
func (h *AppHandler) GetAppByID(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaimsFromContext(r.Context())
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		slog.Warn("invalid app id in request", "id", idStr)
		render.Error(w, http.StatusBadRequest, "invalid app id")
		return
	}

	a, allowed := h.authorizeApp(r.Context(), claims, id)
	if a == nil {
		slog.Warn("app not found", "id", id)
		render.Error(w, http.StatusNotFound, "app not found")
		return
	}
	if !allowed {
		render.Error(w, http.StatusForbidden, "access denied")
		return
	}

	render.JSON(w, http.StatusOK, a)
}

// UpdateApp godoc
// @Summary Update app
// @Description Update an existing app's details
// @Tags apps
// @Accept json
// @Produce json
// @Param id path int true "App ID"
// @Param app body UpdateAppRequest true "Updated app details"
// @Success 200 {object} render.Response{data=App}
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 500 {object} render.Response
// @Security Bearer
// @Router /apps/{id} [put]
func (h *AppHandler) UpdateApp(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaimsFromContext(r.Context())
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		slog.Warn("invalid app id in update request", "id", idStr)
		render.Error(w, http.StatusBadRequest, "invalid app id")
		return
	}

	existing, allowed := h.authorizeApp(r.Context(), claims, id)
	if existing == nil {
		render.Error(w, http.StatusNotFound, "app not found")
		return
	}
	if !allowed {
		render.Error(w, http.StatusForbidden, "access denied")
		return
	}

	var req UpdateAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("failed to decode update app request", "id", id, "error", err)
		render.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		slog.Warn("invalid update app request", "id", id, "error", err)
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if !policy.Can(r.Context(), h.policy, claims, id, "app", "update", "") {
		slog.Warn("update app rejected: caller lacks app.update permission", "id", id, "user_id", claims.UserID)
		render.Error(w, http.StatusForbidden, "access denied")
		return
	}

	if req.ManagerID != nil && !policy.Can(r.Context(), h.policy, claims, id, "app", "update", "manager_id") {
		slog.Warn("update app rejected: caller lacks app.update.manager_id permission", "id", id, "user_id", claims.UserID)
		render.Error(w, http.StatusForbidden, "access denied")
		return
	}

	if req.Status != nil && !policy.Can(r.Context(), h.policy, claims, id, "app", "update", "status") {
		slog.Warn("update app rejected: caller lacks app.update.status permission", "id", id, "user_id", claims.UserID)
		render.Error(w, http.StatusForbidden, "access denied")
		return
	}

	a, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	render.JSON(w, http.StatusOK, a)
}

// DeleteApp godoc
// @Summary Delete app
// @Description Remove an app from the system by ID
// @Tags apps
// @Produce json
// @Param id path int true "App ID"
// @Success 204 "No Content"
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 500 {object} render.Response
// @Security Bearer
// @Router /apps/{id} [delete]
func (h *AppHandler) DeleteApp(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaimsFromContext(r.Context())
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		slog.Warn("invalid app id in delete request", "id", idStr)
		render.Error(w, http.StatusBadRequest, "invalid app id")
		return
	}

	if !claims.IsSysAdmin() && id != claims.AppID {
		render.Error(w, http.StatusForbidden, "access denied")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		render.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	render.JSON(w, http.StatusNoContent, nil)
}
