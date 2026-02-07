# CLAUDE.md — Project Context for Claude Code

## Project Overview

SATVOS is a multi-tenant document processing service written in Go. It provides tenant-isolated file management with JWT authentication, role-based access control (admin/manager/member/viewer), and AWS S3 storage. It supports document collections with permission-based access control (owner/editor/viewer) for grouping files, LLM-powered document parsing that extracts structured invoice data from uploaded PDFs and images (with multi-provider support and optional dual-parse merge mode), an automated validation engine with 50+ built-in GST invoice rules, reconciliation tiering that classifies validation rules as reconciliation-critical for GSTR-2A/2B matching, and a document tagging system with user-provided and auto-generated tags from parsed invoice data. The architecture follows the hexagonal (ports & adapters) pattern.

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
    models.go                Tenant, User, FileMeta, Collection (with DocumentCount, computed via SQL subquery),
                             CollectionPermissionEntry, CollectionFile, Document (with Name, SecondaryParserModel, ParseAttempts, RetryAfter fields),
                             DocumentTag (with Source field), DocumentValidationRule structs
    enums.go                 FileType (pdf/jpg/png), UserRole (admin/manager/member/viewer), FileStatus,
                             CollectionPermission (owner/editor/viewer),
                             ParsingStatus (pending/processing/completed/failed/queued),
                             ReviewStatus (pending/approved/rejected),
                             ValidationStatus (pending/valid/warning/invalid),
                             ReconciliationStatus (pending/valid/warning/invalid),
                             ParseMode (single/dual),
                             ValidationSeverity (error/warning),
                             ValidationRuleType (required_field/regex/sum_check/cross_field/custom),
                             FieldValidationStatus (valid/invalid/unsure)
    errors.go                Sentinel errors (ErrNotFound, ErrForbidden, ErrInsufficientRole, ErrCollectionNotFound,
                             ErrDocumentNotFound, ErrDocumentNotParsed, ErrValidationRuleNotFound,
                             ErrInvalidStructuredData, etc.)
  handler/
    auth_handler.go          POST /auth/login, POST /auth/refresh
    file_handler.go          POST /files/upload (optional collection_id), GET /files, GET /files/:id, DELETE /files/:id
    collection_handler.go    CRUD /collections, batch file upload, permission management
    document_handler.go      POST /documents, GET /documents, GET /documents/:id,
                             PUT /documents/:id (edit structured data),
                             POST /documents/:id/retry, PUT /documents/:id/review,
                             PUT /documents/:id/structured-data (alias),
                             POST /documents/:id/validate, GET /documents/:id/validation,
                             GET /documents/:id/tags, POST /documents/:id/tags,
                             DELETE /documents/:id/tags/:tagId,
                             GET /documents/search/tags, DELETE /documents/:id
    user_handler.go          CRUD /users, /users/:id
    tenant_handler.go        CRUD /admin/tenants, /admin/tenants/:id
    stats_handler.go         GET /stats (tenant-scoped aggregate counts, role-filtered)
    health_handler.go        GET /healthz, GET /readyz
    response.go              Standard envelope (success/data/error/meta) + error mapping
  middleware/
    auth.go                  JWT validation, injects tenant_id/user_id/role into context
    cors.go                  CORS handling with configurable allowed origins (SATVOS_CORS_ALLOWED_ORIGINS)
    tenant.go                Tenant context guard
    logger.go                Request ID injection, logging, panic recovery
  service/
    auth_service.go          Login (bcrypt verify), JWT generation/refresh
    file_service.go          Upload (validate + S3 + DB), download URL, delete
    collection_service.go    Collection CRUD, batch upload, permission checking, file association
    document_service.go      Document CRUD, background LLM parsing, retry, review status,
                             validation orchestration (ValidateDocument, GetValidation),
                             collection permission checks via collectionPermRepo,
                             tag management (ListTags, AddTags, DeleteTag, SearchByTag),
                             auto-tag extraction from parsed invoice data,
                             ParseDocument (core parse logic used by background and queue worker),
                             handleParseError (rate-limit detection → queued status)
    parse_queue_worker.go    Background worker that polls for queued documents and dispatches parsing
                             with bounded concurrency (semaphore) and clean shutdown (WaitGroup)
    stats_service.go         Aggregate stats (role-branching: tenant-wide vs user-scoped)
    user_service.go          User CRUD with tenant scoping
    tenant_service.go        Tenant CRUD
  port/
    repository.go            TenantRepository, UserRepository, FileMetaRepository interfaces
    collection_repository.go CollectionRepository, CollectionPermissionRepository, CollectionFileRepository interfaces
    document_repository.go   DocumentRepository (incl. UpdateValidationResults, ClaimQueued),
                             DocumentTagRepository (incl. DeleteByDocumentAndSource),
                             DocumentValidationRuleRepository interfaces
    stats_repository.go      StatsRepository interface (GetTenantStats, GetUserStats)
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
    document_repo.go         Document CRUD queries (incl. UpdateValidationResults, ClaimQueued)
    document_tag_repo.go     Document tag queries (batch create, search by tag)
    document_validation_rule_repo.go  Validation rule queries (builtin key listing, scoped loading)
    stats_repo.go            Aggregate stats queries (conditional COUNTs on documents + collections)
  storage/s3/
    s3_client.go             S3 implementation (supports LocalStack endpoint)
  parser/
    factory.go               Parser provider registry and factory (RegisterProvider, NewParser)
    prompt.go                Shared GST invoice extraction prompt (BuildGSTInvoicePrompt)
    merge.go                 MergeParser — wraps two DocumentParsers, runs in parallel, merges results
    fallback.go              FallbackParser — tries parsers in order with per-parser circuit breaker
    errors.go                RateLimitError type + ParseRetryAfterHeader helper
    claude/
      claude_parser.go       Anthropic Messages API parser (PDF as document blocks, images as image blocks)
    gemini/
      gemini_parser.go       Google Gemini parser (Gemini REST API)
    openai/
      openai_parser.go       OpenAI Chat Completions API parser (images as data URI, JSON mode)
  validator/
    engine.go                Validation orchestrator — loads rules, runs validators, computes
                             validation_status + reconciliation_status, auto-seeds builtin rules,
                             returns ValidationResponse with reconciliation summary
    validator.go             Validator interface (Validate, RuleKey, RuleName, RuleType, Severity,
                             ReconciliationCritical)
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

tests/unit/                  Unit tests for handlers, services, middleware, validators, parsers, config
db/migrations/               12 SQL migrations: tenants -> users -> file_metadata -> collections
                             -> documents -> validation columns -> consolidated validation results
                             -> reconciliation tiering -> multi-parser fields
                             -> document name + tag source
                             -> secondary_parser_model + parse_attempts
                             -> parse queue (retry_after column + partial index)
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
                                                        │ 2. Send to LLM parser    │
                                                        │ 3. Save structured_data  │
                                                        │ 4. Save confidence_scores│
                                                        │ 5. Extract auto-tags     │
                                                        │ parsing: completed       │
                                                        │         /failed/queued   │
                                                        └──────┬──────────┬────────┘
                                                               │          │
                                                   on success  │          │ on RateLimitError
                                                               │          │ (all parsers 429)
                                                               │          v
                                                               │  ┌──────────────────┐
                                                               │  │ Queue for Retry   │
                                                               │  │ parsing: queued   │
                                                               │  │ retry_after: T+Ns │
                                                               │  └────────┬─────────┘
                                                               │           │
                                                               │  ParseQueueWorker    │
                                                               │  (polls every 10s)   │
                                                               │           │
                                                               │           v
                                                               │  ┌──────────────────┐
                                                               │  │ Re-dispatch Parse │
                                                               │  │ (bounded concurr.)│
                                                               │  │ max 5 attempts    │
                                                               │  └──────────────────┘
                                                               │
                                                     auto-trigger
                                                        ┌──────v──────────────────┐
  GET /documents/:id/validation                         │ Validate                │
  ┌──────────────┐                                      │ validation: pending     │
  │ View Results │<─────────────────────────────────────│                         │
  │ per-rule     │                                      │ 1. Ensure builtin rules │
  │ per-field    │                                      │ 2. Load active rules    │
  │ summary      │                                      │ 3. Run 50+ validators   │
  └──────────────┘                                      │ 4. Compute field status │
                                                        │ 5. Save JSONB results   │
  PUT /documents/:id/review                             │ validation: valid/      │
  ┌──────────────┐                                      │   warning/invalid       │
  │ Approve or   │                                      └─────────────────────────┘
  │ Reject       │                                                 ^
  │ review:      │                                                 │
  │ approved/    │                                                 │ re-validate
  │ rejected     │                                                 │
  └──────────────┘           PUT /documents/:id                     │
                             PUT /documents/:id/structured-data    │
                             ┌──────────────────────────┐          │
                             │ Manual Edit              │──────────┘
                             │ 1. Validate JSON schema  │
                             │ 2. Set confidence → 1.0  │
                             │ 3. Reset review status   │
                             │ 4. Re-extract auto-tags  │
                             │ 5. Re-run validation     │
                             │ provenance: manual_edit   │
                             └──────────────────────────┘
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
  │       - Track reconciliation_critical flag per result           │
  │    4. Determine overall validation_status:                      │
  │       - Any error failure → ValidationStatusInvalid             │
  │       - Only warning failures → ValidationStatusWarning         │
  │       - All passed → ValidationStatusValid                      │
  │    5. Determine reconciliation_status (from critical rules only)│
  │       - Any recon-critical error → ReconciliationStatusInvalid  │
  │       - Only recon-critical warnings → Warning                  │
  │       - All recon-critical passed → ReconciliationStatusValid   │
  │    6. Save results as JSONB to documents.validation_results     │
  │                                                                 │
  │  GetValidation(ctx, tenantID, docID)                           │
  │    1. Load document (structured_data, confidence_scores, etc.)  │
  │    2. Load validation rules                                     │
  │    3. ComputeFieldStatuses (results + confidence → per-field)   │
  │    4. Return ValidationResponse (summary, reconciliation        │
  │       summary, results with reconciliation_critical flag,       │
  │       field statuses)                                           │
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
 "message": "...", "reconciliation_critical": true,
 "validated_at": "2025-01-15T10:30:00Z"}
```

### Reconciliation Tiering

21 of the 50 validation rules are classified as **reconciliation-critical** for GSTR-2A/2B matching. These are the fields the GST portal uses for invoice matching: supplier GSTIN, invoice number/date, total value, taxable value, CGST/SGST/IGST amounts, place of supply, and reverse charge.

The `reconciliation_status` field on documents is computed independently from `validation_status`:
- Only reconciliation-critical rule failures affect `reconciliation_status`
- Non-critical failures (e.g., missing PAN, payment details, HSN codes) affect `validation_status` but leave `reconciliation_status` valid

Each validator implements `ReconciliationCritical() bool`. Each `DocumentValidationRule` row has a `reconciliation_critical` boolean. The engine tracks recon errors/warnings separately and saves the result to `documents.reconciliation_status`.

**Reconciliation-critical rule keys** (21 total): `req.invoice.number`, `req.invoice.date`, `req.invoice.place_of_supply`, `req.seller.name`, `req.seller.gstin`, `req.buyer.gstin`, `fmt.seller.gstin`, `fmt.buyer.gstin`, `fmt.seller.state_code`, `fmt.buyer.state_code`, `math.totals.taxable_amount`, `math.totals.cgst`, `math.totals.sgst`, `math.totals.igst`, `math.totals.grand_total`, `xf.seller.gstin_state`, `xf.buyer.gstin_state`, `xf.tax_type.intrastate`, `xf.tax_type.interstate`, `logic.line_items.at_least_one`, `logic.line_item.exclusive_tax`.

### Multi-Parser Architecture

The system supports multiple LLM parser backends with an opt-in dual-parse merge mode:

```
  ParserConfig (config.go)
    Primary + Secondary + Tertiary
         │
  Parser Factory (factory.go)
    RegisterProvider() / NewParser()
         │
    ┌────┼────┐
    │    │    │
  Claude Gemini OpenAI
  (live) (live) (live)
         │
  FallbackParser (fallback.go)
    tries parsers in order
    per-parser circuit breaker
    skips rate-limited parsers
         │
  MergeParser (merge.go)
    wraps 2 FallbackParsers
    runs both in parallel
    merges field-by-field
```

**Parse modes** (`parse_mode` field on documents):
- `single` (default): Uses FallbackParser wrapping [primary, secondary, tertiary]
- `dual`: Uses MergeParser wrapping two FallbackParsers: [primary, tertiary] + [secondary, tertiary]

**Merge strategy** (in `internal/parser/merge.go`):
- **Agreement**: Both extract same value → use it, confidence boosted by 20%
- **One empty**: Use the non-empty value with its confidence
- **Disagreement**: Prefer value matching expected format (e.g., GSTIN regex), reduce confidence, record provenance
- **Line items**: Pick the array from whichever parser has more items (no per-item merging)

**Field provenance** (`field_provenance` JSONB on documents): Records which model provided each field — `"agree"`, `"primary"`, `"secondary"`, `"primary_format"`, `"secondary_format"`, or `"disagreement"`.

**FallbackParser** (in `internal/parser/fallback.go`):
- Tries parsers in order; on any error, falls back to the next parser
- On `RateLimitError` (HTTP 429), opens a per-parser circuit breaker with `RetryAfter` duration
- Open circuits are automatically skipped until `time.Now()` passes `resetAt`
- If all parsers are rate-limited, returns a `RateLimitError` with the earliest retry time
- If all parsers fail with non-rate-limit errors, returns `"all parsers failed: <last error>"`
- Thread-safe via `sync.RWMutex` per circuit; in-memory state (no DB persistence)

**RateLimitError** (in `internal/parser/errors.go`):
- Wraps underlying error with `Provider`, `RetryAfter`, and `Unwrap()` support
- `ParseRetryAfterHeader()` parses `Retry-After` response headers (shared across providers)
- Defaults to 60s retry if header is missing or unparseable

**Config** (`ParserConfig` in `config.go`):
- Legacy flat fields (`SATVOS_PARSER_PROVIDER`, etc.) still work via `PrimaryConfig()` fallback
- Multi-provider: `SATVOS_PARSER_PRIMARY_PROVIDER`, `SATVOS_PARSER_PRIMARY_API_KEY`, `SATVOS_PARSER_SECONDARY_PROVIDER`, `SATVOS_PARSER_TERTIARY_PROVIDER`, etc.
- `TertiaryConfig()` returns nil if `SATVOS_PARSER_TERTIARY_PROVIDER` is not set
- If no secondary configured, dual parse mode falls back to single with log warning

## Key Conventions

- **Environment config**: All vars prefixed with `SATVOS_` (e.g., `SATVOS_DB_HOST`). Loaded by viper in `internal/config/config.go`. Queue worker config: `SATVOS_QUEUE_POLL_INTERVAL_SECS` (default 10), `SATVOS_QUEUE_MAX_RETRIES` (default 5), `SATVOS_QUEUE_CONCURRENCY` (default 5).
- **Response envelope**: All HTTP responses use `{"success": bool, "data": ..., "error": ..., "meta": ...}` defined in `handler/response.go`.
- **Tenant isolation**: Every DB query includes `tenant_id` from JWT claims. Users authenticate with tenant slug + email + password.
- **Error handling**: Domain errors in `domain/errors.go` are mapped to HTTP status codes in `handler/response.go`.
- **File validation**: Extension whitelist (pdf/jpg/jpeg/png) + magic bytes content-type detection.
- **S3 key format**: `tenants/{tenant_id}/files/{file_id}/{original_filename}`
- **Tenant roles**: 4-tier hierarchy — admin (level 4, implicit owner), manager (level 3, implicit editor), member (level 2, implicit viewer), viewer (level 1, no implicit access). Effective permission = `max(implicit_from_role, explicit_collection_perm)`. Viewer role is capped at viewer-level regardless of explicit grants. Helper functions in `domain/enums.go`: `RoleLevel()`, `ImplicitCollectionPerm()`, `ValidUserRoles`.
- **Collections**: Permission-based access (owner/editor/viewer) combined with tenant role hierarchy. Owner can manage permissions and delete. Editor can add/remove files. Viewer can read. Admin bypasses all permission checks. Files can belong to multiple collections or none. Deleting a collection preserves files. `document_count` is computed dynamically via SQL subquery (not a stored column) — always fresh on read.
- **Batch upload**: `POST /collections/:id/files` accepts multiple files via multipart `"files"` field. Returns per-file results (207 on partial success).
- **Document parsing**: Background goroutine downloads file from S3, sends to LLM, saves structured JSON + confidence scores + field provenance, then extracts auto-tags from parsed data. Status progresses: pending -> processing -> completed/failed/queued. Supports `parse_mode`: `single` (default, primary parser) or `dual` (MergeParser runs primary + secondary in parallel). `parse_attempts` is incremented on each parse/retry attempt. `secondary_parser_model` records the secondary model name in dual-parse mode (from `ParseOutput.SecondaryModel`). Core parse logic lives in `ParseDocument` (called by both `parseInBackground` and `ParseQueueWorker`). **Important**: `CreateAndParse` and `RetryParse` return a copy of the document struct to avoid data races with the background goroutine.
- **Parse queue (rate-limit retry)**: When all LLM parsers return `RateLimitError` (HTTP 429), `handleParseError` sets `parsing_status=queued` and `retry_after` to the earliest retry time (instead of permanently failing). A `ParseQueueWorker` polls the DB every 10s via `ClaimQueued` (atomic `UPDATE ... FOR UPDATE SKIP LOCKED`), re-dispatches queued documents with bounded concurrency (semaphore, default 5). Each document gets up to `max_retries` (default 5) parse attempts before permanent failure. Each worker goroutine uses `context.WithTimeout(context.Background(), 5min)` so in-flight parses complete even during shutdown. Config: `SATVOS_QUEUE_POLL_INTERVAL_SECS`, `SATVOS_QUEUE_MAX_RETRIES`, `SATVOS_QUEUE_CONCURRENCY`. **Known limitation**: If the server crashes mid-parse, a document may stay in `processing` indefinitely (staleness detector is a future enhancement).
- **Document naming**: Documents have a `name` field. If not provided at creation, defaults to the uploaded file's `OriginalName`.
- **Document tags**: Key-value pairs with a `source` field (`user` or `auto`). User tags are provided at document creation or via `POST /documents/:id/tags`. Auto-tags are extracted from parsed invoice data (invoice_number, invoice_date, seller_name, seller_gstin, buyer_name, buyer_gstin, invoice_type, place_of_supply, total_amount) after successful parsing. Auto-tags are deleted and regenerated on retry or manual edit. Tags support search via `GET /documents/search/tags?key=...&value=...`.
- **Manual editing**: `PUT /documents/:id` (or alias `PUT /documents/:id/structured-data`) allows users to manually edit parsed invoice data. Validates JSON against GSTInvoice schema, sets all confidence scores to 1.0 (human-verified), resets review status to pending, re-extracts auto-tags, and synchronously re-runs validation. Sets `field_provenance` to `{"source":"manual_edit"}`.
- **Parser abstraction**: `DocumentParser` interface in `port/document_parser.go`. `ParseOutput` includes `FieldProvenance` (map of field → source) and `SecondaryModel` (for audit trail). Claude implementation in `parser/claude/`, Gemini implementation in `parser/gemini/`, OpenAI implementation in `parser/openai/`. New providers register via `parser.RegisterProvider()` in `internal/parser/factory.go`. Shared GST extraction prompt in `parser/prompt.go`. `FallbackParser` in `parser/fallback.go` provides rate-limit-aware failover across providers.
- **Validation**: Runs automatically after parsing completes. 50 built-in GST invoice rules across 5 categories. Rules stored in `document_validation_rules` table (per-tenant, optionally per-collection, with `reconciliation_critical` flag). Results stored as JSONB on `documents.validation_results`. Status: pending -> valid/warning/invalid. Reconciliation status computed independently from only reconciliation-critical rules: pending -> valid/warning/invalid.
- **Review workflow**: Documents start with `review_status=pending`. After parsing completes, users can approve or reject with notes.
- **Pagination**: `offset` and `limit` query params on list endpoints.
- **Testing**: Unit tests use testify assertions and uber/mock for interface mocking. Tests run with `-race` flag in CI.
- **Database**: PostgreSQL with sqlx for query mapping and pgx/v5 as the driver. Parameterized queries throughout.
- **Passwords**: bcrypt with cost 12, minimum 8 characters.
- **JWT**: HS256 signing. Access token 15m, refresh token 7d. Claims carry tenant_id, user_id, email, role.
- **Concurrency safety**: Background goroutines (parsing) operate on their own document pointers. Callers receive a copy to prevent shared state. The `go` statement provides a happens-before edge for the copy.

## Tech Stack

- Go 1.24, Gin, PostgreSQL 16 (sqlx + pgx/v5), AWS S3 (aws-sdk-go-v2), JWT (golang-jwt/v5), bcrypt, Viper, golang-migrate, Docker/Docker Compose, LocalStack (dev S3), Anthropic Claude API (document parsing), Google Gemini API (document parsing), OpenAI API (document parsing)

## Important Files for Common Tasks

- **Adding an endpoint**: `internal/router/router.go` (routes), then create handler in `internal/handler/`, service in `internal/service/`
- **Adding a domain model**: `internal/domain/models.go`, then repo interface in `internal/port/`, implementation in `internal/repository/postgres/`
- **Adding a migration**: `db/migrations/` (sequential numbered SQL files, up + down)
- **Modifying config**: `internal/config/config.go` (struct + viper binding), `.env.example`
- **Adding middleware**: `internal/middleware/`, wire it in `internal/router/router.go`
- **Adding a new parser provider**: Implement `port.DocumentParser` interface in `internal/parser/<provider>/`, register via `parser.RegisterProvider()` in `cmd/server/main.go`, use shared prompt from `parser.BuildGSTInvoicePrompt()`
- **Adding a new validation rule**: Create validator in `internal/validator/invoice/`, add to the appropriate `*Validators()` function, it auto-registers via `AllBuiltinValidators()`
- **Adding a new document type**: Create typed model in `internal/validator/<type>/types.go`, implement validators, register in `cmd/server/main.go`
- **Modifying validation behavior**: Rules can be toggled per-tenant in `document_validation_rules` table (`is_active` flag). Collection-scoped rules use `collection_id`.

## Gotchas & Past Issues

- **Data races in tests**: The `RetryParse` and `CreateAndParse` methods launch background goroutines. The mock repository returns the same pointer for `GetByID`, so both the caller and goroutine share memory. Fixed by copying the document struct *before* `go` to ensure the caller's value is independent. The `go` statement provides a happens-before edge for the copy.
- **Range value copies**: LineItem (136 bytes), DocumentValidationRule (208 bytes) — always use `for i := range` with pointer indexing (`item := &slice[i]`) to avoid per-iteration copies. Enforced by golangci-lint `gocritic.rangeValCopy`.
- **Builtin shadowing**: Avoid naming parameters `max`, `min`, `len`, `cap`, etc. — Go's `gocritic.builtinShadow` catches these.
- **CI runs tests with `-race`**: The GitHub Actions CI enables the Go race detector. Tests that pass locally without `-race` may fail in CI.
- **Validation results are JSONB, not a separate table**: Migration 007 consolidated results from a separate `document_validation_results` table into `documents.validation_results` JSONB column.
- **Import shadow lint (`importShadow`)**: In `document_service.go`, constructor params for parsers must not be named `parser` since it shadows the imported `satvos/internal/parser` package — use `docParser`/`mergeDocParser`. Similarly, test files importing `parser` must use `p` for mock parser variables.
- **Stale processing documents (known limitation)**: If the server crashes mid-parse, a document stays in `processing` status indefinitely. A staleness detector (timeout-based requeue) is a planned future enhancement.
