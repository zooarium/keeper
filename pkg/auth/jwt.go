package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTManager handles generation and validation of JWT tokens.
type JWTManager struct {
	secretKey     string
	tokenDuration time.Duration
}

// NewJWTManager creates a new JWT manager.
func NewJWTManager(secretKey string, tokenDuration time.Duration) *JWTManager {
	return &JWTManager{secretKey, tokenDuration}
}

// UserClaims is a custom JWT claims that contains user's information.
//
// Impersonation tokens reuse this struct: AppID/UserID/DivisionID carry
// the *impersonated* user's exact identity (so downstream claims-based authz
// grants that user's full rights automatically), while the Imp* fields record
// who is really driving and how the token is scoped/revoked. Standard login
// and guest tokens leave the Imp* fields zero-valued, so existing consumers are
// unaffected.
//
// TODO(falcon): role/permission is now resolved entirely from falcon via
// Roles below — no local role identity is carried on the claims anymore.
type UserClaims struct {
	jwt.RegisteredClaims
	AppID      int `json:"app_id"`
	UserID     int `json:"user_id"`
	DivisionID int `json:"division_id"`
	// Roles carries fine-grained role assignments resolved from falcon at
	// login time (empty on impersonation/guest tokens, which don't resolve
	// them). ServiceID/AppID travel per-assignment because sudo's tenant
	// scope (fal_user_role.app_id) is a property of the assignment, not the
	// role — Can() needs it at check time, and it can't be recomputed from
	// just a role name.
	Roles []RoleAssignment `json:"roles,omitempty"`

	// Imp marks this token as an impersonation token. When true, UserID is the
	// impersonated user and Impersonator is the acting sysadmin.
	Imp bool `json:"imp,omitempty"`
	// Impersonator is the user_id of the sysadmin who minted the impersonation
	// token. Present only when Imp is true; used for audit attribution.
	Impersonator int `json:"impersonator,omitempty"`
	// ImpRO marks an impersonation token as read-only; downstream services that
	// honor it must reject mutating requests. Default (false) = full parity.
	ImpRO bool `json:"imp_ro,omitempty"`
	// SessionID ties all per-audience tokens minted from one "login as" action
	// together so a single revoke kills every one of them server-side.
	SessionID string `json:"sid,omitempty"`
}

// RoleAssignment is one falcon-resolved role grant carried in a login JWT.
// ServiceID/AppID mirror fal_user_role's denormalized scope columns: AppID
// nil means the assignment (if sudo) spans every tenant, set means it's
// scoped to that one app.
type RoleAssignment struct {
	Name      string `json:"name"`
	ServiceID int    `json:"service_id"`
	AppID     *int   `json:"app_id,omitempty"`
}

// IsImpersonating returns true when the claims belong to an impersonation token.
func (c *UserClaims) IsImpersonating() bool {
	return c.Imp
}

// HasAudience reports whether the token's audience list contains aud. Used by
// downstream services to reject impersonation tokens minted for a different
// service.
func (c *UserClaims) HasAudience(aud string) bool {
	for _, a := range c.Audience {
		if a == aud {
			return true
		}
	}
	return false
}

// Generate generates and signs a new token for a user. roles is optional —
// pass falcon-resolved role assignments for a full login token, or omit it
// for self-signed s2s/guest tokens that don't carry them.
func (manager *JWTManager) Generate(appID, userID, divisionID int, roles ...RoleAssignment) (string, error) {
	claims := UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(manager.tokenDuration)),
		},
		AppID:      appID,
		UserID:     userID,
		DivisionID: divisionID,
		Roles:      roles,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(manager.secretKey))
}

// ImpersonationParams describes the impersonation token to mint. AppID/UserID/
// DivisionID are the *impersonated* user's identity; Impersonator is the
// acting sysadmin; Audience scopes the token to a single service; SessionID and
// JTI enable revocation; ReadOnly downgrades to view-only.
type ImpersonationParams struct {
	AppID        int
	UserID       int
	DivisionID   int
	Impersonator int
	Audience     string
	SessionID    string
	JTI          string
	ReadOnly     bool
}

// GenerateImpersonation mints a signed impersonation token. The manager must be
// constructed with the dedicated impersonation secret (AUTH.IMPERSONATION_JWT_
// SECRET) so these tokens are cryptographically useless on surfaces verifying
// with the primary or guest secrets.
func (manager *JWTManager) GenerateImpersonation(p ImpersonationParams) (string, error) {
	claims := UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(manager.tokenDuration)),
			ID:        p.JTI,
			Audience:  jwt.ClaimStrings{p.Audience},
		},
		AppID:        p.AppID,
		UserID:       p.UserID,
		DivisionID:   p.DivisionID,
		Imp:          true,
		Impersonator: p.Impersonator,
		ImpRO:        p.ReadOnly,
		SessionID:    p.SessionID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(manager.secretKey))
}

// Verify verifies the access token string and return a user claims if the token is valid.
func (manager *JWTManager) Verify(accessToken string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(
		accessToken,
		&UserClaims{},
		func(token *jwt.Token) (interface{}, error) {
			_, ok := token.Method.(*jwt.SigningMethodHMAC)
			if !ok {
				return nil, fmt.Errorf("unexpected token signing method")
			}

			return []byte(manager.secretKey), nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*UserClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// VerifyWithAudience verifies the token (signature + expiry) and additionally
// requires the given audience to be present. Downstream services use this for
// impersonation tokens so a token minted for service A cannot be replayed
// against service B even though both share the impersonation secret.
func (manager *JWTManager) VerifyWithAudience(accessToken, audience string) (*UserClaims, error) {
	claims, err := manager.Verify(accessToken)
	if err != nil {
		return nil, err
	}
	if !claims.HasAudience(audience) {
		return nil, fmt.Errorf("invalid token: audience mismatch")
	}
	return claims, nil
}
