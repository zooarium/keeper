package app

import (
	"context"
	"fmt"
	"log/slog"

	"keeper/ent"
	entapp "keeper/ent/app"
)

type appRepository struct {
	client *ent.Client
}

// NewAppRepository creates a new app repository.
func NewAppRepository(client *ent.Client) *appRepository {
	return &appRepository{client: client}
}

func (r *appRepository) Create(ctx context.Context, a App) (*App, error) {
	created, err := r.client.App.
		Create().
		SetName(a.Name).
		SetTagline(a.Tagline).
		SetLogoURL(a.LogoURL).
		SetAboutHeading(a.About.Heading).
		SetAboutBody(a.About.Body).
		SetContactAddressLine1(a.Contact.Address.Line1).
		SetContactAddressLine2(a.Contact.Address.Line2).
		SetContactCity(a.Contact.Address.City).
		SetContactState(a.Contact.Address.State).
		SetContactCountry(a.Contact.Address.Country).
		SetContactPostalCode(a.Contact.Address.PostalCode).
		SetContactPhone1(a.Contact.Phone1).
		SetContactPhone2(a.Contact.Phone2).
		SetContactEmail(a.Contact.Email).
		SetContactHours(a.Contact.Hours).
		SetContactSocial(a.Contact.Social).
		SetStatus(a.Status).
		Save(ctx)
	if err != nil {
		slog.Error("database error: failed to create app", "name", a.Name, "error", err)
		return nil, err
	}
	return r.mapToModel(created), nil
}

func (r *appRepository) GetByID(ctx context.Context, id int) (*App, error) {
	a, err := r.client.App.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.Warn("app not found in database", "id", id)
			return nil, fmt.Errorf("app not found: %w", err)
		}
		slog.Error("database error: failed to get app by id", "id", id, "error", err)
		return nil, err
	}
	return r.mapToModel(a), nil
}

func (r *appRepository) List(ctx context.Context, limit, offset int) ([]*App, error) {
	apps, err := r.client.App.Query().
		Order(ent.Asc(entapp.FieldID)).
		Limit(limit).
		Offset(offset).
		All(ctx)
	if err != nil {
		slog.Error("database error: failed to list apps", "error", err)
		return nil, err
	}
	result := make([]*App, len(apps))
	for i, a := range apps {
		result[i] = r.mapToModel(a)
	}
	return result, nil
}

func (r *appRepository) Update(ctx context.Context, id int, a *App) (*App, error) {
	updated, err := r.client.App.UpdateOneID(id).
		SetName(a.Name).
		SetTagline(a.Tagline).
		SetLogoURL(a.LogoURL).
		SetAboutHeading(a.About.Heading).
		SetAboutBody(a.About.Body).
		SetContactAddressLine1(a.Contact.Address.Line1).
		SetContactAddressLine2(a.Contact.Address.Line2).
		SetContactCity(a.Contact.Address.City).
		SetContactState(a.Contact.Address.State).
		SetContactCountry(a.Contact.Address.Country).
		SetContactPostalCode(a.Contact.Address.PostalCode).
		SetContactPhone1(a.Contact.Phone1).
		SetContactPhone2(a.Contact.Phone2).
		SetContactEmail(a.Contact.Email).
		SetContactHours(a.Contact.Hours).
		SetContactSocial(a.Contact.Social).
		SetStatus(a.Status).
		Save(ctx)
	if err != nil {
		slog.Error("database error: failed to update app", "id", id, "error", err)
		return nil, err
	}
	return r.mapToModel(updated), nil
}

func (r *appRepository) Delete(ctx context.Context, id int) error {
	err := r.client.App.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			slog.Warn("app not found for deletion", "id", id)
			return fmt.Errorf("app not found: %w", err)
		}
		slog.Error("database error: failed to delete app", "id", id, "error", err)
		return err
	}
	return nil
}

func (r *appRepository) mapToModel(a *ent.App) *App {
	return &App{
		ID:      a.ID,
		Name:    a.Name,
		Tagline: a.Tagline,
		LogoURL: a.LogoURL,
		About: About{
			Heading: a.AboutHeading,
			Body:    a.AboutBody,
		},
		Contact: Contact{
			Address: Address{
				Line1:      a.ContactAddressLine1,
				Line2:      a.ContactAddressLine2,
				City:       a.ContactCity,
				State:      a.ContactState,
				Country:    a.ContactCountry,
				PostalCode: a.ContactPostalCode,
			},
			Phone1: a.ContactPhone1,
			Phone2: a.ContactPhone2,
			Email:  a.ContactEmail,
			Hours:  a.ContactHours,
			Social: a.ContactSocial,
		},
		Status:    a.Status,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}
