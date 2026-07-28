package user

import (
	"time"
)

const (
	RoleAdmin    int8 = 0
	RoleSysAdmin int8 = 1
	RoleManager  int8 = 3
)

// User represents the domain model for a user.
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
	Role         int8      `json:"role"`
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
	Role       int8   `json:"role"`
}

// UpdateUserRequest defines the payload for updating a user.
type UpdateUserRequest struct {
	AppID      *int    `json:"app_id"`
	DivisionID *int    `json:"division_id"`
	Firstname  *string `json:"firstname"`
	Lastname   *string `json:"lastname"`
	Email      *string `json:"email"     validate:"omitempty,email"`
	Password   *string `json:"password"  validate:"omitempty,min=8"`
	Role       *int8   `json:"role"`
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
