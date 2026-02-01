# CLAUDE.md — Project Context for Claude Code

## Project Overview

SATVOS is a multi-tenant document processing service written in Go. It provides tenant-isolated file management with JWT authentication, role-based access control (admin/manager/member/viewer), and AWS S3 storage. It supports document collections with permission-based access control (owner/editor/viewer) for grouping files, LLM-powered document parsing that extracts structured invoice data from uploaded PDFs and images, and an automated validation engine with 50+ built-in GST invoice rules. The architecture follows the hexagonal (ports & adapters) pattern.

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
cmd/server/main.go           Entry point — wires config, DB, storage, services, validator engine, router
cmd/migrate/main.go          Migration CLI (up/down/steps/version)

internal/
  config/config.go           Loads env vars (SATVOS_ prefix) via viper
  domain/
    models.go                Tenant, User, FileMeta, Collection, CollectionPermissionEntry,
                             CollectionFile, Document, DocumentTag, DocumentValidationRule structs
    enums.go                 FileType (pdf/jpg/png), UserRole (admin/manager/member/viewer), FileStatus,
                             CollectionPermission (owner/editor/viewer),
                             ParsingStatus (pending/processing/completed/failed),
                             ReviewStatus (pending/approved/rejected),
                             ValidationStatus (pending/valid/warning/invalid),
                             ValidationSeverity (error/warning),
                             ValidationRuleType (required_field/regex/sum_check/cross_field/custom),
                             FieldValidationStatus (valid/invalid/unsure)
    errors.go                Sentinel errors (ErrNotFound, ErrForbidden, ErrInsufficientRole, ErrCollectionNotFound,
                             ErrDocumentNotFound, ErrDocumentNotParsed, ErrValidationRuleNotFound, etc.)
  handler/
    auth_handler.go          POST /auth/login, POST /auth/refresh
    file_handler.go          POST /files/upload (optional collection_id), GET /files, GET /files/:id, DELETE /files/:id
    collection_handler.go    CRUD /collections, batch file upload, permission management
    document_handler.go      POST /documents, GET /documents, GET /documents/:id,
                             POST /documents/:id/retry, PUT /documents/:id/review,
                             POST /documents/:id/validate, GET /documents/:id/validation,
                             DELETE /documents/:id
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
    document_service.go      Document CRUD, background LLM parsing, retry, review status,
                             validation orchestration (ValidateDocument, GetValidation),
                             collection permission checks via collectionPermRepo
    user_service.go          User CRUD with tenant scoping
    tenant_service.go        Tenant CRUD
  port/
    repository.go            TenantRepository, UserRepository, FileMetaRepository interfaces
    collection_repository.go CollectionRepository, CollectionPermissionRepository, CollectionFileRepository interfaces
    document_repository.go   DocumentRepository (incl. UpdateValidationResults),
                             DocumentTagRepository, DocumentValidationRuleRepository interfaces
    document_parser.go       DocumentParser interface (Parse) with ParseInput/ParseOutput DTOs
    storage.go               ObjectStorage interface (Upload, Download, Delete, GetPresignedURL)
  repository/postgres/
    db.go                    Database connection setup (sqlx + pgx)
    tenant_repo.go           Tenant SQL queries
    user_repo.go             User SQL queries (tenant-scoped)
    file_meta_repo.go        File metadata SQL queries
    collection_repo.go       Collection SQL queries (ListByUser joins permissions, ListByTenant for admin/manager/member)
    collection_permission_repo.go  Collection permission queries (upsert via ON CONFLICT)
    collection_file_repo.go  Collection-file association queries (joins file_metadata)
    document_repo.go         Document CRUD queries (incl. UpdateValidationResults)
    document_tag_repo.go     Document tag queries (batch create, search by tag)
    document_validation_rule_repo.go  Validation rule queries (builtin key listing, scoped loading)
  storage/s3/
    s3_client.go             S3 implementation (supports LocalStack endpoint)
  parser/claude/
    claude_parser.go         Anthropic Messages API parser (PDF as document blocks, images as image blocks)
  validator/
    engine.go                Validation orchestrator — loads rules, runs validators, computes status,
                             auto-seeds builtin rules per tenant, returns ValidationResponse
    validator.go             Validator interface (Validate, RuleKey, RuleName, RuleType, Severity)
    registry.go              Map-based validator registry (Register, Get, All)
    field_status.go          Computes per-field status from rule results + confidence scores
    invoice/                 GST invoice validators (50 built-in rules):
      types.go               GSTInvoice, Party, LineItem, Totals, Payment, ConfidenceScores structs
      builtin_rules.go       AllBuiltinValidators() — collects all validators into BuiltinValidator wrappers
      required.go            12 required field validators (invoice number, GSTIN, state codes, etc.)
      format.go              13 format validators (GSTIN regex, PAN, IFSC, HSN/SAC, dates, currency, state codes)
      math.go                11 mathematical validators (line item arithmetic, totals reconciliation)
      crossfield.go          7 cross-field validators (GSTIN-state match, GSTIN-PAN match, tax type consistency)
      logical.go             7 logical validators (non-negative amounts, valid GST rates, exclusive tax type)
  router/
    router.go                Route definitions, middleware wiring
  mocks/                     Generated mocks (uber/mock) for testing

tests/unit/                  Unit tests for handlers, services, middleware, validators
db/migrations/               7 SQL migrations: tenants -> users -> file_metadata -> collections
                             -> documents -> validation columns -> consolidated validation results
```

## Data Flow

```
Request -> Gin Router -> Middleware (Logger -> Auth -> TenantGuard)
  -> Handler -> Service -> Repository/Storage -> PostgreSQL/S3
```

## Document Processing Pipeline

```
                              ┌─────────────────────────────────────────┐
                              │            DOCUMENT LIFECYCLE           │
                              └─────────────────────────────────────────┘

  POST /files/upload          POST /documents              Background Goroutine
  ┌──────────────┐           ┌──────────────┐           ┌──────────────────────────┐
  │ Upload File  │──────────>│ Create Doc   │──────────>│ Parse (LLM)              │
  │ to S3        │           │ parsing:     │  go func  │ parsing: processing      │
  │              │           │ pending      │           │                          │
  └──────────────┘           └──────────────┘           │ 1. Download from S3      │
                                                        │ 2. Send to Claude API    │
                                                        │ 3. Save structured_data  │
                                                        │ 4. Save confidence_scores│
                                                        │ parsing: completed/failed│
                                                        └────────────┬─────────────┘
                                                                     │ auto-trigger
                                                        ┌────────────v─────────────┐
  GET /documents/:id/validation                         │ Validate                 │
  ┌──────────────┐                                      │ validation: pending      │
  │ View Results │<─────────────────────────────────────│                          │
  │ per-rule     │                                      │ 1. Ensure builtin rules  │
  │ per-field    │                                      │ 2. Load active rules     │
  │ summary      │                                      │ 3. Run 50+ validators    │
  └──────────────┘                                      │ 4. Compute field status  │
                                                        │ 5. Save JSONB results    │
  PUT /documents/:id/review                             │ validation: valid/       │
  ┌──────────────┐                                      │   warning/invalid        │
  │ Approve or   │                                      └──────────────────────────┘
  │ Reject       │
  │ review:      │
  │ approved/    │
  │ rejected     │
  └──────────────┘
```

## Validation Engine Architecture

```
  ┌─────────────────────────────────────────────────────────────────┐
  │                    validator.Engine                              │
  │                                                                 │
  │  ValidateDocument(ctx, tenantID, docID)                        │
  │    1. EnsureBuiltinRules() — auto-seed missing rules           │
  │    2. Load active rules from document_validation_rules table    │
  │    3. For each rule:                                            │
  │       - Look up validator in Registry by builtin_rule_key       │
  │       - Run Validator.Validate(ctx, *GSTInvoice)               │
  │       - Collect ValidationResult (passed, field_path, message)  │
  │    4. Determine overall status:                                 │
  │       - Any error failure → ValidationStatusInvalid             │
  │       - Only warning failures → ValidationStatusWarning         │
  │       - All passed → ValidationStatusValid                      │
  │    5. Save results as JSONB to documents.validation_results     │
  │                                                                 │
  │  GetValidation(ctx, tenantID, docID)                           │
  │    1. Load document (structured_data, confidence_scores, etc.)  │
  │    2. Load validation rules                                     │
  │    3. ComputeFieldStatuses (results + confidence → per-field)   │
  │    4. Return ValidationResponse (summary, results, statuses)    │
  └─────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              │               │               │
              v               v               v
  ┌───────────────┐  ┌──────────────┐  ┌──────────────┐
  │   Registry    │  │  Rule Repo   │  │  Doc Repo    │
  │ map[key]      │  │ (PostgreSQL) │  │ (PostgreSQL) │
  │  Validator    │  │              │  │              │
  └───────────────┘  └──────────────┘  └──────────────┘
```

### Validator Categories (50 built-in rules)

| Category | Count | Rule Type | Severity | Key Prefix | File |
|----------|-------|-----------|----------|------------|------|
| Required Fields | 12 | `required_field` | Error/Warning | `req.*` | `invoice/required.go` |
| Format | 13 | `regex` | Error/Warning | `fmt.*` | `invoice/format.go` |
| Mathematical | 11 | `sum_check` | Error/Warning | `math.*` | `invoice/math.go` |
| Cross-field | 7 | `cross_field` | Error/Warning | `xf.*` | `invoice/crossfield.go` |
| Logical | 7 | `custom` | Error/Warning | `logic.*` | `invoice/logical.go` |

### Field Status Computation (`field_status.go`)

Per-field status is determined by combining validation results and confidence scores:
- Any **error**-severity rule fails for field → `invalid`
- Any **warning**-severity rule fails for field → `unsure`
- Confidence score ≤ 0.5 (even if rules pass) → `unsure`
- Otherwise → `valid`

### GST Invoice Type Model (`invoice/types.go`)

The `GSTInvoice` struct mirrors the LLM parser output:
- `Invoice` — header (number, date, due_date, type, currency, place_of_supply, reverse_charge)
- `Seller` / `Buyer` — party (name, address, gstin, pan, state, state_code)
- `LineItems[]` — (description, hsn_sac_code, quantity, unit, unit_price, discount, taxable_amount, cgst/sgst/igst rate+amount, total)
- `Totals` — (subtotal, total_discount, taxable_amount, cgst, sgst, igst, cess, round_off, total, amount_in_words)
- `Payment` — (bank_name, account_number, ifsc_code, payment_terms)
- Parallel `ConfidenceScores` struct with 0.0-1.0 float64 per field

### Auto-seeding Builtin Rules

On first validation for a tenant+document_type:
1. `EnsureBuiltinRules()` queries existing `builtin_rule_key` values
2. For each registered validator not yet in DB, creates a `DocumentValidationRule` row
3. Uses unique index `(tenant_id, builtin_rule_key)` to prevent duplicates
4. Rules are tenant-scoped and can be individually toggled (`is_active`)

### Validation Result Storage

Results are stored as JSONB directly on the `documents` table (`validation_results` column), not in a separate table. Each entry:
```json
{"rule_id": "uuid", "passed": true, "field_path": "seller.gstin",
 "expected_value": "non-empty", "actual_value": "29ABCDE1234F1Z5",
 "message": "...", "validated_at": "2025-01-15T10:30:00Z"}
```

## Key Conventions

- **Environment config**: All vars prefixed with `SATVOS_` (e.g., `SATVOS_DB_HOST`). Loaded by viper in `internal/config/config.go`.
- **Response envelope**: All HTTP responses use `{"success": bool, "data": ..., "error": ..., "meta": ...}` defined in `handler/response.go`.
- **Tenant isolation**: Every DB query includes `tenant_id` from JWT claims. Users authenticate with tenant slug + email + password.
- **Error handling**: Domain errors in `domain/errors.go` are mapped to HTTP status codes in `handler/response.go`.
- **File validation**: Extension whitelist (pdf/jpg/jpeg/png) + magic bytes content-type detection.
- **S3 key format**: `tenants/{tenant_id}/files/{file_id}/{original_filename}`
- **Tenant roles**: 4-tier hierarchy — admin (level 4, implicit owner), manager (level 3, implicit editor), member (level 2, implicit viewer), viewer (level 1, no implicit access). Effective permission = `max(implicit_from_role, explicit_collection_perm)`. Viewer role is capped at viewer-level regardless of explicit grants. Helper functions in `domain/enums.go`: `RoleLevel()`, `ImplicitCollectionPerm()`, `ValidUserRoles`.
- **Collections**: Permission-based access (owner/editor/viewer) combined with tenant role hierarchy. Owner can manage permissions and delete. Editor can add/remove files. Viewer can read. Admin bypasses all permission checks. Files can belong to multiple collections or none. Deleting a collection preserves files.
- **Batch upload**: `POST /collections/:id/files` accepts multiple files via multipart `"files"` field. Returns per-file results (207 on partial success).
- **Document parsing**: Background goroutine downloads file from S3, sends to LLM (Claude via direct HTTP to Messages API), saves structured JSON + confidence scores. Status progresses: pending -> processing -> completed/failed. **Important**: `CreateAndParse` and `RetryParse` return a copy of the document struct to avoid data races with the background goroutine.
- **Parser abstraction**: `DocumentParser` interface in `port/document_parser.go`. Claude implementation in `parser/claude/`. Swappable to Gemini/OpenAI by implementing the interface.
- **Validation**: Runs automatically after parsing completes. 50 built-in GST invoice rules across 5 categories. Rules stored in `document_validation_rules` table (per-tenant, optionally per-collection). Results stored as JSONB on `documents.validation_results`. Status: pending -> valid/warning/invalid.
- **Review workflow**: Documents start with `review_status=pending`. After parsing completes, users can approve or reject with notes.
- **Pagination**: `offset` and `limit` query params on list endpoints.
- **Testing**: Unit tests use testify assertions and uber/mock for interface mocking. Tests run with `-race` flag in CI.
- **Database**: PostgreSQL with sqlx for query mapping and pgx/v5 as the driver. Parameterized queries throughout.
- **Passwords**: bcrypt with cost 12, minimum 8 characters.
- **JWT**: HS256 signing. Access token 15m, refresh token 7d. Claims carry tenant_id, user_id, email, role.
- **Concurrency safety**: Background goroutines (parsing) operate on their own document pointers. Callers receive a copy to prevent shared state. The `go` statement provides a happens-before edge for the copy.

## Tech Stack

- Go 1.24, Gin, PostgreSQL 16 (sqlx + pgx/v5), AWS S3 (aws-sdk-go-v2), JWT (golang-jwt/v5), bcrypt, Viper, golang-migrate, Docker/Docker Compose, LocalStack (dev S3), Anthropic Claude API (document parsing)

## Important Files for Common Tasks

- **Adding an endpoint**: `internal/router/router.go` (routes), then create handler in `internal/handler/`, service in `internal/service/`
- **Adding a domain model**: `internal/domain/models.go`, then repo interface in `internal/port/`, implementation in `internal/repository/postgres/`
- **Adding a migration**: `db/migrations/` (sequential numbered SQL files, up + down)
- **Modifying config**: `internal/config/config.go` (struct + viper binding), `.env.example`
- **Adding middleware**: `internal/middleware/`, wire it in `internal/router/router.go`
- **Adding a new parser provider**: Implement `port.DocumentParser` interface in `internal/parser/<provider>/`, wire in `cmd/server/main.go` based on `cfg.Parser.Provider`
- **Adding a new validation rule**: Create validator in `internal/validator/invoice/`, add to the appropriate `*Validators()` function, it auto-registers via `AllBuiltinValidators()`
- **Adding a new document type**: Create typed model in `internal/validator/<type>/types.go`, implement validators, register in `cmd/server/main.go`
- **Modifying validation behavior**: Rules can be toggled per-tenant in `document_validation_rules` table (`is_active` flag). Collection-scoped rules use `collection_id`.

## Gotchas & Past Issues

- **Data races in tests**: The `RetryParse` and `CreateAndParse` methods launch background goroutines. The mock repository returns the same pointer for `GetByID`, so both the caller and goroutine share memory. Fixed by copying the document struct *before* `go` to ensure the caller's value is independent. The `go` statement provides a happens-before edge for the copy.
- **Range value copies**: LineItem (136 bytes), DocumentValidationRule (208 bytes) — always use `for i := range` with pointer indexing (`item := &slice[i]`) to avoid per-iteration copies. Enforced by golangci-lint `gocritic.rangeValCopy`.
- **Builtin shadowing**: Avoid naming parameters `max`, `min`, `len`, `cap`, etc. — Go's `gocritic.builtinShadow` catches these.
- **CI runs tests with `-race`**: The GitHub Actions CI enables the Go race detector. Tests that pass locally without `-race` may fail in CI.
- **Validation results are JSONB, not a separate table**: Migration 007 consolidated results from a separate `document_validation_results` table into `documents.validation_results` JSONB column.
