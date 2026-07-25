# Changelog

All notable changes to keeper are documented in this file.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow [SemVer](https://semver.org).
Release with `make release VERSION=x.y.z` — rotates this file, commits, tags `vx.y.z`.

## [Unreleased]

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
