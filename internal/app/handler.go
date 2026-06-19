package app

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"keeper/pkg/auth"
	"keeper/pkg/render"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

// AppHandler handles HTTP requests for apps.
type AppHandler struct {
	svc      AppService
	validate *validator.Validate
}

// NewAppHandler creates a new app handler.
func NewAppHandler(svc AppService) *AppHandler {
	v := validator.New()
	// httpurl accepts an empty value (optional field) or a valid http(s) URL.
	// Needed because validator's omitempty does not skip a non-nil *string
	// pointing to "" (e.g. clearing logo_url on update).
	_ = v.RegisterValidation("httpurl", func(fl validator.FieldLevel) bool {
		return isHTTPURL(fl.Field().String())
	})
	return &AppHandler{
		svc:      svc,
		validate: v,
	}
}

// Routes returns the chi router for app endpoints.
func (h *AppHandler) Routes(jwtManager *auth.JWTManager) chi.Router {
	r := chi.NewRouter()

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
	if !claims.IsSysAdmin() {
		slog.Warn("non-sysadmin attempted to create app", "app_id", claims.AppID, "user_id", claims.UserID)
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

	// Non-sysadmins may only see their own app.
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

	if !claims.IsSysAdmin() && id != claims.AppID {
		render.Error(w, http.StatusForbidden, "access denied")
		return
	}

	a, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		slog.Warn("app not found", "id", id)
		render.Error(w, http.StatusNotFound, "app not found")
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

	if !claims.IsSysAdmin() && id != claims.AppID {
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
