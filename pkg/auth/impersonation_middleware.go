package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"keeper/pkg/cache"
	"keeper/pkg/render"
	"keeper/pkg/s2s"
)

// RevocationChecker reports whether an impersonation session id is still active.
// Returning false rejects the request. Implementations should be fast (cached).
// A nil checker disables the live revocation check, relying on the token's short
// expiry alone.
type RevocationChecker func(sessionID string) bool

// isMutating reports whether the HTTP method writes state. Used to enforce
// read-only impersonation tokens.
func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// ImpersonationAwareMiddleware authenticates requests with either the primary
// JWT secret (normal user/sysadmin tokens) or the impersonation secret. An
// impersonation token is accepted only when:
//   - its signature verifies with the impersonation secret, AND
//   - its audience matches this service (so a token minted for service A cannot
//     be replayed against service B even though both share the secret), AND
//   - the session is not revoked (when a checker is supplied), AND
//   - the request is non-mutating when the token is read-only.
//
// On success the impersonated user's claims are placed in context exactly like a
// normal token, so downstream claims-based authorization grants that user's
// rights automatically. Mutating requests under impersonation are logged with
// the impersonator for audit.
func ImpersonationAwareMiddleware(primary, imp *JWTManager, audience string, revoked RevocationChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				slog.Warn("missing authorization header", "path", r.URL.Path, "remote_addr", r.RemoteAddr)
				render.Error(w, http.StatusUnauthorized, "missing authorization header")
				return
			}
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				slog.Warn("invalid authorization header format", "path", r.URL.Path, "remote_addr", r.RemoteAddr)
				render.Error(w, http.StatusUnauthorized, "invalid authorization header format")
				return
			}
			token := parts[1]

			// Primary tokens are the common case: verify first.
			if claims, err := primary.Verify(token); err == nil && !claims.IsImpersonating() {
				ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Fall back to an impersonation token signed with the dedicated secret.
			if imp == nil {
				slog.Warn("invalid or expired token", "path", r.URL.Path, "remote_addr", r.RemoteAddr)
				render.Error(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}
			claims, err := imp.Verify(token)
			if err != nil || !claims.IsImpersonating() {
				slog.Warn("invalid or expired token", "path", r.URL.Path, "remote_addr", r.RemoteAddr)
				render.Error(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			if !claims.HasAudience(audience) {
				slog.Warn("impersonation token audience mismatch", "path", r.URL.Path, "audience", audience, "impersonator", claims.Impersonator)
				render.Error(w, http.StatusUnauthorized, "invalid token audience")
				return
			}

			if revoked != nil && !revoked(claims.SessionID) {
				slog.Warn("impersonation token revoked", "path", r.URL.Path, "session_id", claims.SessionID, "impersonator", claims.Impersonator)
				render.Error(w, http.StatusUnauthorized, "impersonation session revoked")
				return
			}

			if claims.ImpRO && isMutating(r.Method) {
				slog.Warn("read-only impersonation token rejected for mutating request", "path", r.URL.Path, "method", r.Method, "impersonator", claims.Impersonator)
				render.Error(w, http.StatusForbidden, "read-only impersonation: mutation not allowed")
				return
			}

			// Audit: attribute mutating actions taken under impersonation.
			if isMutating(r.Method) {
				slog.Warn("impersonated mutation",
					"path", r.URL.Path,
					"method", r.Method,
					"impersonator", claims.Impersonator,
					"impersonated_user_id", claims.UserID,
					"app_id", claims.AppID,
					"session_id", claims.SessionID,
				)
			}

			ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// NewHTTPRevocationChecker returns a RevocationChecker that asks keeper whether a
// session is active via GET {keeperBaseURL}/impersonations/active/{sid}, caching
// results (positive and negative) for ttl to keep the auth hot path cheap.
//
// On a network/transport error it fails OPEN (treats the session as active) and
// logs a warning: the token's signature is already valid and its lifetime
// short, so a keeper outage degrades to expiry-only enforcement rather than
// taking the downstream service down. client must carry a non-zero timeout.
func NewHTTPRevocationChecker(client *http.Client, keeperBaseURL string, ttl time.Duration) RevocationChecker {
	rest := s2s.New(client, keeperBaseURL)
	cached := cache.New(ttl)

	return func(sessionID string) bool {
		if v, ok := cached.Get(sessionID); ok {
			return v.(bool)
		}

		var body struct {
			Active bool `json:"active"`
		}
		if err := rest.Get(context.Background(), "/impersonations/active/"+sessionID, &body); err != nil {
			slog.Warn("revocation check: failed, failing open", "error", err)
			return true // fail-open: signature already valid, expiry already short
		}

		cached.Set(sessionID, body.Active)
		return body.Active
	}
}
