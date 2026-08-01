# Keeper

A microservice providing functionality of authentication and authorisation.

# Architecture

## Go packages

- `ent` (https://github.com/ent/ent) ORM
- `chi` (https://github.com/go-chi/chi) for routing
- `testify` (https://github.com/stretchr/testify) for writing and running unit tests
- `viper` (https://github.com/spf13/viper) manage multiple environments i.e dev, test, CAT, prod configurations
- `slog` go standard library for logging
- `swag` (https://github.com/swaggo/swag) generate RESTful API documentation.
- `golangci-lint` (https://github.com/golangci/golangci-lint) linter
- `jwt` (https://github.com/golang-jwt/jwt) implementation of JSON Web Tokens (JWT)
- `cors` (https://github.com/go-chi/cors) CORS net/http middleware for Go
- `httprate` (https://github.com/go-chi/httprate) net/http rate limiter middleware
- `validator` (https://github.com/go-playground/validator) field validation, including Cross Field, Cross Struct, Map, Slice and Array diving

## Directory structure

```
/cmd/api/main.go
/internal/
  user/
    handler.go
    service.go
    repository.go
    model.go
  platform/
    http/
    middleware.go
    response.go
    router.go
  db/
    client.go
/pkg/
  logger/
  config/
```

## Code architecture

### Use directional dependencies:
HTTP → Service → Repository

#### Handler (Delivery Layer):
- Only HTTP concerns
- No business logic
```
type UserHandler struct {
    svc UserService
}
```

#### Service (Business Logic):
- Pure Go logic
- No HTTP, no SQL
```
type UserService interface {
    Create(ctx context.Context, u User) error
}
```

#### Repository (Persistence)
- DB logic only
- Implements interfaces


## Design patterns

- Dependancy injection (DI)
- Interface Segregation (Very Important in Go)
```
type UserWriter interface {
    Save(ctx context.Context, u User) error
}

type UserReader interface {
    FindByID(ctx context.Context, id string) (User, error)
}
```
- Error Handling Pattern (No Exceptions)
Sentinel + Wrapped Errors
```
var ErrUserNotFound = errors.New("user not found")

if err != nil {
    return fmt.Errorf("create user: %w", err)
}
```
Translate errors at the boundary (HTTP)
```
if errors.Is(err, ErrUserNotFound) {
    http.Error(w, "not found", http.StatusNotFound)
}
```
- Context Propagation (Mandatory)
```
func (s *service) Create(ctx context.Context, u User) error
```

## Requirement

- Go v1.26
- SQLite v3.51.2

## Development

The project uses Docker and a Makefile for development.

- `make all`: Run the full pipeline (fmt, vet, lint, test, swag, build, up).
- `make build`: Build the Docker images.
- `make up`: Start the containers in the background.
- `make down`: Stop and remove the containers.
- `make restart`: Restart the services.
- `make logs`: Follow the container logs.
- `make ps`: List the running containers.
- `make deps-upgrade`: Update all Go dependencies to their latest versions and run tests.
- `make go-upgrade version=1.x`: Upgrade the Go version across the project (go.mod, Dockerfile, Makefile) and rebuild.
- `make fmt`: Format code and organize imports using `goimports`.
- `make tidy`: Clean up `go.mod` and `go.sum` files.
- `make vet`: Run `go vet` for static analysis.
- `make generate`: Run `go generate` for all packages.
- `make vendor`: Create and update the `vendor` directory.
- `make coverage`: Generate an HTML test coverage report.
- `make coverage-view`: Open the HTML coverage report in your default browser.
- `make build-local`: Build the API binary on the host machine.
- `make help`: Display all available Makefile commands.
- `make test`: Run all Go tests inside the container.
- `make lint`: Run `golangci-lint` using a dedicated Docker image.
- `make swag`: Generate Swagger documentation.
- `make shell`: Open an interactive shell inside the API container.
- `make migrate-gen name=migration_name`: Generate a new versioned migration file.
- `make migrate-apply`: Apply all pending migrations to the database.
- `make config-check`: Validate `config/config.yaml` (server, secondary listeners, route patterns) without starting servers.
- `make clean`: Deep clean of containers, images, and volumes.

## Upgrading Go Version

To upgrade the Go version used in this project, run the following command with the desired version:

```bash
make go-upgrade version=1.27
```

This command automatically performs the following:
1. **Updates `go.mod`**: Changes the `go` version directive.
2. **Updates `Dockerfile`**: Changes the `FROM golang:<version>-alpine` base image.
3. **Updates `Makefile`**: Updates all `golang:<version>-alpine` image references used for tests and migrations.
4. **Rebuilds Images**: Runs `make build` to apply the changes.

## Database Migrations

This project uses **Ent** with **Atlas** for versioned migrations. Follow these steps when you need to change the database schema:

### 1. Create or Modify the Schema

#### To create a new table:
Initialize a new schema file:
```bash
docker run --rm -v $(pwd):/app -w /app golang:1.26-alpine go run -mod=mod entgo.io/ent/cmd/ent new TableName
```
Then define the fields in `ent/schema/tablename.go`.

#### To modify an existing table:
Update the schema definitions in the `ent/schema/` directory (e.g., `ent/schema/user.go`).

### 2. Generate Ent Code
After modifying the schema, regenerate the Ent runtime code:
```bash
make generate
```

### 3. Generate Migration Files
Generate a new SQL migration file by comparing your schema changes against an in-memory database:
```bash
make migrate-gen name=add_new_field_to_user
```
This will create new `.sql` files in `ent/migrate/migrations/`.

### 4. Apply Migrations
You can manually apply migrations to the database using:
```bash
make migrate-apply
```

Additionally, in the current development setup, the application automatically applies migrations on startup using `client.Schema.Create` in `internal/db/client.go`. You can restart the service to trigger this:
```bash
make restart
```

## Database Persistence

The SQLite database is stored at `/app/data/keeper.db` inside the container. This path is persisted using a bind mount to the local `./data` directory in the project root.

- **Host Path**: `./data/keeper.db`
- **Container Path**: `/app/data/keeper.db`
- **Environment Variable**: `DB_PATH`

The database initialization is fully aligned with the Ent migration setup. On every startup, the application verifies the schema against the generated Ent code and applies any necessary changes to the SQLite file, ensuring the physical database always matches your versioned migration files.

## Database schema

### app

- ID - int - primary key - auto increment
- Name - string - unique
- Status - smallint - 0 or 1
- Created at
- Updated at

### division

Hierarchical grouping entity using **Materialized Path** for subtree queries. Enables multi-level categorisation (company → department → team) scoped per app. Other microservices store `division_id` alongside `app_id` for granular filtering.

- ID - int - primary key - auto increment
- AppID - int - foreign key to app (CASCADE DELETE)
- ParentID - int - nullable self-referential FK (root divisions have NULL)
- Name - string
- Path - string - materialized path e.g. `/1/3/7/` (indexed)
- Depth - smallint - 0 for root, auto-computed from path
- Status - smallint - 0 or 1
- Created at
- Updated at

### user

- ID - int - primary key - auto increment
- AppID - int - foreign key to app
- DivisionID - int - foreign key to division (required)
- Firstname
- Lastname
- Email
- Password
- Status - smallint - 0 or 1
- Created at
- Updated at

## Configuration

The application can be configured using environment variables or YAML files (`config.yaml`, `config.dev.yaml`).

| Variable | Description | Default |
|----------|-------------|---------|
| `ENVIRONMENT` | Deployment environment (`dev`, `production`) | `production` |
| `SERVER_ADDR` | Internal network address the server binds to | `:8080` |
| `SERVER_HOST` | Public-facing host/port for Swagger documentation | `localhost:8080` |
| `DB_PATH` | Path to the SQLite database file | `data/keeper.db` |
| `LOG_DIR` | Directory where log files are stored | `log` |
| `AUTH_JWT_SECRET` | Secret key used for signing JWT tokens | `very-secret-key` |
| `AUTH_JWT_EXPIRY` | Expiration time for JWT tokens | `24h` |

### Running on a different Port/Host
- To change the port the server listens on: set `SERVER_ADDR=:9090`.
- To change the address used in Swagger documentation: set `SERVER_HOST=api.example.com`.

## Secondary listeners

Besides the primary server, any number of **secondary listeners** can be
declared in config: extra ports served by the same process, each exposing
only an allow-listed subset of the API, with rate limiting configured per
listener. Example use case: a dedicated, tightly rate-limited token-issuing
port exposing only `POST /users/auth`.

```yaml
SECONDARY:
  - NAME: "auth-only"            # used in logs; defaults to secondary-<index>
    ENABLED: true                # must be true to start the listener
    ADDR: ":8090"                # required, must be unique across listeners
    RATE_LIMIT:
      REQUESTS: 30               # default 100
      WINDOW: 1m                 # default 1m
    ROUTES:                      # chi-syntax "METHOD /path" allow-list;
      - "POST /users/auth"       # anything not listed returns 404
```

Behavior:

- Listeners reuse the same handlers, services and DB client as the primary
  server — no extra process, no duplicate state.
- `/health` and `/metrics` are always exposed on every listener. Swagger is
  only served on the primary port; it documents all routes since the
  handlers are shared.
- Keeper bakes JWT auth into its entity routes (`POST /users/auth` and
  `POST /guest-keys/auth` are public by design), so listeners always
  inherit that protection. A per-listener `JWT_SECRET` swaps the key the
  entity routes verify with (e.g. the guest secret).
- Config is validated at startup: missing/duplicate `ADDR` or malformed
  `ROUTES` patterns abort boot. Run `make config-check` to vet config
  without starting servers.
- Caveat: environment variables cannot override list entries (viper
  limitation) — secondary listeners are configured via YAML only.
- Docker: publish each secondary port in `docker-compose.yml` (e.g. add
  `- "8090:8090"` under `ports:`).

### Service-to-service (internal) use

A secondary listener doubles as an internal API port for other zooarium
services — a dedicated allow-listed surface instead of sharing the public
port. For keeper, the natural internal surface is token issuing and user
lookups for downstream services.

```yaml
SECONDARY:
  - NAME: "internal-s2s"
    ENABLED: true
    ADDR: ":8091"              # do NOT publish in docker-compose ports:
    RATE_LIMIT:
      REQUESTS: 1000           # generous — internal traffic comes from few IPs
      WINDOW: 1m
    ROUTES:
      - "POST /users/auth"
      - "GET /users/{id}"
```

Rules of thumb:

- **Isolation is the guard**: keep the port out of `docker-compose.yml`
  `ports:` — it stays reachable only on the compose network via service DNS
  (`http://keeper:8091/users/auth`). On bare metal, bind to a private
  interface (`ADDR: "127.0.0.1:8091"`).
- **Auth**: entity routes carry their own JWT protection (`POST /users/auth`
  and `POST /guest-keys/auth` are public by design), so an internal keeper
  listener is never weaker than the public one.
- **Rate limit**: internal traffic comes from few caller IPs — raise
  `RATE_LIMIT` well above the public default so legitimate bursts don't
  throttle.
- **Caller side**: per the zooarium constraint, the calling service must use
  a shared HTTP client with a timeout sourced from config (never the
  zero-timeout default client). Note: downstream services validate JWTs
  locally with the shared secret — they don't need to call keeper per
  request.

## Guest keys & guest tokens

Keeper is the constellation's token issuer for **public (unauthenticated)
surfaces**: a guest key is a publishable site key (Stripe-publishable-key
style) bound to an app, a division and a designated guest user. Public UIs
embed the site key and exchange it for a short-lived, tenant-scoped guest
JWT — no anonymous access anywhere; identity always travels as JWT claims.

```
browser (shop UI)
  0. GET keeper /guest-keys/lookup?url=https://shop.acme.com   (optional bootstrap)
       <- { site_key: "gk_..." }    resolves the site key for the URL it is served from
  1. POST keeper /guest-keys/auth { "site_key": "gk_..." }
       <- { token, expires_at }     claims: app_id, division_id, user_id, role=guest
  2. call the consuming service's intake listener with Bearer <token>
  3. on 401/expiry -> silently re-fetch
```

Properties:

- **Separate signing secret**: guest tokens are signed with
  `AUTH.GUEST_JWT_SECRET` (not `AUTH.JWT_SECRET`). Only listeners that
  explicitly configure that secret (e.g. ant's `order-intake`) accept them —
  on every other surface they fail verification. Containment is
  cryptographic, not convention.
- **Short expiry**: `AUTH.GUEST_JWT_EXPIRY` (default 30m); clients re-fetch
  silently.
- **Publishable by design**: "stealing" a site key only grants guest scope
  for that tenant — the same thing visiting the shop grants. Revoke by
  setting the key inactive or deleting it.
- **Hard rate limit**: `POST /guest-keys/auth` and `GET /guest-keys/lookup`
  are limited to 10 req/min per IP (they are the public spam surfaces),
  independent of the global limiter.
- **URL → site key lookup**: a key is bound to a unique `domain` — the
  normalized URL the UI is served from (scheme/port stripped, host lowercased,
  `host[+path]`, trailing slash trimmed). Public `GET /guest-keys/lookup?url=...`
  normalizes the URL the same way, matches it exactly, and returns only the
  site key (tenant binding stays private). Lets a UI bootstrap its site key
  from its own URL instead of hard-coding it. One active key per domain.
- **Validation**: the designated guest user must exist and belong to the
  key's app + division, and a non-empty `domain` is required (enforced on
  create). Tenant binding, site key and domain are immutable — rotate by
  delete + create.
- Management endpoints are JWT-protected: sysadmins manage all keys,
  app users only their own app's keys.

| Config | Description | Default |
|--------|-------------|---------|
| `AUTH.GUEST_JWT_SECRET` | Signing key for guest tokens (must match the consuming listener's `JWT_SECRET`) | placeholder — set via env in prod |
| `AUTH.GUEST_JWT_EXPIRY` | Guest token lifetime | `30m` |

## Impersonation (sysadmin "login as user")

Keeper lets a **sysadmin** open a session as another user on a downstream
service UI, with full parity of that user's rights, while keeping the admin's
own session alive. Authorization is claims-based, so the minted token simply
carries the impersonated user's identity (`app_id`, `user_id`, `division_id`);
extra claims (`impersonator`, `imp`, `imp_ro`, `sid`, `aud`) add
audit, scoping and revocation without reducing rights.

```
admin tab (keeper-ui)
  1. POST keeper /impersonations { target_user_id, audience }   (sysadmin JWT)
       <- { code, session_id, expires_at }    one-time code, ~60s TTL
  2. open <service-ui>/impersonate/exchange#code=...            (new tab; fragment, not query)
service-ui tab
  3. POST keeper /impersonations/exchange { code }              (public, rate-limited)
       <- { token, user, audience, session_id }  imp token signed with IMPERSONATION_JWT_SECRET
  4. store token+user in per-tab sessionStorage overlay -> act as the user
  5. on exit/logout -> POST keeper /impersonations/logout { session_id }  (revokes server-side)
```

Properties:

- **Separate signing secret**: impersonation tokens are signed with
  `AUTH.IMPERSONATION_JWT_SECRET` (not the primary or guest secret). Only
  services configured with it accept them; on every other surface they fail
  verification. Containment is cryptographic.
- **Audience-scoped**: each token carries `aud` = a single service key. A
  downstream service rejects any impersonation token whose audience is not its
  own — a token minted for `squirrel` cannot be replayed against `ant`.
- **No sysadmin target**: minting refuses when the target user is a sysadmin
  (privilege-escalation guard).
- **Short expiry + revocation**: `AUTH.IMPERSONATION_JWT_EXPIRY` (default
  `10m`); revoke a session server-side via `POST /impersonations/{id}/revoke`
  (`impersonation.create`) or `POST /impersonations/logout` (self, by `session_id`).
  Downstream services optionally check `GET /impersonations/active/{sid}`
  (cached) so a revoked session dies before its expiry.
- **One-time code handoff**: the JWT never travels in a URL — only a single-use,
  ~60s code does, in the URL *fragment* (not sent to servers/logs). Exchanged
  once for the token.
- **Read-only mode (optional)**: a session may be marked read-only (`imp_ro`);
  downstream services then reject mutating verbs. Default is full read/write.
- **Audit**: every session is persisted (`kpr_impersonation_session`) and every
  impersonated mutation downstream is logged with the impersonator.
- **Service registry**: targets are declared under `SERVICES:` (key, audience,
  UI exchange URL, UI origin) — validated at startup; drives the redirect
  allow-list, the exchange CORS allow-list, and the token audience.

| Config | Description | Default |
|--------|-------------|---------|
| `AUTH.IMPERSONATION_JWT_SECRET` | Signing key for impersonation tokens (must match the downstream service's `IMPERSONATION.JWT_SECRET`) | placeholder — set via env in prod |
| `AUTH.IMPERSONATION_JWT_EXPIRY` | Impersonation token lifetime | `10m` |
| `SERVICES` | Registered impersonation target services (`KEY`, `AUDIENCE`, `UI_EXCHANGE_URL`, `UI_ORIGIN`) | `[]` |

## Service URLs

By default, the services are available at:

- **API Gateway**: `http://<SERVER_HOST>`
- **Health Check**: `http://<SERVER_HOST>/health`
- **Swagger UI**: `http://<SERVER_HOST>/swagger/index.html`

## API Endpoints

> **Note**: Every API endpoint requires authentication via a valid JWT token passed in the `Authorization` header as a Bearer token.

- `GET /health`: Check service health.
- `POST /users`: Create a new user. Requires `user.create`; `own` scope rejects a target `app_id` other than the caller's own (falcon-resolved; see RBAC below).
- `GET /users`: List users, scoped by `user.read` (`own` returns just the caller's own app; `any`, e.g. sysadmin, returns every app).
- `POST /users/auth`: Authenticate and get JWT.
- `GET /users/{id}`: Get user by ID. Requires `user.read`, scoped as above.
- `PUT /users/{id}`: Update user by ID. Requires `user.update`, scoped as above.
- `DELETE /users/{id}`: Delete user by ID. Requires `user.delete`, scoped as above (falcon-resolved; see RBAC below).
- `GET /apps/lookup?site_key=...`: Resolve the public profile (name, tagline, logo, about, contact, tax_number, tax_percent, currency) for the app bound to a publishable guest site key (**public**, no auth, 10 req/min per IP). Returns 404 for an unknown/inactive site key or inactive app, without distinguishing between them. Excludes status + timestamps.
- `POST /apps`: Create a new app. Requires `app.create` permission (falcon-resolved; see RBAC below).
- `GET /apps`: List apps, scoped by `app.read` (`own` returns the caller's own app plus any apps they manage via `manager_id`; `any`, e.g. sysadmin, returns every app).
- `GET /apps/{id}`: Get app by ID (sysadmin, own app, or assigned manager via `manager_id`).
- `PUT /apps/{id}`: Update app by ID (sysadmin, own app, or assigned manager). Setting/clearing `manager_id` and changing `status` require the corresponding `app.update.<field>` permission (falcon-resolved).
- `DELETE /apps/{id}`: Delete app by ID (sysadmin or own app — no manager fallback). Requires `app.delete` permission (falcon-resolved).

App carries profile fields editable by sysadmin (any app) or the tenant's own
users (own app only): `tagline`, `logo_url` (URL, light `url` validation, no file
upload), nested `about` (`heading`, `body` — HTML allowed), and nested `contact`
(`address`: `line1`, `line2`, `city`, `state`, `country`, `postal_code`; `phone1`,
`phone2`, `email`, `hours` — free text, `social` — arbitrary `platform→url` JSON
map, each value validated as an http(s) URL), plus flat `tax_number` (free text,
e.g. VAT/GST registration) and `tax_percent` (float, 0–100), both optional. `currency`
(ISO 4217 code, e.g. `INR`) is **required** on create; NOT NULL in the DB. On
update, the `about` and `contact` sections replace wholesale when present.

- `POST /divisions`: Create a new division.
- `GET /divisions`: List divisions (filter: `?parent_id=`).
- `GET /divisions/{id}`: Get division by ID.
- `GET /divisions/{id}/descendants`: Get full subtree.
- `PUT /divisions/{id}`: Update division name/status.
- `PUT /divisions/{id}/move`: Move division to new parent.
- `DELETE /divisions/{id}`: Delete division (blocked if has children or users).
- `POST /guest-keys/auth`: Exchange a publishable site key for a guest JWT (public, 10 req/min per IP).
- `GET /guest-keys/lookup?url=...`: Resolve the publishable site key for a UI's URL (public, 10 req/min per IP; returns site key only).
- `POST /guest-keys`: Create a guest key (site key generated server-side).
- `GET /guest-keys`: List guest keys (non-sysadmins: own app only).
- `GET /guest-keys/{id}`: Get guest key by ID.
- `PUT /guest-keys/{id}`: Update guest key name/status (tenant binding and site key immutable).
- `DELETE /guest-keys/{id}`: Delete (revoke) a guest key.
- `POST /impersonations`: Start an impersonation session; returns a one-time handoff code. Requires `impersonation.create` (falcon-resolved; coarse gate, no ownership scoping); refuses any target holding a sudo (sysadmin-tier) role.
- `POST /impersonations/exchange`: Exchange a one-time code for an impersonation token + user (public, 10 req/min per IP).
- `POST /impersonations/logout`: Self-revoke a session by `session_id` (public, 10 req/min per IP).
- `GET /impersonations/active/{session_id}`: Report whether a session is active — boolean only (public, 10 req/min per IP; for downstream revocation checks).
- `GET /impersonations`: List active impersonation sessions. Requires `impersonation.read` (falcon-resolved).
- `GET /impersonations/services`: List registered impersonation target services for the UI picker. Requires `impersonation.read` (falcon-resolved).
- `GET /impersonations/{id}`: Get an impersonation session by ID. Requires `impersonation.read` (falcon-resolved).
- `POST /impersonations/{id}/revoke`: Revoke an impersonation session by ID. Requires `impersonation.create` (falcon-resolved).
- `GET /swagger/*`: Swagger UI.

## Rate Limiting

The API implements rate limiting using `httprate` middleware. By default, it is limited to **100 requests per minute per IP address**. This is configured in `internal/platform/http/router.go`. Each secondary listener has its own independent limit from its `RATE_LIMIT` config (default 100 req/min).

## Logging

Structured logging is implemented project-wide using the standard library `log/slog`. Important events such as database initialization, user creation, authentication attempts, and errors are logged with appropriate levels (INFO, WARN, ERROR).

Logs are written to both **stdout** and to a file named `api.log` located in the `log/` directory.

## Persistence

The project uses Docker volumes to persist data and logs outside the container:
- **Database**: Stored in `./data/keeper.db`.
- **Logs**: Stored in `./log/api.log`.

## API specs

https://jsonapi.org/format/1.2/

## TODO

- Pagination
