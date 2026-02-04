# SATVOS API Reference

Complete API documentation for the SATVOS multi-tenant document processing service.

## Table of Contents

- [Overview](#overview)
- [Authentication](#authentication)
- [Response Format](#response-format)
- [Pagination](#pagination)
- [Endpoints](#endpoints)
  - [Health](#health)
  - [Auth](#auth)
  - [Files](#files)
  - [Collections](#collections)
  - [Documents](#documents)
  - [Users](#users)
  - [Tenants](#tenants)
- [TypeScript Types](#typescript-types)
- [Webhooks & Polling](#webhooks--polling)

---

## Overview

**Base URL**: `/api/v1`

**Content-Type**: `application/json` for all request bodies (except file uploads which use `multipart/form-data`)

**Authentication**: All endpoints except `/auth/*` and `/healthz`, `/readyz` require a valid JWT in the `Authorization` header:

```
Authorization: Bearer <access_token>
```

---

## Authentication

### Login Flow

1. Call `POST /auth/login` with tenant slug, email, and password
2. Store the returned `access_token` and `refresh_token`
3. Include `access_token` in all subsequent requests
4. When `access_token` expires (15 min), call `POST /auth/refresh` with the `refresh_token`
5. Store the new tokens and continue

### Token Details

| Token Type | Expiry | Storage Recommendation |
|------------|--------|----------------------|
| Access Token | 15 minutes | Memory or session storage |
| Refresh Token | 7 days | Secure HTTP-only cookie or encrypted local storage |

### JWT Claims

```typescript
interface JWTClaims {
  tenant_id: string;    // UUID
  user_id: string;      // UUID
  email: string;
  role: "admin" | "manager" | "member" | "viewer";
  iat: number;          // Issued at
  exp: number;          // Expiration
  nbf: number;          // Not before
  iss: string;          // Issuer (satvos)
}
```

---

## Response Format

### Success Response

```json
{
  "success": true,
  "data": { /* resource or array */ },
  "meta": {
    "total": 100,
    "offset": 0,
    "limit": 20
  }
}
```

### Error Response

```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message"
  }
}
```

### Common Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `UNAUTHORIZED` | 401 | Missing or invalid token |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `INSUFFICIENT_ROLE` | 403 | Tenant role too low |
| `NOT_FOUND` | 404 | Resource not found |
| `INVALID_REQUEST` | 400 | Malformed request |
| `INTERNAL_ERROR` | 500 | Server error |

For the complete error code reference, see [ERROR_CODES.md](ERROR_CODES.md).

---

## Pagination

All list endpoints support pagination with these query parameters:

| Parameter | Type | Default | Max | Description |
|-----------|------|---------|-----|-------------|
| `offset` | integer | 0 | - | Skip this many records |
| `limit` | integer | 20 | 100 | Return this many records |

The response includes a `meta` object:

```json
{
  "meta": {
    "total": 150,
    "offset": 20,
    "limit": 20
  }
}
```

---

## Endpoints

### Health

#### Liveness Probe

```http
GET /healthz
```

**Auth**: None

**Response**:
```json
{
  "status": "ok"
}
```

#### Readiness Probe

```http
GET /readyz
```

**Auth**: None

**Response (healthy)**:
```json
{
  "status": "ok"
}
```

**Response (unhealthy)**:
```json
{
  "status": "unavailable",
  "error": "database connection failed"
}
```

---

### Auth

#### Login

```http
POST /api/v1/auth/login
Content-Type: application/json
```

**Request**:
```json
{
  "tenant_slug": "acme",
  "email": "admin@acme.com",
  "password": "securepassword123"
}
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_at": "2025-01-15T10:30:00Z"
  }
}
```

**Errors**:
- `INVALID_CREDENTIALS` (401): Wrong email or password
- `TENANT_INACTIVE` (403): Tenant disabled
- `USER_INACTIVE` (403): User disabled

#### Refresh Token

```http
POST /api/v1/auth/refresh
Content-Type: application/json
```

**Request**:
```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_at": "2025-01-15T10:45:00Z"
  }
}
```

---

### Files

#### Upload File

```http
POST /api/v1/files/upload
Authorization: Bearer <token>
Content-Type: multipart/form-data
```

**Form Fields**:
- `file` (required): The file to upload (PDF, JPG, or PNG, max 50MB)
- `collection_id` (optional): UUID to auto-add file to a collection

**Curl Example**:
```bash
curl -X POST http://localhost:8080/api/v1/files/upload \
  -H "Authorization: Bearer <token>" \
  -F "file=@invoice.pdf" \
  -F "collection_id=550e8400-e29b-41d4-a716-446655440001"
```

**Response** (201 Created):
```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "tenant_id": "123e4567-e89b-12d3-a456-426614174000",
    "uploaded_by": "987fcdeb-51a2-3bc4-d567-890123456789",
    "file_name": "550e8400-e29b-41d4-a716-446655440000.pdf",
    "original_name": "invoice.pdf",
    "file_type": "pdf",
    "file_size": 245678,
    "s3_bucket": "satvos-uploads",
    "s3_key": "tenants/123e4567.../files/550e8400.../invoice.pdf",
    "content_type": "application/pdf",
    "status": "uploaded",
    "created_at": "2025-01-15T10:00:00Z",
    "updated_at": "2025-01-15T10:00:00Z"
  }
}
```

**Response with warning** (201 Created):
```json
{
  "success": true,
  "data": { /* file object */ },
  "warning": "file uploaded but could not be added to collection: collection not found"
}
```

**Errors**:
- `UNSUPPORTED_FILE_TYPE` (400): Not PDF/JPG/PNG
- `FILE_TOO_LARGE` (413): Exceeds 50MB
- `UPLOAD_FAILED` (500): S3 error

**Required Role**: `member` or higher

#### List Files

```http
GET /api/v1/files?offset=0&limit=20
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "original_name": "invoice.pdf",
      "file_type": "pdf",
      "file_size": 245678,
      "status": "uploaded",
      "created_at": "2025-01-15T10:00:00Z",
      ...
    }
  ],
  "meta": {
    "total": 42,
    "offset": 0,
    "limit": 20
  }
}
```

#### Get File

```http
GET /api/v1/files/:id
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "original_name": "invoice.pdf",
    "file_type": "pdf",
    "file_size": 245678,
    "status": "uploaded",
    "download_url": "https://s3.amazonaws.com/satvos-uploads/...?X-Amz-Signature=...",
    ...
  }
}
```

The `download_url` is a presigned S3 URL valid for 1 hour.

#### Delete File

```http
DELETE /api/v1/files/:id
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "message": "file deleted"
  }
}
```

**Required Role**: `admin`

---

### Collections

#### Create Collection

```http
POST /api/v1/collections
Authorization: Bearer <token>
Content-Type: application/json
```

**Request**:
```json
{
  "name": "Q4 2024 Invoices",
  "description": "Invoices from Q4 2024 fiscal quarter"
}
```

**Response** (201 Created):
```json
{
  "success": true,
  "data": {
    "id": "660e8400-e29b-41d4-a716-446655440001",
    "tenant_id": "123e4567-e89b-12d3-a456-426614174000",
    "name": "Q4 2024 Invoices",
    "description": "Invoices from Q4 2024 fiscal quarter",
    "created_by": "987fcdeb-51a2-3bc4-d567-890123456789",
    "document_count": 0,
    "created_at": "2025-01-15T10:00:00Z",
    "updated_at": "2025-01-15T10:00:00Z"
  }
}
```

**Required Role**: `member` or higher

#### List Collections

```http
GET /api/v1/collections?offset=0&limit=20
Authorization: Bearer <token>
```

Returns only collections the user has access to (through explicit permissions or implicit role access).

**Response** (200 OK):
```json
{
  "success": true,
  "data": [
    {
      "id": "660e8400-e29b-41d4-a716-446655440001",
      "name": "Q4 2024 Invoices",
      "description": "...",
      "document_count": 12,
      "created_at": "2025-01-15T10:00:00Z"
    }
  ],
  "meta": {
    "total": 5,
    "offset": 0,
    "limit": 20
  }
}
```

#### Get Collection

```http
GET /api/v1/collections/:id?offset=0&limit=20
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "collection": {
      "id": "660e8400-e29b-41d4-a716-446655440001",
      "name": "Q4 2024 Invoices",
      "description": "...",
      "created_by": "987fcdeb-51a2-3bc4-d567-890123456789",
      "document_count": 12,
      "created_at": "2025-01-15T10:00:00Z",
      "updated_at": "2025-01-15T10:00:00Z"
    },
    "files": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "original_name": "invoice.pdf",
        "file_type": "pdf",
        "file_size": 245678,
        "status": "uploaded",
        "created_at": "2025-01-15T10:00:00Z"
      }
    ],
    "files_meta": {
      "total": 12,
      "offset": 0,
      "limit": 20
    }
  }
}
```

**Required Permission**: `viewer` or higher

#### Update Collection

```http
PUT /api/v1/collections/:id
Authorization: Bearer <token>
Content-Type: application/json
```

**Request**:
```json
{
  "name": "Q4 2024 Invoices - Final",
  "description": "Updated description"
}
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "id": "660e8400-e29b-41d4-a716-446655440001",
    "name": "Q4 2024 Invoices - Final",
    "document_count": 12,
    ...
  }
}
```

**Required Permission**: `editor` or higher

#### Delete Collection

```http
DELETE /api/v1/collections/:id
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "message": "collection deleted"
  }
}
```

Files are NOT deleted, only the collection and associations.

**Required Permission**: `owner` or tenant role `admin`

#### Batch Upload Files

```http
POST /api/v1/collections/:id/files
Authorization: Bearer <token>
Content-Type: multipart/form-data
```

**Form Fields**:
- `files` (required): Multiple files to upload

**Curl Example**:
```bash
curl -X POST http://localhost:8080/api/v1/collections/660e8400.../files \
  -H "Authorization: Bearer <token>" \
  -F "files=@invoice1.pdf" \
  -F "files=@invoice2.pdf" \
  -F "files=@invoice3.jpg"
```

**Response (all succeed)** (201 Created):
```json
{
  "success": true,
  "data": [
    {
      "file": { "id": "...", "original_name": "invoice1.pdf", ... },
      "success": true,
      "error": null
    },
    {
      "file": { "id": "...", "original_name": "invoice2.pdf", ... },
      "success": true,
      "error": null
    }
  ]
}
```

**Response (partial success)** (207 Multi-Status):
```json
{
  "success": true,
  "data": [
    {
      "file": { "id": "...", "original_name": "invoice1.pdf", ... },
      "success": true,
      "error": null
    },
    {
      "file": null,
      "success": false,
      "error": "unsupported file type"
    }
  ]
}
```

**Required Permission**: `editor` or higher

#### Remove File from Collection

```http
DELETE /api/v1/collections/:id/files/:fileId
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "message": "file removed from collection"
  }
}
```

The file itself is not deleted.

**Required Permission**: `editor` or higher

#### Set Permission

```http
POST /api/v1/collections/:id/permissions
Authorization: Bearer <token>
Content-Type: application/json
```

**Request**:
```json
{
  "user_id": "987fcdeb-51a2-3bc4-d567-890123456789",
  "permission": "editor"
}
```

Valid permissions: `owner`, `editor`, `viewer`

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "message": "permission set"
  }
}
```

**Required Permission**: `owner`

#### List Permissions

```http
GET /api/v1/collections/:id/permissions?offset=0&limit=20
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": [
    {
      "id": "770e8400-e29b-41d4-a716-446655440002",
      "collection_id": "660e8400-e29b-41d4-a716-446655440001",
      "user_id": "987fcdeb-51a2-3bc4-d567-890123456789",
      "permission": "owner",
      "granted_by": "987fcdeb-51a2-3bc4-d567-890123456789",
      "created_at": "2025-01-15T10:00:00Z"
    }
  ],
  "meta": {
    "total": 3,
    "offset": 0,
    "limit": 20
  }
}
```

**Required Permission**: `owner`

#### Remove Permission

```http
DELETE /api/v1/collections/:id/permissions/:userId
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "message": "permission removed"
  }
}
```

Cannot remove your own permission.

**Required Permission**: `owner`

---

### Documents

#### Create Document

```http
POST /api/v1/documents
Authorization: Bearer <token>
Content-Type: application/json
```

**Request**:
```json
{
  "file_id": "550e8400-e29b-41d4-a716-446655440000",
  "collection_id": "660e8400-e29b-41d4-a716-446655440001",
  "document_type": "invoice",
  "parse_mode": "single",
  "name": "Acme Corp Invoice Q4-2024",
  "tags": {
    "vendor": "Acme Corp",
    "quarter": "Q4",
    "year": "2024"
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `file_id` | UUID | Yes | The uploaded file to parse |
| `collection_id` | UUID | Yes | Collection to associate with |
| `document_type` | string | Yes | Currently only "invoice" |
| `parse_mode` | string | No | "single" (default) or "dual" |
| `name` | string | No | Display name (defaults to file's original name) |
| `tags` | object | No | Key-value pairs (source: "user") |

**Response** (201 Created):
```json
{
  "success": true,
  "data": {
    "id": "880e8400-e29b-41d4-a716-446655440003",
    "tenant_id": "123e4567-e89b-12d3-a456-426614174000",
    "collection_id": "660e8400-e29b-41d4-a716-446655440001",
    "file_id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Acme Corp Invoice Q4-2024",
    "document_type": "invoice",
    "parser_model": "",
    "parser_prompt": "",
    "structured_data": {},
    "confidence_scores": {},
    "field_provenance": {},
    "parsing_status": "pending",
    "parsing_error": "",
    "parsed_at": null,
    "review_status": "pending",
    "reviewed_by": null,
    "reviewed_at": null,
    "reviewer_notes": "",
    "validation_status": "pending",
    "validation_results": null,
    "reconciliation_status": "pending",
    "parse_mode": "single",
    "created_by": "987fcdeb-51a2-3bc4-d567-890123456789",
    "created_at": "2025-01-15T10:00:00Z",
    "updated_at": "2025-01-15T10:00:00Z"
  }
}
```

Parsing begins immediately in the background. Poll `GET /documents/:id` until `parsing_status` is `completed` or `failed`.

**Errors**:
- `DOCUMENT_ALREADY_EXISTS` (409): A document already exists for this file
- `NOT_FOUND` (404): File not found
- `COLLECTION_NOT_FOUND` (404): Collection not found

#### List Documents

```http
GET /api/v1/documents?offset=0&limit=20
Authorization: Bearer <token>
```

**Optional Query Parameters**:
- `collection_id`: Filter by collection

**Response** (200 OK):
```json
{
  "success": true,
  "data": [
    {
      "id": "880e8400-e29b-41d4-a716-446655440003",
      "name": "Acme Corp Invoice Q4-2024",
      "document_type": "invoice",
      "parsing_status": "completed",
      "review_status": "pending",
      "validation_status": "valid",
      "reconciliation_status": "valid",
      "created_at": "2025-01-15T10:00:00Z"
    }
  ],
  "meta": {
    "total": 25,
    "offset": 0,
    "limit": 20
  }
}
```

#### Get Document

```http
GET /api/v1/documents/:id
Authorization: Bearer <token>
```

**Response** (200 OK, parsing complete):
```json
{
  "success": true,
  "data": {
    "id": "880e8400-e29b-41d4-a716-446655440003",
    "name": "Acme Corp Invoice Q4-2024",
    "document_type": "invoice",
    "parser_model": "claude-sonnet-4-20250514",
    "parsing_status": "completed",
    "parsed_at": "2025-01-15T10:01:30Z",
    "structured_data": {
      "invoice": {
        "invoice_number": "INV-2024-001234",
        "invoice_date": "2024-12-15",
        "due_date": "2025-01-15",
        "invoice_type": "tax_invoice",
        "currency": "INR",
        "place_of_supply": "Karnataka",
        "reverse_charge": false
      },
      "seller": {
        "name": "Acme Corporation Pvt Ltd",
        "address": "123 Tech Park, Bangalore",
        "gstin": "29AABCU9603R1ZM",
        "pan": "AABCU9603R",
        "state": "Karnataka",
        "state_code": "29"
      },
      "buyer": {
        "name": "Buyer Company Ltd",
        "address": "456 Business District",
        "gstin": "29BBBCB1234A1Z1",
        "pan": "BBBCB1234A",
        "state": "Karnataka",
        "state_code": "29"
      },
      "line_items": [
        {
          "description": "Software Development Services",
          "hsn_sac_code": "998314",
          "quantity": 100,
          "unit": "hours",
          "unit_price": 2500.00,
          "discount": 5000.00,
          "taxable_amount": 245000.00,
          "cgst_rate": 9,
          "cgst_amount": 22050.00,
          "sgst_rate": 9,
          "sgst_amount": 22050.00,
          "igst_rate": 0,
          "igst_amount": 0,
          "total": 289100.00
        }
      ],
      "totals": {
        "subtotal": 250000.00,
        "total_discount": 5000.00,
        "taxable_amount": 245000.00,
        "cgst": 22050.00,
        "sgst": 22050.00,
        "igst": 0,
        "cess": 0,
        "round_off": 0,
        "total": 289100.00,
        "amount_in_words": "Two Lakh Eighty Nine Thousand One Hundred Rupees Only"
      },
      "payment": {
        "bank_name": "HDFC Bank",
        "account_number": "50100123456789",
        "ifsc_code": "HDFC0001234",
        "payment_terms": "Net 30"
      },
      "notes": ""
    },
    "confidence_scores": {
      "invoice": {
        "invoice_number": 0.98,
        "invoice_date": 0.95,
        "due_date": 0.92,
        ...
      },
      "seller": {
        "name": 0.97,
        "gstin": 0.99,
        ...
      },
      ...
    },
    "validation_status": "valid",
    "reconciliation_status": "valid",
    "review_status": "pending",
    ...
  }
}
```

#### Retry Parsing

```http
POST /api/v1/documents/:id/retry
Authorization: Bearer <token>
```

Re-triggers LLM parsing for a failed document. Also refreshes auto-generated tags.

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "id": "880e8400-e29b-41d4-a716-446655440003",
    "parsing_status": "pending",
    ...
  }
}
```

#### Review Document

```http
PUT /api/v1/documents/:id/review
Authorization: Bearer <token>
Content-Type: application/json
```

**Request**:
```json
{
  "status": "approved",
  "notes": "Verified against source PDF. All data correct."
}
```

| Field | Type | Required | Values |
|-------|------|----------|--------|
| `status` | string | Yes | "approved" or "rejected" |
| `notes` | string | No | Review comments |

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "id": "880e8400-e29b-41d4-a716-446655440003",
    "review_status": "approved",
    "reviewed_by": "987fcdeb-51a2-3bc4-d567-890123456789",
    "reviewed_at": "2025-01-15T11:00:00Z",
    "reviewer_notes": "Verified against source PDF. All data correct.",
    ...
  }
}
```

**Errors**:
- `DOCUMENT_NOT_PARSED` (400): Parsing not yet complete

#### Edit Structured Data

```http
PUT /api/v1/documents/:id
Authorization: Bearer <token>
Content-Type: application/json
```

Also available at the alias path `PUT /api/v1/documents/:id/structured-data`.

Replace the parsed invoice data with manually corrected data. The JSON body must conform to the `GSTInvoice` schema. After saving:
- All confidence scores are set to 1.0 (human-verified)
- Field provenance is set to `{"source": "manual_edit"}`
- Review status is reset to `pending` (reviewed_by, reviewed_at, reviewer_notes cleared)
- Auto-tags are deleted and re-extracted from the new data
- Validation runs synchronously and results are returned with the updated document

**Request**:
```json
{
  "structured_data": {
    "invoice": {
      "invoice_number": "INV-2024-001234",
      "invoice_date": "2024-12-15",
      "due_date": "2025-01-15",
      "invoice_type": "tax_invoice",
      "currency": "INR",
      "place_of_supply": "Karnataka",
      "reverse_charge": false
    },
    "seller": {
      "name": "Acme Corporation Pvt Ltd",
      "address": "123 Tech Park, Bangalore",
      "gstin": "29AABCU9603R1ZM",
      "pan": "AABCU9603R",
      "state": "Karnataka",
      "state_code": "29"
    },
    "buyer": {
      "name": "Buyer Industries Ltd",
      "address": "456 Business Hub, Mumbai",
      "gstin": "27BBBCD1234E1Z5",
      "pan": "BBBCD1234E",
      "state": "Maharashtra",
      "state_code": "27"
    },
    "line_items": [
      {
        "description": "Software Development Services",
        "hsn_sac_code": "998314",
        "quantity": 100,
        "unit": "hours",
        "unit_price": 2500.00,
        "discount": 5000.00,
        "taxable_amount": 245000.00,
        "cgst_rate": 0,
        "cgst_amount": 0,
        "sgst_rate": 0,
        "sgst_amount": 0,
        "igst_rate": 18,
        "igst_amount": 44100.00,
        "total": 289100.00
      }
    ],
    "totals": {
      "subtotal": 250000.00,
      "total_discount": 5000.00,
      "taxable_amount": 245000.00,
      "cgst": 0,
      "sgst": 0,
      "igst": 44100.00,
      "cess": 0,
      "round_off": 0,
      "total": 289100.00,
      "amount_in_words": "Two Lakh Eighty Nine Thousand One Hundred Rupees Only"
    },
    "payment": {
      "bank_name": "HDFC Bank",
      "account_number": "50100123456789",
      "ifsc_code": "HDFC0001234",
      "payment_terms": "Net 30"
    },
    "notes": ""
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `structured_data` | GSTInvoice | Yes | The full invoice data conforming to the GSTInvoice schema |

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "id": "880e8400-e29b-41d4-a716-446655440003",
    "structured_data": { /* updated GSTInvoice */ },
    "confidence_scores": { /* all fields set to 1.0 */ },
    "field_provenance": { "source": "manual_edit" },
    "review_status": "pending",
    "reviewed_by": null,
    "reviewed_at": null,
    "reviewer_notes": "",
    "validation_status": "valid",
    "reconciliation_status": "valid",
    ...
  }
}
```

**Errors**:
- `DOCUMENT_NOT_FOUND` (404): Document not found
- `DOCUMENT_NOT_PARSED` (400): Parsing not yet complete
- `INVALID_STRUCTURED_DATA` (400): JSON doesn't match GSTInvoice schema
- `COLLECTION_PERMISSION_DENIED` (403): Insufficient permission (requires editor+)

**Required Permission**: `editor` or higher on collection

#### Validate Document

```http
POST /api/v1/documents/:id/validate
Authorization: Bearer <token>
```

Re-runs the validation engine. Validation also runs automatically after parsing.

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "message": "validation completed"
  }
}
```

#### Get Validation Results

```http
GET /api/v1/documents/:id/validation
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "document_id": "880e8400-e29b-41d4-a716-446655440003",
    "validation_status": "warning",
    "summary": {
      "total": 50,
      "passed": 48,
      "errors": 0,
      "warnings": 2
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
        "actual_value": "INV-2024-001234",
        "message": "Required: Invoice Number: invoice.invoice_number is present",
        "reconciliation_critical": true
      },
      {
        "rule_name": "Format: HSN/SAC Code",
        "rule_type": "regex",
        "severity": "warning",
        "passed": false,
        "field_path": "line_items[0].hsn_sac_code",
        "expected_value": "4-8 digit code",
        "actual_value": "9983",
        "message": "Format: HSN/SAC Code: line_items[0].hsn_sac_code has invalid format",
        "reconciliation_critical": false
      }
    ],
    "field_statuses": {
      "invoice.invoice_number": {
        "status": "valid",
        "messages": []
      },
      "seller.gstin": {
        "status": "valid",
        "messages": []
      },
      "line_items[0].hsn_sac_code": {
        "status": "unsure",
        "messages": ["Format: HSN/SAC Code: invalid format"]
      }
    }
  }
}
```

**Field Status Values**:
- `valid`: All rules passed
- `invalid`: Error-severity rule failed
- `unsure`: Warning-severity rule failed OR confidence score <= 0.5

For the complete validation rules reference, see [VALIDATION.md](VALIDATION.md).

#### List Tags

```http
GET /api/v1/documents/:id/tags
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": [
    {
      "id": "990e8400-e29b-41d4-a716-446655440004",
      "document_id": "880e8400-e29b-41d4-a716-446655440003",
      "key": "vendor",
      "value": "Acme Corp",
      "source": "user",
      "created_at": "2025-01-15T10:00:00Z"
    },
    {
      "id": "990e8400-e29b-41d4-a716-446655440005",
      "document_id": "880e8400-e29b-41d4-a716-446655440003",
      "key": "seller_gstin",
      "value": "29AABCU9603R1ZM",
      "source": "auto",
      "created_at": "2025-01-15T10:01:30Z"
    }
  ]
}
```

**Tag Sources**:
- `user`: Manually added by user
- `auto`: Extracted from parsed invoice data

**Auto-generated Tag Keys**:
`invoice_number`, `invoice_date`, `seller_name`, `seller_gstin`, `buyer_name`, `buyer_gstin`, `invoice_type`, `place_of_supply`, `total_amount`

#### Add Tags

```http
POST /api/v1/documents/:id/tags
Authorization: Bearer <token>
Content-Type: application/json
```

**Request**:
```json
{
  "tags": {
    "department": "Engineering",
    "cost_center": "CC-1234"
  }
}
```

**Response** (201 Created):
```json
{
  "success": true,
  "data": [
    {
      "id": "990e8400-e29b-41d4-a716-446655440006",
      "key": "department",
      "value": "Engineering",
      "source": "user",
      "created_at": "2025-01-15T12:00:00Z"
    },
    {
      "id": "990e8400-e29b-41d4-a716-446655440007",
      "key": "cost_center",
      "value": "CC-1234",
      "source": "user",
      "created_at": "2025-01-15T12:00:00Z"
    }
  ]
}
```

**Required Permission**: `editor` or higher on collection

#### Delete Tag

```http
DELETE /api/v1/documents/:id/tags/:tagId
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "message": "tag deleted"
  }
}
```

Only user-created tags can be deleted. Auto-tags are managed by the system.

**Required Permission**: `editor` or higher

#### Search by Tag

```http
GET /api/v1/documents/search/tags?key=vendor&value=Acme%20Corp&offset=0&limit=20
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": [
    {
      "id": "880e8400-e29b-41d4-a716-446655440003",
      "name": "Acme Corp Invoice Q4-2024",
      "document_type": "invoice",
      "parsing_status": "completed",
      ...
    }
  ],
  "meta": {
    "total": 5,
    "offset": 0,
    "limit": 20
  }
}
```

#### Delete Document

```http
DELETE /api/v1/documents/:id
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "message": "document deleted"
  }
}
```

**Required Role**: `admin`

---

### Users

#### Create User

```http
POST /api/v1/users
Authorization: Bearer <token>
Content-Type: application/json
```

**Request**:
```json
{
  "email": "jane.doe@acme.com",
  "password": "securepassword123",
  "full_name": "Jane Doe",
  "role": "member"
}
```

| Field | Type | Required | Validation |
|-------|------|----------|------------|
| `email` | string | Yes | Valid email, unique per tenant |
| `password` | string | Yes | Min 8 characters |
| `full_name` | string | No | Display name |
| `role` | string | Yes | "admin", "manager", "member", "viewer" |

**Response** (201 Created):
```json
{
  "success": true,
  "data": {
    "id": "abc12345-e29b-41d4-a716-446655440008",
    "tenant_id": "123e4567-e89b-12d3-a456-426614174000",
    "email": "jane.doe@acme.com",
    "full_name": "Jane Doe",
    "role": "member",
    "is_active": true,
    "created_at": "2025-01-15T10:00:00Z",
    "updated_at": "2025-01-15T10:00:00Z"
  }
}
```

**Required Role**: `admin`

#### List Users

```http
GET /api/v1/users?offset=0&limit=20
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": [
    {
      "id": "abc12345-e29b-41d4-a716-446655440008",
      "email": "jane.doe@acme.com",
      "full_name": "Jane Doe",
      "role": "member",
      "is_active": true,
      "created_at": "2025-01-15T10:00:00Z"
    }
  ],
  "meta": {
    "total": 15,
    "offset": 0,
    "limit": 20
  }
}
```

**Required Role**: `admin`

#### Get User

```http
GET /api/v1/users/:id
Authorization: Bearer <token>
```

Users can view their own profile; admins can view any user in the tenant.

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "id": "abc12345-e29b-41d4-a716-446655440008",
    "email": "jane.doe@acme.com",
    "full_name": "Jane Doe",
    "role": "member",
    "is_active": true,
    "created_at": "2025-01-15T10:00:00Z",
    "updated_at": "2025-01-15T10:00:00Z"
  }
}
```

#### Update User

```http
PUT /api/v1/users/:id
Authorization: Bearer <token>
Content-Type: application/json
```

**Request**:
```json
{
  "full_name": "Jane Smith",
  "email": "jane.smith@acme.com",
  "role": "manager",
  "is_active": true
}
```

All fields are optional. Users can update their own `email` and `full_name`. Only admins can change `role` and `is_active`.

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "id": "abc12345-e29b-41d4-a716-446655440008",
    "email": "jane.smith@acme.com",
    "full_name": "Jane Smith",
    "role": "manager",
    ...
  }
}
```

#### Delete User

```http
DELETE /api/v1/users/:id
Authorization: Bearer <token>
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "message": "user deleted"
  }
}
```

**Required Role**: `admin`

---

### Tenants

All tenant endpoints require `admin` role.

#### Create Tenant

```http
POST /api/v1/admin/tenants
Authorization: Bearer <token>
Content-Type: application/json
```

**Request**:
```json
{
  "name": "Acme Corporation",
  "slug": "acme"
}
```

**Response** (201 Created):
```json
{
  "success": true,
  "data": {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "name": "Acme Corporation",
    "slug": "acme",
    "is_active": true,
    "created_at": "2025-01-15T10:00:00Z",
    "updated_at": "2025-01-15T10:00:00Z"
  }
}
```

#### List Tenants

```http
GET /api/v1/admin/tenants?offset=0&limit=20
Authorization: Bearer <token>
```

#### Get Tenant

```http
GET /api/v1/admin/tenants/:id
Authorization: Bearer <token>
```

#### Update Tenant

```http
PUT /api/v1/admin/tenants/:id
Authorization: Bearer <token>
Content-Type: application/json
```

**Request**:
```json
{
  "name": "Acme Industries",
  "slug": "acme-ind",
  "is_active": false
}
```

#### Delete Tenant

```http
DELETE /api/v1/admin/tenants/:id
Authorization: Bearer <token>
```

---

## TypeScript Types

Complete TypeScript type definitions for API integration:

```typescript
// ============== Common Types ==============

type UUID = string;
type Timestamp = string; // ISO 8601

interface PaginationMeta {
  total: number;
  offset: number;
  limit: number;
}

interface APIResponse<T> {
  success: boolean;
  data: T | null;
  error?: {
    code: string;
    message: string;
  };
  meta?: PaginationMeta;
  warning?: string;
}

// ============== Auth ==============

interface LoginRequest {
  tenant_slug: string;
  email: string;
  password: string;
}

interface RefreshRequest {
  refresh_token: string;
}

interface TokenResponse {
  access_token: string;
  refresh_token: string;
  expires_at: Timestamp;
}

// ============== Enums ==============

type UserRole = "admin" | "manager" | "member" | "viewer";
type CollectionPermission = "owner" | "editor" | "viewer";
type FileType = "pdf" | "jpg" | "png";
type FileStatus = "pending" | "uploaded" | "failed" | "deleted";
type ParsingStatus = "pending" | "processing" | "completed" | "failed";
type ReviewStatus = "pending" | "approved" | "rejected";
type ValidationStatus = "pending" | "valid" | "warning" | "invalid";
type ReconciliationStatus = "pending" | "valid" | "warning" | "invalid";
type ParseMode = "single" | "dual";
type FieldValidationStatus = "valid" | "invalid" | "unsure";
type ValidationRuleType = "required_field" | "regex" | "sum_check" | "cross_field" | "custom";
type ValidationSeverity = "error" | "warning";
type TagSource = "user" | "auto";

// ============== Tenant ==============

interface Tenant {
  id: UUID;
  name: string;
  slug: string;
  is_active: boolean;
  created_at: Timestamp;
  updated_at: Timestamp;
}

interface CreateTenantRequest {
  name: string;
  slug: string;
}

interface UpdateTenantRequest {
  name?: string;
  slug?: string;
  is_active?: boolean;
}

// ============== User ==============

interface User {
  id: UUID;
  tenant_id: UUID;
  email: string;
  full_name: string;
  role: UserRole;
  is_active: boolean;
  created_at: Timestamp;
  updated_at: Timestamp;
}

interface CreateUserRequest {
  email: string;
  password: string;
  full_name?: string;
  role: UserRole;
}

interface UpdateUserRequest {
  email?: string;
  full_name?: string;
  role?: UserRole;      // admin only
  is_active?: boolean;  // admin only
}

// ============== File ==============

interface FileMeta {
  id: UUID;
  tenant_id: UUID;
  uploaded_by: UUID;
  file_name: string;
  original_name: string;
  file_type: FileType;
  file_size: number;
  s3_bucket: string;
  s3_key: string;
  content_type: string;
  status: FileStatus;
  created_at: Timestamp;
  updated_at: Timestamp;
  download_url?: string; // Only in GET /files/:id
}

// ============== Collection ==============

interface Collection {
  id: UUID;
  tenant_id: UUID;
  name: string;
  description: string;
  created_by: UUID;
  document_count: number;
  created_at: Timestamp;
  updated_at: Timestamp;
}

interface CreateCollectionRequest {
  name: string;
  description?: string;
}

interface UpdateCollectionRequest {
  name?: string;
  description?: string;
}

interface CollectionWithFiles {
  collection: Collection;
  files: FileMeta[];
  files_meta: PaginationMeta;
}

interface CollectionPermissionEntry {
  id: UUID;
  collection_id: UUID;
  tenant_id: UUID;
  user_id: UUID;
  permission: CollectionPermission;
  granted_by: UUID;
  created_at: Timestamp;
}

interface SetPermissionRequest {
  user_id: UUID;
  permission: CollectionPermission;
}

interface BatchUploadResult {
  file: FileMeta | null;
  success: boolean;
  error: string | null;
}

// ============== Document ==============

interface Document {
  id: UUID;
  tenant_id: UUID;
  collection_id: UUID;
  file_id: UUID;
  name: string;
  document_type: string;
  parser_model: string;
  parser_prompt: string;
  structured_data: GSTInvoice | Record<string, never>;
  confidence_scores: ConfidenceScores | Record<string, never>;
  field_provenance: Record<string, string>;
  parsing_status: ParsingStatus;
  parsing_error: string;
  parsed_at: Timestamp | null;
  review_status: ReviewStatus;
  reviewed_by: UUID | null;
  reviewed_at: Timestamp | null;
  reviewer_notes: string;
  validation_status: ValidationStatus;
  validation_results: ValidationResultEntry[] | null;
  reconciliation_status: ReconciliationStatus;
  parse_mode: ParseMode;
  created_by: UUID;
  created_at: Timestamp;
  updated_at: Timestamp;
}

interface CreateDocumentRequest {
  file_id: UUID;
  collection_id: UUID;
  document_type: string;
  parse_mode?: ParseMode;
  name?: string;
  tags?: Record<string, string>;
}

interface ReviewDocumentRequest {
  status: "approved" | "rejected";
  notes?: string;
}

interface EditStructuredDataRequest {
  structured_data: GSTInvoice;
}

// ============== Document Tags ==============

interface DocumentTag {
  id: UUID;
  document_id: UUID;
  tenant_id: UUID;
  key: string;
  value: string;
  source: TagSource;
  created_at: Timestamp;
}

interface AddTagsRequest {
  tags: Record<string, string>;
}

// ============== Parsed Invoice Schema ==============

interface GSTInvoice {
  invoice: InvoiceHeader;
  seller: Party;
  buyer: Party;
  line_items: LineItem[];
  totals: Totals;
  payment: Payment;
  notes: string;
}

interface InvoiceHeader {
  invoice_number: string;
  invoice_date: string;
  due_date: string;
  invoice_type: string;
  currency: string;
  place_of_supply: string;
  reverse_charge: boolean;
}

interface Party {
  name: string;
  address: string;
  gstin: string;
  pan: string;
  state: string;
  state_code: string;
}

interface LineItem {
  description: string;
  hsn_sac_code: string;
  quantity: number;
  unit: string;
  unit_price: number;
  discount: number;
  taxable_amount: number;
  cgst_rate: number;
  cgst_amount: number;
  sgst_rate: number;
  sgst_amount: number;
  igst_rate: number;
  igst_amount: number;
  total: number;
}

interface Totals {
  subtotal: number;
  total_discount: number;
  taxable_amount: number;
  cgst: number;
  sgst: number;
  igst: number;
  cess: number;
  round_off: number;
  total: number;
  amount_in_words: string;
}

interface Payment {
  bank_name: string;
  account_number: string;
  ifsc_code: string;
  payment_terms: string;
}

// ============== Confidence Scores ==============

// Mirrors GSTInvoice structure with 0.0-1.0 floats
interface ConfidenceScores {
  invoice: {
    invoice_number: number;
    invoice_date: number;
    due_date: number;
    invoice_type: number;
    currency: number;
    place_of_supply: number;
    reverse_charge: number;
  };
  seller: PartyConfidence;
  buyer: PartyConfidence;
  line_items: LineItemConfidence[];
  totals: TotalsConfidence;
  payment: PaymentConfidence;
}

interface PartyConfidence {
  name: number;
  address: number;
  gstin: number;
  pan: number;
  state: number;
  state_code: number;
}

interface LineItemConfidence {
  description: number;
  hsn_sac_code: number;
  quantity: number;
  unit: number;
  unit_price: number;
  discount: number;
  taxable_amount: number;
  cgst_rate: number;
  cgst_amount: number;
  sgst_rate: number;
  sgst_amount: number;
  igst_rate: number;
  igst_amount: number;
  total: number;
}

interface TotalsConfidence {
  subtotal: number;
  total_discount: number;
  taxable_amount: number;
  cgst: number;
  sgst: number;
  igst: number;
  cess: number;
  round_off: number;
  total: number;
  amount_in_words: number;
}

interface PaymentConfidence {
  bank_name: number;
  account_number: number;
  ifsc_code: number;
  payment_terms: number;
}

// ============== Validation ==============

interface ValidationResponse {
  document_id: UUID;
  validation_status: ValidationStatus;
  summary: ValidationSummary;
  reconciliation_status: ReconciliationStatus;
  reconciliation_summary: ValidationSummary;
  results: ValidationResultEntry[];
  field_statuses: Record<string, FieldStatus>;
}

interface ValidationSummary {
  total: number;
  passed: number;
  errors: number;
  warnings: number;
}

interface ValidationResultEntry {
  rule_name: string;
  rule_type: ValidationRuleType;
  severity: ValidationSeverity;
  passed: boolean;
  field_path: string;
  expected_value: string;
  actual_value: string;
  message: string;
  reconciliation_critical: boolean;
}

interface FieldStatus {
  status: FieldValidationStatus;
  messages: string[];
}
```

---

## Webhooks & Polling

SATVOS does not currently support webhooks. For real-time updates on document parsing:

### Polling Strategy

```typescript
async function waitForParsing(
  documentId: string,
  maxAttempts = 60,
  intervalMs = 2000
): Promise<Document> {
  for (let i = 0; i < maxAttempts; i++) {
    const response = await fetch(`/api/v1/documents/${documentId}`, {
      headers: { Authorization: `Bearer ${token}` }
    });
    const { data } = await response.json();

    if (data.parsing_status === "completed") {
      return data;
    }
    if (data.parsing_status === "failed") {
      throw new Error(data.parsing_error);
    }

    await new Promise(r => setTimeout(r, intervalMs));
  }
  throw new Error("Parsing timed out");
}
```

### Recommended Polling Intervals

| Document Size | Interval | Max Wait |
|---------------|----------|----------|
| < 1 page | 1s | 30s |
| 1-5 pages | 2s | 60s |
| 5-20 pages | 3s | 120s |
| > 20 pages | 5s | 300s |
