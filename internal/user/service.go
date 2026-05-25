package user

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"keeper/pkg/auth"

	"golang.org/x/crypto/bcrypt"
)

// UserRepository defines the data access contract for users.
type UserRepository interface {
	Create(ctx context.Context, u User) (*User, error)
	GetByID(ctx context.Context, appID, id int) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	List(ctx context.Context, appID int) ([]*User, error)
	Update(ctx context.Context, appID, id int, u *User) (*User, error)
	Delete(ctx context.Context, appID, id int) error
}

// UserService defines the business logic for users.
type UserService interface {
	Create(ctx context.Context, req CreateUserRequest) (*User, error)
	GetByID(ctx context.Context, appID, id int) (*User, error)
	List(ctx context.Context, appID int) ([]*User, error)
	Update(ctx context.Context, appID, id int, req UpdateUserRequest) (*User, error)
	Delete(ctx context.Context, appID, id int) error
	Authenticate(ctx context.Context, req AuthRequest) (*AuthResponse, error)
}

type userService struct {
	repo UserRepository
	jwt  *auth.JWTManager
}

// NewUserService creates a new user service.
func NewUserService(repo UserRepository, jwt *auth.JWTManager) UserService {
	return &userService{repo: repo, jwt: jwt}
}

func (s *userService) Create(ctx context.Context, req CreateUserRequest) (*User, error) {
	slog.Info("creating user", "email", req.Email)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("failed to hash password", "error", err)
		return nil, fmt.Errorf("hash password: %w", err)
	}

	created, err := s.repo.Create(ctx, User{
		AppID:     req.AppID,
		Firstname: req.Firstname,
		Lastname:  req.Lastname,
		Email:     req.Email,
		Password:  string(hashedPassword),
		Status:    1,
	})
	if err != nil {
		slog.Error("failed to create user in repository", "email", req.Email, "error", err)
		return nil, fmt.Errorf("repository create: %w", err)
	}

	slog.Info("user created successfully", "id", created.ID, "email", created.Email)
	return created, nil
}

func (s *userService) GetByID(ctx context.Context, appID, id int) (*User, error) {
	slog.Info("getting user by id", "id", id)
	return s.repo.GetByID(ctx, appID, id)
}

func (s *userService) List(ctx context.Context, appID int) ([]*User, error) {
	users, err := s.repo.List(ctx, appID)
	if err != nil {
		slog.Error("failed to list users", "error", err)
		return nil, err
	}
	return users, nil
}

func (s *userService) Update(ctx context.Context, appID, id int, req UpdateUserRequest) (*User, error) {
	slog.Info("updating user", "id", id)
	existing, err := s.repo.GetByID(ctx, appID, id)
	if err != nil {
		slog.Error("failed to get user for update", "id", id, "error", err)
		return nil, err
	}

	if req.Firstname != nil {
		existing.Firstname = *req.Firstname
	}
	if req.Lastname != nil {
		existing.Lastname = *req.Lastname
	}
	if req.AppID != nil {
		existing.AppID = *req.AppID
	}
	if req.Email != nil {
		existing.Email = *req.Email
	}
	if req.Password != nil {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			slog.Error("failed to hash new password", "id", id, "error", err)
			return nil, fmt.Errorf("hash password: %w", err)
		}
		existing.Password = string(hashedPassword)
	}
	if req.Status != nil {
		existing.Status = *req.Status
	}

	updated, err := s.repo.Update(ctx, appID, id, existing)
	if err != nil {
		slog.Error("failed to update user in repository", "id", id, "error", err)
		return nil, err
	}

	slog.Info("user updated successfully", "id", id)
	return updated, nil
}

func (s *userService) Delete(ctx context.Context, appID, id int) error {
	slog.Info("deleting user", "id", id)
	err := s.repo.Delete(ctx, appID, id)
	if err != nil {
		slog.Error("failed to delete user", "id", id, "error", err)
		return err
	}
	slog.Info("user deleted successfully", "id", id)
	return nil
}

func (s *userService) Authenticate(ctx context.Context, req AuthRequest) (*AuthResponse, error) {
	slog.Info("authenticating user", "email", req.Email)
	u, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		slog.Warn("authentication failed: user not found", "email", req.Email)
		return nil, errors.New("invalid credentials")
	}

	if u.Status != 1 {
		slog.Warn("authentication failed: user inactive", "email", req.Email, "user_id", u.ID)
		return nil, errors.New("invalid credentials")
	}

	if u.AppStatus != 1 {
		slog.Warn("authentication failed: app inactive", "email", req.Email, "app_id", u.AppID)
		return nil, errors.New("invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password))
	if err != nil {
		slog.Warn("authentication failed: invalid password", "email", req.Email)
		return nil, errors.New("invalid credentials")
	}

	token, err := s.jwt.Generate(u.AppID, u.ID)
	if err != nil {
		slog.Error("failed to generate JWT token", "id", u.ID, "error", err)
		return nil, fmt.Errorf("generate token: %w", err)
	}

	slog.Info("user authenticated successfully", "id", u.ID, "email", u.Email)
	return &AuthResponse{
		Token: token,
		User:  *u,
	}, nil
}
