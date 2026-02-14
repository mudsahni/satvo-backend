# Design: Review Assignment, Permission Fixes & Review Queue

## Summary

Add document review assignment (soft assignment to a specific user), fix the viewer role permission cap, add tenant validation to collection permission grants, and add a review queue endpoint. These changes improve team workflow by enabling accountability in the review process and fixing permission inconsistencies.

## Scope

### In Scope
1. **Review assignment** — assign a document to a specific user for review
2. **Review queue endpoint** — list documents assigned to the current user awaiting review
3. **Viewer cap removal** — honor explicit collection grants for viewer-role users
4. **Tenant validation in SetPermission** — verify target user belongs to same tenant
5. **Audit trail** — new `document.assigned` action
6. **assigned_to filter** — filter document lists by assignee
7. **Documentation updates** — CLAUDE.md, frontend integration guide

### Out of Scope
- Multi-step approval workflows
- Notifications (email/in-app) for assignments
- Review locks / exclusive assignment
- Automatic approval based on validation
- Collection-level `review_required` setting
- Free tier review capability changes

## Data Model

### New Document Fields

```go
AssignedTo *uuid.UUID `db:"assigned_to" json:"assigned_to"`
AssignedAt *time.Time `db:"assigned_at" json:"assigned_at,omitempty"`
AssignedBy *uuid.UUID `db:"assigned_by" json:"assigned_by"`
```

### Migration 000019

```sql
-- UP
ALTER TABLE documents ADD COLUMN assigned_to UUID;
ALTER TABLE documents ADD COLUMN assigned_at TIMESTAMPTZ;
ALTER TABLE documents ADD COLUMN assigned_by UUID;
CREATE INDEX idx_documents_assigned_to ON documents (tenant_id, assigned_to) WHERE assigned_to IS NOT NULL;

-- DOWN
DROP INDEX IF EXISTS idx_documents_assigned_to;
ALTER TABLE documents DROP COLUMN IF EXISTS assigned_by;
ALTER TABLE documents DROP COLUMN IF EXISTS assigned_at;
ALTER TABLE documents DROP COLUMN IF EXISTS assigned_to;
```

No FK constraints — consistent with audit trail pattern. Partial index keeps it small.

### New Audit Action

```go
AuditDocumentAssigned AuditAction = "document.assigned"
```

Audit `changes` payload:
- Assign: `{"assigned_to": "<uuid>", "assigned_by": "<uuid>"}`
- Unassign: `{"assigned_to": null, "assigned_by": "<uuid>", "previous_assignee": "<uuid>"}`

## API Changes

### New: PUT /documents/:id/assign

Assign or unassign a document for review.

**Request:**
```json
{"assignee_id": "uuid"}
```
To unassign:
```json
{"assignee_id": null}
```

**Response:** Standard document envelope (200).

**Errors:**
- 400 INVALID_ID — bad document ID
- 400 INVALID_REQUEST — bad assignee_id format
- 400 DOCUMENT_NOT_PARSED — document not yet parsed
- 403 COLLECTION_PERM_DENIED — caller lacks editor+ on collection
- 404 NOT_FOUND — document or assignee not found in tenant
- 400 ASSIGNEE_CANNOT_REVIEW — assignee lacks editor+ on collection

**Permission:** Editor+ on the document's collection.

### New: GET /documents/review-queue

List documents assigned to the current user that are parsed and pending review.

**Query params:** `offset` (default 0), `limit` (default 20, max 100)

**Response:** Paginated document list (same envelope as GET /documents).

**SQL:**
```sql
SELECT * FROM documents
WHERE tenant_id = $1 AND assigned_to = $2
  AND parsing_status = 'completed' AND review_status = 'pending'
ORDER BY assigned_at ASC
LIMIT $3 OFFSET $4
```

**Permission:** Any authenticated user. Only returns their own assignments.

### Modified: GET /documents

New optional query param: `assigned_to={userId}`

Filters documents where `assigned_to` matches. Combined with existing `collection_id` filter. Respects existing role-based visibility (viewers still only see collections they have access to).

### Modified: PUT /documents/:id/review

No API change. When a review is submitted (approve/reject), the assignment is NOT cleared. The document simply moves from `pending` to `approved`/`rejected` and disappears from the review queue (which filters by `review_status = 'pending'`).

## Service Layer

### New: AssignDocument

```go
type AssignDocumentInput struct {
    TenantID   uuid.UUID
    DocumentID uuid.UUID
    CallerID   uuid.UUID
    CallerRole domain.UserRole
    AssigneeID *uuid.UUID // nil = unassign
}
```

Logic:
1. Fetch document
2. Require editor+ on collection (caller)
3. Require parsing completed
4. If assigning (non-nil):
   a. Verify assignee exists in tenant (userRepo.GetByID)
   b. Verify assignee has editor+ effective permission on collection
5. Set assigned_to, assigned_at, assigned_by
6. Persist via UpdateAssignment
7. Emit AuditDocumentAssigned
8. Return document

### New: ListReviewQueue

Simple delegation to `docRepo.ListReviewQueue(ctx, tenantID, userID, offset, limit)`.

### Modified: RetryParse

Clear assignment fields (assigned_to = nil, assigned_at = nil, assigned_by = nil) when retrying. The document goes back to pending parsing, so the assignment is no longer meaningful.

### Modified: ListByTenant / ListByCollection

Add optional `assignedTo *uuid.UUID` parameter. When non-nil, add `AND assigned_to = $N` to the WHERE clause.

## Repository Layer

### New Methods on DocumentRepository

```go
UpdateAssignment(ctx context.Context, doc *domain.Document) error
ListReviewQueue(ctx context.Context, tenantID, userID uuid.UUID, offset, limit int) ([]domain.Document, int, error)
```

### Modified Methods

- `ListByTenant` — add optional assignedTo filter
- `ListByCollection` — add optional assignedTo filter
- `ListByUserCollections` — add optional assignedTo filter

## Permission Changes

### Viewer Cap Removal

In `documentService.effectiveCollectionPerm` and `collectionService.effectivePermission`:

Remove:
```go
if role == domain.RoleViewer {
    if domain.CollectionPermLevel(explicit) > domain.CollectionPermLevel(domain.CollectionPermViewer) {
        explicit = domain.CollectionPermViewer
    }
}
```

After: A viewer with explicit editor grant on a collection can edit/review documents in that collection.

### Tenant Validation in SetPermission

In `collectionService.SetPermission`, after the owner permission check:

```go
if _, err := s.userRepo.GetByID(ctx, input.TenantID, input.UserID); err != nil {
    return fmt.Errorf("target user not found in tenant: %w", domain.ErrNotFound)
}
```

Requires adding `userRepo port.UserRepository` to `collectionService` struct and constructor.

## Side Effects on Existing Features

| Feature | Impact |
|---------|--------|
| EditStructuredData | No change — keeps assignment (user confirmed) |
| RetryParse | Clears assignment fields |
| DeleteDocument | No impact — assignment fields deleted with document |
| CSV Export | No change — assignment fields not in CSV columns |
| Stats | No change — no assignment stats |
| ParseQueueWorker | No impact |
| Audit Trail | New action added; existing actions unchanged |
| Collection batch upload | No impact |

## Error Handling

### New Sentinel Error

```go
ErrAssigneeCannotReview = errors.New("assignee does not have review permission on this collection")
```

Maps to HTTP 400 BAD_REQUEST with code "ASSIGNEE_CANNOT_REVIEW".

## Test Plan

### Service Tests (document_service_test.go)
- TestAssignDocument_Success — happy path assign
- TestAssignDocument_Unassign — null assignee clears fields
- TestAssignDocument_NotParsed — error when parsing incomplete
- TestAssignDocument_AssigneeNotInTenant — error on missing user
- TestAssignDocument_AssigneeNoPermission — error when assignee lacks editor+
- TestAssignDocument_CallerNoPermission — error when caller lacks editor+
- TestAssignDocument_DocNotFound — error on missing document
- TestAssignDocument_AuditEntry — verify audit trail written
- TestRetryParse_ClearsAssignment — verify fields cleared on retry
- TestListReviewQueue_Success — returns correct filtered docs
- TestListReviewQueue_Empty — no matching docs returns empty
- TestListByTenant_FilterAssignedTo — assignedTo filter works
- Update existing viewer permission tests to reflect cap removal

### Service Tests (collection_service_test.go)
- TestSetPermission_UserNotInTenant — error on user not found
- Existing tests should still pass

### Handler Tests (document_handler_test.go)
- TestAssignDocument_Success — HTTP 200
- TestAssignDocument_Unassign — HTTP 200
- TestAssignDocument_InvalidDocID — HTTP 400
- TestAssignDocument_InvalidBody — HTTP 400
- TestAssignDocument_NoAuth — HTTP 401
- TestReviewQueue_Success — HTTP 200 with pagination
- TestReviewQueue_NoAuth — HTTP 401
- TestListDocuments_FilterAssignedTo — query param works

### Handler Tests (collection_handler_test.go)
- TestSetPermission_UserNotInTenant — HTTP 404

## Documentation Updates

### CLAUDE.md
- Add assigned_to/assigned_at/assigned_by to Document model description
- Add AuditDocumentAssigned to audit actions
- Add PUT /documents/:id/assign and GET /documents/review-queue to endpoint list
- Update permission table (viewer cap removed)
- Add ErrAssigneeCannotReview to errors
- Update "Modifying review" section
- Note that RetryParse clears assignment

### Frontend Integration Guide
- New document: docs/frontend-review-assignment-integration.md
- Covers: assignment API, review queue, permission changes, UI patterns
