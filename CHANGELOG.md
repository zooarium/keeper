# Changelog

All notable changes to keeper are documented in this file.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow [SemVer](https://semver.org).
Release with `make release VERSION=x.y.z` — rotates this file, commits, tags `vx.y.z`.

## [Unreleased]

### Changed
- `internal/db/client.go` SQLite DSN: enabled WAL journal mode and 5s busy timeout (`_journal_mode=WAL&_busy_timeout=5000`) for better write concurrency.
- Renamed `RoleUser` to `RoleAdmin` (`pkg/auth`, `internal/user`) — value unchanged (`0`), no JWT/DB impact. Re-vendored `keeper/pkg/auth` into squirrel, ant, camel.
- `POST /users/auth` login now resolves the user's roles from falcon (`GET /user-roles` on falcon's `internal-s2s` listener) before minting the JWT, and embeds them as `roles` (`pkg/auth.UserClaims.Roles`, omitted when empty). Fails closed: if falcon is unreachable or errors, login is rejected with `503` (`ErrRoleServiceUnavailable`) and never falls back to a stale/cached role set — falcon is now as critical a dependency as keeper's own DB. Keeper has no incoming JWT to forward at login time, so it self-signs a short-lived token with its own `AUTH.JWT_SECRET` (shared with falcon's) to call falcon. New `FALCON.BASE_URL`/`FALCON.TIMEOUT` config. `GET /ready` now also checks falcon reachability (was DB-only).
- `pkg/auth.JWTManager.Generate` takes optional trailing `roles ...auth.RoleAssignment` (name + falcon `service_id`/`app_id` scope, not just a plain name) — existing callers (guest tokens, impersonation) are unaffected. `UserClaims.Roles` changed type from `[]string` to `[]RoleAssignment` accordingly, so `Can()`'s sudo tenant-scope check (`fal_user_role.app_id`) has per-assignment scope to check against at request time. Requires re-vendoring `keeper/pkg/auth` into squirrel, ant, camel.
- falcon: `internal-s2s` listener's `ROUTES` allow-list gained `GET /user-roles`, for the login role-resolution call above.
- `FALCON.SERVICE_ID` set to `1` — keeper's fixed id in falcon's `fal_service` table, seeded by falcon's `20260729092300_seed_fal_service` migration (identical across all envs).

### Added
- `internal/policy.Can()`: RBAC Tier 1 (coarse CRUD + sudo bypass) and Tier 2 (field-level) enforcement, consuming the policy cache below. Checks a sudo bypass first (global, or tenant-scoped via the assignment's `app_id`), then falls back to a coarse resource/action permission match, unioned across all of a user's roles; pass a non-empty `field` for the Tier 2 check (a field-restricted permission never grants a different field, and a base permission never grants a restricted one). Ownership checks are deferred to Tier 3 (step 7.3).
- RBAC Tier 1 (coarse CRUD) enforcement wired into `internal/app` (`POST /apps`, `PUT /apps/{id}`) and `internal/user` (`POST /users`, `PUT /users/{id}`), replacing the hardcoded `claims.IsSysAdmin()` gates on those actions with `policy.Can()`.
- RBAC Tier 2 (field-level) enforcement for keeper's restricted fields: app's `manager_id`/`status` (`PUT /apps/{id}`) and user's `role` (`POST /users`, `PUT /users/{id}`, only checked when elevating to sysadmin/manager) — each now requires its own `<resource>.<action>.<field>` permission on top of the base action, rejecting the whole request (400/403) rather than silently dropping the field.
- `internal/policy`: Tier 1 (coarse CRUD) authorization cache — pulls falcon's role->permission export (`GET /services/{id}/permissions/map` on falcon's `internal-s2s` listener), compiles it into `map[roleName]RolePolicy`, and serves it from an in-memory TTL cache (`CACHE.POLICY_TTL`, default 60s), refreshing lazily on read past expiry. New `FALCON.SERVICE_ID` config (keeper's own id in falcon's `fal_service` table). Warmed eagerly at startup (non-fatal on failure); fails closed (empty map) if falcon is unreachable past the TTL. Enforcement (checking a request against this map) is a separate step.
- `RoleManager` role (`pkg/auth`, `internal/user`): a user granted access to one or more apps via `kpr_app.manager_id`, independent of their own tenant `app_id`. Sysadmin-only to assign (create/update user, and set/clear `manager_id` on `PUT /apps/{id}`).
- `kpr_app.manager_id` (nullable FK to `kpr_user`, indexed, `SetNull` on manager deletion): one manager per app, one manager may be assigned to many apps.
- `GET /apps/{id}` / `PUT /apps/{id}`: managers may access apps they're assigned to (in addition to sysadmin/own-tenant). `DELETE /apps/{id}` remains sysadmin/own-tenant only.
- `GET /apps`: managers see only their assigned apps (`AppService.ListByManager`).
- `GET /users?role=`: optional role filter, so a sysadmin can list `RoleManager` users to assign to an app.
- `POST /apps`: accepts `manager_id` to assign a manager at creation time (previously only settable via `PUT /apps/{id}`); create is already sysadmin-only so no extra gate needed.
- `kpr_app.currency` (required, NOT NULL, ISO 4217 code e.g. `INR`, max 3 chars, validated via `iso4217`): included in `App`/`PublicApp` responses and `POST`/`PUT /apps` payloads.
- `GET /managers` (`internal/user`): lists all users with the manager role across all apps, sysadmin only. Mounted top-level (not under `/users`) since managers span tenants; reuses `UserService.List`, wired into both the primary router and the secondary-listener mount hook.
- `PUT /apps/{id}`: changing `status` is now sysadmin only (same guard as `manager_id`), closing a gap where own-tenant users and assigned managers could activate/deactivate an app.

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
