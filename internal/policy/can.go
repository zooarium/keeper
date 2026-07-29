package policy

import (
	"context"

	"keeper/pkg/auth"
)

// assignment pairs one of a user's falcon-resolved roles with its compiled
// policy and per-assignment tenant scope, ready for a Can() check.
type assignment struct {
	RolePolicy
	AppID *int
}

// userRoleAssignments resolves claims.Roles against the locally-cached
// policy map. Falcon's export (behind Store) is already scoped to this
// service's FALCON_SERVICE_ID, so a role name absent from policies belongs
// to a different service and is silently skipped here — no separate
// per-assignment service filter is needed on the consumer side.
func userRoleAssignments(claims *auth.UserClaims, policies map[string]RolePolicy) []assignment {
	assignments := make([]assignment, 0, len(claims.Roles))
	for _, r := range claims.Roles {
		rp, ok := policies[r.Name]
		if !ok {
			continue
		}
		assignments = append(assignments, assignment{RolePolicy: rp, AppID: r.AppID})
	}
	return assignments
}

// hasPermission reports whether any permission grants resource+action.
// field == "" is the Tier 1 coarse check: it ignores the permission's own
// Field, so a field-restricted permission (e.g. admin's app.update on base
// fields only) still grants the coarse action. field != "" is the Tier 2
// check (step 7.2): it additionally requires an exact Field match — a
// permission scoped to one field never grants a different field, and a
// field-less (base) permission never grants a restricted field.
func hasPermission(perms []Permission, resource, action, field string) bool {
	for _, p := range perms {
		if p.Resource != resource || p.Action != action {
			continue
		}
		if field == "" || p.Field == field {
			return true
		}
	}
	return false
}

// Can reports whether the user identified by claims is authorized for
// action on resource, scoped to appID (the target record's tenant — usually
// but not always claims.AppID, e.g. a sysadmin acting on another tenant).
// Pass field == "" for the Tier 1 base-action check; pass a restricted
// field name (e.g. "status") for the Tier 2 field-level check (step 7.2) —
// sudo still bypasses both, since it never consults the permission map.
//
// Tenant isolation and the sudo bypass are orthogonal gates and both must
// pass: a sudo assignment scoped to one app (AppID set) only bypasses the
// permission check for that app, never across tenants.
func Can(ctx context.Context, store *Store, claims *auth.UserClaims, appID int, resource, action, field string) bool {
	assignments := userRoleAssignments(claims, store.Policies(ctx))

	for _, a := range assignments {
		if !a.IsSudo {
			continue
		}
		if a.AppID == nil || *a.AppID == appID {
			return true
		}
	}
	for _, a := range assignments {
		if hasPermission(a.Permissions, resource, action, field) {
			return true
		}
	}
	return false
}
