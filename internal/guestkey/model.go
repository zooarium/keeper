package guestkey

import (
	"time"
)

// GuestKey represents the domain model for a guest (publishable site) key.
// Public UIs exchange the site key for a short-lived tenant-scoped guest JWT.
type GuestKey struct {
	ID         int       `json:"id"`
	AppID      int       `json:"app_id"`
	DivisionID int       `json:"division_id"`
	UserID     int       `json:"user_id"`
	Name       string    `json:"name"`
	SiteKey    string    `json:"site_key"`
	Domain     string    `json:"domain"`
	Status     int8      `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// CreateGuestKeyRequest defines the payload for creating a guest key. The
// referenced user is the designated guest identity for the tenant; it must
// exist and belong to the given app and division. The site key itself is
// generated server-side.
type CreateGuestKeyRequest struct {
	AppID      int    `json:"app_id" validate:"required"`
	DivisionID int    `json:"division_id" validate:"required"`
	UserID     int    `json:"user_id" validate:"required"`
	Name       string `json:"name" validate:"required"`
	// Domain is the URL the publishable UI is served from. It is normalized
	// server-side (scheme/port stripped, host lowercased, host[+path]) and
	// must be unique — a public URL lookup resolves to exactly one site key.
	Domain string `json:"domain" validate:"required"`
	Status int8   `json:"status" validate:"omitempty"`
}

// UpdateGuestKeyRequest defines the payload for updating a guest key. Tenant
// binding (app/division/user) and the site key are immutable — rotate by
// deleting and creating a new key.
type UpdateGuestKeyRequest struct {
	Name   *string `json:"name" validate:"omitempty"`
	Status *int8   `json:"status" validate:"omitempty"`
}

// GuestAuthRequest defines the payload for exchanging a site key for a token.
type GuestAuthRequest struct {
	SiteKey string `json:"site_key" validate:"required"`
}

// GuestAuthResponse carries the minted guest token.
type GuestAuthResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// SiteKeyLookupResponse carries the publishable site key resolved for a URL.
// Only the site key is exposed — tenant binding (app/division/user) stays
// private since this endpoint is public.
type SiteKeyLookupResponse struct {
	SiteKey string `json:"site_key"`
}
