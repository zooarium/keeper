package app

import (
	"time"
)

// Address represents a postal address for an app's contact details.
type Address struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2"`
	City       string `json:"city"`
	State      string `json:"state"`
	Country    string `json:"country"`
	PostalCode string `json:"postal_code"`
}

// About represents an app's "about" section.
type About struct {
	Heading string `json:"heading"`
	Body    string `json:"body"` // may contain HTML
}

// Contact represents an app's contact details.
type Contact struct {
	Address Address           `json:"address"`
	Phone1  string            `json:"phone1"`
	Phone2  string            `json:"phone2"`
	Email   string            `json:"email"`
	Hours   string            `json:"hours"` // free text
	Social  map[string]string `json:"social"`
}

// App represents the domain model for an app.
type App struct {
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	Tagline    string    `json:"tagline"`
	LogoURL    string    `json:"logo_url"`
	About      About     `json:"about"`
	Contact    Contact   `json:"contact"`
	TaxNumber  string    `json:"tax_number"`
	TaxPercent float64   `json:"tax_percent"`
	Status     int8      `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// PublicApp is the public-safe projection of an app returned by the
// unauthenticated site-key lookup endpoint. Status and timestamps are omitted.
type PublicApp struct {
	ID         int     `json:"id"`
	Name       string  `json:"name"`
	Tagline    string  `json:"tagline"`
	LogoURL    string  `json:"logo_url"`
	About      About   `json:"about"`
	Contact    Contact `json:"contact"`
	TaxNumber  string  `json:"tax_number"`
	TaxPercent float64 `json:"tax_percent"`
}

// AddressInput is the request payload for an app's address.
type AddressInput struct {
	Line1      string `json:"line1" validate:"omitempty"`
	Line2      string `json:"line2" validate:"omitempty"`
	City       string `json:"city" validate:"omitempty"`
	State      string `json:"state" validate:"omitempty"`
	Country    string `json:"country" validate:"omitempty"`
	PostalCode string `json:"postal_code" validate:"omitempty"`
}

// AboutInput is the request payload for an app's about section.
type AboutInput struct {
	Heading string `json:"heading" validate:"omitempty"`
	Body    string `json:"body" validate:"omitempty"`
}

// ContactInput is the request payload for an app's contact details.
// Social values are validated as URLs in the service layer.
type ContactInput struct {
	Address AddressInput      `json:"address"`
	Phone1  string            `json:"phone1" validate:"omitempty"`
	Phone2  string            `json:"phone2" validate:"omitempty"`
	Email   string            `json:"email" validate:"omitempty,email"`
	Hours   string            `json:"hours" validate:"omitempty"`
	Social  map[string]string `json:"social" validate:"omitempty"`
}

// CreateAppRequest defines the payload for creating an app.
type CreateAppRequest struct {
	Name       string       `json:"name" validate:"required"`
	Tagline    string       `json:"tagline" validate:"omitempty"`
	LogoURL    string       `json:"logo_url" validate:"omitempty,httpurl"`
	About      AboutInput   `json:"about"`
	Contact    ContactInput `json:"contact"`
	TaxNumber  string       `json:"tax_number" validate:"omitempty"`
	TaxPercent float64      `json:"tax_percent" validate:"omitempty,gte=0,lte=100"`
	Status     int8         `json:"status" validate:"omitempty"`
}

// UpdateAppRequest defines the payload for updating an app. Nested sections
// (about, contact) replace the whole section when present.
type UpdateAppRequest struct {
	Name       *string       `json:"name" validate:"omitempty"`
	Tagline    *string       `json:"tagline" validate:"omitempty"`
	LogoURL    *string       `json:"logo_url" validate:"omitempty,httpurl"`
	About      *AboutInput   `json:"about" validate:"omitempty"`
	Contact    *ContactInput `json:"contact" validate:"omitempty"`
	TaxNumber  *string       `json:"tax_number" validate:"omitempty"`
	TaxPercent *float64      `json:"tax_percent" validate:"omitempty,gte=0,lte=100"`
	Status     *int8         `json:"status" validate:"omitempty"`
}
