package guestkey

import (
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

// GuestKeyHandler handles HTTP requests for guest keys.
type GuestKeyHandler struct {
	svc      GuestKeyService
	policy   *policy.Store
	validate *validator.Validate
}

// NewGuestKeyHandler creates a new guest key handler.
func NewGuestKeyHandler(svc GuestKeyService, policyStore *policy.Store) *GuestKeyHandler {
	return &GuestKeyHandler{
		svc:      svc,
		policy:   policyStore,
		validate: validator.New(),
	}
}

// Routes returns the chi router for guest key endpoints.
func (h *GuestKeyHandler) Routes(jwtManager *auth.JWTManager) chi.Router {
	r := chi.NewRouter()

	// Public token exchange — hard rate limit: site keys are publishable,
	// so this is the spam surface.
	r.Group(func(r chi.Router) {
		r.Use(httprate.LimitByIP(10, 1*time.Minute))
		r.Post("/auth", h.GuestAuth)
		r.Get("/lookup", h.LookupSiteKey)
	})

	// Protected management routes
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(jwtManager))

		r.Post("/", h.CreateGuestKey)
		r.Get("/", h.ListGuestKeys)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.GetGuestKeyByID)
			r.Put("/", h.UpdateGuestKey)
			r.Delete("/", h.DeleteGuestKey)
		})
	})

	return r
}

// GuestAuth godoc
// @Summary Exchange a site key for a guest token
// @Description Exchange a publishable site key for a short-lived tenant-scoped guest JWT (role=guest, signed with the guest secret)
// @Tags guest-keys
// @Accept json
// @Produce json
// @Param request body GuestAuthRequest true "Site key"
// @Success 200 {object} render.Response{data=GuestAuthResponse}
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 429 {object} render.Response
// @Router /guest-keys/auth [post]
func (h *GuestKeyHandler) GuestAuth(w http.ResponseWriter, r *http.Request) {
	var req GuestAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("failed to decode guest auth request", "error", err)
		render.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		slog.Warn("invalid guest auth request", "error", err)
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.svc.Authenticate(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidSiteKey) {
			slog.Warn("guest auth failed: invalid site key", "remote_addr", r.RemoteAddr)
			render.Error(w, http.StatusUnauthorized, ErrInvalidSiteKey.Error())
			return
		}
		render.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	render.JSON(w, http.StatusOK, resp)
}

// LookupSiteKey godoc
// @Summary Look up a publishable site key by URL
// @Description Resolve the publishable site key registered for the URL a UI is served from. Public (no auth) and hard rate-limited; returns only the site key, not tenant binding. The url is normalized (scheme/port stripped, host lowercased, host[+path]) and matched exactly.
// @Tags guest-keys
// @Produce json
// @Param url query string true "URL the UI is served from (e.g. https://shop.acme.com or acme.com/store)"
// @Success 200 {object} render.Response{data=SiteKeyLookupResponse}
// @Failure 400 {object} render.Response
// @Failure 404 {object} render.Response
// @Failure 429 {object} render.Response
// @Router /guest-keys/lookup [get]
func (h *GuestKeyHandler) LookupSiteKey(w http.ResponseWriter, r *http.Request) {
	rawURL := r.URL.Query().Get("url")
	if rawURL == "" {
		render.Error(w, http.StatusBadRequest, "url query param is required")
		return
	}

	resp, err := h.svc.LookupSiteKey(r.Context(), rawURL)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidDomain):
			slog.Warn("site key lookup with invalid url", "remote_addr", r.RemoteAddr)
			render.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrSiteKeyNotFound):
			slog.Warn("site key lookup found no match", "remote_addr", r.RemoteAddr)
			render.Error(w, http.StatusNotFound, err.Error())
		default:
			render.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	render.JSON(w, http.StatusOK, resp)
}

// CreateGuestKey godoc
// @Summary Create a new guest key
// @Description Create a publishable site key bound to an app/division and a designated guest user. Non-sysadmins can only create keys for their own app.
// @Tags guest-keys
// @Accept json
// @Produce json
// @Param request body CreateGuestKeyRequest true "Guest key details"
// @Success 201 {object} render.Response{data=GuestKey}
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 403 {object} render.Response
// @Failure 500 {object} render.Response
// @Security Bearer
// @Router /guest-keys [post]
func (h *GuestKeyHandler) CreateGuestKey(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaimsFromContext(r.Context())
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreateGuestKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("failed to decode create guest key request", "error", err)
		render.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		slog.Warn("invalid create guest key request", "error", err)
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	scope, ok := policy.Scope(r.Context(), h.policy, claims, "guestkey", "create")
	if !ok {
		slog.Warn("create guest key rejected: caller lacks guestkey.create permission", "user_id", claims.UserID)
		render.Error(w, http.StatusForbidden, "access denied")
		return
	}
	if scope == "own" && req.AppID != claims.AppID {
		slog.Warn("create guest key rejected: cross-tenant create not permitted", "user_id", claims.UserID, "app_id", claims.AppID, "target_app_id", req.AppID)
		render.Error(w, http.StatusForbidden, "access denied")
		return
	}

	k, err := h.svc.Create(r.Context(), req)
	if err != nil {
		if errors.Is(err, ErrGuestUserMismatch) {
			render.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		render.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	render.JSON(w, http.StatusCreated, k)
}

// ListGuestKeys godoc
// @Summary List guest keys
// @Description List guest keys. Non-sysadmins only see keys for their own app.
// @Tags guest-keys
// @Produce json
// @Param limit query int false "Max results (default 50, max 500)"
// @Param offset query int false "Result offset (default 0)"
// @Success 200 {object} render.Response{data=[]GuestKey}
// @Failure 401 {object} render.Response
// @Failure 500 {object} render.Response
// @Security Bearer
// @Router /guest-keys [get]
func (h *GuestKeyHandler) ListGuestKeys(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaimsFromContext(r.Context())
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	page := render.ParsePage(r)

	scope, ok := policy.Scope(r.Context(), h.policy, claims, "guestkey", "read")
	if !ok {
		slog.Warn("list guest keys rejected: caller lacks guestkey.read permission", "user_id", claims.UserID)
		render.Error(w, http.StatusForbidden, "access denied")
		return
	}
	appID := claims.AppID
	if scope == "any" {
		appID = 0
	}

	keys, err := h.svc.List(r.Context(), appID, page.Limit, page.Offset)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	render.JSON(w, http.StatusOK, keys)
}

// GetGuestKeyByID godoc
// @Summary Get guest key by ID
// @Description Get a single guest key by its unique ID. Non-sysadmins can only access keys of their own app.
// @Tags guest-keys
// @Produce json
// @Param id path int true "Guest key ID"
// @Success 200 {object} render.Response{data=GuestKey}
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 403 {object} render.Response
// @Failure 404 {object} render.Response
// @Security Bearer
// @Router /guest-keys/{id} [get]
func (h *GuestKeyHandler) GetGuestKeyByID(w http.ResponseWriter, r *http.Request) {
	_, k, ok := h.loadScoped(w, r, "read")
	if !ok {
		return
	}

	render.JSON(w, http.StatusOK, k)
}

// UpdateGuestKey godoc
// @Summary Update guest key
// @Description Update a guest key's name or status. Tenant binding and the site key are immutable. Non-sysadmins can only update keys of their own app.
// @Tags guest-keys
// @Accept json
// @Produce json
// @Param id path int true "Guest key ID"
// @Param request body UpdateGuestKeyRequest true "Fields to update"
// @Success 200 {object} render.Response{data=GuestKey}
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 403 {object} render.Response
// @Failure 404 {object} render.Response
// @Security Bearer
// @Router /guest-keys/{id} [put]
func (h *GuestKeyHandler) UpdateGuestKey(w http.ResponseWriter, r *http.Request) {
	_, k, ok := h.loadScoped(w, r, "update")
	if !ok {
		return
	}

	var req UpdateGuestKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("failed to decode update guest key request", "error", err)
		render.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		slog.Warn("invalid update guest key request", "error", err)
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.svc.Update(r.Context(), k.AppID, k.ID, req)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	render.JSON(w, http.StatusOK, updated)
}

// DeleteGuestKey godoc
// @Summary Delete guest key
// @Description Delete (revoke) a guest key. Non-sysadmins can only delete keys of their own app.
// @Tags guest-keys
// @Produce json
// @Param id path int true "Guest key ID"
// @Success 200 {object} render.Response
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 403 {object} render.Response
// @Failure 404 {object} render.Response
// @Security Bearer
// @Router /guest-keys/{id} [delete]
func (h *GuestKeyHandler) DeleteGuestKey(w http.ResponseWriter, r *http.Request) {
	_, k, ok := h.loadScoped(w, r, "delete")
	if !ok {
		return
	}

	if err := h.svc.Delete(r.Context(), k.AppID, k.ID); err != nil {
		render.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	render.JSON(w, http.StatusOK, map[string]string{"message": "guest key deleted"})
}

// loadScoped parses the id param and loads the key scoped to the caller's
// falcon-resolved permission for action (sysadmin/"any" grant sees every
// tenant, "own" is restricted to the caller's app — a cross-tenant id then
// 404s rather than leaking existence via a 403). Writes the error response
// itself when ok is false.
func (h *GuestKeyHandler) loadScoped(w http.ResponseWriter, r *http.Request, action string) (*auth.UserClaims, *GuestKey, bool) {
	claims, ok := auth.GetClaimsFromContext(r.Context())
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return nil, nil, false
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		slog.Warn("invalid guest key id in request", "id", idStr)
		render.Error(w, http.StatusBadRequest, "invalid guest key id")
		return nil, nil, false
	}

	scope, ok := policy.Scope(r.Context(), h.policy, claims, "guestkey", action)
	if !ok {
		slog.Warn("guest key access rejected: caller lacks guestkey permission", "action", action, "id", id, "user_id", claims.UserID)
		render.Error(w, http.StatusForbidden, "access denied")
		return nil, nil, false
	}
	appID := claims.AppID
	if scope == "any" {
		appID = 0
	}

	k, err := h.svc.GetByID(r.Context(), appID, id)
	if err != nil {
		render.Error(w, http.StatusNotFound, "guest key not found")
		return nil, nil, false
	}

	return claims, k, true
}
