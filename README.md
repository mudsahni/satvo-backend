# SATVOS

A multi-tenant document processing service built in Go. Supports tenant-isolated file management with JWT authentication, role-based access control, AWS S3 storage, document collections with permission-based access control, multi-provider LLM-powered document parsing (with optional dual-parse merge mode), automated GST invoice validation with 50+ built-in rules, and reconciliation tiering for GSTR-2A/2B matching.

## Table of Contents

- [Architecture](#architecture)
- [Tech Stack](#tech-stack)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Local Development (Docker Compose)](#local-development-docker-compose)
  - [Manual Setup](#manual-setup)
  - [Makefile Targets](#makefile-targets)
- [Configuration](#configuration)
- [Database Migrations](#database-migrations)
- [API Reference](#api-reference)
  - [Health](#health)
  - [Authentication](#authentication)
  - [Files](#files)
  - [Collections](#collections)
  - [Users](#users)
  - [Tenants](#tenants)
  - [Documents (AI-Powered Parsing + Validation)](#documents-ai-powered-parsing--validation)
    - [Built-in Validation Rules](#built-in-validation-rules)
    - [Parsed Invoice Schema](#parsed-invoice-schema)
- [Authentication & Authorization](#authentication--authorization)
- [Error Codes](#error-codes)
- [Docker](#docker)
- [Additional Documentation](#additional-documentation)

## Architecture

Hexagonal architecture (ports & adapters) with clear separation of concerns:

```
cmd/
  server/         Application entry point
  migrate/        Database migration CLI
internal/
  config/         Environment-based configuration (viper)
  domain/         Models, enums, custom errors
  handler/        HTTP request handlers (gin)
  middleware/     Auth, tenant guard, request logging
  service/        Business logic layer
  port/           Interface definitions (repository, storage, parser)
  parser/         LLM parser implementations (factory, shared prompt, merge parser)
    claude/       Anthropic Claude parser (Messages API)
    gemini/       Google Gemini parser (stub)
  validator/      Document validation engine
    invoice/      GST invoice validators (required, format, math, crossfield, logical)
  repository/     PostgreSQL implementations (sqlx + pgx)
  storage/        S3 client implementation
  router/         Route definitions and middleware wiring
  mocks/          Generated mocks (uber/mock)
tests/unit/       Unit tests (handlers, services, middleware, validators, parsers, config)
db/migrations/    SQL migration files (10 migrations)
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.24 |
| HTTP Framework | Gin |
| Database | PostgreSQL 16 |
| Object Storage | AWS S3 (LocalStack for dev) |
| Auth | JWT (HS256) + bcrypt |
| Document Parsing | Anthropic Claude API, Google Gemini (stub); multi-provider with dual-parse merge |
| Validation | Pluggable rule engine (50+ built-in GST rules) with reconciliation tiering |
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
| `make swagger` | Regenerate Swagger/OpenAPI docs |

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

# Document Parser (LLM) — Single provider (legacy)
SATVOS_PARSER_PROVIDER=claude              # "claude" or "gemini"
SATVOS_PARSER_API_KEY=sk-ant-...           # Anthropic API key
SATVOS_PARSER_DEFAULT_MODEL=claude-sonnet-4-20250514
SATVOS_PARSER_MAX_RETRIES=2
SATVOS_PARSER_TIMEOUT_SECS=120

# Document Parser — Multi-provider (overrides legacy if set)
SATVOS_PARSER_PRIMARY_PROVIDER=claude
SATVOS_PARSER_PRIMARY_API_KEY=sk-ant-...
SATVOS_PARSER_PRIMARY_DEFAULT_MODEL=claude-sonnet-4-20250514
SATVOS_PARSER_SECONDARY_PROVIDER=gemini    # optional; enables dual-parse mode
SATVOS_PARSER_SECONDARY_API_KEY=...
SATVOS_PARSER_SECONDARY_DEFAULT_MODEL=gemini-2.0-flash
```

## Database Migrations

```bash
go run ./cmd/migrate up          # Apply all pending
go run ./cmd/migrate down        # Rollback one
go run ./cmd/migrate steps N     # Apply N migrations
go run ./cmd/migrate version     # Show current version
```

Schema: `tenants` -> `users` (per-tenant, cascade) -> `file_metadata` (per-tenant, cascade) -> `collections`, `collection_permissions`, `collection_files` (per-tenant, cascade) -> `documents`, `document_tags`, `document_validation_rules` (per-tenant, cascade). Validation results are stored as JSONB on the `documents` table. Migration 008 adds reconciliation tiering columns; migration 009 adds multi-parser columns (`parse_mode`, `field_provenance`); migration 010 adds `name` to documents and `source` to document_tags.

## API Reference

**Base URL**: `/api/v1`

**Interactive API Documentation**: Swagger UI is available at `/swagger/index.html` when the server is running:

```bash
# Start the server, then open in browser:
http://localhost:8080/swagger/index.html
```

The Swagger UI provides:
- Interactive API explorer with "Try it out" functionality
- Complete request/response schemas
- JWT Bearer authentication support
- Auto-generated from code annotations

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

**Collection permissions**: `owner` (full control), `editor` (add/remove files), `viewer` (read-only). The creator is automatically assigned `owner`.

**Tenant role interaction**: Tenant roles provide implicit collection access (see [Authentication & Authorization](#authentication--authorization)). A user's effective permission on a collection is `max(implicit_from_role, explicit_collection_permission)`.

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

#### Update a collection (editor+)

```bash
curl -X PUT http://localhost:8080/api/v1/collections/<collection_id> \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Q4 Reports 2025",
    "description": "Updated description"
  }'
```

#### Delete a collection (owner or admin)

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

Roles: `admin`, `manager`, `member`, `viewer`. Email is unique per tenant.

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

### Documents (AI-Powered Parsing + Validation)

Documents represent parsed and validated versions of uploaded files. When you create a document, SATVOS sends the file to an LLM (currently Claude) in a background goroutine which extracts structured invoice data including seller/buyer info, line items, tax breakdowns, and payment details. After parsing completes, the validation engine automatically runs 50+ built-in GST rules against the extracted data.

**Document Lifecycle**:

```
Upload File ──> Create Document ──> Background LLM Parsing ──> Auto-Validation ──> Human Review
                (parse_mode:             |                         |                      |
                 single/dual)      parsing_status:           validation_status:      Manual Edit
                                   pending -> processing     pending -> valid        (resets review,
                                   -> completed/failed       -> warning/invalid      re-validates)
                                                                                    reconciliation_status:
                                                                                    pending -> valid
                                                                                    -> warning/invalid
```

**Full workflow**: Upload a file -> Create a document (triggers parsing, optionally in dual-parse mode) -> Poll until `parsing_status=completed` -> Validation runs automatically (computing both `validation_status` and `reconciliation_status`) -> Optionally edit structured data manually (resets review, re-validates) -> Check validation results -> Approve or reject.

All document endpoints require `Authorization: Bearer {access_token}`.

#### Create a document (triggers parsing)

Links an uploaded file to a collection and begins LLM parsing in the background. The response returns immediately with `parsing_status: "pending"`.

```bash
curl -X POST http://localhost:8080/api/v1/documents \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "file_id": "<file_id>",
    "collection_id": "<collection_id>",
    "document_type": "invoice",
    "parse_mode": "single",
    "name": "Q4 Invoice from Acme",
    "tags": {
      "vendor": "Acme Corp",
      "quarter": "Q4"
    }
  }'
```

- `parse_mode` is optional. Valid values: `single` (default, uses primary parser) or `dual` (runs primary + secondary parsers in parallel and merges results). If `dual` is requested but no secondary parser is configured, falls back to `single`.
- `name` is optional. If omitted, defaults to the uploaded file's original filename.
- `tags` is optional. Key-value pairs stored with `source: "user"`. After parsing completes, the system also auto-generates tags (with `source: "auto"`) from extracted invoice fields (invoice number, date, seller/buyer name and GSTIN, etc.).

Response:

```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "name": "Q4 Invoice from Acme",
    "file_id": "uuid",
    "collection_id": "uuid",
    "document_type": "invoice",
    "parsing_status": "pending",
    "review_status": "pending",
    "parse_mode": "single",
    "reconciliation_status": "pending",
    "structured_data": {},
    "confidence_scores": {},
    "field_provenance": {}
  }
}
```

#### Get document by ID (poll for results)

```bash
curl http://localhost:8080/api/v1/documents/<document_id> \
  -H "Authorization: Bearer <access_token>"
```

When parsing completes, `structured_data` contains the extracted invoice JSON (seller, buyer, line items, totals, payment info) and `confidence_scores` mirrors the structure with 0.0-1.0 confidence values per field.

#### List documents (paginated)

Filter by collection or list all for the tenant:

```bash
# All documents for the tenant
curl http://localhost:8080/api/v1/documents?offset=0&limit=20 \
  -H "Authorization: Bearer <access_token>"

# Documents in a specific collection
curl "http://localhost:8080/api/v1/documents?collection_id=<collection_id>&offset=0&limit=20" \
  -H "Authorization: Bearer <access_token>"
```

#### Retry parsing (for failed documents)

Re-triggers LLM parsing for a document that previously failed.

```bash
curl -X POST http://localhost:8080/api/v1/documents/<document_id>/retry \
  -H "Authorization: Bearer <access_token>"
```

#### Review a document (approve/reject)

Set the human review status after inspecting parsed data. Only works on documents with `parsing_status=completed`.

```bash
curl -X PUT http://localhost:8080/api/v1/documents/<document_id>/review \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "status": "approved",
    "notes": "Data verified against source document"
  }'
```

Valid statuses: `approved`, `rejected`.

#### Edit structured data manually

Replace the parsed invoice data with manually corrected data. Validates the JSON against the GSTInvoice schema, sets all confidence scores to 1.0 (human-verified), resets review status to pending, re-extracts auto-tags, and synchronously re-runs validation. Requires editor+ permission.

```bash
curl -X PUT http://localhost:8080/api/v1/documents/<document_id>/structured-data \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "structured_data": {
      "invoice": {"invoice_number": "INV-001", "invoice_date": "2025-01-15"},
      "seller": {"name": "Acme Corp", "gstin": "29AABCU9603R1ZM"},
      "buyer": {"name": "Buyer Inc"},
      "line_items": [],
      "totals": {"total": 1000},
      "payment": {}
    }
  }'
```

#### Document Tags

Documents support key-value tags with two sources: `user` (manually provided) and `auto` (extracted from parsed invoice data). Auto-tags are generated after parsing completes and refreshed on retry or manual edit of structured data.

**Auto-generated tag keys**: `invoice_number`, `invoice_date`, `seller_name`, `seller_gstin`, `buyer_name`, `buyer_gstin`, `invoice_type`, `place_of_supply`, `total_amount`.

##### List tags

```bash
curl http://localhost:8080/api/v1/documents/<document_id>/tags \
  -H "Authorization: Bearer <access_token>"
```

##### Add tags (editor+)

```bash
curl -X POST http://localhost:8080/api/v1/documents/<document_id>/tags \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "tags": {
      "vendor": "Acme Corp",
      "category": "utilities"
    }
  }'
```

##### Delete a tag (editor+)

```bash
curl -X DELETE http://localhost:8080/api/v1/documents/<document_id>/tags/<tag_id> \
  -H "Authorization: Bearer <access_token>"
```

##### Search documents by tag

```bash
curl "http://localhost:8080/api/v1/documents/search/tags?key=vendor&value=Acme+Corp&offset=0&limit=20" \
  -H "Authorization: Bearer <access_token>"
```

Returns a paginated list of documents matching the given tag key-value pair.

#### Delete a document (admin only)

```bash
curl -X DELETE http://localhost:8080/api/v1/documents/<document_id> \
  -H "Authorization: Bearer <access_token>"
```

#### Validate a document (re-run validation)

Triggers the validation engine on a parsed document. Validation also runs automatically after parsing completes, so this is only needed to re-validate after rule changes.

```bash
curl -X POST http://localhost:8080/api/v1/documents/<document_id>/validate \
  -H "Authorization: Bearer <access_token>"
```

#### Get validation results

Returns detailed validation results including per-rule outcomes, a summary, and per-field statuses.

```bash
curl http://localhost:8080/api/v1/documents/<document_id>/validation \
  -H "Authorization: Bearer <access_token>"
```

Response:

```json
{
  "success": true,
  "data": {
    "document_id": "uuid",
    "validation_status": "warning",
    "summary": {
      "total": 50,
      "passed": 47,
      "errors": 0,
      "warnings": 3
    },
    "reconciliation_status": "valid",
    "reconciliation_summary": {
      "total": 21,
      "passed": 21,
      "errors": 0,
      "warnings": 0
    },
    "results": [
      {
        "rule_name": "Required: Invoice Number",
        "rule_type": "required_field",
        "severity": "error",
        "passed": true,
        "field_path": "invoice.invoice_number",
        "expected_value": "non-empty value",
        "actual_value": "INV-001",
        "message": "Required: Invoice Number: invoice.invoice_number is present",
        "reconciliation_critical": true
      }
    ],
    "field_statuses": {
      "invoice.invoice_number": {
        "status": "valid",
        "messages": []
      },
      "seller.gstin": {
        "status": "unsure",
        "messages": ["Format: Seller GSTIN: invalid format"]
      }
    }
  }
}
```

**Validation status values**: `pending` (not yet validated), `valid` (all rules passed), `warning` (only warning-severity failures), `invalid` (error-severity failures).

**Reconciliation status values**: `pending`, `valid` (all reconciliation-critical rules passed), `warning` (only warning-severity recon-critical failures), `invalid` (error-severity recon-critical failures). Computed independently from `validation_status` using only the 21 reconciliation-critical rules needed for GSTR-2A/2B matching.

**Field status values**: `valid`, `invalid` (error-severity rule failed), `unsure` (warning-severity rule failed or low confidence score).

#### Built-in Validation Rules

The validation engine includes 50 built-in rules across 5 categories, automatically seeded per tenant on first use:

| Category | Count | Type | Examples |
|----------|-------|------|----------|
| Required Fields | 12 | `required_field` | Invoice number, seller/buyer GSTIN, state codes |
| Format | 13 | `regex` | GSTIN (15-char pattern), PAN (10-char), IFSC, HSN/SAC, date formats, ISO currency codes, state codes (01-38) |
| Mathematical | 12 | `sum_check` | Line item taxable = qty x price - discount, CGST/SGST/IGST amounts, totals reconciliation, round-off <= 0.50 |
| Cross-field | 8 | `cross_field` | GSTIN-state match, GSTIN-PAN match, intrastate uses CGST+SGST, interstate uses IGST, due date after invoice date |
| Logical | 7 | `custom` | Non-negative amounts, valid GST rates (0/0.25/3/5/12/18/28%), CGST=SGST rate, exclusive tax type, at least one line item |

Rules have severity levels: `error` (blocks approval) or `warning` (informational).

For the complete list of every rule key, field path, formula, and validation logic, see **[VALIDATION.md](VALIDATION.md)**.

#### Parsed Invoice Schema

When parsing completes, `structured_data` contains:

```json
{
  "invoice": {
    "invoice_number": "INV-2024-001",
    "invoice_date": "2024-01-15",
    "due_date": "2024-02-15",
    "invoice_type": "tax_invoice",
    "currency": "INR",
    "place_of_supply": "Delhi",
    "reverse_charge": false
  },
  "seller": {
    "name": "...", "address": "...",
    "gstin": "07AAACR5055K1Z5", "pan": "AAACR5055K",
    "state": "Delhi", "state_code": "07"
  },
  "buyer": {
    "name": "...", "address": "...",
    "gstin": "29BBBCB1234A1Z1", "pan": "BBBCB1234A",
    "state": "Karnataka", "state_code": "29"
  },
  "line_items": [
    {
      "description": "Consulting Services",
      "hsn_sac_code": "998311",
      "quantity": 1, "unit": "hrs",
      "unit_price": 5000.00, "discount": 0,
      "taxable_amount": 5000.00,
      "cgst_rate": 9, "cgst_amount": 450.00,
      "sgst_rate": 9, "sgst_amount": 450.00,
      "igst_rate": 0, "igst_amount": 0,
      "total": 5900.00
    }
  ],
  "totals": {
    "subtotal": 5000.00, "total_discount": 0,
    "taxable_amount": 5000.00,
    "cgst": 450.00, "sgst": 450.00, "igst": 0, "cess": 0,
    "round_off": 0, "total": 5900.00,
    "amount_in_words": "Five Thousand Nine Hundred Rupees Only"
  },
  "payment": {
    "bank_name": "HDFC Bank",
    "account_number": "...",
    "ifsc_code": "HDFC0001234",
    "payment_terms": "Net 30"
  },
  "notes": "Thank you for your business"
}
```

---

## Authentication & Authorization

- **JWT** with HS256 signing. Access tokens expire in 15 minutes, refresh tokens in 7 days.
- **Passwords** hashed with bcrypt (cost 12).
- **Tenant isolation**: enforced at JWT claims level and database query level. Users log in with their tenant slug.

### Tenant Role Hierarchy

Users are assigned one of four tenant-level roles. Each role provides implicit collection access that combines with explicit per-collection permissions.

| Tenant Role | Implicit Collection Access | Can Upload Files | Can Create Collections | Can Delete Collections | Can Manage Users |
|-------------|---------------------------|-----------------|----------------------|----------------------|-----------------|
| `admin` | owner (full access everywhere) | Yes | Yes | Yes | Yes |
| `manager` | editor (view + edit everywhere) | Yes | Yes | No | No |
| `member` | viewer (view everywhere, edit only where explicitly granted) | Yes | Yes | No | No |
| `viewer` | none (only explicit collection permissions, capped at viewer) | No | No | No | No |

**Effective permission** = `max(implicit_from_role, explicit_collection_permission)`

- **admin** bypasses all collection permission checks (implicit owner on every collection)
- **manager** can view and edit any collection without explicit permission, but cannot delete collections or manage permissions
- **member** can view any collection, but needs explicit editor/owner permission to modify content
- **viewer** has zero implicit access; needs explicit per-collection permissions for everything, and effective permission is capped at viewer level (read-only regardless of what's granted)

## Error Codes

All errors are returned in the standard response envelope with a `code` and `message`. Common codes at a glance:

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `UNAUTHORIZED` | 401 | Missing or invalid token |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `INSUFFICIENT_ROLE` | 403 | Tenant role too low for this action |
| `NOT_FOUND` | 404 | Resource not found |
| `INVALID_REQUEST` | 400 | Malformed request body or params |
| `INTERNAL_ERROR` | 500 | Unhandled server error |

For the complete list organized by domain (auth, files, collections, documents, validation), see **[ERROR_CODES.md](ERROR_CODES.md)**.

## Docker

**Build**:

```bash
docker build -t satvos:latest .
```

Multi-stage build: `golang:1.24-alpine` (build) -> `alpine:3.20` (runtime). Produces a minimal image with the server binary, migrate binary, migration files, and a startup script that runs migrations before starting the server.

---

## Additional Documentation

| Document | Description |
|----------|-------------|
| **[API.md](API.md)** | Complete API reference with request/response examples, TypeScript types, and integration guide |
| **[VALIDATION.md](VALIDATION.md)** | Detailed validation rules reference (50 built-in GST rules) |
| **[ERROR_CODES.md](ERROR_CODES.md)** | Complete error codes reference organized by domain |
| **[FRONTEND_GUIDE.md](FRONTEND_GUIDE.md)** | Comprehensive guide for building a frontend application |
| **[CLAUDE.md](CLAUDE.md)** | Project context for AI assistants (architecture, conventions, gotchas)
