package app

import (
	"context"
	"testing"

	"keeper/ent/enttest"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func TestService_Create(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_app?mode=memory&cache=shared&_fk=1")
	defer func() {
		err := client.Close()
		assert.NoError(t, err)
	}()

	repo := NewAppRepository(client)
	svc := NewAppService(repo)

	ctx := context.Background()
	req := CreateAppRequest{
		Name: "Test App",
	}

	a, err := svc.Create(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, a)
	assert.Equal(t, req.Name, a.Name)
	assert.Equal(t, int8(1), a.Status)
}

func TestService_Create_FullProfile(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_app_full?mode=memory&cache=shared&_fk=1")
	defer func() {
		err := client.Close()
		assert.NoError(t, err)
	}()

	svc := NewAppService(NewAppRepository(client))
	ctx := context.Background()

	req := CreateAppRequest{
		Name:    "Profile App",
		Tagline: "We keep things",
		LogoURL: "https://cdn.example.com/logo.png",
		About: AboutInput{
			Heading: "About us",
			Body:    "<p>Hello <strong>world</strong></p>",
		},
		Contact: ContactInput{
			Address: AddressInput{
				Line1: "1 Main St", Line2: "Suite 2", City: "Town",
				State: "ST", Country: "NL", PostalCode: "1234AB",
			},
			Phone1: "+31 100",
			Phone2: "+31 200",
			Email:  "hi@example.com",
			Hours:  "Mon-Fri 9-5\nSat closed",
			Social: map[string]string{
				"twitter":  "https://twitter.com/me",
				"facebook": "https://facebook.com/me",
			},
		},
	}

	a, err := svc.Create(ctx, req)
	assert.NoError(t, err)

	got, err := svc.GetByID(ctx, a.ID)
	assert.NoError(t, err)
	assert.Equal(t, "We keep things", got.Tagline)
	assert.Equal(t, "https://cdn.example.com/logo.png", got.LogoURL)
	assert.Equal(t, "About us", got.About.Heading)
	assert.Equal(t, "<p>Hello <strong>world</strong></p>", got.About.Body)
	assert.Equal(t, "1 Main St", got.Contact.Address.Line1)
	assert.Equal(t, "1234AB", got.Contact.Address.PostalCode)
	assert.Equal(t, "+31 100", got.Contact.Phone1)
	assert.Equal(t, "+31 200", got.Contact.Phone2)
	assert.Equal(t, "hi@example.com", got.Contact.Email)
	assert.Equal(t, "Mon-Fri 9-5\nSat closed", got.Contact.Hours)
	assert.Equal(t, "https://twitter.com/me", got.Contact.Social["twitter"])
	assert.Len(t, got.Contact.Social, 2)
}

func TestService_Create_InvalidSocialURL(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_app_badsocial?mode=memory&cache=shared&_fk=1")
	defer func() {
		err := client.Close()
		assert.NoError(t, err)
	}()

	svc := NewAppService(NewAppRepository(client))
	ctx := context.Background()

	_, err := svc.Create(ctx, CreateAppRequest{
		Name:    "Bad Social",
		Contact: ContactInput{Social: map[string]string{"twitter": "not-a-url"}},
	})
	assert.Error(t, err)
}

func TestService_Update_Profile(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_app_updprofile?mode=memory&cache=shared&_fk=1")
	defer func() {
		err := client.Close()
		assert.NoError(t, err)
	}()

	svc := NewAppService(NewAppRepository(client))
	ctx := context.Background()

	a, err := svc.Create(ctx, CreateAppRequest{Name: "Orig"})
	assert.NoError(t, err)

	tagline := "New tagline"
	req := UpdateAppRequest{
		Tagline: &tagline,
		Contact: &ContactInput{
			Phone1: "+31 999",
			Social: map[string]string{"linkedin": "https://linkedin.com/in/me"},
		},
	}
	updated, err := svc.Update(ctx, a.ID, req)
	assert.NoError(t, err)
	assert.Equal(t, "New tagline", updated.Tagline)
	assert.Equal(t, "+31 999", updated.Contact.Phone1)
	assert.Equal(t, "https://linkedin.com/in/me", updated.Contact.Social["linkedin"])
}

func TestService_Update(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_app_update?mode=memory&cache=shared&_fk=1")
	defer func() {
		err := client.Close()
		assert.NoError(t, err)
	}()

	repo := NewAppRepository(client)
	svc := NewAppService(repo)

	ctx := context.Background()
	a, err := svc.Create(ctx, CreateAppRequest{
		Name: "Original App",
	})
	assert.NoError(t, err)

	newName := "Updated App"
	newStatus := int8(0)
	req := UpdateAppRequest{
		Name:   &newName,
		Status: &newStatus,
	}

	updated, err := svc.Update(ctx, a.ID, req)
	assert.NoError(t, err)
	assert.Equal(t, newName, updated.Name)
	assert.Equal(t, newStatus, updated.Status)
}

func TestService_Delete(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_app_delete?mode=memory&cache=shared&_fk=1")
	defer func() {
		err := client.Close()
		assert.NoError(t, err)
	}()

	repo := NewAppRepository(client)
	svc := NewAppService(repo)

	ctx := context.Background()
	a, err := svc.Create(ctx, CreateAppRequest{
		Name: "Delete App",
	})
	assert.NoError(t, err)

	err = svc.Delete(ctx, a.ID)
	assert.NoError(t, err)

	// Verify it's gone
	_, err = svc.GetByID(ctx, a.ID)
	assert.Error(t, err)
}

func TestService_List(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_app_list?mode=memory&cache=shared&_fk=1")
	defer func() {
		err := client.Close()
		assert.NoError(t, err)
	}()

	repo := NewAppRepository(client)
	svc := NewAppService(repo)

	ctx := context.Background()
	_, _ = svc.Create(ctx, CreateAppRequest{Name: "App 1"})
	_, _ = svc.Create(ctx, CreateAppRequest{Name: "App 2"})

	apps, err := svc.List(ctx, 50, 0)
	assert.NoError(t, err)
	assert.Len(t, apps, 2)
}

func TestIsHTTPURL(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true}, // optional: empty allowed (e.g. clearing logo_url)
		{"https://example.com/logo.png", true},
		{"http://example.com", true},
		{"notaurl", false},
		{"ftp://example.com", false},
		{"https://", false},
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, isHTTPURL(c.in), "isHTTPURL(%q)", c.in)
	}
}
