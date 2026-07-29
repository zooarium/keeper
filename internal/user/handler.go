package user

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"keeper/internal/policy"
	"keeper/pkg/auth"
	"keeper/pkg/render"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

// UserHandler handles HTTP requests for users.
type UserHandler struct {
	svc      UserService
	policy   *policy.Store
	validate *validator.Validate
}

// NewUserHandler creates a new user handler.
func NewUserHandler(svc UserService, policyStore *policy.Store) *UserHandler {
	return &UserHandler{
		svc:      svc,
		policy:   policyStore,
		validate: validator.New(),
	}
}

// Routes returns the chi router for user endpoints.
func (h *UserHandler) Routes(jwtManager *auth.JWTManager) chi.Router {
	r := chi.NewRouter()

	// Public routes
	r.Post("/auth", h.AuthenticateUser)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(jwtManager))

		r.Post("/", h.CreateUser)
		r.Get("/", h.ListUsers)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.GetUserByID)
			r.Put("/", h.UpdateUser)
			r.Delete("/", h.DeleteUser)
		})
	})

	return r
}

// ManagerRoutes returns the chi router for the top-level /managers endpoint.
// Mounted separately from Routes because managers span apps (via
// kpr_app.manager_id) rather than belonging to the caller's own tenant.
func (h *UserHandler) ManagerRoutes(jwtManager *auth.JWTManager) chi.Router {
	r := chi.NewRouter()
	r.Use(auth.Middleware(jwtManager))
	r.Get("/", h.ListManagers)
	return r
}

func (h *UserHandler) claims(r *http.Request) (*auth.UserClaims, bool) {
	return auth.GetClaimsFromContext(r.Context())
}

// CreateUser godoc
// @Summary Create a new user
// @Description Create a new user with the provided details
// @Tags users
// @Accept json
// @Produce json
// @Param user body CreateUserRequest true "User details"
// @Success 201 {object} render.Response{data=User}
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 500 {object} render.Response
// @Security Bearer
// @Router /users [post]
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	c, ok := h.claims(r)
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("failed to decode create user request", "error", err)
		render.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		slog.Warn("invalid create user request", "error", err)
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if !c.IsSysAdmin() && req.AppID != c.AppID {
		slog.Warn("create user rejected: app_id mismatch", "claims_app_id", c.AppID, "req_app_id", req.AppID)
		render.Error(w, http.StatusForbidden, "access denied")
		return
	}

	if !policy.Can(r.Context(), h.policy, c, req.AppID, "user", "create", "") {
		slog.Warn("create user rejected: caller lacks user.create permission", "user_id", c.UserID)
		render.Error(w, http.StatusForbidden, "access denied")
		return
	}

	if req.Role == int8(RoleSysAdmin) || req.Role == int8(RoleManager) {
		if !policy.Can(r.Context(), h.policy, c, req.AppID, "user", "create", "role") {
			slog.Warn("create user rejected: caller lacks user.create.role permission", "user_id", c.UserID)
			render.Error(w, http.StatusForbidden, "access denied")
			return
		}
	}

	u, err := h.svc.Create(r.Context(), req)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	render.JSON(w, http.StatusCreated, u)
}

// ListUsers godoc
// @Summary List all users
// @Description Get a list of all users belonging to the caller's app
// @Tags users
// @Produce json
// @Param limit query int false "Max results (default 50, max 500)"
// @Param offset query int false "Result offset (default 0)"
// @Param role query int false "Filter by role (0=user, 1=sysadmin, 3=manager)"
// @Success 200 {object} render.Response{data=[]User}
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 500 {object} render.Response
// @Security Bearer
// @Router /users [get]
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	c, ok := h.claims(r)
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	appID := c.AppID
	if c.IsSysAdmin() {
		appID = 0
	}

	role := int8(-1)
	if rs := r.URL.Query().Get("role"); rs != "" {
		v, err := strconv.ParseInt(rs, 10, 8)
		if err != nil {
			slog.Warn("invalid role filter in list users request", "role", rs)
			render.Error(w, http.StatusBadRequest, "invalid role")
			return
		}
		role = int8(v)
	}

	page := render.ParsePage(r)
	users, err := h.svc.List(r.Context(), appID, role, page.Limit, page.Offset)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	render.JSON(w, http.StatusOK, users)
}

// ListManagers godoc
// @Summary List all managers
// @Description Get a list of all users with the manager role, across all apps. Sysadmin only.
// @Tags users
// @Produce json
// @Param limit query int false "Max results (default 50, max 500)"
// @Param offset query int false "Result offset (default 0)"
// @Success 200 {object} render.Response{data=[]User}
// @Failure 401 {object} render.Response
// @Failure 403 {object} render.Response
// @Failure 500 {object} render.Response
// @Security Bearer
// @Router /managers [get]
func (h *UserHandler) ListManagers(w http.ResponseWriter, r *http.Request) {
	c, ok := h.claims(r)
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !c.IsSysAdmin() {
		slog.Warn("non-sysadmin attempted to list managers", "user_id", c.UserID)
		render.Error(w, http.StatusForbidden, "access denied")
		return
	}

	page := render.ParsePage(r)
	managers, err := h.svc.List(r.Context(), 0, RoleManager, page.Limit, page.Offset)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	render.JSON(w, http.StatusOK, managers)
}

// GetUserByID godoc
// @Summary Get user by ID
// @Description Get a single user by their unique ID (must belong to caller's app)
// @Tags users
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} render.Response{data=User}
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 403 {object} render.Response
// @Failure 404 {object} render.Response
// @Security Bearer
// @Router /users/{id} [get]
func (h *UserHandler) GetUserByID(w http.ResponseWriter, r *http.Request) {
	c, ok := h.claims(r)
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		slog.Warn("invalid user id in request", "id", idStr)
		render.Error(w, http.StatusBadRequest, "invalid user id")
		return
	}

	appID := c.AppID
	if c.IsSysAdmin() {
		appID = 0
	}

	u, err := h.svc.GetByID(r.Context(), appID, id)
	if err != nil {
		slog.Warn("user not found", "id", id)
		render.Error(w, http.StatusNotFound, "user not found")
		return
	}

	render.JSON(w, http.StatusOK, u)
}

// UpdateUser godoc
// @Summary Update user
// @Description Update an existing user's details (must belong to caller's app)
// @Tags users
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Param user body UpdateUserRequest true "Updated user details"
// @Success 200 {object} render.Response{data=User}
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 403 {object} render.Response
// @Failure 500 {object} render.Response
// @Security Bearer
// @Router /users/{id} [put]
func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	c, ok := h.claims(r)
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		slog.Warn("invalid user id in update request", "id", idStr)
		render.Error(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("failed to decode update user request", "id", id, "error", err)
		render.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		slog.Warn("invalid update user request", "id", id, "error", err)
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if !policy.Can(r.Context(), h.policy, c, c.AppID, "user", "update", "") {
		slog.Warn("update user rejected: caller lacks user.update permission", "id", id, "user_id", c.UserID)
		render.Error(w, http.StatusForbidden, "access denied")
		return
	}

	if req.Role != nil && (*req.Role == RoleSysAdmin || *req.Role == RoleManager) {
		if !policy.Can(r.Context(), h.policy, c, c.AppID, "user", "update", "role") {
			slog.Warn("update user rejected: caller lacks user.update.role permission", "id", id, "user_id", c.UserID)
			render.Error(w, http.StatusForbidden, "access denied")
			return
		}
	}

	appID := c.AppID
	if c.IsSysAdmin() {
		appID = 0
	}

	u, err := h.svc.Update(r.Context(), appID, id, req)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	render.JSON(w, http.StatusOK, u)
}

// DeleteUser godoc
// @Summary Delete user
// @Description Remove a user from the system by ID (must belong to caller's app)
// @Tags users
// @Produce json
// @Param id path int true "User ID"
// @Success 204 "No Content"
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 403 {object} render.Response
// @Failure 500 {object} render.Response
// @Security Bearer
// @Router /users/{id} [delete]
func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	c, ok := h.claims(r)
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		slog.Warn("invalid user id in delete request", "id", idStr)
		render.Error(w, http.StatusBadRequest, "invalid user id")
		return
	}

	appID := c.AppID
	if c.IsSysAdmin() {
		appID = 0
	}

	if err := h.svc.Delete(r.Context(), appID, id); err != nil {
		render.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}

	render.JSON(w, http.StatusNoContent, nil)
}

// AuthenticateUser godoc
// @Summary Authenticate user
// @Description Login with email and password to receive a JWT token
// @Tags users
// @Accept json
// @Produce json
// @Param credentials body AuthRequest true "Login credentials"
// @Success 200 {object} render.Response{data=AuthResponse}
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 503 {object} render.Response
// @Router /users/auth [post]
func (h *UserHandler) AuthenticateUser(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("failed to decode auth request", "error", err)
		render.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		slog.Warn("invalid auth request", "error", err)
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	res, err := h.svc.Authenticate(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrRoleServiceUnavailable) {
			render.Error(w, http.StatusServiceUnavailable, "authentication temporarily unavailable")
			return
		}
		render.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	render.JSON(w, http.StatusOK, res)
}
