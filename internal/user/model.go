package user

import (
	"time"
)

// User represents the domain model for a user.
//
// TODO(falcon): role/permission no longer lives on the user record — it's
// resolved entirely from falcon at login (see policy.Can()).
type User struct {
	ID           int       `json:"id"`
	AppID        int       `json:"app_id"`
	AppName      string    `json:"app_name"`
	AppStatus    int8      `json:"-"`
	DivisionID   int       `json:"division_id"`
	DivisionName string    `json:"division_name"`
	Firstname    string    `json:"firstname"`
	Lastname     string    `json:"lastname"`
	Email        string    `json:"email"`
	Password     string    `json:"-"`
	Status       int8      `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateUserRequest defines the payload for creating a user.
type CreateUserRequest struct {
	AppID      int    `json:"app_id"      validate:"required"`
	DivisionID int    `json:"division_id" validate:"required"`
	Firstname  string `json:"firstname"   validate:"required"`
	Lastname   string `json:"lastname"    validate:"required"`
	Email      string `json:"email"       validate:"required,email"`
	Password   string `json:"password"    validate:"required,min=8"`
}

// UpdateUserRequest defines the payload for updating a user.
type UpdateUserRequest struct {
	AppID      *int    `json:"app_id"`
	DivisionID *int    `json:"division_id"`
	Firstname  *string `json:"firstname"`
	Lastname   *string `json:"lastname"`
	Email      *string `json:"email"     validate:"omitempty,email"`
	Password   *string `json:"password"  validate:"omitempty,min=8"`
	Status     *int8   `json:"status"`
}

// AuthRequest defines the payload for user authentication.
type AuthRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// AuthResponse defines the response after successful authentication.
type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
