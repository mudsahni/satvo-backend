# SATVOS

A multi-tenant, secure file upload service built in Go. Supports tenant-isolated file management with JWT authentication, role-based access control, and AWS S3 storage.

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

Schema: `tenants` -> `users` (per-tenant, cascade) -> `file_metadata` (per-tenant, cascade).

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

### Health

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/healthz` | None | Liveness probe (always 200) |
| `GET` | `/readyz` | None | Readiness probe (checks DB) |

### Authentication

#### `POST /auth/login`

```json
{
  "tenant_slug": "acme",
  "email": "admin@acme.com",
  "password": "securepassword"
}
```

Response:

```json
{
  "access_token": "eyJ...",
  "refresh_token": "eyJ...",
  "expires_at": "2025-01-01T00:15:00Z"
}
```

#### `POST /auth/refresh`

```json
{
  "refresh_token": "eyJ..."
}
```

Returns a new token pair.

### Files

All file endpoints require `Authorization: Bearer {access_token}`.

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/files/upload` | User | Upload a file (multipart/form-data) |
| `GET` | `/files` | User | List files (paginated: `?offset=0&limit=20`) |
| `GET` | `/files/:id` | User | Get file metadata + presigned download URL |
| `DELETE` | `/files/:id` | Admin | Soft-delete a file |

**Upload** (`POST /files/upload`):
- Content-Type: `multipart/form-data`
- Form field: `file`
- Allowed types: PDF, JPG/JPEG, PNG
- Max size: 50 MB (configurable)
- Validates both file extension and magic bytes

**S3 key format**: `tenants/{tenant_id}/files/{file_id}/{original_filename}`

### Users

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/users` | Admin | Create a user |
| `GET` | `/users` | Admin | List users (paginated) |
| `GET` | `/users/:id` | Self/Admin | Get user details |
| `PUT` | `/users/:id` | Self/Admin | Update user |
| `DELETE` | `/users/:id` | Admin | Delete user |

**Create user** (`POST /users`):

```json
{
  "email": "user@acme.com",
  "password": "min8chars",
  "full_name": "Jane Doe",
  "role": "member"
}
```

Roles: `admin`, `member`. Email is unique per tenant.

### Tenants

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/admin/tenants` | Admin | Create tenant |
| `GET` | `/admin/tenants` | Admin | List tenants (paginated) |
| `GET` | `/admin/tenants/:id` | Admin | Get tenant |
| `PUT` | `/admin/tenants/:id` | Admin | Update tenant |
| `DELETE` | `/admin/tenants/:id` | Admin | Delete tenant |

**Create tenant** (`POST /admin/tenants`):

```json
{
  "name": "Acme Corp",
  "slug": "acme"
}
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

## Docker

**Build**:

```bash
docker build -t satvos:latest .
```

Multi-stage build: `golang:1.23-alpine` (build) -> `alpine:3.20` (runtime). Produces a minimal image with the compiled binary and migration files.
