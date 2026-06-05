package division

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

// DivisionHandler handles HTTP requests for divisions.
type DivisionHandler struct {
	svc      DivisionService
	validate *validator.Validate
}

// NewDivisionHandler creates a new division handler.
func NewDivisionHandler(svc DivisionService) *DivisionHandler {
	return &DivisionHandler{
		svc:      svc,
		validate: validator.New(),
	}
}

// Routes returns the chi router for division endpoints.
func (h *DivisionHandler) Routes(jwtManager *auth.JWTManager) chi.Router {
	r := chi.NewRouter()

	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(jwtManager))

		r.Post("/", h.CreateDivision)
		r.Get("/", h.ListDivisions)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.GetDivisionByID)
			r.Put("/", h.UpdateDivision)
			r.Delete("/", h.DeleteDivision)
			r.Get("/descendants", h.GetDescendants)
			r.Put("/move", h.MoveDivision)
		})
	})

	return r
}

func (h *DivisionHandler) claims(r *http.Request) (*auth.UserClaims, bool) {
	return auth.GetClaimsFromContext(r.Context())
}

func (h *DivisionHandler) parseID(r *http.Request) (int, error) {
	return strconv.Atoi(chi.URLParam(r, "id"))
}

// CreateDivision godoc
// @Summary Create a new division
// @Description Create a new division with optional parent for hierarchical grouping
// @Tags divisions
// @Accept json
// @Produce json
// @Param division body CreateDivisionRequest true "Division details"
// @Success 201 {object} render.Response{data=Division}
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 403 {object} render.Response
// @Failure 500 {object} render.Response
// @Security Bearer
// @Router /divisions [post]
func (h *DivisionHandler) CreateDivision(w http.ResponseWriter, r *http.Request) {
	c, ok := h.claims(r)
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreateDivisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("failed to decode create division request", "error", err)
		render.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		slog.Warn("invalid create division request", "error", err)
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if !c.IsSysAdmin() && req.AppID != c.AppID {
		slog.Warn("create division rejected: app_id mismatch", "claims_app_id", c.AppID, "req_app_id", req.AppID)
		render.Error(w, http.StatusForbidden, "access denied")
		return
	}

	d, err := h.svc.Create(r.Context(), req)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	render.JSON(w, http.StatusCreated, d)
}

// ListDivisions godoc
// @Summary List divisions
// @Description List divisions for the caller's app. Filter by parent_id via query param.
// @Tags divisions
// @Produce json
// @Param parent_id query int false "Filter by parent ID"
// @Param limit query int false "Max results (default 50, max 500)"
// @Param offset query int false "Result offset (default 0)"
// @Success 200 {object} render.Response{data=[]Division}
// @Failure 401 {object} render.Response
// @Failure 500 {object} render.Response
// @Security Bearer
// @Router /divisions [get]
func (h *DivisionHandler) ListDivisions(w http.ResponseWriter, r *http.Request) {
	c, ok := h.claims(r)
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var parentID *int
	if pidStr := r.URL.Query().Get("parent_id"); pidStr != "" {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			slog.Warn("invalid parent_id query param", "parent_id", pidStr)
			render.Error(w, http.StatusBadRequest, "invalid parent_id")
			return
		}
		parentID = &pid
	}

	appID := c.AppID
	if c.IsSysAdmin() {
		appID = 0
	}

	page := render.ParsePage(r)
	divisions, err := h.svc.List(r.Context(), appID, parentID, page.Limit, page.Offset)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	render.JSON(w, http.StatusOK, divisions)
}

// GetDivisionByID godoc
// @Summary Get division by ID
// @Description Get a single division by its ID (must belong to caller's app)
// @Tags divisions
// @Produce json
// @Param id path int true "Division ID"
// @Success 200 {object} render.Response{data=Division}
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 404 {object} render.Response
// @Security Bearer
// @Router /divisions/{id} [get]
func (h *DivisionHandler) GetDivisionByID(w http.ResponseWriter, r *http.Request) {
	c, ok := h.claims(r)
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := h.parseID(r)
	if err != nil {
		slog.Warn("invalid division id in request", "id", chi.URLParam(r, "id"))
		render.Error(w, http.StatusBadRequest, "invalid division id")
		return
	}

	appID := c.AppID
	if c.IsSysAdmin() {
		appID = 0
	}

	d, err := h.svc.GetByID(r.Context(), appID, id)
	if err != nil {
		slog.Warn("division not found", "id", id)
		render.Error(w, http.StatusNotFound, "division not found")
		return
	}

	render.JSON(w, http.StatusOK, d)
}

// GetDescendants godoc
// @Summary Get all descendants of a division
// @Description Returns all divisions in the subtree rooted at the given division
// @Tags divisions
// @Produce json
// @Param id path int true "Division ID"
// @Success 200 {object} render.Response{data=[]Division}
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 404 {object} render.Response
// @Security Bearer
// @Router /divisions/{id}/descendants [get]
func (h *DivisionHandler) GetDescendants(w http.ResponseWriter, r *http.Request) {
	c, ok := h.claims(r)
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := h.parseID(r)
	if err != nil {
		slog.Warn("invalid division id in descendants request", "id", chi.URLParam(r, "id"))
		render.Error(w, http.StatusBadRequest, "invalid division id")
		return
	}

	appID := c.AppID
	if c.IsSysAdmin() {
		appID = 0
	}

	descendants, err := h.svc.Descendants(r.Context(), appID, id)
	if err != nil {
		slog.Warn("division not found for descendants query", "id", id)
		render.Error(w, http.StatusNotFound, "division not found")
		return
	}

	render.JSON(w, http.StatusOK, descendants)
}

// UpdateDivision godoc
// @Summary Update a division
// @Description Update name or status of an existing division
// @Tags divisions
// @Accept json
// @Produce json
// @Param id path int true "Division ID"
// @Param division body UpdateDivisionRequest true "Updated division details"
// @Success 200 {object} render.Response{data=Division}
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 500 {object} render.Response
// @Security Bearer
// @Router /divisions/{id} [put]
func (h *DivisionHandler) UpdateDivision(w http.ResponseWriter, r *http.Request) {
	c, ok := h.claims(r)
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := h.parseID(r)
	if err != nil {
		slog.Warn("invalid division id in update request", "id", chi.URLParam(r, "id"))
		render.Error(w, http.StatusBadRequest, "invalid division id")
		return
	}

	var req UpdateDivisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("failed to decode update division request", "id", id, "error", err)
		render.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		slog.Warn("invalid update division request", "id", id, "error", err)
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	appID := c.AppID
	if c.IsSysAdmin() {
		appID = 0
	}

	d, err := h.svc.Update(r.Context(), appID, id, req)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	render.JSON(w, http.StatusOK, d)
}

// MoveDivision godoc
// @Summary Move a division to a new parent
// @Description Move a division (and its subtree) to a different parent. Set parent_id to null to promote to root.
// @Tags divisions
// @Accept json
// @Produce json
// @Param id path int true "Division ID"
// @Param move body MoveDivisionRequest true "New parent"
// @Success 200 {object} render.Response{data=Division}
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 500 {object} render.Response
// @Security Bearer
// @Router /divisions/{id}/move [put]
func (h *DivisionHandler) MoveDivision(w http.ResponseWriter, r *http.Request) {
	c, ok := h.claims(r)
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := h.parseID(r)
	if err != nil {
		slog.Warn("invalid division id in move request", "id", chi.URLParam(r, "id"))
		render.Error(w, http.StatusBadRequest, "invalid division id")
		return
	}

	var req MoveDivisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("failed to decode move division request", "id", id, "error", err)
		render.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	appID := c.AppID
	if c.IsSysAdmin() {
		appID = 0
	}

	d, err := h.svc.Move(r.Context(), appID, id, req)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	render.JSON(w, http.StatusOK, d)
}

// DeleteDivision godoc
// @Summary Delete a division
// @Description Delete a division. Blocked if it has children or assigned users.
// @Tags divisions
// @Produce json
// @Param id path int true "Division ID"
// @Success 204 "No Content"
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 500 {object} render.Response
// @Security Bearer
// @Router /divisions/{id} [delete]
func (h *DivisionHandler) DeleteDivision(w http.ResponseWriter, r *http.Request) {
	c, ok := h.claims(r)
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := h.parseID(r)
	if err != nil {
		slog.Warn("invalid division id in delete request", "id", chi.URLParam(r, "id"))
		render.Error(w, http.StatusBadRequest, "invalid division id")
		return
	}

	appID := c.AppID
	if c.IsSysAdmin() {
		appID = 0
	}

	if err := h.svc.Delete(r.Context(), appID, id); err != nil {
		render.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	render.JSON(w, http.StatusNoContent, nil)
}
