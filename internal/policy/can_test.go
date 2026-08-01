package policy

import (
	"context"
	"testing"
	"time"

	"keeper/pkg/auth"
	"keeper/pkg/httpclient"
)

// newTestStore warms a Store from the given rows via a throwaway falcon
// stand-in, mirroring the Worked Example in falcon/docs/rbac-plan.md.
func newTestStore(t *testing.T, rows []Row) *Store {
	t.Helper()
	srv := newTestServer(t, rows)
	t.Cleanup(srv.Close)

	jwt := auth.NewJWTManager("secret", time.Hour)
	client := httpclient.New(httpclient.Config{Timeout: time.Second, Name: "test-can"})
	store := NewStore(NewFetcher(client, srv.URL, 1, jwt), time.Hour)
	if err := store.Warm(context.Background()); err != nil {
		t.Fatalf("warm: %v", err)
	}
	return store
}

func appIDPtr(id int) *int { return &id }

// worked-example policy export: falcon/docs/rbac-plan.md "Worked Example".
var workedExampleRows = []Row{
	{Role: "sysadmin", IsSudo: true},
	{Role: "admin", Resource: strPtr("app"), Action: strPtr("update"), Scope: strPtr("any")},
	{Role: "ant_manager", Resource: strPtr("order"), Action: strPtr("read"), Scope: strPtr("own")},
	{Role: "ant_manager", Resource: strPtr("order"), Action: strPtr("update"), Scope: strPtr("own")},
	{Role: "ant_admin", IsSudo: true},
	{Role: "billing_manager", Resource: strPtr("pricing"), Action: strPtr("update"), Scope: strPtr("any")},
}

func TestCan_WorkedExample(t *testing.T) {
	store := newTestStore(t, workedExampleRows)
	ctx := context.Background()

	tests := []struct {
		name     string
		roles    []auth.RoleAssignment
		appID    int
		resource string
		action   string
		want     bool
	}{
		{
			name:     "global sudo bypasses everything",
			roles:    []auth.RoleAssignment{{Name: "sysadmin"}}, // user 501, app_id NULL
			appID:    999,
			resource: "anything",
			action:   "anything",
			want:     true,
		},
		{
			name:     "field-restricted admin still passes coarse check",
			roles:    []auth.RoleAssignment{{Name: "admin"}}, // user 502
			appID:    1,
			resource: "app",
			action:   "update",
			want:     true, // field/ownership deferred to Tier 2/3
		},
		{
			name:     "admin has no grant outside its permission",
			roles:    []auth.RoleAssignment{{Name: "admin"}},
			appID:    1,
			resource: "app",
			action:   "delete",
			want:     false,
		},
		{
			name:     "ownership-scoped manager passes coarse action check",
			roles:    []auth.RoleAssignment{{Name: "ant_manager"}}, // user 503
			appID:    1,
			resource: "order",
			action:   "read",
			want:     true, // ownership filter itself is Tier 3, not Can()
		},
		{
			name:     "manager has no grant on unrelated resource",
			roles:    []auth.RoleAssignment{{Name: "ant_manager"}},
			appID:    1,
			resource: "pricing",
			action:   "update",
			want:     false,
		},
		{
			name:     "service-scoped sudo (ant_admin) bypasses within its service",
			roles:    []auth.RoleAssignment{{Name: "ant_admin"}}, // user 504
			appID:    1,
			resource: "order",
			action:   "delete",
			want:     true,
		},
		{
			name:     "single-permission role matches its one grant",
			roles:    []auth.RoleAssignment{{Name: "billing_manager"}}, // user 505
			appID:    1,
			resource: "pricing",
			action:   "update",
			want:     true,
		},
		{
			name:     "single-permission role rejects everything else",
			roles:    []auth.RoleAssignment{{Name: "billing_manager"}},
			appID:    1,
			resource: "app",
			action:   "update",
			want:     false,
		},
		{
			name:     "app-owner sudo passes for its own tenant",
			roles:    []auth.RoleAssignment{{Name: "sysadmin", AppID: appIDPtr(42)}}, // user 506
			appID:    42,
			resource: "anything",
			action:   "anything",
			want:     true,
		},
		{
			name:     "app-owner sudo denied outside its tenant",
			roles:    []auth.RoleAssignment{{Name: "sysadmin", AppID: appIDPtr(42)}},
			appID:    99,
			resource: "anything",
			action:   "anything",
			want:     false,
		},
		{
			name: "union across multiple roles: either match passes",
			roles: []auth.RoleAssignment{
				{Name: "billing_manager"},
				{Name: "ant_manager"},
			},
			appID:    1,
			resource: "order",
			action:   "update",
			want:     true,
		},
		{
			name:     "role absent from local policy map (different service) grants nothing",
			roles:    []auth.RoleAssignment{{Name: "unknown_role"}},
			appID:    1,
			resource: "app",
			action:   "update",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &auth.UserClaims{Roles: tt.roles}
			got := Can(ctx, store, claims, tt.appID, tt.resource, tt.action, "")
			if got != tt.want {
				t.Fatalf("Can() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCan_Tier2FieldRestriction covers the "admin updates their own app,
// except status/manager_id" requirement (rbac-plan.md Worked Example #3):
// admin (role id 2) only has permission 10 (app.update, base fields, no
// Field set) — not 11 (app.update.status) or 12 (app.update.manager_id).
func TestCan_Tier2FieldRestriction(t *testing.T) {
	store := newTestStore(t, workedExampleRows)
	ctx := context.Background()
	adminClaims := &auth.UserClaims{Roles: []auth.RoleAssignment{{Name: "admin"}}} // user 502

	tests := []struct {
		name  string
		field string
		want  bool
	}{
		{"base app.update passes (field-less Tier 1 check)", "", true},
		{"status field rejected: no matching permission row", "status", false},
		{"manager_id field rejected: no matching permission row", "manager_id", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Can(ctx, store, adminClaims, 1, "app", "update", tt.field)
			if got != tt.want {
				t.Fatalf("Can(field=%q) = %v, want %v", tt.field, got, tt.want)
			}
		})
	}

	t.Run("sudo still bypasses field restriction", func(t *testing.T) {
		sysadminClaims := &auth.UserClaims{Roles: []auth.RoleAssignment{{Name: "sysadmin"}}}
		if !Can(ctx, store, sysadminClaims, 1, "app", "update", "status") {
			t.Fatal("Can() = false, want true (sudo bypasses Tier 2 too)")
		}
	})
}

// scopeRows carries one own-scoped and one any-scoped permission on the same
// resource+action, plus a tenant-scoped sudo, to exercise every Scope()
// branch.
var scopeRows = []Row{
	{Role: "sysadmin", IsSudo: true},
	{Role: "tenant_sudo", IsSudo: true},
	{Role: "member", Resource: strPtr("user"), Action: strPtr("read"), Scope: strPtr("own")},
	{Role: "auditor", Resource: strPtr("user"), Action: strPtr("read"), Scope: strPtr("any")},
	{Role: "legacy", Resource: strPtr("user"), Action: strPtr("read")}, // no Scope set
}

func TestScope(t *testing.T) {
	store := newTestStore(t, scopeRows)
	ctx := context.Background()

	tests := []struct {
		name      string
		roles     []auth.RoleAssignment
		wantScope string
		wantOK    bool
	}{
		{
			name:      "global sudo grants any regardless of resource",
			roles:     []auth.RoleAssignment{{Name: "sysadmin"}},
			wantScope: "any",
			wantOK:    true,
		},
		{
			name:      "tenant-scoped sudo grants own",
			roles:     []auth.RoleAssignment{{Name: "tenant_sudo", AppID: appIDPtr(1)}},
			wantScope: "own",
			wantOK:    true,
		},
		{
			name:      "own-scoped permission grants own",
			roles:     []auth.RoleAssignment{{Name: "member"}},
			wantScope: "own",
			wantOK:    true,
		},
		{
			name:      "any-scoped permission grants any",
			roles:     []auth.RoleAssignment{{Name: "auditor"}},
			wantScope: "any",
			wantOK:    true,
		},
		{
			name:      "unset scope defaults to any",
			roles:     []auth.RoleAssignment{{Name: "legacy"}},
			wantScope: "any",
			wantOK:    true,
		},
		{
			name:      "own and any across roles: any wins",
			roles:     []auth.RoleAssignment{{Name: "member"}, {Name: "auditor"}},
			wantScope: "any",
			wantOK:    true,
		},
		{
			name:      "no matching permission denies",
			roles:     []auth.RoleAssignment{{Name: "unknown_role"}},
			wantScope: "",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &auth.UserClaims{Roles: tt.roles}
			gotScope, gotOK := Scope(ctx, store, claims, "user", "read")
			if gotScope != tt.wantScope || gotOK != tt.wantOK {
				t.Fatalf("Scope() = (%q, %v), want (%q, %v)", gotScope, gotOK, tt.wantScope, tt.wantOK)
			}
		})
	}
}
