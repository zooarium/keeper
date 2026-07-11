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
	svc := NewAppService(repo, nil)

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

	svc := NewAppService(NewAppRepository(client), nil)
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
		TaxNumber:  "NL123456789B01",
		TaxPercent: 21,
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
	assert.Equal(t, "NL123456789B01", got.TaxNumber)
	assert.Equal(t, float64(21), got.TaxPercent)
}

func TestService_Create_InvalidSocialURL(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_app_badsocial?mode=memory&cache=shared&_fk=1")
	defer func() {
		err := client.Close()
		assert.NoError(t, err)
	}()

	svc := NewAppService(NewAppRepository(client), nil)
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

	svc := NewAppService(NewAppRepository(client), nil)
	ctx := context.Background()

	a, err := svc.Create(ctx, CreateAppRequest{Name: "Orig"})
	assert.NoError(t, err)

	tagline := "New tagline"
	taxNumber := "NL987654321B01"
	taxPercent := 9.0
	req := UpdateAppRequest{
		Tagline: &tagline,
		Contact: &ContactInput{
			Phone1: "+31 999",
			Social: map[string]string{"linkedin": "https://linkedin.com/in/me"},
		},
		TaxNumber:  &taxNumber,
		TaxPercent: &taxPercent,
	}
	updated, err := svc.Update(ctx, a.ID, req)
	assert.NoError(t, err)
	assert.Equal(t, "New tagline", updated.Tagline)
	assert.Equal(t, "+31 999", updated.Contact.Phone1)
	assert.Equal(t, "https://linkedin.com/in/me", updated.Contact.Social["linkedin"])
	assert.Equal(t, "NL987654321B01", updated.TaxNumber)
	assert.Equal(t, 9.0, updated.TaxPercent)

	// Absent tax fields leave existing values untouched.
	updated, err = svc.Update(ctx, a.ID, UpdateAppRequest{Tagline: &tagline})
	assert.NoError(t, err)
	assert.Equal(t, "NL987654321B01", updated.TaxNumber)
	assert.Equal(t, 9.0, updated.TaxPercent)
}

func TestService_Update(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_app_update?mode=memory&cache=shared&_fk=1")
	defer func() {
		err := client.Close()
		assert.NoError(t, err)
	}()

	repo := NewAppRepository(client)
	svc := NewAppService(repo, nil)

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
	svc := NewAppService(repo, nil)

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
	svc := NewAppService(repo, nil)

	ctx := context.Background()
	_, _ = svc.Create(ctx, CreateAppRequest{Name: "App 1"})
	_, _ = svc.Create(ctx, CreateAppRequest{Name: "App 2"})

	apps, err := svc.List(ctx, 50, 0)
	assert.NoError(t, err)
	assert.Len(t, apps, 2)
}

// fakeResolver is a stub GuestKeyResolver for the public lookup tests.
type fakeResolver struct {
	appID int
	err   error
}

func (f fakeResolver) AppIDBySiteKey(ctx context.Context, siteKey string) (int, error) {
	return f.appID, f.err
}

func int8Ptr(v int8) *int8 { return &v }

func TestService_PublicBySiteKey_Active(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_app_pub?mode=memory&cache=shared&_fk=1")
	defer func() { assert.NoError(t, client.Close()) }()

	repo := NewAppRepository(client)
	ctx := context.Background()
	a, err := NewAppService(repo, nil).Create(ctx, CreateAppRequest{Name: "Pub App", Tagline: "t", TaxNumber: "NL123456789B01", TaxPercent: 21})
	assert.NoError(t, err)

	svc := NewAppService(repo, fakeResolver{appID: a.ID})
	pub, err := svc.PublicBySiteKey(ctx, "gk_x")
	assert.NoError(t, err)
	assert.Equal(t, a.ID, pub.ID)
	assert.Equal(t, "Pub App", pub.Name)
	assert.Equal(t, "NL123456789B01", pub.TaxNumber)
	assert.Equal(t, float64(21), pub.TaxPercent)
}

func TestService_PublicBySiteKey_Inactive(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_app_pub_inactive?mode=memory&cache=shared&_fk=1")
	defer func() { assert.NoError(t, client.Close()) }()

	repo := NewAppRepository(client)
	ctx := context.Background()
	a, err := NewAppService(repo, nil).Create(ctx, CreateAppRequest{Name: "Off App"})
	assert.NoError(t, err)
	_, err = NewAppService(repo, nil).Update(ctx, a.ID, UpdateAppRequest{Status: int8Ptr(0)})
	assert.NoError(t, err)

	svc := NewAppService(repo, fakeResolver{appID: a.ID})
	_, err = svc.PublicBySiteKey(ctx, "gk_x")
	assert.ErrorIs(t, err, ErrAppNotPublic)
}

func TestService_PublicBySiteKey_BadSiteKey(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_app_pub_bad?mode=memory&cache=shared&_fk=1")
	defer func() { assert.NoError(t, client.Close()) }()

	svc := NewAppService(NewAppRepository(client), fakeResolver{err: assert.AnError})
	_, err := svc.PublicBySiteKey(context.Background(), "gk_bad")
	assert.ErrorIs(t, err, ErrAppNotPublic)

	// Empty site key short-circuits without touching the resolver.
	_, err = svc.PublicBySiteKey(context.Background(), "")
	assert.ErrorIs(t, err, ErrAppNotPublic)
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
