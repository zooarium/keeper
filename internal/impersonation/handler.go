package impersonation

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

// ImpersonationHandler handles HTTP requests for impersonation.
type ImpersonationHandler struct {
	svc      ImpersonationService
	policy   *policy.Store
	validate *validator.Validate
}

// NewImpersonationHandler creates a new impersonation handler.
func NewImpersonationHandler(svc ImpersonationService, policyStore *policy.Store) *ImpersonationHandler {
	return &ImpersonationHandler{
		svc:      svc,
		policy:   policyStore,
		validate: validator.New(),
	}
}

// Routes returns the chi router for impersonation endpoints.
func (h *ImpersonationHandler) Routes(jwtManager *auth.JWTManager) chi.Router {
	r := chi.NewRouter()

	// Public surfaces. The one-time code (exchange) and the opaque session id
	// (logout / status) are the capabilities — no JWT, hard rate limit. These
	// are called cross-origin by service UIs, so they must not require the
	// primary auth that the management routes carry.
	r.Group(func(r chi.Router) {
		r.Use(httprate.LimitByIP(10, 1*time.Minute))
		r.Post("/exchange", h.Exchange)
		r.Post("/logout", h.Logout)
		r.Get("/active/{session_id}", h.SessionActive)
	})

	// Management surfaces — impersonation.create/impersonation.read gated per
	// handler (falcon-resolved, see requirePermission).
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(jwtManager))
		r.Post("/", h.Start)
		r.Get("/", h.ListSessions)
		r.Get("/services", h.ListServices)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.GetSession)
			r.Post("/revoke", h.RevokeSession)
		})
	})

	return r
}

// Start godoc
// @Summary Start an impersonation session
// @Description Requires impersonation.create (falcon-resolved). Mints a one-time handoff code for logging in as another user on a registered downstream service. The code is exchanged (cross-origin) for the actual token. Refuses to target a sysadmin.
// @Tags impersonation
// @Accept json
// @Produce json
// @Param request body StartImpersonationRequest true "Target user and audience"
// @Success 201 {object} render.Response{data=StartImpersonationResponse}
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 403 {object} render.Response
// @Failure 500 {object} render.Response
// @Security Bearer
// @Router /impersonations [post]
func (h *ImpersonationHandler) Start(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requirePermission(w, r, "create")
	if !ok {
		return
	}

	var req StartImpersonationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Warn("failed to decode start impersonation request", "error", err)
		render.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		slog.Warn("invalid start impersonation request", "error", err)
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.svc.Start(r.Context(), claims.UserID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrServiceNotRegistered):
			render.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrTargetNotFound):
			render.Error(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrPrivilegeEscalation):
			slog.Warn("impersonation start rejected: privilege escalation", "impersonator", claims.UserID, "target_user_id", req.TargetUserID)
			render.Error(w, http.StatusForbidden, err.Error())
		default:
			render.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	render.JSON(w, http.StatusCreated, resp)
}

// Exchange godoc
// @Summary Exchange a handoff code for an impersonation token
// @Description Public (no auth) and rate-limited. Redeems a one-time code for a short-lived, audience-scoped impersonation token plus the impersonated user object. Called cross-origin by the target service UI's exchange page.
// @Tags impersonation
// @Accept json
// @Produce json
// @Param request body ExchangeRequest true "Handoff code"
// @Success 200 {object} render.Response{data=ExchangeResponse}
// @Failure 400 {object} render.Response
// @Failure 401 {object} render.Response
// @Failure 429 {object} render.Response
// @Router /impersonations/exchange [post]
func (h *ImpersonationHandler) Exchange(w http.ResponseWriter, r *http.Request) {
	var req ExchangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.svc.Exchange(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCode):
			slog.Warn("impersonation exchange rejected", "remote_addr", r.RemoteAddr)
			render.Error(w, http.StatusUnauthorized, err.Error())
		case errors.Is(err, ErrSessionRevoked):
			render.Error(w, http.StatusUnauthorized, err.Error())
		default:
			render.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	render.JSON(w, http.StatusOK, resp)
}

// Logout godoc
// @Summary Revoke an impersonation session (self-service)
// @Description Public (no auth) and rate-limited. Revokes a session by its opaque id — used by the impersonation tab on exit. Knowing the unguessable session id is the capability; revocation only reduces access.
// @Tags impersonation
// @Accept json
// @Produce json
// @Param request body LogoutRequest true "Session id"
// @Success 200 {object} render.Response
// @Failure 400 {object} render.Response
// @Failure 429 {object} render.Response
// @Router /impersonations/logout [post]
func (h *ImpersonationHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Revoke(r.Context(), req.SessionID); err != nil {
		render.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	render.JSON(w, http.StatusOK, map[string]string{"message": "impersonation session revoked"})
}

// SessionActive godoc
// @Summary Check whether an impersonation session is active
// @Description Public (no auth) and rate-limited. Returns a boolean only — no identity or tenant data — so downstream services can cheaply enforce revocation.
// @Tags impersonation
// @Produce json
// @Param session_id path string true "Opaque session id"
// @Success 200 {object} render.Response{data=SessionStatusResponse}
// @Failure 429 {object} render.Response
// @Router /impersonations/active/{session_id} [get]
func (h *ImpersonationHandler) SessionActive(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "session_id")
	if sessionID == "" {
		render.Error(w, http.StatusBadRequest, "session_id is required")
		return
	}

	active, err := h.svc.IsSessionActive(r.Context(), sessionID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	render.JSON(w, http.StatusOK, SessionStatusResponse{Active: active})
}

// ListSessions godoc
// @Summary List active impersonation sessions
// @Description Requires impersonation.read (falcon-resolved). Lists active impersonation sessions for audit.
// @Tags impersonation
// @Produce json
// @Param limit query int false "Max results (default 50, max 500)"
// @Param offset query int false "Result offset (default 0)"
// @Success 200 {object} render.Response{data=[]ImpersonationSession}
// @Failure 401 {object} render.Response
// @Failure 403 {object} render.Response
// @Failure 500 {object} render.Response
// @Security Bearer
// @Router /impersonations [get]
func (h *ImpersonationHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requirePermission(w, r, "read"); !ok {
		return
	}

	page := render.ParsePage(r)
	sessions, err := h.svc.List(r.Context(), 0, page.Limit, page.Offset)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	render.JSON(w, http.StatusOK, sessions)
}

// ListServices godoc
// @Summary List registered impersonation target services
// @Description Requires impersonation.read (falcon-resolved). Returns the registered services a caller can impersonate a user into, for the UI service picker.
// @Tags impersonation
// @Produce json
// @Success 200 {object} render.Response{data=[]ServiceInfo}
// @Failure 401 {object} render.Response
// @Failure 403 {object} render.Response
// @Security Bearer
// @Router /impersonations/services [get]
func (h *ImpersonationHandler) ListServices(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requirePermission(w, r, "read"); !ok {
		return
	}
	render.JSON(w, http.StatusOK, h.svc.Services())
}

// GetSession godoc
// @Summary Get an impersonation session by ID
// @Description Requires impersonation.read (falcon-resolved).
// @Tags impersonation
// @Produce json
// @Param id path int true "Session ID"
// @Success 200 {object} render.Response{data=ImpersonationSession}
// @Failure 401 {object} render.Response
// @Failure 403 {object} render.Response
// @Failure 404 {object} render.Response
// @Security Bearer
// @Router /impersonations/{id} [get]
func (h *ImpersonationHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requirePermission(w, r, "read"); !ok {
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		render.Error(w, http.StatusBadRequest, "invalid session id")
		return
	}

	sess, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		render.Error(w, http.StatusNotFound, "impersonation session not found")
		return
	}

	render.JSON(w, http.StatusOK, sess)
}

// RevokeSession godoc
// @Summary Revoke an impersonation session by ID
// @Description Requires impersonation.create (falcon-resolved). Revokes a session server-side; downstream services reject its token on next request.
// @Tags impersonation
// @Produce json
// @Param id path int true "Session ID"
// @Success 200 {object} render.Response{data=ImpersonationSession}
// @Failure 401 {object} render.Response
// @Failure 403 {object} render.Response
// @Failure 404 {object} render.Response
// @Security Bearer
// @Router /impersonations/{id}/revoke [post]
func (h *ImpersonationHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requirePermission(w, r, "create"); !ok {
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		render.Error(w, http.StatusBadRequest, "invalid session id")
		return
	}

	sess, err := h.svc.RevokeByID(r.Context(), id)
	if err != nil {
		render.Error(w, http.StatusNotFound, "impersonation session not found")
		return
	}

	render.JSON(w, http.StatusOK, sess)
}

// requirePermission enforces authentication plus a coarse impersonation.<action>
// grant. Impersonation has no per-record ownership (Tier 3 scoping doesn't
// apply here) — this is a Tier 1 gate only, checked against the caller's own
// tenant. action is "create" for the mutating endpoints (start, revoke) and
// "read" for the audit/listing endpoints.
func (h *ImpersonationHandler) requirePermission(w http.ResponseWriter, r *http.Request, action string) (*auth.UserClaims, bool) {
	claims, ok := auth.GetClaimsFromContext(r.Context())
	if !ok {
		render.Error(w, http.StatusUnauthorized, "unauthorized")
		return nil, false
	}
	if !policy.Can(r.Context(), h.policy, claims, claims.AppID, "impersonation", action, "") {
		render.Error(w, http.StatusForbidden, "access denied")
		return nil, false
	}
	return claims, true
}
