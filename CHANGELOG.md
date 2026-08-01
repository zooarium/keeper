# Changelog

All notable changes to keeper are documented in this file.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow [SemVer](https://semver.org).
Release with `make release VERSION=x.y.z` — rotates this file, commits, tags `vx.y.z`.

## [Unreleased]

### Changed
- `GET /apps` `own`-scope branch now merges in apps the caller manages via `manager_id` (`AppService.ListByManager`, previously wired but unused), matching the manager fallback already enforced on `GET`/`PUT /apps/{id}`. Previously a manager's assigned apps outside their own tenant were invisible in the list.

### Removed
- `kpr_user.role` DB column, dropped manually (`ALTER TABLE ... DROP COLUMN`) — ent auto-migrate doesn't drop columns, so this was left live after the Go-level `Role` field removal until now.

### Changed
- `internal/db/client.go` SQLite DSN: enabled WAL journal mode and 5s busy timeout (`_journal_mode=WAL&_busy_timeout=5000`) for better write concurrency.
- `POST /users/auth` login now resolves the user's roles from falcon (`GET /user-roles` on falcon's `internal-s2s` listener) before minting the JWT, and embeds them as `roles` (`pkg/auth.UserClaims.Roles`, omitted when empty). Fails closed: if falcon is unreachable or errors, login is rejected with `503` (`ErrRoleServiceUnavailable`) and never falls back to a stale/cached role set — falcon is now as critical a dependency as keeper's own DB. Keeper has no incoming JWT to forward at login time, so it self-signs a short-lived token with its own `AUTH.JWT_SECRET` (shared with falcon's) to call falcon. New `FALCON.BASE_URL`/`FALCON.TIMEOUT` config. `GET /ready` now also checks falcon reachability (was DB-only).
- `pkg/auth.JWTManager.Generate` takes optional trailing `roles ...auth.RoleAssignment` (name + falcon `service_id`/`app_id` scope, not just a plain name) — existing callers (guest tokens, impersonation) are unaffected. `UserClaims.Roles` changed type from `[]string` to `[]RoleAssignment` accordingly, so `Can()`'s sudo tenant-scope check (`fal_user_role.app_id`) has per-assignment scope to check against at request time. Requires re-vendoring `keeper/pkg/auth` into squirrel, ant, camel.
- falcon: `internal-s2s` listener's `ROUTES` allow-list gained `GET /user-roles`, for the login role-resolution call above.
- `FALCON.SERVICE_ID` set to `1` — keeper's fixed id in falcon's `fal_service` table, seeded by falcon's `20260729092300_seed_fal_service` migration (identical across all envs).
- RBAC Tier 3 (row-level scope) now enforced via `policy.Scope()` on `GET /apps`, `GET /users`, `GET /users/{id}`, `PUT /users/{id}`, `DELETE /users/{id}`, and `POST /users` — each now requires a resolved `own`/`any` grant on the relevant resource+action; `own` scopes list/read/update/delete to the caller's own tenant (`app_id`), `any` (sysadmin sudo, or a permission with `scope=any`) is unrestricted. `GET`/`PUT /apps/{id}` and `DELETE /apps/{id}`'s cross-tenant check switched from a `policy.Can()` coarse-permission check (which doesn't consider `scope` at all and so would have wrongly granted cross-tenant access once `app.read` is `own`-scoped) to `policy.Scope()`. This closes the interim gap noted below and supersedes it.
- RBAC Tier 3 (row-level scope) extended to all division endpoints (`POST /divisions`, `GET /divisions`, `GET /divisions/{id}`, `GET /divisions/{id}/descendants`, `PUT /divisions/{id}`, `PUT /divisions/{id}/move`, `DELETE /divisions/{id}`) via `policy.Scope()` on a new `division` resource (actions `create`/`read`/`update`/`move`/`delete`), replacing the previous hardcoded caller-app-only scoping (no permission gate at all) with the same own/any resolution used by `app`/`user`. No repository changes needed — `internal/division`'s repository already took an `appID` scope parameter (0 = unscoped) mirroring `internal/user`'s existing pattern.
- RBAC Tier 3 (row-level scope) wired into `internal/guestkey` (`POST /guest-keys`, `GET /guest-keys`, `GET /guest-keys/{id}`, `PUT /guest-keys/{id}`, `DELETE /guest-keys/{id}`) via `policy.Scope()` on a new `guestkey` resource (actions `create`/`read`/`update`/`delete`), replacing the unenforced `TODO(falcon)` stubs — previously any authenticated caller could read/update/delete any tenant's guest key with no check at all. `internal/guestkey`'s repository/service `GetByID`/`Update`/`Delete` now take an `appID` scope parameter (0 = unscoped), mirroring `division`/`user`; a cross-tenant id under an `own` scope 404s rather than leaking existence via a 403. `internal/guestkey/service_test.go`'s `mockRepo` and `internal/platform/http/router_test.go`'s `NewGuestKeyHandler` call site updated for the new signatures; new `internal/guestkey/handler_test.go` covers own/any scoping, cross-tenant 404s, and no-permission 403s, mirroring `division`'s handler test suite.
- ~~**Interim, pending full falcon consumer enforcement:** `GET /apps`, `GET /users`, `GET /users/{id}`, `PUT /users/{id}` now accept any authenticated caller — the previous local sysadmin/own-app/manager tenant-scoping on these was removed along with the local role identity and not yet replaced with falcon-resolved scoping (tracked as `TODO(falcon)` in the affected handlers). `POST`/`PUT /apps`, `POST`/`PUT /users`, `DELETE /apps/{id}`, `DELETE /users/{id}`, and per-record ownership on `GET`/`PUT /apps/{id}` are unaffected (still enforced via `policy.Can()` / `manager_id` ownership).~~ (resolved above)

### Added
- `internal/policy.Scope()`: RBAC Tier 3 (row-level scope) resolver. Answers `own`/`any` for a resource+action, unioned across a user's roles (any "any" grant wins over an "own" one) — global sudo and a permission row with `scope=any` (or unset) resolve to `any`; a tenant-scoped sudo assignment or a permission row with `scope=own` resolves to `own`. Never inspects the target record itself — callers apply the resulting scope as an `app_id` filter (or equivalent) at the query layer.
- `internal/policy.Can()`: RBAC Tier 1 (coarse CRUD + sudo bypass) and Tier 2 (field-level) enforcement, consuming the policy cache below. Checks a sudo bypass first (global, or tenant-scoped via the assignment's `app_id`), then falls back to a coarse resource/action permission match, unioned across all of a user's roles; pass a non-empty `field` for the Tier 2 check (a field-restricted permission never grants a different field, and a base permission never grants a restricted one). Ownership checks are deferred to Tier 3 (step 7.3).
- RBAC Tier 1 (coarse CRUD) enforcement wired into `internal/app` (`POST /apps`, `PUT /apps/{id}`) and `internal/user` (`POST /users`, `PUT /users/{id}`), replacing the previous hardcoded sysadmin-only gates on those actions with `policy.Can()`.
- RBAC Tier 2 (field-level) enforcement for app's restricted fields `manager_id`/`status` (`PUT /apps/{id}`) — each now requires its own `<resource>.<action>.<field>` permission on top of the base action, rejecting the whole request (400/403) rather than silently dropping the field.
- `internal/policy`: Tier 1 (coarse CRUD) authorization cache — pulls falcon's role->permission export (`GET /services/{id}/permissions/map` on falcon's `internal-s2s` listener), compiles it into `map[roleName]RolePolicy`, and serves it from an in-memory TTL cache (`CACHE.POLICY_TTL`, default 60s), refreshing lazily on read past expiry. New `FALCON.SERVICE_ID` config (keeper's own id in falcon's `fal_service` table). Warmed eagerly at startup (non-fatal on failure); fails closed (empty map) if falcon is unreachable past the TTL. Enforcement (checking a request against this map) is a separate step.
- `kpr_app.manager_id` (nullable FK to `kpr_user`, indexed, `SetNull` on manager deletion): one manager per app, one manager may be assigned to many apps. `GET /apps/{id}` / `PUT /apps/{id}` grant access to the assigned manager (Tier 3 ownership fallback, in addition to sysadmin/own-tenant).
- `POST /apps`: accepts `manager_id` to assign a manager at creation time (previously only settable via `PUT /apps/{id}`); create already requires `app.create` so no extra gate needed.
- `kpr_app.currency` (required, NOT NULL, ISO 4217 code e.g. `INR`, max 3 chars, validated via `iso4217`): included in `App`/`PublicApp` responses and `POST`/`PUT /apps` payloads.
- `PUT /apps/{id}`: changing `status` now requires `app.update.status` (same Tier 2 guard as `manager_id`), closing a gap where own-tenant users and assigned managers could activate/deactivate an app.
- `DELETE /apps/{id}` now requires `app.delete` (sysadmin sudo or own-tenant; no manager fallback — managers cannot delete apps), and `DELETE /users/{id}` now requires `user.delete` — both previously carried a `TODO(falcon)` comment but called no `policy.Can()` at all, so any authenticated caller could delete any app or user.
- RBAC coarse-gate (Tier 1 only, no ownership scoping) enforcement wired into `internal/impersonation`: `POST /impersonations` and `POST /impersonations/{id}/revoke` now require `impersonation.create`; `GET /impersonations`, `GET /impersonations/{id}`, and `GET /impersonations/services` now require `impersonation.read`. Replaces the previous `requireSysAdmin` helper, which only checked authentication and carried a `TODO(falcon)` comment.
- Impersonation privilege-escalation guard re-added: `POST /impersonations` now refuses to start a session against a target user holding any sudo (sysadmin-tier) falcon role, resolved fresh per-request via `RoleResolver.ResolveRoles` (the target isn't the caller, so this can't come off the request JWT) and checked against the cached policy map's `IsSudo`. This had been removed with the local `kpr_user.role` column and left as a `TODO(falcon)`, so nothing previously stopped a lower-privileged caller from impersonating a sysadmin.

### Removed
- `pkg/auth.Role*` constants and `claims.IsSysAdmin()`/`IsManager()`/`IsGuest()`/`IsAdmin()` helpers — permissioning is now resolved entirely via falcon (`policy.Can()`), not a local role identity on the claims. Required re-vendoring `keeper/pkg/auth` into squirrel, ant, camel.
- `kpr_user`'s exposed role concept: `User.Role`, `CreateUserRequest.Role`, `UpdateUserRequest.Role`, the `GET /users?role=` filter, and the `GET /managers` endpoint. Role/permission assignment for users now lives entirely in falcon.

### Fixed
- `internal/division` `CreateDivision`, `internal/app`/`internal/user`/`internal/platform/http` test mocks, and `internal/guestkey` service tests updated for the `Role*`/`Is*()` removal above (stale signatures and assertions left over from the strip).
- `internal/platform/http/router_test.go`'s `TestRouterAuthentication_ValidToken` built every handler with a `nil` `*policy.Store`, so its one authenticated request nil-pointer panicked inside `policy.Scope()`; chi's `Recoverer` middleware silently turned it into a `500` and the test's `status != 401` assertion still passed. Now builds a real `policy.NewStoreFromPolicies` store with a sudo role and mints the token with a matching `RoleAssignment`, so the request exercises the actual gate and returns `200`.

## [0.0.4] - 2026-07-25

### Added
- Request ID middleware (`internal/platform/http/requestlog.go`): `chi/middleware.RequestID` first in the chain on primary and secondary routers, structured JSON request-completion log (method/path/status/duration/remote addr) replacing chi's plain-text logger, `X-Request-Id` echoed on every response.
- `GET /ready`: DB-ping readiness check (`internal/db/client.go` `Ping`), separate from the pure-liveness `GET /health`; 503 on unreachable DB. Registered on primary and secondary listeners.
- `make backup` / `make restore`: online-safe SQLite `.backup` to `data/backups/` (14-day retention) and manual restore from a backup file. Documented in `docs/DEPLOYMENT_USING_DOCKER.md`.
- `pkg/httpclient`: shared outbound `*http.Client` with retry (2 attempts, backoff+jitter, on network errors/5xx/429) and circuit breaker (`sony/gobreaker`), for any future keeper-side outbound call. Vendored into squirrel/ant.
- `internal/audit`: Ent client-level mutation hook logging one JSON line per create/update/delete to a dedicated `log/audit.log` (separate from `api.log`) — actor/app/division from JWT claims + mutation, no DB table. `make audit-logs` to tail it.

## [0.0.3] - 2026-07-25

### Added
- `pkg/s2s`: shared REST client for service-to-service calls (envelope decode, caller-owned retry/fail policy).
- `pkg/cache`: shared in-memory TTL cache with lazy expiry, extracted for reuse across services.
- `docker-compose.yml`: `mem_limit`/`cpus` caps and `json-file` log rotation (max-size 10m, max-file 3) on the `api` service.
- `logrotate.conf`: host-level rotation for the bind-mounted `./log/*.log` files (daily, 7 rotations, copytruncate).
- `docs/HARDWARE_REQUIREMENTS.md`: production sizing guidance for keeper/squirrel/ant.
- `docs/LOGGING.md`: logging setup and rotation reference.
- `docs/DEPLOYMENT_USING_DOCKER.md`: Docker-based production setup guide.

### Changed
- `pkg/auth` `NewHTTPRevocationChecker` now built on `pkg/s2s` + `pkg/cache` instead of a hand-rolled HTTP client and map+mutex cache. Behavior (fail-open on transport error) unchanged.
- `DEPLOYMENT.md` moved to `docs/DEPLOYMENT_WITHOUT_DOCKER.md` (bare-binary + systemd path), alongside the new Docker path doc.

## [0.0.2] - 2026-07-11

### Added
- Version in `GET /health` response, read from CHANGELOG.md.
- `make version` target; version shown in `make info`.

## [0.0.1] - 2026-07-11

### Added
- Changelog and `make release` versioning workflow.
