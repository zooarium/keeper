# Production Environment Setup — Docker (keeper)

This is the Docker-based production path (build the image, run via
`docker-compose.yml`). For the alternative bare-binary + systemd path, see
`docs/DEPLOYMENT_WITHOUT_DOCKER.md`.

## 1. Server sizing

See `docs/HARDWARE_REQUIREMENTS.md` for the full measured breakdown (covers
keeper + squirrel + ant together, since they typically share one small box).

- Absolute floor (all 3 services + nginx): 1 vCPU / 1GB RAM / 5GB disk.
- Recommended: 2 vCPU / 2GB RAM / 10GB disk.

## 2. Install Docker

Install Docker Engine + the Compose plugin via your distro's package manager
or Docker's official install instructions (e.g. `docker.io`/`docker-ce` +
`docker-compose-plugin`). Confirm with:

```bash
docker compose version
```

## 3. Deploy the repo

```bash
git clone <keeper-repo> /opt/keeper   # or copy the working tree
cd /opt/keeper
```

Build on a separate CI/dev machine and ship only the final image if the prod
box is resource-constrained — the builder stage pulls `golang:alpine`
(~241MB) plus `build-base`, which is wasted disk/CPU on a small server.

## 4. Inject real secrets

`config/config.yaml` ships with **placeholder** secrets only — production
must override via env (viper prefix `KEEPER_`, `.` → `_`). Required:

| Config key | Env var | Notes |
|---|---|---|
| `AUTH.JWT_SECRET` | `KEEPER_AUTH_JWT_SECRET` | primary JWT signing key |
| `AUTH.GUEST_JWT_SECRET` | `KEEPER_AUTH_GUEST_JWT_SECRET` | separate secret — guest tokens must stay useless on normal surfaces |
| `AUTH.IMPERSONATION_JWT_SECRET` | `KEEPER_AUTH_IMPERSONATION_JWT_SECRET` | separate secret — only services explicitly configured with it accept these |
| `SEED.ADMIN_PASSWORD` | `KEEPER_SEED_ADMIN_PASSWORD` | change from the `admin` default before first boot |

Set these in `docker-compose.yml`'s `environment:` block (or an untracked
`.env`/override file) — never commit real secrets.

## 5. Reverse proxy

Put nginx (or equivalent) in front for TLS termination and routing to the
container's published port (`8080` by default). Secondary listeners, if
enabled in `config/config.yaml`, need their own published port too (skip for
internal service-to-service listeners — network isolation is the guard).

## 6. Build and run

```bash
make build && make up
```

Compose already sets `restart: always`, `mem_limit: 256m`, `cpus: "1.0"`,
and `json-file` log rotation (10MB × 3) on the `api` service.

## 7. Log rotation for the bind-mounted file

Docker's log driver only rotates its own stdout copy. The bind-mounted
`./log/api.log` needs the host-level `logrotate.conf` shipped in this repo:

```bash
cp logrotate.conf /etc/logrotate.d/keeper-api
```

Adjust the path glob inside first to match the real deployment directory.
See `docs/LOGGING.md` for the full picture (why two log copies exist, why
`copytruncate`).

## 8. Verify

```bash
curl http://localhost:8080/health
docker compose ps
docker stats --no-stream
```

## 9. Updating

```bash
git pull
make build && make up
```

Auto-migration runs on startup — no separate migration step needed for
routine updates.

## 10. Backup & Restore

`make backup` runs `sqlite3 .backup` against `data/keeper.db` (online-safe —
safe to run while the service is up) into `data/backups/`, timestamped, and
prunes anything older than 14 days. Schedule it with cron:

```bash
0 3 * * * cd /opt/keeper && make backup >> log/backup.log 2>&1
```

To restore: `make restore FILE=data/backups/keeper-20260101-030000.db`. This
stops the container, snapshots the current live DB into `data/backups/`
first (so a bad restore is itself recoverable), copies the chosen backup
over `data/keeper.db`, then restarts.

This covers SQLite only. Postgres backup (`pg_dump`/WAL archiving) is
deferred to the documented future migration.
