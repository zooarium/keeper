# Keeper Project Guide for Gemini CLI

This document provides a comprehensive overview of the Keeper project, its architecture, development workflows, and technical details to assist Gemini CLI in understanding and maintaining the codebase.

## Project Overview
Keeper is a microservice for user management, providing RESTful APIs for authentication, user creation, and management. It is built with Go, uses SQLite for persistence, and is containerized with Docker.

## Technical Stack
- **Language**: Go v1.26
- **Database**: SQLite v3.51.2
- **ORM**: [Ent](https://entgo.io/)
- **Router**: [chi](https://github.com/go-chi/chi)
- **Validation**: [validator v10](https://github.com/go-playground/validator)
- **Authentication**: JWT (JSON Web Tokens)
- **Documentation**: Swagger (via `swag`)
- **Logging**: Structured logging with `log/slog`
- **Rate Limiting**: `httprate` (100 req/min per IP)
- **Migrations**: Atlas (integrated with Ent)

## Directory Structure
```text
/
├── cmd/
│   └── api/
│       └── main.go         # Application entry point
├── internal/
│   ├── app/                # App domain logic
│   │   ├── handler.go      # HTTP handlers
│   │   ├── service.go      # Business logic
│   │   ├── repository.go   # Data access logic
│   │   ├── model.go        # Domain & Request/Response models
│   │   ├── service_test.go # Unit tests for service
│   │   └── handler_test.go # Unit tests for handler
│   ├── user/               # User domain logic
│   │   ├── handler.go      # HTTP handlers
│   │   ├── service.go      # Business logic
│   │   ├── repository.go   # Data access logic
│   │   ├── model.go        # Domain & Request/Response models
│   │   ├── service_test.go # Unit tests for service
│   │   └── handler_test.go # Unit tests for handler
│   ├── platform/           # Cross-cutting concerns
│   │   ├── auth/           # JWT & Authentication logic
│   │   ├── http/           # Router & Middleware
│   │   └── render/         # Standard API responses
│   └── db/
│       └── sqlite.go       # SQLite client initialization
├── ent/                    # Ent ORM generated code & schema
│   └── schema/
│       ├── app.go          # App database schema definition
│       └── user.go         # User database schema definition
├── pkg/                    # Shared packages (logger, config)
├── data/                   # SQLite database file (persisted via volume)
├── log/                    # Application logs (persisted via volume)
├── docs/                   # Swagger documentation
├── Dockerfile              # Docker build configuration
├── docker-compose.yml      # Service orchestration
└── Makefile                # Development automation
```

## Architecture & Design Patterns
- **Directional Dependencies**: HTTP (Handler) → Service → Repository.
- **Dependency Injection**: Used to decouple components and facilitate testing.
- **Interface Segregation**: Core logic is defined through interfaces.
- **Standardized Responses**: All API responses follow a consistent JSON format defined in `internal/platform/render`.
- **Context Propagation**: `context.Context` is passed through all layers for cancellation and timeouts.
- **Graceful Shutdown**: The API server handles `SIGINT` and `SIGTERM` for graceful termination.
- **Database Conventions**: All database table names **must** be in singular format (e.g., `user` instead of `users`) and **must** include a `kpr_` prefix (e.g., `kpr_user`). This is enforced in the Ent schema using `entsql.Annotation`.

## Naming Conventions
- **Packages**: Short, lowercase, single-word names (e.g., `user`, `auth`). Avoid underscores or mixedCaps.
- **Files**: Lowercase, using underscores only if necessary (e.g., `handler.go`, `service_test.go`).
- **Variables & Constants**: Use `CamelCase` (`MixedCaps` for exported, `mixedCaps` for unexported). Keep acronyms consistent (e.g., `userID`, `APIKey`).
- **Receivers**: Use short, consistent names (1-3 letters) representing the type (e.g., `func (u *User) ...`).
- **Interfaces**: Name based on behavior, often ending in `-er` for single-action interfaces (e.g., `Reader`), or use descriptive nouns for domain logic (e.g., `Service`, `Repository`).
- **REST API Components**:
    - **Handlers**: `[Action][Entity]` (e.g., `CreateUser`, `ListUsers`).
    - **Services**: `[Entity]Service`.
    - **Repositories**: `[Entity]Repository`.
    - **Models**: Use `[Entity]` for domain models and `[Action][Entity]Request/Response` for DTOs.
- **Database**: Table names and Ent schemas **must** be singular and include the `kpr_` prefix (e.g., `kpr_user`).

## Development Workflow

### Command Preference
Always prefer using `make` commands defined in the `Makefile` over direct `docker` or `go` commands. The `Makefile` ensures a consistent environment (using specific Go versions and dependencies) by running tools inside Docker containers.

### Mandatory Workflow for Every Change
To ensure codebase health and consistency, the following steps **must** be completed for every modification or new feature:
1. **Follow Naming Conventions**: Adhere to the project's naming conventions for packages, files, variables, and API components as defined in this document.
2.  **Structured Logging**: Add or update structured logging (using `slog`) to capture important events, business logic milestones, and error conditions.
3.  **Write Unit Tests**: Every new feature or bug fix must include corresponding unit tests (e.g., `*_test.go`).
4.  **Update Makefile**: If new development commands are required, add them to the `Makefile` and update the documentation accordingly.
5.  **Run Formatter**: Every code change MUST be formatted to ensure consistent code style and import management. Run `make fmt` after any modification to source files.
6.  **Run Linter**: Ensure code quality by running `make lint` after code and test changes.
7.  **Update Swagger Documentation**: If any API endpoints are added or modified, regenerate documentation using `make swag`.
8.  **Update README.md**: Ensure any new features, endpoints, or configuration changes are documented in `README.md`.
9.  **Update GEMINI.md**: Ensure this project guide is updated to reflect any changes in architecture, workflows, or documentation standards.
10.  **Run All Tests**: Verify that all tests pass by running `make test`. (Note: `make test` automatically runs `make fmt` as a prerequisite).
11.  **Custom Scripts**: You **MUST** always run any custom script using the `make run-script` command to ensure a consistent environment and proper dependency handling.

### Common Commands (Makefile)
- `make build`: Build Docker images.
- `make up`: Start services in the background.
- `make down`: Stop services.
- `make deps-upgrade`: Update Go dependencies using a Docker container.
- `make fmt`: Format code and organize imports using `goimports`.
- `make tidy`: Clean up `go.mod` and `go.sum` files.
- `make vet`: Run `go vet` for static analysis.
- `make generate`: Run `go generate` for all packages.
- `make vendor`: Create and update the `vendor` directory.
- `make coverage`: Generate an HTML test coverage report.
- `make coverage-view`: Open the HTML coverage report in your default browser.
- `make build-local`: Build the API binary on the host machine.
- `make help`: Display all available Makefile commands.
- `make test`: Run unit tests in a fresh Go container.
- `make benchmark`: Run performance benchmarks in a fresh Go container.
- `make logs`: Follow container logs.
- `make swag`: Regenerate Swagger documentation.
- `make migrate-gen name=NAME`: Generate a new database migration.
- `make migrate-apply`: Apply pending migrations.
- `make run-script name=NAME args="ARGS"`: Run a script from the `scripts/` directory in a fresh Go container.
- `make sql query=QUERY`: Run a SQL query against the SQLite database.

### Database Migrations
1.  **Modify Schema**: Edit files in `ent/schema/` (e.g., `user.go`, `app.go`).
2.  **Generate Code**: `make generate`
3.  **Generate Migration**: `make migrate-gen name=change_description`.
4.  **Apply**: `make migrate-apply` (or restart the app for auto-migration).

### Database Schema (kpr_user table)

| Field      | Type      | Description                          |
|------------|-----------|--------------------------------------|
| ID         | int       | Primary Key (Auto-increment)         |
| AppID      | int       | Foreign Key to kpr_app               |
| Firstname  | string    | User's first name                    |
| Lastname   | string    | User's last name                     |
| Email      | string    | Unique email address                 |
| Password   | string    | Hashed password (sensitive)          |
| Status     | smallint  | 0 (Inactive), 1 (Active)             |
| CreatedAt  | datetime  | Creation timestamp                   |
| UpdatedAt  | datetime  | Last update timestamp                |



### Database Schema (kpr_app table)

| Field      | Type      | Description                          |
|------------|-----------|--------------------------------------|
| ID         | int       | Primary Key (Auto-increment)         |
| Name       | string    | Unique app name                      |
| Status     | smallint  | 0 (Inactive), 1 (Active)             |
| CreatedAt  | datetime  | Creation timestamp                   |
| UpdatedAt  | datetime  | Last update timestamp                |



## API Endpoints
- `GET /health`: Check service health.
- `POST /users`: Create a new user.
- `GET /users`: List all users.
- `POST /users/auth`: Authenticate and get JWT.
- `GET /users/{id}`: Get user by ID.
- `PUT /users/{id}`: Update user by ID.
- `DELETE /users/{id}`: Delete user by ID.
- `POST /apps`: Create a new app.
- `GET /apps`: List all apps.
- `GET /apps/{id}`: Get app by ID.
- `PUT /apps/{id}`: Update app by ID.
- `DELETE /apps/{id}`: Delete app by ID.
- `GET /swagger/*`: Swagger UI.

## Logging & Monitoring
- Logs are written to **stdout** and `./log/api.log`.
- Log format is JSON (structured).
- Levels: `INFO` for normal operations, `WARN` for client errors/auth failures, `ERROR` for system failures.

## Persistence & Volumes
- **Database**: `./data/keeper.db` mapped to `/app/data/keeper.db`.
- **Logs**: `./log/` mapped to `/app/log/`.
- **Environment**: `DB_PATH` and `LOG_DIR` control these paths.
