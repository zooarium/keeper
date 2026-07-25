# Changelog

All notable changes to keeper are documented in this file.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow [SemVer](https://semver.org).
Release with `make release VERSION=x.y.z` — rotates this file, commits, tags `vx.y.z`.

## [Unreleased]

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
