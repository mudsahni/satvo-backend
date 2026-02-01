# Error Codes Reference

All SATVOS API responses use a standard envelope. Error responses include a `code` and `message`:

```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "NOT_FOUND",
    "message": "resource not found"
  }
}
```

## Table of Contents

- [Authentication Errors](#authentication-errors)
- [Tenant Errors](#tenant-errors)
- [User Errors](#user-errors)
- [File Errors](#file-errors)
- [Collection Errors](#collection-errors)
- [Document Errors](#document-errors)
- [Validation Errors](#validation-errors)
- [General Errors](#general-errors)
- [Error Handling Internals](#error-handling-internals)

---

## Authentication Errors

| Code | HTTP Status | Message | When |
|------|-------------|---------|------|
| `UNAUTHORIZED` | 401 | unauthorized | Missing, expired, or malformed JWT token |
| `INVALID_CREDENTIALS` | 401 | invalid credentials | Wrong email or password during login |
| `FORBIDDEN` | 403 | forbidden | Authenticated user lacks the required role (e.g., member trying an admin-only endpoint) |

---

## Tenant Errors

| Code | HTTP Status | Message | When |
|------|-------------|---------|------|
| `TENANT_INACTIVE` | 403 | tenant is inactive | Tenant has been deactivated by an admin |
| `DUPLICATE_SLUG` | 409 | tenant slug already exists | Creating a tenant with a slug that's already taken |
| `NOT_FOUND` | 404 | resource not found | Tenant ID does not exist |

---

## User Errors

| Code | HTTP Status | Message | When |
|------|-------------|---------|------|
| `DUPLICATE_EMAIL` | 409 | email already exists for this tenant | Creating a user with an email already registered under the same tenant |
| `USER_INACTIVE` | 403 | user is inactive | User account has been deactivated |
| `NOT_FOUND` | 404 | resource not found | User ID does not exist within the tenant |

---

## File Errors

| Code | HTTP Status | Message | When |
|------|-------------|---------|------|
| `UNSUPPORTED_FILE_TYPE` | 400 | unsupported file type; allowed: pdf, jpg, png | File extension or content type not in whitelist (PDF, JPG/JPEG, PNG) |
| `FILE_TOO_LARGE` | 413 | file exceeds maximum allowed size | File exceeds `SATVOS_S3_MAX_FILE_SIZE_MB` (default 50 MB) |
| `UPLOAD_FAILED` | 500 | file upload to storage failed | S3 upload failed (network error, permissions, etc.) |
| `NOT_FOUND` | 404 | resource not found | File ID does not exist within the tenant |

---

## Collection Errors

| Code | HTTP Status | Message | When |
|------|-------------|---------|------|
| `COLLECTION_NOT_FOUND` | 404 | collection not found | Collection ID does not exist within the tenant |
| `COLLECTION_PERMISSION_DENIED` | 403 | insufficient collection permission | User doesn't have the required permission level (owner/editor/viewer) for the action |
| `DUPLICATE_COLLECTION_FILE` | 409 | file already exists in collection | Adding a file that's already associated with the collection |
| `SELF_PERMISSION_REMOVAL` | 400 | cannot remove your own permission | Owner attempting to remove their own permission on a collection |
| `INVALID_PERMISSION` | 400 | invalid collection permission; allowed: owner, editor, viewer | Permission value is not one of the three valid levels |

### Collection Permission Requirements

| Action | Minimum Permission |
|--------|--------------------|
| View collection / list files | `viewer` |
| Add / remove files | `editor` |
| Update collection metadata | `owner` |
| Delete collection | `owner` |
| Manage permissions | `owner` |

---

## Document Errors

| Code | HTTP Status | Message | When |
|------|-------------|---------|------|
| `DOCUMENT_NOT_FOUND` | 404 | document not found | Document ID does not exist within the tenant |
| `DOCUMENT_ALREADY_EXISTS` | 409 | document already exists for this file | Creating a document for a file that already has one |
| `DOCUMENT_NOT_PARSED` | 400 | document has not been parsed yet | Attempting to review, validate, or retrieve validation results before parsing completes |

### Document Status Values

| Field | Values | Description |
|-------|--------|-------------|
| `parsing_status` | `pending`, `processing`, `completed`, `failed` | LLM parsing progress |
| `review_status` | `pending`, `approved`, `rejected` | Human review status |
| `validation_status` | `pending`, `valid`, `warning`, `invalid` | Automated validation result |

---

## Validation Errors

| Code | HTTP Status | Message | When |
|------|-------------|---------|------|
| `VALIDATION_RULE_NOT_FOUND` | 404 | validation rule not found | Referencing a validation rule ID that does not exist |

For details on validation rules and statuses, see **[VALIDATION.md](VALIDATION.md)**.

---

## General Errors

| Code | HTTP Status | Message | When |
|------|-------------|---------|------|
| `NOT_FOUND` | 404 | resource not found | Generic resource not found (any entity) |
| `INVALID_REQUEST` | 400 | *(varies)* | Request body validation failed (missing required fields, malformed JSON, invalid query params) |
| `INTERNAL_ERROR` | 500 | an internal error occurred | Unhandled server error (details logged server-side, not exposed to client) |

---

## Error Handling Internals

Errors flow through the system as follows:

```
Domain Layer                Handler Layer              HTTP Response
─────────────              ─────────────              ─────────────
domain.ErrNotFound     ──>  MapDomainError()      ──>  404 NOT_FOUND
domain.ErrForbidden    ──>  MapDomainError()      ──>  403 FORBIDDEN
domain.ErrDocumentXxx  ──>  MapDomainError()      ──>  4xx DOCUMENT_XXX
(unknown error)        ──>  MapDomainError()      ──>  500 INTERNAL_ERROR
```

- **Domain errors** are defined as sentinel values in `internal/domain/errors.go`
- **Error mapping** is handled by `MapDomainError()` in `internal/handler/response.go`
- **5xx errors** are logged with the request ID but the actual error message is not exposed to the client
- **4xx errors** return the domain error message directly

### Adding a New Error Code

1. Define a new sentinel error in `internal/domain/errors.go`:
   ```go
   ErrMyNewError = errors.New("description of the error")
   ```

2. Add a case to `MapDomainError()` in `internal/handler/response.go`:
   ```go
   case errors.Is(err, domain.ErrMyNewError):
       return http.StatusBadRequest, "MY_NEW_ERROR", "human-readable message"
   ```

3. Return the domain error from the appropriate service method
