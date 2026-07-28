package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func doReq(h http.Handler, method, token string) int {
	r := httptest.NewRequest(method, "/x", nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code
}

func TestImpMiddlewareAcceptsPrimaryToken(t *testing.T) {
	primary := NewJWTManager("primary", time.Hour)
	imp := NewJWTManager("imp", time.Hour)
	mw := ImpersonationAwareMiddleware(primary, imp, "squirrel", nil)(okHandler())

	tok, _ := primary.Generate(1, 2, 3, RoleAdmin)
	if code := doReq(mw, http.MethodGet, tok); code != http.StatusOK {
		t.Errorf("expected 200 for primary token, got %d", code)
	}
}

func TestImpMiddlewareAcceptsImpTokenForOwnAudience(t *testing.T) {
	primary := NewJWTManager("primary", time.Hour)
	imp := NewJWTManager("imp", time.Hour)
	mw := ImpersonationAwareMiddleware(primary, imp, "squirrel", nil)(okHandler())

	tok, _ := imp.GenerateImpersonation(ImpersonationParams{
		AppID: 1, UserID: 2, DivisionID: 3, Role: RoleAdmin,
		Impersonator: 9, Audience: "squirrel", SessionID: "s", JTI: "j",
	})
	if code := doReq(mw, http.MethodPost, tok); code != http.StatusOK {
		t.Errorf("expected 200 for imp token (own audience), got %d", code)
	}
}

func TestImpMiddlewareRejectsForeignAudience(t *testing.T) {
	primary := NewJWTManager("primary", time.Hour)
	imp := NewJWTManager("imp", time.Hour)
	mw := ImpersonationAwareMiddleware(primary, imp, "ant", nil)(okHandler())

	tok, _ := imp.GenerateImpersonation(ImpersonationParams{
		AppID: 1, UserID: 2, DivisionID: 3, Role: RoleAdmin,
		Impersonator: 9, Audience: "squirrel", SessionID: "s", JTI: "j",
	})
	if code := doReq(mw, http.MethodGet, tok); code != http.StatusUnauthorized {
		t.Errorf("expected 401 for foreign audience, got %d", code)
	}
}

func TestImpMiddlewareReadOnlyBlocksMutation(t *testing.T) {
	primary := NewJWTManager("primary", time.Hour)
	imp := NewJWTManager("imp", time.Hour)
	mw := ImpersonationAwareMiddleware(primary, imp, "squirrel", nil)(okHandler())

	tok, _ := imp.GenerateImpersonation(ImpersonationParams{
		AppID: 1, UserID: 2, DivisionID: 3, Role: RoleAdmin,
		Impersonator: 9, Audience: "squirrel", SessionID: "s", JTI: "j", ReadOnly: true,
	})
	if code := doReq(mw, http.MethodGet, tok); code != http.StatusOK {
		t.Errorf("expected 200 for read-only GET, got %d", code)
	}
	if code := doReq(mw, http.MethodPost, tok); code != http.StatusForbidden {
		t.Errorf("expected 403 for read-only POST, got %d", code)
	}
}

func TestImpMiddlewareRevocationRejects(t *testing.T) {
	primary := NewJWTManager("primary", time.Hour)
	imp := NewJWTManager("imp", time.Hour)
	revoked := func(string) bool { return false } // always inactive
	mw := ImpersonationAwareMiddleware(primary, imp, "squirrel", revoked)(okHandler())

	tok, _ := imp.GenerateImpersonation(ImpersonationParams{
		AppID: 1, UserID: 2, DivisionID: 3, Role: RoleAdmin,
		Impersonator: 9, Audience: "squirrel", SessionID: "s", JTI: "j",
	})
	if code := doReq(mw, http.MethodGet, tok); code != http.StatusUnauthorized {
		t.Errorf("expected 401 for revoked session, got %d", code)
	}
}
