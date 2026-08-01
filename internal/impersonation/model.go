package impersonation

import (
	"time"
)

// ImpersonationSession is the domain model for an impersonation audit record.
type ImpersonationSession struct {
	ID                 int        `json:"id"`
	SessionID          string     `json:"session_id"`
	AppID              int        `json:"app_id"`
	DivisionID         int        `json:"division_id"`
	ImpersonatorUserID int        `json:"impersonator_user_id"`
	TargetUserID       int        `json:"target_user_id"`
	Audience           string     `json:"audience"`
	ReadOnly           bool       `json:"read_only"`
	Reason             string     `json:"reason,omitempty"`
	Status             int8       `json:"status"`
	CreatedAt          time.Time  `json:"created_at"`
	ExpiresAt          time.Time  `json:"expires_at"`
	RevokedAt          *time.Time `json:"revoked_at,omitempty"`
}

// ServiceInfo is a registered impersonation target service, exposed to the
// admin UI so it can present a service picker and know where to hand off the
// one-time code. UIOrigin is intentionally omitted (CORS/internal concern).
type ServiceInfo struct {
	Key           string `json:"key"`
	Audience      string `json:"audience"`
	UIExchangeURL string `json:"ui_exchange_url"`
}

// TargetUser is the minimal snapshot of the impersonated user needed to mint a
// token and populate the UI's stored user object.
type TargetUser struct {
	ID         int    `json:"id"`
	AppID      int    `json:"app_id"`
	DivisionID int    `json:"division_id"`
	Firstname  string `json:"firstname"`
	Lastname   string `json:"lastname"`
	Email      string `json:"email"`
	Status     int8   `json:"status"`
}

// StartImpersonationRequest is the sysadmin-issued request to begin a session.
// Audience is the single downstream service to enter (must be registered).
type StartImpersonationRequest struct {
	TargetUserID int    `json:"target_user_id" validate:"required"`
	Audience     string `json:"audience" validate:"required"`
	Reason       string `json:"reason" validate:"omitempty"`
	ReadOnly     bool   `json:"read_only" validate:"omitempty"`
}

// StartImpersonationResponse carries the one-time handoff code. The code — not
// the JWT — is what travels to the target service UI (in the URL fragment); it
// is single-use and short-lived, and is exchanged for the actual token.
type StartImpersonationResponse struct {
	Code      string    `json:"code"`
	SessionID string    `json:"session_id"`
	Audience  string    `json:"audience"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ExchangeRequest exchanges a one-time code for an impersonation token. Called
// cross-origin by the target service UI's exchange page.
type ExchangeRequest struct {
	Code string `json:"code" validate:"required"`
}

// ExchangeResponse carries the minted impersonation token plus the impersonated
// user object the UI stores for role gating.
type ExchangeResponse struct {
	Token     string     `json:"token"`
	User      TargetUser `json:"user"`
	Audience  string     `json:"audience"`
	SessionID string     `json:"session_id"`
	ExpiresAt time.Time  `json:"expires_at"`
}

// LogoutRequest self-revokes a session by its opaque id — used by the
// impersonation tab on "exit". Knowing the unguessable session_id is the
// capability; revocation only ever reduces access, so this is safe to expose.
type LogoutRequest struct {
	SessionID string `json:"session_id" validate:"required"`
}

// SessionStatusResponse reports whether a session is still active. Boolean only
// — no tenant or identity data — so downstream services can cheaply check
// revocation without exposing anything if the id is probed.
type SessionStatusResponse struct {
	Active bool `json:"active"`
}
