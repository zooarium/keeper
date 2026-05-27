package division

import "time"

// Division represents the domain model for a division.
type Division struct {
	ID        int       `json:"id"`
	AppID     int       `json:"app_id"`
	ParentID  *int      `json:"parent_id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Depth     int8      `json:"depth"`
	Status    int8      `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateDivisionRequest defines the payload for creating a division.
type CreateDivisionRequest struct {
	AppID    int    `json:"app_id"    validate:"required"`
	ParentID *int   `json:"parent_id"`
	Name     string `json:"name"      validate:"required,max=100"`
}

// UpdateDivisionRequest defines the payload for updating a division.
type UpdateDivisionRequest struct {
	Name   *string `json:"name"   validate:"omitempty,max=100"`
	Status *int8   `json:"status" validate:"omitempty,oneof=0 1"`
}

// MoveDivisionRequest defines the payload for moving a division to a new parent.
type MoveDivisionRequest struct {
	ParentID *int `json:"parent_id"`
}
