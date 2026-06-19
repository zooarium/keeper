package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
)

// isHTTPURL reports whether s is empty (optional) or a valid http(s) URL.
func isHTTPURL(s string) bool {
	if s == "" {
		return true
	}
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// validateSocial ensures every social link value is a valid http(s) URL.
// Keys (platform names) are free-form; absent/empty maps are allowed.
func validateSocial(social map[string]string) error {
	for platform, link := range social {
		if !isHTTPURL(link) {
			return fmt.Errorf("invalid social url for %q: %s", platform, link)
		}
	}
	return nil
}

// toAbout maps an about input payload to the domain model.
func toAbout(in AboutInput) About {
	return About(in)
}

// toContact maps a contact input payload to the domain model.
func toContact(in ContactInput) Contact {
	return Contact{
		Address: Address{
			Line1:      in.Address.Line1,
			Line2:      in.Address.Line2,
			City:       in.Address.City,
			State:      in.Address.State,
			Country:    in.Address.Country,
			PostalCode: in.Address.PostalCode,
		},
		Phone1: in.Phone1,
		Phone2: in.Phone2,
		Email:  in.Email,
		Hours:  in.Hours,
		Social: in.Social,
	}
}

// AppRepository defines the data access contract for apps.
type AppRepository interface {
	Create(ctx context.Context, a App) (*App, error)
	GetByID(ctx context.Context, id int) (*App, error)
	List(ctx context.Context, limit, offset int) ([]*App, error)
	Update(ctx context.Context, id int, a *App) (*App, error)
	Delete(ctx context.Context, id int) error
}

// AppService defines the business logic for apps.
type AppService interface {
	Create(ctx context.Context, req CreateAppRequest) (*App, error)
	GetByID(ctx context.Context, id int) (*App, error)
	List(ctx context.Context, limit, offset int) ([]*App, error)
	Update(ctx context.Context, id int, req UpdateAppRequest) (*App, error)
	Delete(ctx context.Context, id int) error
}

type appService struct {
	repo AppRepository
}

// NewAppService creates a new app service.
func NewAppService(repo AppRepository) AppService {
	return &appService{repo: repo}
}

func (s *appService) Create(ctx context.Context, req CreateAppRequest) (*App, error) {
	slog.Info("creating app", "name", req.Name)

	if err := validateSocial(req.Contact.Social); err != nil {
		slog.Warn("invalid social url on create", "name", req.Name, "error", err)
		return nil, err
	}

	status := int8(1)
	if req.Status != 0 {
		status = req.Status
	}

	created, err := s.repo.Create(ctx, App{
		Name:    req.Name,
		Tagline: req.Tagline,
		LogoURL: req.LogoURL,
		About:   toAbout(req.About),
		Contact: toContact(req.Contact),
		Status:  status,
	})
	if err != nil {
		return nil, fmt.Errorf("repository create: %w", err)
	}

	slog.Info("app created successfully", "id", created.ID, "name", created.Name)
	return created, nil
}

func (s *appService) GetByID(ctx context.Context, id int) (*App, error) {
	slog.Info("getting app by id", "id", id)
	return s.repo.GetByID(ctx, id)
}

func (s *appService) List(ctx context.Context, limit, offset int) ([]*App, error) {
	slog.Info("listing apps", "limit", limit, "offset", offset)
	return s.repo.List(ctx, limit, offset)
}

func (s *appService) Update(ctx context.Context, id int, req UpdateAppRequest) (*App, error) {
	slog.Info("updating app", "id", id)
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Tagline != nil {
		existing.Tagline = *req.Tagline
	}
	if req.LogoURL != nil {
		existing.LogoURL = *req.LogoURL
	}
	if req.About != nil {
		existing.About = toAbout(*req.About)
	}
	if req.Contact != nil {
		if err := validateSocial(req.Contact.Social); err != nil {
			slog.Warn("invalid social url on update", "id", id, "error", err)
			return nil, err
		}
		existing.Contact = toContact(*req.Contact)
	}
	if req.Status != nil {
		existing.Status = *req.Status
	}

	updated, err := s.repo.Update(ctx, id, existing)
	if err != nil {
		return nil, err
	}

	slog.Info("app updated successfully", "id", id)
	return updated, nil
}

func (s *appService) Delete(ctx context.Context, id int) error {
	slog.Info("deleting app", "id", id)
	err := s.repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	slog.Info("app deleted successfully", "id", id)
	return nil
}
