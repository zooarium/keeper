package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
)

// ErrAppNotPublic is returned by the public site-key lookup when no active app
// can be resolved for the supplied site key. It intentionally conflates an
// unknown/inactive site key with a missing/inactive app so the public endpoint
// never leaks which of the two failed.
var ErrAppNotPublic = errors.New("no app found for the given site key")

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

// toPublicApp maps a full app to its public-safe projection.
func toPublicApp(a *App) *PublicApp {
	return &PublicApp{
		ID:         a.ID,
		Name:       a.Name,
		Tagline:    a.Tagline,
		LogoURL:    a.LogoURL,
		About:      a.About,
		Contact:    a.Contact,
		TaxNumber:  a.TaxNumber,
		TaxPercent: a.TaxPercent,
	}
}

// GuestKeyResolver resolves a publishable guest site key to its bound app ID.
// Implemented by the guestkey service; injected to keep the app package free of
// a direct dependency on guestkey.
type GuestKeyResolver interface {
	AppIDBySiteKey(ctx context.Context, siteKey string) (int, error)
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
	PublicBySiteKey(ctx context.Context, siteKey string) (*PublicApp, error)
	PublicByID(ctx context.Context, id int) (*PublicApp, error)
}

type appService struct {
	repo     AppRepository
	resolver GuestKeyResolver
}

// NewAppService creates a new app service. resolver may be nil where the public
// site-key lookup is not exercised (e.g. in unit tests).
func NewAppService(repo AppRepository, resolver GuestKeyResolver) AppService {
	return &appService{repo: repo, resolver: resolver}
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
		Name:       req.Name,
		Tagline:    req.Tagline,
		LogoURL:    req.LogoURL,
		About:      toAbout(req.About),
		Contact:    toContact(req.Contact),
		TaxNumber:  req.TaxNumber,
		TaxPercent: req.TaxPercent,
		Status:     status,
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
	if req.TaxNumber != nil {
		existing.TaxNumber = *req.TaxNumber
	}
	if req.TaxPercent != nil {
		existing.TaxPercent = *req.TaxPercent
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

// PublicBySiteKey resolves a publishable guest site key to its app and returns
// the public-safe profile. Returns ErrAppNotPublic when the site key is
// unknown/inactive or the resolved app is missing/inactive.
func (s *appService) PublicBySiteKey(ctx context.Context, siteKey string) (*PublicApp, error) {
	if siteKey == "" || s.resolver == nil {
		return nil, ErrAppNotPublic
	}

	appID, err := s.resolver.AppIDBySiteKey(ctx, siteKey)
	if err != nil {
		slog.Warn("public app lookup: site key did not resolve", "error", err)
		return nil, ErrAppNotPublic
	}

	a, err := s.repo.GetByID(ctx, appID)
	if err != nil {
		slog.Warn("public app lookup: app not found", "app_id", appID, "error", err)
		return nil, ErrAppNotPublic
	}

	if a.Status != 1 {
		slog.Warn("public app lookup: app inactive", "app_id", appID)
		return nil, ErrAppNotPublic
	}

	slog.Info("public app profile served", "app_id", appID)
	return toPublicApp(a), nil
}

// PublicByID returns the public-safe profile for an app by its ID. Used by
// downstream services (e.g. ant order enrichment) that hold an app_id from
// JWT claims or persisted rows. Same projection and inactive-app containment
// as PublicBySiteKey.
func (s *appService) PublicByID(ctx context.Context, id int) (*PublicApp, error) {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		slog.Warn("public app profile: app not found", "app_id", id, "error", err)
		return nil, ErrAppNotPublic
	}

	if a.Status != 1 {
		slog.Warn("public app profile: app inactive", "app_id", id)
		return nil, ErrAppNotPublic
	}

	return toPublicApp(a), nil
}
