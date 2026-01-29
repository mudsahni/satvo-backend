# CLAUDE.md — Project Context for Claude Code

## Project Overview

SATVOS is a multi-tenant file upload service written in Go. It provides tenant-isolated file management with JWT authentication, role-based access control (admin/member), and AWS S3 storage. It also supports document collections with permission-based access control (owner/editor/viewer) for grouping files. The architecture follows the hexagonal (ports & adapters) pattern.

## Key Commands

```bash
make run              # Start the server (go run ./cmd/server)
make build            # Compile to bin/server
make test             # Run all tests (go test ./... -v -count=1)
make test-unit        # Run unit tests only (tests/unit/)
make lint             # Run golangci-lint
make migrate-up       # Apply database migrations
make migrate-down     # Rollback one migration
make docker-up        # Start full stack via Docker Compose
make docker-down      # Stop Docker Compose stack
```

## Architecture & Code Layout

```
cmd/server/main.go           Entry point — wires config, DB, storage, services, router
cmd/migrate/main.go          Migration CLI (up/down/steps/version)

internal/
  config/config.go           Loads env vars (SATVOS_ prefix) via viper
  domain/
    models.go                Tenant, User, FileMeta, Collection, CollectionPermissionEntry, CollectionFile structs
    enums.go                 FileType (pdf/jpg/png), UserRole (admin/member), FileStatus, CollectionPermission (owner/editor/viewer)
    errors.go                Sentinel errors (ErrNotFound, ErrForbidden, ErrCollectionNotFound, etc.)
  handler/
    auth_handler.go          POST /auth/login, POST /auth/refresh
    file_handler.go          POST /files/upload (optional collection_id), GET /files, GET /files/:id, DELETE /files/:id
    collection_handler.go    CRUD /collections, batch file upload, permission management
    user_handler.go          CRUD /users, /users/:id
    tenant_handler.go        CRUD /admin/tenants, /admin/tenants/:id
    health_handler.go        GET /healthz, GET /readyz
    response.go              Standard envelope (success/data/error/meta) + error mapping
  middleware/
    auth.go                  JWT validation, injects tenant_id/user_id/role into context
    tenant.go                Tenant context guard
    logger.go                Request ID injection, logging, panic recovery
  service/
    auth_service.go          Login (bcrypt verify), JWT generation/refresh
    file_service.go          Upload (validate + S3 + DB), download URL, delete
    collection_service.go    Collection CRUD, batch upload, permission checking, file association
    user_service.go          User CRUD with tenant scoping
    tenant_service.go        Tenant CRUD
  port/
    repository.go            TenantRepository, UserRepository, FileMetaRepository interfaces
    collection_repository.go CollectionRepository, CollectionPermissionRepository, CollectionFileRepository interfaces
    storage.go               ObjectStorage interface (Upload, Delete, GetPresignedURL)
  repository/postgres/
    db.go                    Database connection setup (sqlx + pgx)
    tenant_repo.go           Tenant SQL queries
    user_repo.go             User SQL queries (tenant-scoped)
    file_meta_repo.go        File metadata SQL queries
    collection_repo.go       Collection SQL queries (ListByUser joins permissions)
    collection_permission_repo.go  Collection permission queries (upsert via ON CONFLICT)
    collection_file_repo.go  Collection-file association queries (joins file_metadata)
  storage/s3/
    s3_client.go             S3 implementation (supports LocalStack endpoint)
  router/
    router.go                Route definitions, middleware wiring
  mocks/                     Generated mocks (uber/mock) for testing

tests/unit/                  Unit tests for handlers, services, middleware
db/migrations/               4 SQL migrations: tenants -> users -> file_metadata -> collections
```

## Data Flow

Request -> Gin Router -> Middleware (Logger -> Auth -> TenantGuard) -> Handler -> Service -> Repository/Storage -> PostgreSQL/S3

## Key Conventions

- **Environment config**: All vars prefixed with `SATVOS_` (e.g., `SATVOS_DB_HOST`). Loaded by viper in `internal/config/config.go`.
- **Response envelope**: All HTTP responses use `{"success": bool, "data": ..., "error": ..., "meta": ...}` defined in `handler/response.go`.
- **Tenant isolation**: Every DB query includes `tenant_id` from JWT claims. Users authenticate with tenant slug + email + password.
- **Error handling**: Domain errors in `domain/errors.go` are mapped to HTTP status codes in `handler/response.go`.
- **File validation**: Extension whitelist (pdf/jpg/jpeg/png) + magic bytes content-type detection.
- **S3 key format**: `tenants/{tenant_id}/files/{file_id}/{original_filename}`
- **Collections**: Permission-based access (owner/editor/viewer). Owner can manage permissions and delete. Editor can add/remove files. Viewer can read. Files can belong to multiple collections or none. Deleting a collection preserves files.
- **Batch upload**: `POST /collections/:id/files` accepts multiple files via multipart `"files"` field. Returns per-file results (207 on partial success).
- **Pagination**: `offset` and `limit` query params on list endpoints.
- **Testing**: Unit tests use testify assertions and uber/mock for interface mocking.
- **Database**: PostgreSQL with sqlx for query mapping and pgx/v5 as the driver. Parameterized queries throughout.
- **Passwords**: bcrypt with cost 12, minimum 8 characters.
- **JWT**: HS256 signing. Access token 15m, refresh token 7d. Claims carry tenant_id, user_id, email, role.

## Tech Stack

- Go 1.24, Gin, PostgreSQL 16 (sqlx + pgx/v5), AWS S3 (aws-sdk-go-v2), JWT (golang-jwt/v5), bcrypt, Viper, golang-migrate, Docker/Docker Compose, LocalStack (dev S3)

## Important Files for Common Tasks

- **Adding an endpoint**: `internal/router/router.go` (routes), then create handler in `internal/handler/`, service in `internal/service/`
- **Adding a domain model**: `internal/domain/models.go`, then repo interface in `internal/port/repository.go`, implementation in `internal/repository/postgres/`
- **Adding a migration**: `db/migrations/` (sequential numbered SQL files, up + down)
- **Modifying config**: `internal/config/config.go` (struct + viper binding), `.env.example`
- **Adding middleware**: `internal/middleware/`, wire it in `internal/router/router.go`
