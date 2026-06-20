package auth

import (
	"testing"
	"time"
)

func TestGenerateImpersonationCarriesClaims(t *testing.T) {
	mgr := NewJWTManager("imp-secret", 10*time.Minute)

	token, err := mgr.GenerateImpersonation(ImpersonationParams{
		AppID:        3,
		UserID:       42,
		DivisionID:   7,
		Role:         RoleUser,
		Impersonator: 1,
		Audience:     "squirrel",
		SessionID:    "sess-abc",
		JTI:          "jti-xyz",
		ReadOnly:     false,
	})
	if err != nil {
		t.Fatalf("generate impersonation: %v", err)
	}

	claims, err := mgr.Verify(token)
	if err != nil {
		t.Fatalf("verify with imp secret: %v", err)
	}

	if !claims.IsImpersonating() {
		t.Error("expected IsImpersonating true")
	}
	if claims.UserID != 42 || claims.AppID != 3 || claims.DivisionID != 7 {
		t.Errorf("impersonated identity not preserved: %+v", claims)
	}
	if claims.Role != RoleUser {
		t.Errorf("expected impersonated user role preserved, got %d", claims.Role)
	}
	if claims.Impersonator != 1 {
		t.Errorf("expected impersonator 1, got %d", claims.Impersonator)
	}
	if claims.SessionID != "sess-abc" {
		t.Errorf("expected sid sess-abc, got %q", claims.SessionID)
	}
	if claims.ID != "jti-xyz" {
		t.Errorf("expected jti jti-xyz, got %q", claims.ID)
	}
	if !claims.HasAudience("squirrel") {
		t.Error("expected audience squirrel")
	}
}

func TestImpersonationTokenUselessWithOtherSecret(t *testing.T) {
	imp := NewJWTManager("imp-secret", 10*time.Minute)
	token, err := imp.GenerateImpersonation(ImpersonationParams{
		AppID: 1, UserID: 2, DivisionID: 3, Role: RoleUser,
		Impersonator: 9, Audience: "ant", SessionID: "s", JTI: "j",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// Primary/guest secret holders must not accept an impersonation token.
	if _, err := NewJWTManager("primary-secret", 0).Verify(token); err == nil {
		t.Error("expected verification failure with primary secret")
	}
}

func TestVerifyWithAudienceRejectsMismatch(t *testing.T) {
	mgr := NewJWTManager("imp-secret", 10*time.Minute)
	token, err := mgr.GenerateImpersonation(ImpersonationParams{
		AppID: 1, UserID: 2, DivisionID: 3, Role: RoleUser,
		Impersonator: 9, Audience: "squirrel", SessionID: "s", JTI: "j",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if _, err := mgr.VerifyWithAudience(token, "squirrel"); err != nil {
		t.Errorf("expected audience match to verify, got %v", err)
	}
	// A token minted for squirrel must be rejected when ant checks audience.
	if _, err := mgr.VerifyWithAudience(token, "ant"); err == nil {
		t.Error("expected audience mismatch to fail")
	}
}

func TestStandardTokenIsNotImpersonating(t *testing.T) {
	mgr := NewJWTManager("primary", time.Hour)
	token, err := mgr.Generate(1, 2, 3, RoleSysAdmin)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	claims, err := mgr.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.IsImpersonating() {
		t.Error("standard token must not be flagged impersonating")
	}
	if !claims.IsSysAdmin() {
		t.Error("expected sysadmin")
	}
}
