# Hardware Requirements — keeper / squirrel / ant

Sizing notes for running all three services (+ their `atlas` migration containers) on
a single production host behind a reverse proxy (nginx). Based on measured behavior,
not estimates — see "How this was measured" below.

## What's running

Each service is one Go binary (CGO-enabled, embedded SQLite — no separate DB server)
built as a multi-stage Docker image (`golang:alpine` builder → `alpine:3.22` runtime).
No Redis, no message queue — caching is in-process (short-TTL map, see each service's
`pkg/cache`).

| service  | final image | binary size |
|----------|-------------|-------------|
| keeper   | 57.5MB      | 47MB        |
| squirrel | 55.7MB      | 45MB        |
| ant      | 59.7MB      | 49MB        |

## Measured resource usage

Idle RSS (fresh container, first-boot migration/seed done):

| service  | idle RSS |
|----------|----------|
| keeper   | ~33MB (includes one-time seed insert) |
| squirrel | ~9MB   |
| ant      | ~12MB  |

All three idle together: **~55MB total application memory.**

Under load (`hey -c 50 -z 5s` against a cheap endpoint): RSS rose to ~30MB for the
loaded service; the primary listener's rate limiter (`httprate`, 100 req/min/IP
default) throttles well before memory becomes a concern.

CPU is negligible at idle; under concurrent load each service uses a small fraction
of a core per request — cheap JSON handlers over SQLite, no heavy compute.

## Recommended minimums

**Absolute floor** (all 3 services + nginx, low traffic): **1 vCPU / 1GB RAM / 5GB disk.**

**Recommended** (headroom for bursts, `atlas` migration runs, log growth, and the
Postgres migration the codebase is already designed for as row counts grow):
**2 vCPU / 2GB RAM / 10GB disk.**

Per-service limits are now enforced in each `docker-compose.yml` (`mem_limit: 256m`,
`cpus: "1.0"`) — generous headroom over measured idle usage (8-30x) without letting
one service starve the others.

## Build vs runtime footprint

Don't build images on a resource-constrained prod box: the builder stage pulls
`golang:1.26-alpine` (~241MB) plus `build-base` (gcc, needed for CGO/sqlite) and
briefly needs real CPU to compile. Build in CI or on a dev machine and ship only the
final ~55-60MB runtime image to production.

## Database

SQLite, file-based, currently a few hundred KB in dev. No separate DB server needed
at current scale. `DATABASE.DRIVER`/`DB.DRIVER` is already configurable to `postgres`
per each service's engineering constraints — budget +512MB RAM minimum for a small
Postgres instance whenever that migration happens.

## Logs

Each service writes structured JSON to **stdout** and to `./log/api.log` (bind-mounted).
Two rotation layers are needed since they're independent log streams:

1. **Docker-captured stdout** — `json-file` driver, `max-size: 10m`, `max-file: 3`
   (~30MB cap per service), set directly in each `docker-compose.yml`.
2. **The bind-mounted `./log/*.log` file** — not covered by Docker's log driver.
   Each service ships a `logrotate.conf` (daily, 7 rotations, compressed,
   `copytruncate` — safe because none of the services reopen their log file on
   `SIGHUP`). Install on the prod host:
   ```
   cp logrotate.conf /etc/logrotate.d/<service>-api
   ```
   Adjust the path glob inside to the actual deployment directory first.

## How this was measured

`docker run` each service's built image standalone (idle RSS via `docker stats`),
then `hey -c 50 -z 5s` against a cheap endpoint while watching `docker stats` for the
peak. Image/binary sizes are `docker images` / `ls -la` inside the container. Not
extrapolated from `go.mod` or code inspection.
