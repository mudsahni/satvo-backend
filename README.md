# SATVOS

A multi-tenant, secure file upload service built in Go. Supports tenant-isolated file management with JWT authentication, role-based access control, AWS S3 storage, and document collections with permission-based access control.

## Architecture

Hexagonal architecture (ports & adapters) with clear separation of concerns:

```
cmd/
  server/       Application entry point
  migrate/      Database migration CLI
internal/
  config/       Environment-based configuration (viper)
  domain/       Models, enums, custom errors
  handler/      HTTP request handlers (gin)
  middleware/   Auth, tenant guard, request logging
  service/      Business logic layer
  port/         Interface definitions (repository, storage)
  repository/   PostgreSQL implementations (sqlx + pgx)
  storage/      S3 client implementation
  router/       Route definitions and middleware wiring
  mocks/        Generated mocks (uber/mock)
tests/unit/     Unit tests
db/migrations/  SQL migration files
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.24 |
| HTTP Framework | Gin |
| Database | PostgreSQL 16 |
| Object Storage | AWS S3 (LocalStack for dev) |
| Auth | JWT (HS256) + bcrypt |
| Config | Viper (env vars) |
| Migrations | golang-migrate |

## Getting Started

### Prerequisites

- Go 1.24+
- PostgreSQL 16
- Docker & Docker Compose (for local development)

### Local Development (Docker Compose)

```bash
# Start PostgreSQL, LocalStack (S3), and the app
make docker-up

# Apply database migrations
make migrate-up

# Stop everything
make docker-down
```

This starts:
- **PostgreSQL** on port `5432` (user: `satvos`, password: `satvos_secret`, db: `satvos_db`)
- **LocalStack** on port `4566` (S3 emulation)
- **Application** on port `8080`

### Manual Setup

1. Copy and configure environment:
   ```bash
   cp .env.example .env
   # Edit .env with your database and S3 credentials
   ```

2. Run migrations:
   ```bash
   make migrate-up
   ```

3. Start the server:
   ```bash
   make run
   ```

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Compile binary to `bin/server` |
| `make run` | Run the server via `go run` |
| `make test` | Run all tests |
| `make test-unit` | Run unit tests only |
| `make lint` | Run golangci-lint |
| `make migrate-up` | Apply pending migrations |
| `make migrate-down` | Rollback one migration |
| `make docker-build` | Build Docker image `satvos:latest` |
| `make docker-up` | Start Docker Compose stack |
| `make docker-down` | Stop and remove containers |

## Configuration

All configuration is via environment variables prefixed with `SATVOS_`:

```bash
# Server
SATVOS_SERVER_PORT=:8080
SATVOS_SERVER_ENVIRONMENT=development    # development | production
SATVOS_SERVER_READ_TIMEOUT=15s
SATVOS_SERVER_WRITE_TIMEOUT=15s

# Database (PostgreSQL)
SATVOS_DB_HOST=localhost
SATVOS_DB_PORT=5432
SATVOS_DB_USER=satvos
SATVOS_DB_PASSWORD=satvos_secret
SATVOS_DB_NAME=satvos_db
SATVOS_DB_SSLMODE=disable
SATVOS_DB_MAX_OPEN=25
SATVOS_DB_MAX_IDLE=10

# JWT
SATVOS_JWT_SECRET=change-me-in-production
SATVOS_JWT_ACCESS_EXPIRY=15m
SATVOS_JWT_REFRESH_EXPIRY=168h
SATVOS_JWT_ISSUER=satvos

# S3 Storage
SATVOS_S3_ACCESS_KEY=your-access-key
SATVOS_S3_SECRET_KEY=your-secret-key
SATVOS_S3_REGION=ap-south-1
SATVOS_S3_BUCKET=satvos-uploads
SATVOS_S3_ENDPOINT=http://localhost:4566  # omit for real AWS
SATVOS_S3_MAX_FILE_SIZE_MB=50
SATVOS_S3_PRESIGN_EXPIRY=3600            # seconds

# Logging
SATVOS_LOG_LEVEL=debug
SATVOS_LOG_FORMAT=console
```

## Database Migrations

```bash
go run ./cmd/migrate up          # Apply all pending
go run ./cmd/migrate down        # Rollback one
go run ./cmd/migrate steps N     # Apply N migrations
go run ./cmd/migrate version     # Show current version
```

Schema: `tenants` -> `users` (per-tenant, cascade) -> `file_metadata` (per-tenant, cascade) -> `collections`, `collection_permissions`, `collection_files` (per-tenant, cascade).

## API Reference

**Base URL**: `/api/v1`

All responses use a standard envelope:

```json
{
  "success": true,
  "data": { },
  "error": null,
  "meta": { "total": 10, "offset": 0, "limit": 20 }
}
```

Error responses:

```json
{
  "success": false,
  "data": null,
  "error": { "code": "NOT_FOUND", "message": "resource not found" }
}
```

---

### Health

#### Liveness probe

```bash
curl http://localhost:8080/api/v1/healthz
```

#### Readiness probe (checks DB)

```bash
curl http://localhost:8080/api/v1/readyz
```

---

### Authentication

#### Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_slug": "acme",
    "email": "admin@acme.com",
    "password": "securepassword"
  }'
```

Response:

```json
{
  "success": true,
  "data": {
    "access_token": "eyJ...",
    "refresh_token": "eyJ...",
    "expires_at": "2025-01-01T00:15:00Z"
  }
}
```

#### Refresh token

```bash
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "eyJ..."
  }'
```

Returns a new access/refresh token pair in the same format as login.

---

### Files

All file endpoints require `Authorization: Bearer {access_token}`.

#### Upload a file

```bash
curl -X POST http://localhost:8080/api/v1/files/upload \
  -H "Authorization: Bearer <access_token>" \
  -F "file=@/path/to/document.pdf"
```

Optionally add the file to a collection during upload:

```bash
curl -X POST http://localhost:8080/api/v1/files/upload \
  -H "Authorization: Bearer <access_token>" \
  -F "file=@/path/to/document.pdf" \
  -F "collection_id=<collection_id>"
```

- Allowed types: PDF, JPG/JPEG, PNG
- Max size: 50 MB (configurable)
- Validates both file extension and magic bytes
- S3 key format: `tenants/{tenant_id}/files/{file_id}/{original_filename}`
- If `collection_id` is provided but association fails, the file is still uploaded and a warning is returned

#### List files (paginated)

```bash
curl http://localhost:8080/api/v1/files?offset=0&limit=20 \
  -H "Authorization: Bearer <access_token>"
```

#### Get file details + presigned download URL

```bash
curl http://localhost:8080/api/v1/files/<file_id> \
  -H "Authorization: Bearer <access_token>"
```

#### Delete a file (admin only)

```bash
curl -X DELETE http://localhost:8080/api/v1/files/<file_id> \
  -H "Authorization: Bearer <access_token>"
```

---

### Collections

Collections group files together with permission-based access control. A file can belong to multiple collections or none. Deleting a collection preserves the underlying files.

**Permissions**: `owner` (full control), `editor` (add/remove files), `viewer` (read-only). The creator is automatically assigned `owner`.

All collection endpoints require `Authorization: Bearer {access_token}`.

#### Create a collection

```bash
curl -X POST http://localhost:8080/api/v1/collections \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Q4 Reports",
    "description": "Quarterly financial reports"
  }'
```

#### List collections (paginated)

Returns only collections the authenticated user has access to.

```bash
curl http://localhost:8080/api/v1/collections?offset=0&limit=20 \
  -H "Authorization: Bearer <access_token>"
```

#### Get collection details (viewer+)

Returns the collection along with a paginated list of its files.

```bash
curl http://localhost:8080/api/v1/collections/<collection_id>?offset=0&limit=20 \
  -H "Authorization: Bearer <access_token>"
```

#### Update a collection (owner only)

```bash
curl -X PUT http://localhost:8080/api/v1/collections/<collection_id> \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Q4 Reports 2025",
    "description": "Updated description"
  }'
```

#### Delete a collection (owner only)

Deletes the collection and its permission/file associations. Files themselves are preserved.

```bash
curl -X DELETE http://localhost:8080/api/v1/collections/<collection_id> \
  -H "Authorization: Bearer <access_token>"
```

#### Batch upload files to a collection (editor+)

Upload multiple files at once via multipart form. Each file is processed independently. Returns 201 if all succeed, 207 Multi-Status on partial success.

```bash
curl -X POST http://localhost:8080/api/v1/collections/<collection_id>/files \
  -H "Authorization: Bearer <access_token>" \
  -F "files=@/path/to/doc1.pdf" \
  -F "files=@/path/to/doc2.jpg"
```

#### Remove a file from a collection (editor+)

Removes the association only; the file itself is not deleted.

```bash
curl -X DELETE http://localhost:8080/api/v1/collections/<collection_id>/files/<file_id> \
  -H "Authorization: Bearer <access_token>"
```

#### Set a user's permission on a collection (owner only)

Upserts the permission (creates or updates).

```bash
curl -X POST http://localhost:8080/api/v1/collections/<collection_id>/permissions \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "<user_id>",
    "permission": "editor"
  }'
```

Valid permissions: `owner`, `editor`, `viewer`.

#### List permissions on a collection (owner only)

```bash
curl http://localhost:8080/api/v1/collections/<collection_id>/permissions?offset=0&limit=20 \
  -H "Authorization: Bearer <access_token>"
```

#### Remove a user's permission (owner only)

Cannot remove your own permission.

```bash
curl -X DELETE http://localhost:8080/api/v1/collections/<collection_id>/permissions/<user_id> \
  -H "Authorization: Bearer <access_token>"
```

---

### Users

#### Create a user (admin only)

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "jane@acme.com",
    "password": "min8chars",
    "full_name": "Jane Doe",
    "role": "member"
  }'
```

Roles: `admin`, `member`. Email is unique per tenant.

#### List users (admin only, paginated)

```bash
curl http://localhost:8080/api/v1/users?offset=0&limit=20 \
  -H "Authorization: Bearer <access_token>"
```

#### Get user details (self or admin)

```bash
curl http://localhost:8080/api/v1/users/<user_id> \
  -H "Authorization: Bearer <access_token>"
```

#### Update a user (self or admin)

```bash
curl -X PUT http://localhost:8080/api/v1/users/<user_id> \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "full_name": "Jane Smith",
    "role": "admin",
    "is_active": true
  }'
```

All fields are optional. Admins can change `role` and `is_active`; users can update their own `email` and `full_name`.

#### Delete a user (admin only)

```bash
curl -X DELETE http://localhost:8080/api/v1/users/<user_id> \
  -H "Authorization: Bearer <access_token>"
```

---

### Tenants

All tenant endpoints require admin role.

#### Create a tenant

```bash
curl -X POST http://localhost:8080/api/v1/admin/tenants \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Corp",
    "slug": "acme"
  }'
```

#### List tenants (paginated)

```bash
curl http://localhost:8080/api/v1/admin/tenants?offset=0&limit=20 \
  -H "Authorization: Bearer <access_token>"
```

#### Get tenant details

```bash
curl http://localhost:8080/api/v1/admin/tenants/<tenant_id> \
  -H "Authorization: Bearer <access_token>"
```

#### Update a tenant

```bash
curl -X PUT http://localhost:8080/api/v1/admin/tenants/<tenant_id> \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Industries",
    "is_active": false
  }'
```

All fields (`name`, `slug`, `is_active`) are optional.

#### Delete a tenant

```bash
curl -X DELETE http://localhost:8080/api/v1/admin/tenants/<tenant_id> \
  -H "Authorization: Bearer <access_token>"
```

## Authentication & Authorization

- **JWT** with HS256 signing. Access tokens expire in 15 minutes, refresh tokens in 7 days.
- **Passwords** hashed with bcrypt (cost 12).
- **Roles**: `admin` (full access) and `member` (file upload/view).
- **Tenant isolation**: enforced at JWT claims level and database query level. Users log in with their tenant slug.

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `NOT_FOUND` | 404 | Resource not found |
| `UNAUTHORIZED` | 401 | Missing or invalid token |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `INVALID_CREDENTIALS` | 401 | Wrong email/password |
| `TENANT_INACTIVE` | 403 | Tenant is deactivated |
| `USER_INACTIVE` | 403 | User is deactivated |
| `UNSUPPORTED_FILE_TYPE` | 400 | File type not in whitelist |
| `FILE_TOO_LARGE` | 413 | Exceeds max file size |
| `DUPLICATE_EMAIL` | 409 | Email already exists for tenant |
| `DUPLICATE_SLUG` | 409 | Tenant slug already taken |
| `UPLOAD_FAILED` | 500 | S3 upload failed |
| `COLLECTION_NOT_FOUND` | 404 | Collection not found |
| `COLLECTION_PERMISSION_DENIED` | 403 | Insufficient collection permission |
| `DUPLICATE_COLLECTION_FILE` | 409 | File already in collection |
| `SELF_PERMISSION_REMOVAL` | 400 | Cannot remove own permission |
| `INVALID_PERMISSION` | 400 | Invalid permission value |

## Docker

**Build**:

```bash
docker build -t satvos:latest .
```

Multi-stage build: `golang:1.24-alpine` (build) -> `alpine:3.20` (runtime). Produces a minimal image with the server binary, migrate binary, migration files, and a startup script that runs migrations before starting the server.
