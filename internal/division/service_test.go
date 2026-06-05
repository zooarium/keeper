package division

import (
	"context"
	"testing"

	"keeper/ent/enttest"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func TestDivisionService_Create_Root(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_div_create?mode=memory&cache=shared&_fk=1")
	defer func() { _ = client.Close() }()

	// Need an app first
	app, err := client.App.Create().SetName("Test App").SetStatus(1).Save(context.Background())
	assert.NoError(t, err)

	repo := NewDivisionRepository(client)
	svc := NewDivisionService(repo)

	ctx := context.Background()
	req := CreateDivisionRequest{
		AppID: app.ID,
		Name:  "Root",
	}

	d, err := svc.Create(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, d)
	assert.Equal(t, "Root", d.Name)
	assert.Nil(t, d.ParentID)
	assert.Equal(t, int8(0), d.Depth)
	assert.Equal(t, "/1/", d.Path)
}

func TestDivisionService_Create_Child(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_div_child?mode=memory&cache=shared&_fk=1")
	defer func() { _ = client.Close() }()

	app, err := client.App.Create().SetName("Test App").SetStatus(1).Save(context.Background())
	assert.NoError(t, err)

	repo := NewDivisionRepository(client)
	svc := NewDivisionService(repo)

	ctx := context.Background()

	root, err := svc.Create(ctx, CreateDivisionRequest{AppID: app.ID, Name: "Root"})
	assert.NoError(t, err)

	child, err := svc.Create(ctx, CreateDivisionRequest{
		AppID:    app.ID,
		ParentID: &root.ID,
		Name:     "Engineering",
	})
	assert.NoError(t, err)
	assert.NotNil(t, child.ParentID)
	assert.Equal(t, root.ID, *child.ParentID)
	assert.Equal(t, int8(1), child.Depth)
	assert.Contains(t, child.Path, root.Path)
}

func TestDivisionService_Create_InvalidParent(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_div_invalid_parent?mode=memory&cache=shared&_fk=1")
	defer func() { _ = client.Close() }()

	app, err := client.App.Create().SetName("Test App").SetStatus(1).Save(context.Background())
	assert.NoError(t, err)

	repo := NewDivisionRepository(client)
	svc := NewDivisionService(repo)

	ctx := context.Background()
	nonExistentID := 9999
	_, err = svc.Create(ctx, CreateDivisionRequest{
		AppID:    app.ID,
		ParentID: &nonExistentID,
		Name:     "Engineering",
	})
	assert.Error(t, err)
}

func TestDivisionService_GetByID(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_div_get?mode=memory&cache=shared&_fk=1")
	defer func() { _ = client.Close() }()

	app, err := client.App.Create().SetName("Test App").SetStatus(1).Save(context.Background())
	assert.NoError(t, err)

	repo := NewDivisionRepository(client)
	svc := NewDivisionService(repo)

	ctx := context.Background()
	created, err := svc.Create(ctx, CreateDivisionRequest{AppID: app.ID, Name: "Root"})
	assert.NoError(t, err)

	d, err := svc.GetByID(ctx, app.ID, created.ID)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, d.ID)
}

func TestDivisionService_List(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_div_list?mode=memory&cache=shared&_fk=1")
	defer func() { _ = client.Close() }()

	app, err := client.App.Create().SetName("Test App").SetStatus(1).Save(context.Background())
	assert.NoError(t, err)

	repo := NewDivisionRepository(client)
	svc := NewDivisionService(repo)

	ctx := context.Background()
	_, _ = svc.Create(ctx, CreateDivisionRequest{AppID: app.ID, Name: "Root 1"})
	_, _ = svc.Create(ctx, CreateDivisionRequest{AppID: app.ID, Name: "Root 2"})

	divisions, err := svc.List(ctx, app.ID, nil, 50, 0)
	assert.NoError(t, err)
	assert.Len(t, divisions, 2)
}

func TestDivisionService_Descendants(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_div_desc?mode=memory&cache=shared&_fk=1")
	defer func() { _ = client.Close() }()

	app, err := client.App.Create().SetName("Test App").SetStatus(1).Save(context.Background())
	assert.NoError(t, err)

	repo := NewDivisionRepository(client)
	svc := NewDivisionService(repo)

	ctx := context.Background()
	root, _ := svc.Create(ctx, CreateDivisionRequest{AppID: app.ID, Name: "Root"})
	eng, _ := svc.Create(ctx, CreateDivisionRequest{AppID: app.ID, ParentID: &root.ID, Name: "Engineering"})
	_, _ = svc.Create(ctx, CreateDivisionRequest{AppID: app.ID, ParentID: &eng.ID, Name: "Backend"})
	_, _ = svc.Create(ctx, CreateDivisionRequest{AppID: app.ID, ParentID: &root.ID, Name: "Marketing"})

	descendants, err := svc.Descendants(ctx, app.ID, root.ID)
	assert.NoError(t, err)
	assert.Len(t, descendants, 3)
}

func TestDivisionService_Update(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_div_update?mode=memory&cache=shared&_fk=1")
	defer func() { _ = client.Close() }()

	app, err := client.App.Create().SetName("Test App").SetStatus(1).Save(context.Background())
	assert.NoError(t, err)

	repo := NewDivisionRepository(client)
	svc := NewDivisionService(repo)

	ctx := context.Background()
	d, err := svc.Create(ctx, CreateDivisionRequest{AppID: app.ID, Name: "Root"})
	assert.NoError(t, err)

	newName := "Corp Root"
	newStatus := int8(0)
	updated, err := svc.Update(ctx, app.ID, d.ID, UpdateDivisionRequest{
		Name:   &newName,
		Status: &newStatus,
	})
	assert.NoError(t, err)
	assert.Equal(t, newName, updated.Name)
	assert.Equal(t, newStatus, updated.Status)
}

func TestDivisionService_Move(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_div_move?mode=memory&cache=shared&_fk=1")
	defer func() { _ = client.Close() }()

	app, err := client.App.Create().SetName("Test App").SetStatus(1).Save(context.Background())
	assert.NoError(t, err)

	repo := NewDivisionRepository(client)
	svc := NewDivisionService(repo)

	ctx := context.Background()
	root1, _ := svc.Create(ctx, CreateDivisionRequest{AppID: app.ID, Name: "Root1"})
	root2, _ := svc.Create(ctx, CreateDivisionRequest{AppID: app.ID, Name: "Root2"})
	child, _ := svc.Create(ctx, CreateDivisionRequest{AppID: app.ID, ParentID: &root1.ID, Name: "Child"})

	moved, err := svc.Move(ctx, app.ID, child.ID, MoveDivisionRequest{ParentID: &root2.ID})
	assert.NoError(t, err)
	assert.NotNil(t, moved.ParentID)
	assert.Equal(t, root2.ID, *moved.ParentID)
	assert.Contains(t, moved.Path, root2.Path)
}

func TestDivisionService_Move_CycleDetected(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_div_cycle?mode=memory&cache=shared&_fk=1")
	defer func() { _ = client.Close() }()

	app, err := client.App.Create().SetName("Test App").SetStatus(1).Save(context.Background())
	assert.NoError(t, err)

	repo := NewDivisionRepository(client)
	svc := NewDivisionService(repo)

	ctx := context.Background()
	root, _ := svc.Create(ctx, CreateDivisionRequest{AppID: app.ID, Name: "Root"})
	child, _ := svc.Create(ctx, CreateDivisionRequest{AppID: app.ID, ParentID: &root.ID, Name: "Child"})

	// Attempt to move root under its own child
	_, err = svc.Move(ctx, app.ID, root.ID, MoveDivisionRequest{ParentID: &child.ID})
	assert.Error(t, err)
}

func TestDivisionService_Delete_WithChildren(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_div_del_children?mode=memory&cache=shared&_fk=1")
	defer func() { _ = client.Close() }()

	app, err := client.App.Create().SetName("Test App").SetStatus(1).Save(context.Background())
	assert.NoError(t, err)

	repo := NewDivisionRepository(client)
	svc := NewDivisionService(repo)

	ctx := context.Background()
	root, _ := svc.Create(ctx, CreateDivisionRequest{AppID: app.ID, Name: "Root"})
	_, _ = svc.Create(ctx, CreateDivisionRequest{AppID: app.ID, ParentID: &root.ID, Name: "Child"})

	err = svc.Delete(ctx, app.ID, root.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "children")
}

func TestDivisionService_Delete_OK(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_div_del_ok?mode=memory&cache=shared&_fk=1")
	defer func() { _ = client.Close() }()

	app, err := client.App.Create().SetName("Test App").SetStatus(1).Save(context.Background())
	assert.NoError(t, err)

	repo := NewDivisionRepository(client)
	svc := NewDivisionService(repo)

	ctx := context.Background()
	d, _ := svc.Create(ctx, CreateDivisionRequest{AppID: app.ID, Name: "Leaf"})

	err = svc.Delete(ctx, app.ID, d.ID)
	assert.NoError(t, err)

	_, err = svc.GetByID(ctx, app.ID, d.ID)
	assert.Error(t, err)
}
