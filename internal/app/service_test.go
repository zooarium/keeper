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
