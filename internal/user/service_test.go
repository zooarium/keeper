package user

import (
	"context"
	"fmt"
	"testing"
	"time"

	"keeper/ent/enttest"
	"keeper/pkg/auth"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func TestService_Create(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer func() {
		err := client.Close()
		assert.NoError(t, err)
	}()

	repo := NewUserRepository(client)
	jwtManager := auth.NewJWTManager("secret", time.Hour)
	svc := NewUserService(repo, jwtManager)

	ctx := context.Background()

	a, err := client.App.Create().SetName("Test App").Save(ctx)
	assert.NoError(t, err)

	div, err := client.Division.Create().
		SetAppID(a.ID).
		SetName("Root").
		SetPath(fmt.Sprintf("/%d/", 1)).
		SetDepth(0).
		SetStatus(1).
		Save(ctx)
	assert.NoError(t, err)
	// Update path with actual ID
	div, err = client.Division.UpdateOneID(div.ID).SetPath(fmt.Sprintf("/%d/", div.ID)).Save(ctx)
	assert.NoError(t, err)

	req := CreateUserRequest{
		AppID:      a.ID,
		DivisionID: div.ID,
		Firstname:  "John",
		Lastname:   "Doe",
		Email:      "john@example.com",
		Password:   "password123",
	}

	u, err := svc.Create(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, u)
	assert.Equal(t, req.Email, u.Email)
	assert.Equal(t, req.Firstname, u.Firstname)
	assert.Equal(t, req.Lastname, u.Lastname)
	assert.Equal(t, "Test App", u.AppName)
	assert.Equal(t, div.ID, u.DivisionID)
	assert.Equal(t, int8(1), u.Status)
}

func TestService_Authenticate(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_auth?mode=memory&cache=shared&_fk=1")
	defer func() {
		err := client.Close()
		assert.NoError(t, err)
	}()

	repo := NewUserRepository(client)
	jwtManager := auth.NewJWTManager("secret", time.Hour)
	svc := NewUserService(repo, jwtManager)

	ctx := context.Background()
	email := "auth@example.com"
	password := "password123"

	a, err := client.App.Create().SetName("Auth App").Save(ctx)
	assert.NoError(t, err)

	div, err := client.Division.Create().
		SetAppID(a.ID).SetName("Root").SetPath("/0/").SetDepth(0).SetStatus(1).Save(ctx)
	assert.NoError(t, err)
	div, err = client.Division.UpdateOneID(div.ID).SetPath(fmt.Sprintf("/%d/", div.ID)).Save(ctx)
	assert.NoError(t, err)

	_, err = svc.Create(ctx, CreateUserRequest{
		AppID: a.ID, DivisionID: div.ID,
		Firstname: "Auth", Lastname: "User",
		Email: email, Password: password,
	})
	assert.NoError(t, err)

	t.Run("Success", func(t *testing.T) {
		res, err := svc.Authenticate(ctx, AuthRequest{Email: email, Password: password})
		assert.NoError(t, err)
		assert.NotEmpty(t, res.Token)
		assert.Equal(t, email, res.User.Email)
		assert.Equal(t, "Auth App", res.User.AppName)
	})

	t.Run("InvalidPassword", func(t *testing.T) {
		res, err := svc.Authenticate(ctx, AuthRequest{Email: email, Password: "wrongpassword"})
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Equal(t, "invalid credentials", err.Error())
	})

	t.Run("UserNotFound", func(t *testing.T) {
		res, err := svc.Authenticate(ctx, AuthRequest{Email: "nonexistent@example.com", Password: password})
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Equal(t, "invalid credentials", err.Error())
	})

	t.Run("InactiveUser", func(t *testing.T) {
		inactive := int8(0)
		_, err := svc.Create(ctx, CreateUserRequest{
			AppID: a.ID, DivisionID: div.ID,
			Firstname: "Inactive", Lastname: "User",
			Email: "inactive@example.com", Password: password,
		})
		assert.NoError(t, err)

		users, err := svc.List(ctx, a.ID)
		assert.NoError(t, err)
		var inactiveID int
		for _, usr := range users {
			if usr.Email == "inactive@example.com" {
				inactiveID = usr.ID
			}
		}

		_, err = svc.Update(ctx, a.ID, inactiveID, UpdateUserRequest{Status: &inactive})
		assert.NoError(t, err)

		res, err := svc.Authenticate(ctx, AuthRequest{Email: "inactive@example.com", Password: password})
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Equal(t, "invalid credentials", err.Error())
	})

	t.Run("InactiveApp", func(t *testing.T) {
		inactiveApp, err := client.App.Create().SetName("Inactive App").SetStatus(0).Save(ctx)
		assert.NoError(t, err)

		inactiveDiv, err := client.Division.Create().
			SetAppID(inactiveApp.ID).SetName("Root").SetPath("/0/").SetDepth(0).SetStatus(1).Save(ctx)
		assert.NoError(t, err)
		inactiveDiv, err = client.Division.UpdateOneID(inactiveDiv.ID).
			SetPath(fmt.Sprintf("/%d/", inactiveDiv.ID)).Save(ctx)
		assert.NoError(t, err)

		_, err = svc.Create(ctx, CreateUserRequest{
			AppID: inactiveApp.ID, DivisionID: inactiveDiv.ID,
			Firstname: "App", Lastname: "User",
			Email: "appuser@example.com", Password: password,
		})
		assert.NoError(t, err)

		res, err := svc.Authenticate(ctx, AuthRequest{Email: "appuser@example.com", Password: password})
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Equal(t, "invalid credentials", err.Error())
	})
}

func TestService_Update(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_update?mode=memory&cache=shared&_fk=1")
	defer func() {
		err := client.Close()
		assert.NoError(t, err)
	}()

	repo := NewUserRepository(client)
	jwtManager := auth.NewJWTManager("secret", time.Hour)
	svc := NewUserService(repo, jwtManager)

	ctx := context.Background()

	a, err := client.App.Create().SetName("Update App").Save(ctx)
	assert.NoError(t, err)

	div, err := client.Division.Create().
		SetAppID(a.ID).SetName("Root").SetPath("/0/").SetDepth(0).SetStatus(1).Save(ctx)
	assert.NoError(t, err)
	div, err = client.Division.UpdateOneID(div.ID).SetPath(fmt.Sprintf("/%d/", div.ID)).Save(ctx)
	assert.NoError(t, err)

	u, err := svc.Create(ctx, CreateUserRequest{
		AppID: a.ID, DivisionID: div.ID,
		Firstname: "Original", Lastname: "Name",
		Email: "original@example.com", Password: "password123",
	})
	assert.NoError(t, err)

	newApp, err := client.App.Create().SetName("New App").Save(ctx)
	assert.NoError(t, err)

	newName := "Updated"
	newStatus := int8(0)
	req := UpdateUserRequest{
		Firstname: &newName,
		Status:    &newStatus,
		AppID:     &newApp.ID,
	}

	updated, err := svc.Update(ctx, a.ID, u.ID, req)
	assert.NoError(t, err)
	assert.Equal(t, newName, updated.Firstname)
	assert.Equal(t, newStatus, updated.Status)
	assert.Equal(t, newApp.ID, updated.AppID)
	assert.Equal(t, u.Email, updated.Email)
}

func TestService_Update_CrossTenant(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_update_cross?mode=memory&cache=shared&_fk=1")
	defer func() {
		err := client.Close()
		assert.NoError(t, err)
	}()

	repo := NewUserRepository(client)
	jwtManager := auth.NewJWTManager("secret", time.Hour)
	svc := NewUserService(repo, jwtManager)

	ctx := context.Background()

	app1, _ := client.App.Create().SetName("App 1").Save(ctx)
	app2, _ := client.App.Create().SetName("App 2").Save(ctx)

	div1, _ := client.Division.Create().
		SetAppID(app1.ID).SetName("Root").SetPath("/0/").SetDepth(0).SetStatus(1).Save(ctx)
	div1, _ = client.Division.UpdateOneID(div1.ID).SetPath(fmt.Sprintf("/%d/", div1.ID)).Save(ctx)

	u, err := svc.Create(ctx, CreateUserRequest{
		AppID: app1.ID, DivisionID: div1.ID,
		Firstname: "User", Lastname: "One",
		Email: "user1@example.com", Password: "password123",
	})
	assert.NoError(t, err)
	_ = app2

	newName := "Hacked"
	_, err = svc.Update(ctx, app2.ID, u.ID, UpdateUserRequest{Firstname: &newName})
	assert.Error(t, err, "cross-tenant update must fail")
}

func TestService_Delete(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_delete?mode=memory&cache=shared&_fk=1")
	defer func() {
		err := client.Close()
		assert.NoError(t, err)
	}()

	repo := NewUserRepository(client)
	jwtManager := auth.NewJWTManager("secret", time.Hour)
	svc := NewUserService(repo, jwtManager)

	ctx := context.Background()

	a, err := client.App.Create().SetName("Delete App").Save(ctx)
	assert.NoError(t, err)

	div, err := client.Division.Create().
		SetAppID(a.ID).SetName("Root").SetPath("/0/").SetDepth(0).SetStatus(1).Save(ctx)
	assert.NoError(t, err)
	div, err = client.Division.UpdateOneID(div.ID).SetPath(fmt.Sprintf("/%d/", div.ID)).Save(ctx)
	assert.NoError(t, err)

	u, err := svc.Create(ctx, CreateUserRequest{
		AppID: a.ID, DivisionID: div.ID,
		Firstname: "Delete", Lastname: "Me",
		Email: "delete@example.com", Password: "password123",
	})
	assert.NoError(t, err)

	err = svc.Delete(ctx, a.ID, u.ID)
	assert.NoError(t, err)

	_, err = svc.GetByID(ctx, a.ID, u.ID)
	assert.Error(t, err)
}

func TestService_Delete_CrossTenant(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_delete_cross?mode=memory&cache=shared&_fk=1")
	defer func() {
		err := client.Close()
		assert.NoError(t, err)
	}()

	repo := NewUserRepository(client)
	jwtManager := auth.NewJWTManager("secret", time.Hour)
	svc := NewUserService(repo, jwtManager)

	ctx := context.Background()

	app1, _ := client.App.Create().SetName("App 1").Save(ctx)
	app2, _ := client.App.Create().SetName("App 2").Save(ctx)

	div1, _ := client.Division.Create().
		SetAppID(app1.ID).SetName("Root").SetPath("/0/").SetDepth(0).SetStatus(1).Save(ctx)
	div1, _ = client.Division.UpdateOneID(div1.ID).SetPath(fmt.Sprintf("/%d/", div1.ID)).Save(ctx)

	u, err := svc.Create(ctx, CreateUserRequest{
		AppID: app1.ID, DivisionID: div1.ID,
		Firstname: "User", Lastname: "One",
		Email: "victim@example.com", Password: "password123",
	})
	assert.NoError(t, err)
	_ = app2

	err = svc.Delete(ctx, app2.ID, u.ID)
	assert.Error(t, err, "cross-tenant delete must fail")
}

func BenchmarkService_Create(b *testing.B) {
	client := enttest.Open(b, "sqlite3", "file:ent_bench_create?mode=memory&cache=shared&_fk=1")
	defer func() { _ = client.Close() }()

	repo := NewUserRepository(client)
	jwtManager := auth.NewJWTManager("secret", time.Hour)
	svc := NewUserService(repo, jwtManager)

	ctx := context.Background()
	a, _ := client.App.Create().SetName("Bench App").Save(ctx)
	div, _ := client.Division.Create().
		SetAppID(a.ID).SetName("Root").SetPath("/0/").SetDepth(0).SetStatus(1).Save(ctx)
	div, _ = client.Division.UpdateOneID(div.ID).SetPath(fmt.Sprintf("/%d/", div.ID)).Save(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		email := fmt.Sprintf("bench_%d@example.com", i)
		_, _ = svc.Create(ctx, CreateUserRequest{
			AppID:      a.ID,
			DivisionID: div.ID,
			Firstname:  "Bench",
			Lastname:   "User",
			Email:      email,
			Password:   "password123",
		})
	}
}

func BenchmarkService_Authenticate(b *testing.B) {
	client := enttest.Open(b, "sqlite3", "file:ent_bench_auth?mode=memory&cache=shared&_fk=1")
	defer func() { _ = client.Close() }()

	repo := NewUserRepository(client)
	jwtManager := auth.NewJWTManager("secret", time.Hour)
	svc := NewUserService(repo, jwtManager)

	ctx := context.Background()
	email := "bench_auth@example.com"
	password := "password123"

	a, _ := client.App.Create().SetName("Bench Auth App").Save(ctx)
	div, _ := client.Division.Create().
		SetAppID(a.ID).SetName("Root").SetPath("/0/").SetDepth(0).SetStatus(1).Save(ctx)
	div, _ = client.Division.UpdateOneID(div.ID).SetPath(fmt.Sprintf("/%d/", div.ID)).Save(ctx)

	_, _ = svc.Create(ctx, CreateUserRequest{
		AppID: a.ID, DivisionID: div.ID,
		Firstname: "Bench", Lastname: "Auth",
		Email: email, Password: password,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.Authenticate(ctx, AuthRequest{Email: email, Password: password})
	}
}
