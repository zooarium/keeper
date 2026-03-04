# MySQL Migration Guide

This document outlines the investigation findings and the necessary steps to switch from SQLite to MySQL in the Keeper project.

## Investigation Summary

The migration from SQLite to MySQL is of **medium complexity**. While the project uses the Ent ORM, which abstracts the database layer, several infrastructure-level changes are required for configuration, Docker orchestration, and migration workflows.

### 1. Dependencies and Drivers
- **Action**: Add `github.com/go-sql-driver/mysql` to `go.mod`.
- **CGO Optimization**: The MySQL driver does not require CGO. You can disable `CGO_ENABLED=1` in the `Dockerfile` and `Makefile`, resulting in a leaner and more portable binary.

### 2. Configuration Updates
- **Current**: Only uses `DB_PATH`.
- **Required**: Expand `pkg/config/config.go` and `config/config.yaml` to include:
  - `DRIVER` (e.g., "mysql")
  - `HOST`
  - `PORT`
  - `USER`
  - `PASSWORD`
  - `DB_NAME`

### 3. Database Initialization
- **Action**: Update `internal/db/sqlite.go` (or rename to `db.go`) to use `ent.Open("mysql", dsn)`.
- **DSN Format**: The Data Source Name will change to: `user:password@tcp(host:port)/dbname?parseTime=true`.

### 4. Ent Schema and Migrations
- **Schema**: Existing schemas in `ent/schema/` are already dialect-agnostic and require no changes.
- **Migration Generation**: Update `ent/migrate/main.go` to use `dialect.MySQL` and provide a MySQL connection string for the `NamedDiff` function.
- **Regeneration**: All existing migrations for MySQL must be regenerated using `make migrate-gen`.

### 5. Docker Orchestration
- **`docker-compose.yml`**: Add a `mysql:8.0` service.
- **`api` service**: Update environment variables and add `depends_on: [mysql]` with a health check.

### 6. Tooling and Scripts
- **`Makefile`**:
  - Update `migrate-apply` to use a MySQL URL for the Atlas migration tool.
  - Replace the `sql` helper (currently using `sqlite3`) with a MySQL client (e.g., `mysql` or `mariadb-client`).
- **Maintenance Scripts**: Update scripts like `scripts/update_password.go` to use the MySQL driver and connection string.
- **Unit Tests**: Decide whether to keep in-memory SQLite for fast testing or transition to MySQL-compatible alternatives like `testcontainers`.

## Implementation Roadmap

### Phase 1: Infrastructure
1.  Add MySQL service to `docker-compose.yml`.
2.  Update `pkg/config/config.go` with new database fields.
3.  Add `github.com/go-sql-driver/mysql` to `go.mod`.

### Phase 2: Database Layer
1.  Rename `internal/db/sqlite.go` to `internal/db/db.go`.
2.  Update the initialization logic to support MySQL.
3.  Update `ent/migrate/main.go` for MySQL dialect.

### Phase 3: Migrations & Tooling
1.  Clear old SQLite migrations and generate new MySQL ones.
2.  Update `Makefile` commands (`migrate-apply`, `sql`).
3.  Update Docker build configuration to disable CGO (optional but recommended).

### Phase 4: Validation
1.  Run all unit tests.
2.  Start the stack with `docker-compose up` and verify the API with a running MySQL instance.
3.  Regenerate Swagger documentation if necessary.
