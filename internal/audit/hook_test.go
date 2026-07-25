package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"keeper/ent/enttest"

	_ "github.com/mattn/go-sqlite3"
)

func TestHookLogsCreateAndUpdate(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:audit_hook?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	var buf strings.Builder
	client.Use(Hook(slog.New(slog.NewJSONHandler(&buf, nil))))

	ctx := context.Background()
	app, err := client.App.Create().SetName("acme").Save(ctx)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := client.App.UpdateOne(app).SetName("acme-2").Save(ctx); err != nil {
		t.Fatalf("update: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 log lines, got %d: %q", len(lines), buf.String())
	}

	var created map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &created); err != nil {
		t.Fatalf("unmarshal create line: %v", err)
	}
	if created["action"] != "OpCreate" || created["entity_type"] != "App" {
		t.Fatalf("unexpected create log line: %v", created)
	}
	if int(created["entity_id"].(float64)) != app.ID {
		t.Fatalf("want entity_id %d, got %v", app.ID, created["entity_id"])
	}

	var updated map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &updated); err != nil {
		t.Fatalf("unmarshal update line: %v", err)
	}
	if updated["action"] != "OpUpdateOne" || updated["entity_type"] != "App" {
		t.Fatalf("unexpected update log line: %v", updated)
	}
	if int(updated["entity_id"].(float64)) != app.ID {
		t.Fatalf("want entity_id %d, got %v", app.ID, updated["entity_id"])
	}
}
