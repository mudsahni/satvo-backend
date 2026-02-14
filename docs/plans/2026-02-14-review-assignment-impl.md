# Review Assignment & Permission Fixes — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add document review assignment, fix viewer permission cap, add tenant validation in SetPermission, and add a review queue endpoint.

**Architecture:** Hexagonal — domain changes propagate outward: model -> errors -> enums -> port -> repo -> service -> handler -> router. Migration adds DB columns. Tests at service and handler layers using testify mocks.

**Tech Stack:** Go 1.24, Gin, PostgreSQL (sqlx), testify/mock, golang-migrate

**Design doc:** `docs/plans/2026-02-14-review-assignment-design.md`
**Frontend guide:** `docs/frontend-review-assignment-integration.md`

---

### Task 1: Database Migration

**Files:**
- Create: `db/migrations/000019_add_review_assignment.up.sql`
- Create: `db/migrations/000019_add_review_assignment.down.sql`

**Step 1: Write the up migration**

```sql
ALTER TABLE documents ADD COLUMN assigned_to UUID;
ALTER TABLE documents ADD COLUMN assigned_at TIMESTAMPTZ;
ALTER TABLE documents ADD COLUMN assigned_by UUID;

CREATE INDEX idx_documents_assigned_to ON documents (tenant_id, assigned_to) WHERE assigned_to IS NOT NULL;
```

**Step 2: Write the down migration**

```sql
DROP INDEX IF EXISTS idx_documents_assigned_to;
ALTER TABLE documents DROP COLUMN IF EXISTS assigned_by;
ALTER TABLE documents DROP COLUMN IF EXISTS assigned_at;
ALTER TABLE documents DROP COLUMN IF EXISTS assigned_to;
```

**Step 3: Commit**

```bash
git add db/migrations/000019_add_review_assignment.up.sql db/migrations/000019_add_review_assignment.down.sql
git commit -m "feat: add migration 000019 for review assignment columns"
```

---

### Task 2: Domain Layer — Model, Enums, Errors

**Files:**
- Modify: `internal/domain/models.go:98` — add 3 fields to Document struct before `CreatedBy`
- Modify: `internal/domain/enums.go:198` — add `AuditDocumentAssigned` after `AuditDocumentDeleted`
- Modify: `internal/domain/errors.go:32` — add `ErrAssigneeCannotReview`

**Step 1: Add assignment fields to Document struct**

In `internal/domain/models.go`, add these 3 fields after `RetryAfter` (line 98) and before `CreatedBy` (line 99):

```go
	AssignedTo            *uuid.UUID           `db:"assigned_to" json:"assigned_to"`
	AssignedAt            *time.Time           `db:"assigned_at" json:"assigned_at,omitempty"`
	AssignedBy            *uuid.UUID           `db:"assigned_by" json:"assigned_by"`
```

**Step 2: Add audit action**

In `internal/domain/enums.go`, add after `AuditDocumentDeleted` (line 198):

```go
	AuditDocumentAssigned         AuditAction = "document.assigned"
```

**Step 3: Add sentinel error**

In `internal/domain/errors.go`, add after `ErrPasswordLoginNotAllowed` (line 32):

```go
	ErrAssigneeCannotReview        = errors.New("assignee does not have review permission on this collection")
```

**Step 4: Add error mapping**

In `internal/handler/response.go`, add a new case in `MapDomainError` after the `ErrPasswordLoginNotAllowed` case (line 113):

```go
	case errors.Is(err, domain.ErrAssigneeCannotReview):
		return http.StatusBadRequest, "ASSIGNEE_CANNOT_REVIEW", "assignee does not have review permission on this collection"
```

**Step 5: Run tests to verify nothing broke**

Run: `make test`
Expected: All existing tests pass (new fields are pointer types, default to nil).

**Step 6: Commit**

```bash
git add internal/domain/models.go internal/domain/enums.go internal/domain/errors.go internal/handler/response.go
git commit -m "feat: add Document assignment fields, AuditDocumentAssigned, ErrAssigneeCannotReview"
```

---

### Task 3: Viewer Cap Removal

**Files:**
- Modify: `internal/service/document_service.go:150-153` — remove viewer cap block
- Modify: `internal/service/collection_service.go:111-116` — remove viewer cap block
- Modify: `tests/unit/service/document_service_test.go` — update any viewer permission tests
- Modify: `tests/unit/service/collection_service_test.go` — update any viewer permission tests

**Step 1: Remove viewer cap from documentService.effectiveCollectionPerm**

In `internal/service/document_service.go`, delete lines 150-153 (the viewer cap block):

```go
	// DELETE THESE LINES:
	if role == domain.RoleViewer {
		if domain.CollectionPermLevel(explicit) > domain.CollectionPermLevel(domain.CollectionPermViewer) {
			explicit = domain.CollectionPermViewer
		}
	}
```

**Step 2: Remove viewer cap from collectionService.effectivePermission**

In `internal/service/collection_service.go`, delete lines 111-116 (the viewer cap block):

```go
	// DELETE THESE LINES:
	// For viewer role: cap at viewer regardless of explicit perm
	if role == domain.RoleViewer {
		if domain.CollectionPermLevel(explicit) > domain.CollectionPermLevel(domain.CollectionPermViewer) {
			explicit = domain.CollectionPermViewer
		}
	}
```

**Step 3: Run tests**

Run: `make test`

Check if any tests relied on the viewer cap behavior and fix them. The expected behavior change: viewers with explicit editor/owner grants now get that permission honored.

**Step 4: Commit**

```bash
git add internal/service/document_service.go internal/service/collection_service.go
git commit -m "fix: remove viewer role permission cap, honor explicit grants"
```

---

### Task 4: Tenant Validation in SetPermission

**Files:**
- Modify: `internal/service/collection_service.go:76-81` — add `userRepo` to struct
- Modify: `internal/service/collection_service.go:83-96` — add `userRepo` param to constructor
- Modify: `internal/service/collection_service.go:291-306` — add tenant validation
- Modify: `cmd/server/main.go:182` — pass `userRepo` to `NewCollectionService`
- Modify: `tests/unit/service/collection_service_test.go` — update setup, add test

**Step 1: Write the failing test**

In `tests/unit/service/collection_service_test.go`, add after the existing SetPermission tests (~line 619):

```go
func TestCollectionService_SetPermission_UserNotInTenant(t *testing.T) {
	svc, _, permRepo, _, _, userRepo := setupCollectionService()

	tenantID := uuid.New()
	collectionID := uuid.New()
	ownerID := uuid.New()
	targetUserID := uuid.New()

	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, ownerID).
		Return(&domain.CollectionPermissionEntry{Permission: domain.CollectionPermOwner}, nil)
	userRepo.On("GetByID", mock.Anything, tenantID, targetUserID).
		Return(nil, domain.ErrNotFound)

	err := svc.SetPermission(context.Background(), &service.SetPermissionInput{
		TenantID:     tenantID,
		CollectionID: collectionID,
		GrantedBy:    ownerID,
		CallerRole:   domain.RoleAdmin,
		UserID:       targetUserID,
		Permission:   domain.CollectionPermEditor,
	})

	assert.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
```

Note: The test setup helper (`setupCollectionService`) needs to be updated to create and return a `MockUserRepo`, and the `NewCollectionService` call needs to include it. Update the setup helper to:
1. Create a `userRepo := new(mocks.MockUserRepo)`.
2. Pass it as the 5th arg to `NewCollectionService`.
3. Return it.
4. In existing tests, add `userRepo.On("GetByID", ...).Return(&domain.User{}, nil).Maybe()` to the setup or per-test so existing tests still pass.

**Step 2: Run test to verify it fails**

Run: `go test ./tests/unit/service/ -run TestCollectionService_SetPermission_UserNotInTenant -v`
Expected: FAIL (compilation error — `NewCollectionService` doesn't accept `userRepo` yet)

**Step 3: Add userRepo to collectionService struct**

In `internal/service/collection_service.go`, modify the struct (line 76-81):

```go
type collectionService struct {
	collectionRepo port.CollectionRepository
	permRepo       port.CollectionPermissionRepository
	fileRepo       port.CollectionFileRepository
	fileSvc        FileService
	userRepo       port.UserRepository
}
```

**Step 4: Update constructor**

In `internal/service/collection_service.go`, modify `NewCollectionService` (lines 83-96):

```go
func NewCollectionService(
	collectionRepo port.CollectionRepository,
	permRepo port.CollectionPermissionRepository,
	fileRepo port.CollectionFileRepository,
	fileSvc FileService,
	userRepo port.UserRepository,
) CollectionService {
	return &collectionService{
		collectionRepo: collectionRepo,
		permRepo:       permRepo,
		fileRepo:       fileRepo,
		fileSvc:        fileSvc,
		userRepo:       userRepo,
	}
}
```

**Step 5: Add tenant validation to SetPermission**

In `internal/service/collection_service.go`, inside `SetPermission` (after the `requirePermission` check at line 294, before the log statement at line 297), add:

```go
	// Verify the target user exists in this tenant
	if _, err := s.userRepo.GetByID(ctx, input.TenantID, input.UserID); err != nil {
		return fmt.Errorf("target user not found in tenant: %w", domain.ErrNotFound)
	}
```

**Step 6: Update main.go**

In `cmd/server/main.go`, line 182, pass `userRepo`:

```go
	collectionSvc := service.NewCollectionService(collectionRepo, collectionPermRepo, collectionFileRepo, fileSvc, userRepo)
```

**Step 7: Update test setup and fix existing tests**

Update `setupCollectionService()` in `tests/unit/service/collection_service_test.go` to include `MockUserRepo` and return it. Add `.Maybe()` expectations for `GetByID` in existing SetPermission success tests so they pass the new tenant validation.

**Step 8: Run all tests**

Run: `make test`
Expected: All pass.

**Step 9: Commit**

```bash
git add internal/service/collection_service.go cmd/server/main.go tests/unit/service/collection_service_test.go
git commit -m "feat: add tenant validation to SetPermission, require target user in tenant"
```

---

### Task 5: Repository Layer — New Methods + Modified Signatures

**Files:**
- Modify: `internal/port/document_repository.go:12-24` — add 2 new methods, modify 3 signatures
- Modify: `internal/repository/postgres/document_repo.go` — implement new methods, modify existing
- Modify: `internal/mocks/mock_document_repo.go` — add mock methods, update signatures

**Step 1: Update DocumentRepository interface**

In `internal/port/document_repository.go`, modify the interface to:

```go
type DocumentRepository interface {
	Create(ctx context.Context, doc *domain.Document) error
	GetByID(ctx context.Context, tenantID, docID uuid.UUID) (*domain.Document, error)
	GetByFileID(ctx context.Context, tenantID, fileID uuid.UUID) (*domain.Document, error)
	ListByCollection(ctx context.Context, tenantID, collectionID uuid.UUID, assignedTo *uuid.UUID, offset, limit int) ([]domain.Document, int, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID, assignedTo *uuid.UUID, offset, limit int) ([]domain.Document, int, error)
	ListByUserCollections(ctx context.Context, tenantID, userID uuid.UUID, assignedTo *uuid.UUID, offset, limit int) ([]domain.Document, int, error)
	UpdateStructuredData(ctx context.Context, doc *domain.Document) error
	UpdateReviewStatus(ctx context.Context, doc *domain.Document) error
	UpdateAssignment(ctx context.Context, doc *domain.Document) error
	UpdateValidationResults(ctx context.Context, doc *domain.Document) error
	ListReviewQueue(ctx context.Context, tenantID, userID uuid.UUID, offset, limit int) ([]domain.Document, int, error)
	ClaimQueued(ctx context.Context, limit int) ([]domain.Document, error)
	Delete(ctx context.Context, tenantID, docID uuid.UUID) error
}
```

Changes: `ListByCollection`, `ListByTenant`, `ListByUserCollections` get `assignedTo *uuid.UUID` parameter. Two new methods: `UpdateAssignment`, `ListReviewQueue`.

**Step 2: Implement new/modified methods in postgres repo**

In `internal/repository/postgres/document_repo.go`:

**Modify `ListByCollection`** — add optional `assignedTo` filter:

```go
func (r *documentRepo) ListByCollection(ctx context.Context, tenantID, collectionID uuid.UUID, assignedTo *uuid.UUID, offset, limit int) ([]domain.Document, int, error) {
	countQuery := "SELECT COUNT(*) FROM documents WHERE tenant_id = $1 AND collection_id = $2"
	selectQuery := "SELECT * FROM documents WHERE tenant_id = $1 AND collection_id = $2"
	args := []interface{}{tenantID, collectionID}

	if assignedTo != nil {
		countQuery += fmt.Sprintf(" AND assigned_to = $%d", len(args)+1)
		selectQuery += fmt.Sprintf(" AND assigned_to = $%d", len(args)+1)
		args = append(args, *assignedTo)
	}

	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("documentRepo.ListByCollection count: %w", err)
	}

	selectQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	var docs []domain.Document
	if err := r.db.SelectContext(ctx, &docs, selectQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("documentRepo.ListByCollection: %w", err)
	}
	return docs, total, nil
}
```

Apply the same pattern to **`ListByTenant`** and **`ListByUserCollections`**.

**Add `UpdateAssignment`:**

```go
func (r *documentRepo) UpdateAssignment(ctx context.Context, doc *domain.Document) error {
	doc.UpdatedAt = time.Now().UTC()
	result, err := r.db.ExecContext(ctx,
		`UPDATE documents SET
			assigned_to = $1, assigned_at = $2, assigned_by = $3, updated_at = $4
		 WHERE id = $5 AND tenant_id = $6`,
		doc.AssignedTo, doc.AssignedAt, doc.AssignedBy, doc.UpdatedAt,
		doc.ID, doc.TenantID)
	if err != nil {
		return fmt.Errorf("documentRepo.UpdateAssignment: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrDocumentNotFound
	}
	return nil
}
```

**Add `ListReviewQueue`:**

```go
func (r *documentRepo) ListReviewQueue(ctx context.Context, tenantID, userID uuid.UUID, offset, limit int) ([]domain.Document, int, error) {
	baseWhere := "WHERE tenant_id = $1 AND assigned_to = $2 AND parsing_status = 'completed' AND review_status = 'pending'"

	var total int
	err := r.db.GetContext(ctx, &total,
		"SELECT COUNT(*) FROM documents "+baseWhere, tenantID, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("documentRepo.ListReviewQueue count: %w", err)
	}

	var docs []domain.Document
	err = r.db.SelectContext(ctx, &docs,
		"SELECT * FROM documents "+baseWhere+" ORDER BY assigned_at ASC LIMIT $3 OFFSET $4",
		tenantID, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("documentRepo.ListReviewQueue: %w", err)
	}
	return docs, total, nil
}
```

**Step 3: Update mock**

In `internal/mocks/mock_document_repo.go`:

Update signatures for `ListByCollection`, `ListByTenant`, `ListByUserCollections` to add `assignedTo *uuid.UUID` parameter. Add `UpdateAssignment` and `ListReviewQueue` mock methods following the existing nil-safety pattern.

Example for `UpdateAssignment`:
```go
func (m *MockDocumentRepo) UpdateAssignment(ctx context.Context, doc *domain.Document) error {
	args := m.Called(ctx, doc)
	return args.Error(0)
}
```

Example for `ListReviewQueue`:
```go
func (m *MockDocumentRepo) ListReviewQueue(ctx context.Context, tenantID, userID uuid.UUID, offset, limit int) ([]domain.Document, int, error) {
	args := m.Called(ctx, tenantID, userID, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]domain.Document), args.Int(1), args.Error(2)
}
```

**Step 4: Fix all callers**

Update every call to `ListByCollection`, `ListByTenant`, `ListByUserCollections` in the service layer and tests to pass `nil` as `assignedTo` (preserving existing behavior). Key locations:
- `internal/service/document_service.go` — `ListByCollection` call (~line 468), `ListByTenant` call (~line 472-475)
- All test files that mock these methods

**Step 5: Run tests**

Run: `make test`
Expected: All pass.

**Step 6: Commit**

```bash
git add internal/port/document_repository.go internal/repository/postgres/document_repo.go internal/mocks/mock_document_repo.go internal/service/document_service.go
git commit -m "feat: add UpdateAssignment, ListReviewQueue repo methods; add assignedTo filter to list methods"
```

---

### Task 6: Service Layer — AssignDocument + ListReviewQueue + Modified RetryParse

**Files:**
- Modify: `internal/service/document_service.go` — add `AssignDocumentInput` struct, `AssignDocument` and `ListReviewQueue` methods to interface and impl; modify `RetryParse`, `ListByTenant`, `ListByCollection`

**Step 1: Write failing tests for AssignDocument**

In `tests/unit/service/document_service_test.go`, add tests after the RetryParse section:

```go
// --- AssignDocument ---

func TestDocumentService_AssignDocument_Success(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _, userRepo, auditRepo := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	callerID := uuid.New()
	assigneeID := uuid.New()
	collectionID := uuid.New()

	doc := &domain.Document{
		ID: docID, TenantID: tenantID, CollectionID: collectionID,
		ParsingStatus: domain.ParsingStatusCompleted,
	}

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(doc, nil)
	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, callerID).
		Return(&domain.CollectionPermissionEntry{Permission: domain.CollectionPermEditor}, nil).Maybe()
	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, assigneeID).
		Return(&domain.CollectionPermissionEntry{Permission: domain.CollectionPermEditor}, nil).Maybe()
	userRepo.On("GetByID", mock.Anything, tenantID, assigneeID).
		Return(&domain.User{ID: assigneeID, TenantID: tenantID}, nil)
	docRepo.On("UpdateAssignment", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil)

	result, err := svc.AssignDocument(context.Background(), &service.AssignDocumentInput{
		TenantID: tenantID, DocumentID: docID, CallerID: callerID,
		CallerRole: domain.RoleMember, AssigneeID: &assigneeID,
	})

	require.NoError(t, err)
	assert.Equal(t, &assigneeID, result.AssignedTo)
	assert.Equal(t, &callerID, result.AssignedBy)
	assert.NotNil(t, result.AssignedAt)
	docRepo.AssertCalled(t, "UpdateAssignment", mock.Anything, mock.AnythingOfType("*domain.Document"))
}

func TestDocumentService_AssignDocument_Unassign(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _, _, auditRepo := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	callerID := uuid.New()
	collectionID := uuid.New()
	prevAssignee := uuid.New()

	doc := &domain.Document{
		ID: docID, TenantID: tenantID, CollectionID: collectionID,
		ParsingStatus: domain.ParsingStatusCompleted,
		AssignedTo: &prevAssignee,
	}

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(doc, nil)
	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, callerID).
		Return(&domain.CollectionPermissionEntry{Permission: domain.CollectionPermEditor}, nil).Maybe()
	docRepo.On("UpdateAssignment", mock.Anything, mock.AnythingOfType("*domain.Document")).Return(nil)

	result, err := svc.AssignDocument(context.Background(), &service.AssignDocumentInput{
		TenantID: tenantID, DocumentID: docID, CallerID: callerID,
		CallerRole: domain.RoleMember, AssigneeID: nil,
	})

	require.NoError(t, err)
	assert.Nil(t, result.AssignedTo)
	assert.Nil(t, result.AssignedAt)
	assert.Nil(t, result.AssignedBy)
}

func TestDocumentService_AssignDocument_NotParsed(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	callerID := uuid.New()
	assigneeID := uuid.New()
	collectionID := uuid.New()

	doc := &domain.Document{
		ID: docID, TenantID: tenantID, CollectionID: collectionID,
		ParsingStatus: domain.ParsingStatusPending,
	}

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(doc, nil)
	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, callerID).
		Return(&domain.CollectionPermissionEntry{Permission: domain.CollectionPermEditor}, nil).Maybe()

	_, err := svc.AssignDocument(context.Background(), &service.AssignDocumentInput{
		TenantID: tenantID, DocumentID: docID, CallerID: callerID,
		CallerRole: domain.RoleMember, AssigneeID: &assigneeID,
	})

	assert.ErrorIs(t, err, domain.ErrDocumentNotParsed)
}

func TestDocumentService_AssignDocument_AssigneeNotInTenant(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _, userRepo, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	callerID := uuid.New()
	assigneeID := uuid.New()
	collectionID := uuid.New()

	doc := &domain.Document{
		ID: docID, TenantID: tenantID, CollectionID: collectionID,
		ParsingStatus: domain.ParsingStatusCompleted,
	}

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(doc, nil)
	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, callerID).
		Return(&domain.CollectionPermissionEntry{Permission: domain.CollectionPermEditor}, nil).Maybe()
	userRepo.On("GetByID", mock.Anything, tenantID, assigneeID).
		Return(nil, domain.ErrNotFound)

	_, err := svc.AssignDocument(context.Background(), &service.AssignDocumentInput{
		TenantID: tenantID, DocumentID: docID, CallerID: callerID,
		CallerRole: domain.RoleMember, AssigneeID: &assigneeID,
	})

	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestDocumentService_AssignDocument_AssigneeNoPermission(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _, userRepo, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	callerID := uuid.New()
	assigneeID := uuid.New()
	collectionID := uuid.New()

	doc := &domain.Document{
		ID: docID, TenantID: tenantID, CollectionID: collectionID,
		ParsingStatus: domain.ParsingStatusCompleted,
	}

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(doc, nil)
	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, callerID).
		Return(&domain.CollectionPermissionEntry{Permission: domain.CollectionPermEditor}, nil).Maybe()
	userRepo.On("GetByID", mock.Anything, tenantID, assigneeID).
		Return(&domain.User{ID: assigneeID, TenantID: tenantID, Role: domain.RoleViewer}, nil)
	// Assignee has only viewer permission
	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, assigneeID).
		Return(&domain.CollectionPermissionEntry{Permission: domain.CollectionPermViewer}, nil).Maybe()

	_, err := svc.AssignDocument(context.Background(), &service.AssignDocumentInput{
		TenantID: tenantID, DocumentID: docID, CallerID: callerID,
		CallerRole: domain.RoleMember, AssigneeID: &assigneeID,
	})

	assert.ErrorIs(t, err, domain.ErrAssigneeCannotReview)
}

func TestDocumentService_AssignDocument_CallerNoPermission(t *testing.T) {
	svc, docRepo, _, permRepo, _, _, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	callerID := uuid.New()
	assigneeID := uuid.New()
	collectionID := uuid.New()

	doc := &domain.Document{
		ID: docID, TenantID: tenantID, CollectionID: collectionID,
		ParsingStatus: domain.ParsingStatusCompleted,
	}

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(doc, nil)
	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, callerID).
		Return(nil, errors.New("not found")).Maybe()

	_, err := svc.AssignDocument(context.Background(), &service.AssignDocumentInput{
		TenantID: tenantID, DocumentID: docID, CallerID: callerID,
		CallerRole: domain.RoleViewer, AssigneeID: &assigneeID,
	})

	assert.ErrorIs(t, err, domain.ErrCollectionPermDenied)
}
```

**Step 2: Write failing tests for ListReviewQueue**

```go
func TestDocumentService_ListReviewQueue_Success(t *testing.T) {
	svc, docRepo, _, _, _, _, _, _, _ := setupDocumentService()

	tenantID := uuid.New()
	userID := uuid.New()

	expected := []domain.Document{{ID: uuid.New(), TenantID: tenantID}}
	docRepo.On("ListReviewQueue", mock.Anything, tenantID, userID, 0, 20).Return(expected, 1, nil)

	docs, total, err := svc.ListReviewQueue(context.Background(), tenantID, userID, 0, 20)

	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, docs, 1)
}
```

**Step 3: Write failing test for RetryParse clearing assignment**

```go
func TestDocumentService_RetryParse_ClearsAssignment(t *testing.T) {
	svc, docRepo, fileRepo, permRepo, p, storage, tagRepo, _, auditRepo := setupDocumentService()

	tenantID := uuid.New()
	docID := uuid.New()
	userID := uuid.New()
	collectionID := uuid.New()
	assignee := uuid.New()

	doc := &domain.Document{
		ID: docID, TenantID: tenantID, CollectionID: collectionID,
		FileID: uuid.New(), ParsingStatus: domain.ParsingStatusFailed,
		AssignedTo: &assignee, AssignedAt: ptrTime(time.Now()), AssignedBy: &userID,
	}

	docRepo.On("GetByID", mock.Anything, tenantID, docID).Return(doc, nil)
	permRepo.On("GetByCollectionAndUser", mock.Anything, collectionID, userID).
		Return(&domain.CollectionPermissionEntry{Permission: domain.CollectionPermEditor}, nil).Maybe()
	fileRepo.On("GetByID", mock.Anything, tenantID, doc.FileID).Return(&domain.FileMeta{}, nil)
	tagRepo.On("DeleteByDocumentAndSource", mock.Anything, docID, "auto").Return(nil).Maybe()
	docRepo.On("UpdateStructuredData", mock.Anything, mock.AnythingOfType("*domain.Document")).
		Run(func(args mock.Arguments) {
			d := args.Get(1).(*domain.Document)
			assert.Nil(t, d.AssignedTo, "assignment should be cleared on retry")
			assert.Nil(t, d.AssignedAt)
			assert.Nil(t, d.AssignedBy)
		}).Return(nil)
	// Background parsing mocks
	p.On("Parse", mock.Anything, mock.Anything).Return(&port.ParseOutput{
		StructuredData: json.RawMessage("{}"), ConfidenceScores: json.RawMessage("{}"),
	}, nil).Maybe()
	storage.On("Download", mock.Anything, mock.Anything, mock.Anything).Return([]byte("test"), nil).Maybe()

	result, err := svc.RetryParse(context.Background(), tenantID, docID, userID, domain.RoleMember)

	require.NoError(t, err)
	assert.Nil(t, result.AssignedTo)
}
```

Helper function (add if not present):
```go
func ptrTime(t time.Time) *time.Time { return &t }
```

**Step 4: Run tests to verify they fail**

Run: `go test ./tests/unit/service/ -run "TestDocumentService_Assign|TestDocumentService_ListReview|TestDocumentService_RetryParse_ClearsAssignment" -v`
Expected: FAIL (methods don't exist yet)

**Step 5: Implement AssignDocument**

In `internal/service/document_service.go`:

Add the DTO after `UpdateReviewInput` (~line 49):

```go
// AssignDocumentInput is the DTO for assigning a document to a reviewer.
type AssignDocumentInput struct {
	TenantID   uuid.UUID
	DocumentID uuid.UUID
	CallerID   uuid.UUID
	CallerRole domain.UserRole
	AssigneeID *uuid.UUID // nil = unassign
}
```

Add to the `DocumentService` interface (after `UpdateReview`):

```go
	AssignDocument(ctx context.Context, input *AssignDocumentInput) (*domain.Document, error)
	ListReviewQueue(ctx context.Context, tenantID, userID uuid.UUID, offset, limit int) ([]domain.Document, int, error)
```

Add the implementation after `UpdateReview`:

```go
func (s *documentService) AssignDocument(ctx context.Context, input *AssignDocumentInput) (*domain.Document, error) {
	doc, err := s.docRepo.GetByID(ctx, input.TenantID, input.DocumentID)
	if err != nil {
		return nil, err
	}

	// Check editor+ permission on the collection (caller)
	if err := s.requireCollectionPerm(ctx, doc.CollectionID, input.CallerID, input.CallerRole, domain.CollectionPermEditor); err != nil {
		return nil, err
	}

	if doc.ParsingStatus != domain.ParsingStatusCompleted {
		return nil, domain.ErrDocumentNotParsed
	}

	if input.AssigneeID != nil {
		// Verify assignee exists in tenant
		assignee, err := s.userRepo.GetByID(ctx, input.TenantID, *input.AssigneeID)
		if err != nil {
			return nil, fmt.Errorf("assignee not found: %w", err)
		}

		// Verify assignee has editor+ effective permission on collection
		if err := s.requireCollectionPerm(ctx, doc.CollectionID, *input.AssigneeID, assignee.Role, domain.CollectionPermEditor); err != nil {
			return nil, domain.ErrAssigneeCannotReview
		}

		now := time.Now().UTC()
		doc.AssignedTo = input.AssigneeID
		doc.AssignedAt = &now
		doc.AssignedBy = &input.CallerID
	} else {
		// Unassign
		doc.AssignedTo = nil
		doc.AssignedAt = nil
		doc.AssignedBy = nil
	}

	if err := s.docRepo.UpdateAssignment(ctx, doc); err != nil {
		return nil, fmt.Errorf("updating assignment: %w", err)
	}

	// Audit
	var changes json.RawMessage
	if input.AssigneeID != nil {
		changes, _ = json.Marshal(map[string]interface{}{
			"assigned_to": input.AssigneeID.String(), "assigned_by": input.CallerID.String(),
		})
	} else {
		prev := ""
		if doc.AssignedTo != nil {
			prev = doc.AssignedTo.String()
		}
		changes, _ = json.Marshal(map[string]interface{}{
			"assigned_to": nil, "assigned_by": input.CallerID.String(), "previous_assignee": prev,
		})
	}
	s.audit(ctx, input.TenantID, input.DocumentID, &input.CallerID, domain.AuditDocumentAssigned, changes)

	return doc, nil
}

func (s *documentService) ListReviewQueue(ctx context.Context, tenantID, userID uuid.UUID, offset, limit int) ([]domain.Document, int, error) {
	return s.docRepo.ListReviewQueue(ctx, tenantID, userID, offset, limit)
}
```

**Step 6: Modify RetryParse to clear assignment**

In `internal/service/document_service.go`, in the `RetryParse` method, after the line that sets `doc.ConfidenceScores` (line 692) and before the `UpdateStructuredData` call (line 693), add:

```go
	doc.AssignedTo = nil
	doc.AssignedAt = nil
	doc.AssignedBy = nil
```

**Step 7: Update ListByTenant and ListByCollection to accept assignedTo**

Modify `ListByTenant` to accept and pass `assignedTo`:

```go
func (s *documentService) ListByTenant(ctx context.Context, tenantID, userID uuid.UUID, role domain.UserRole, assignedTo *uuid.UUID, offset, limit int) ([]domain.Document, int, error) {
	if role == domain.RoleAdmin || role == domain.RoleManager || role == domain.RoleMember {
		return s.docRepo.ListByTenant(ctx, tenantID, assignedTo, offset, limit)
	}
	return s.docRepo.ListByUserCollections(ctx, tenantID, userID, assignedTo, offset, limit)
}
```

Similarly update `ListByCollection`.

Update the `DocumentService` interface signatures to match.

**Step 8: Update mock DocumentService**

Add `AssignDocument` and `ListReviewQueue` to `MockDocumentService` in `internal/mocks/mock_document_service.go`. Update `ListByTenant` and `ListByCollection` signatures there as well.

**Step 9: Run all tests**

Run: `make test`
Expected: All pass.

**Step 10: Commit**

```bash
git add internal/service/document_service.go internal/mocks/mock_document_service.go tests/unit/service/document_service_test.go
git commit -m "feat: add AssignDocument, ListReviewQueue service methods; RetryParse clears assignment"
```

---

### Task 7: Handler Layer — Assign + ReviewQueue + List Filter

**Files:**
- Modify: `internal/handler/document_handler.go` — add `AssignDocument` and `ReviewQueue` handlers, modify `List`
- Modify: `internal/router/router.go` — add routes
- Add tests: `tests/unit/handler/document_handler_test.go`

**Step 1: Write failing handler tests**

In `tests/unit/handler/document_handler_test.go`:

```go
// --- AssignDocument ---

func TestDocumentHandler_AssignDocument_Success(t *testing.T) {
	h, mockSvc := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	docID := uuid.New()
	assigneeID := uuid.New()

	doc := &domain.Document{ID: docID, AssignedTo: &assigneeID}
	mockSvc.On("AssignDocument", mock.Anything, mock.MatchedBy(func(input *service.AssignDocumentInput) bool {
		return input.TenantID == tenantID && input.DocumentID == docID && *input.AssigneeID == assigneeID
	})).Return(doc, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAuthContext(c, tenantID, userID, "member")
	c.Params = gin.Params{{Key: "id", Value: docID.String()}}
	body, _ := json.Marshal(map[string]interface{}{"assignee_id": assigneeID.String()})
	c.Request, _ = http.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.AssignDocument(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDocumentHandler_AssignDocument_Unassign(t *testing.T) {
	h, mockSvc := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()
	docID := uuid.New()

	doc := &domain.Document{ID: docID}
	mockSvc.On("AssignDocument", mock.Anything, mock.MatchedBy(func(input *service.AssignDocumentInput) bool {
		return input.AssigneeID == nil
	})).Return(doc, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAuthContext(c, tenantID, userID, "manager")
	c.Params = gin.Params{{Key: "id", Value: docID.String()}}
	body, _ := json.Marshal(map[string]interface{}{"assignee_id": nil})
	c.Request, _ = http.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.AssignDocument(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDocumentHandler_AssignDocument_InvalidDocID(t *testing.T) {
	h, _ := newDocumentHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAuthContext(c, uuid.New(), uuid.New(), "admin")
	c.Params = gin.Params{{Key: "id", Value: "bad-id"}}
	c.Request, _ = http.NewRequest(http.MethodPut, "/", nil)

	h.AssignDocument(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDocumentHandler_AssignDocument_NoAuth(t *testing.T) {
	h, _ := newDocumentHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: uuid.New().String()}}
	c.Request, _ = http.NewRequest(http.MethodPut, "/", nil)

	h.AssignDocument(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- ReviewQueue ---

func TestDocumentHandler_ReviewQueue_Success(t *testing.T) {
	h, mockSvc := newDocumentHandler()

	tenantID := uuid.New()
	userID := uuid.New()

	docs := []domain.Document{{ID: uuid.New()}}
	mockSvc.On("ListReviewQueue", mock.Anything, tenantID, userID, 0, 20).Return(docs, 1, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setAuthContext(c, tenantID, userID, "member")
	c.Request, _ = http.NewRequest(http.MethodGet, "/?offset=0&limit=20", nil)

	h.ReviewQueue(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDocumentHandler_ReviewQueue_NoAuth(t *testing.T) {
	h, _ := newDocumentHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

	h.ReviewQueue(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./tests/unit/handler/ -run "TestDocumentHandler_Assign|TestDocumentHandler_ReviewQueue" -v`
Expected: FAIL (methods don't exist)

**Step 3: Implement handlers**

In `internal/handler/document_handler.go`:

**AssignDocument handler:**

```go
// AssignDocument handles PUT /api/v1/documents/:id/assign
func (h *DocumentHandler) AssignDocument(c *gin.Context) {
	tenantID, userID, role, ok := extractAuthContext(c)
	if !ok {
		return
	}

	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid document ID")
		return
	}

	var req struct {
		AssigneeID *string `json:"assignee_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "request body is required")
		return
	}

	var assigneeID *uuid.UUID
	if req.AssigneeID != nil && *req.AssigneeID != "" {
		parsed, err := uuid.Parse(*req.AssigneeID)
		if err != nil {
			RespondError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid assignee_id format")
			return
		}
		assigneeID = &parsed
	}

	doc, err := h.documentService.AssignDocument(c.Request.Context(), &service.AssignDocumentInput{
		TenantID:   tenantID,
		DocumentID: docID,
		CallerID:   userID,
		CallerRole: role,
		AssigneeID: assigneeID,
	})
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, doc)
}
```

**ReviewQueue handler:**

```go
// ReviewQueue handles GET /api/v1/documents/review-queue
func (h *DocumentHandler) ReviewQueue(c *gin.Context) {
	tenantID, userID, _, ok := extractAuthContext(c)
	if !ok {
		return
	}

	offset, limit := parsePagination(c)

	docs, total, err := h.documentService.ListReviewQueue(c.Request.Context(), tenantID, userID, offset, limit)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondPaginated(c, docs, PagMeta{Total: total, Offset: offset, Limit: limit})
}
```

**Modify List handler** to support `assigned_to` query param:

In the existing `List` handler, after extracting `collectionIDStr`, add:

```go
	var assignedTo *uuid.UUID
	if assignedToStr := c.Query("assigned_to"); assignedToStr != "" {
		parsed, err := uuid.Parse(assignedToStr)
		if err != nil {
			RespondError(c, http.StatusBadRequest, "INVALID_ID", "invalid assigned_to")
			return
		}
		assignedTo = &parsed
	}
```

Then pass `assignedTo` to the `ListByCollection` and `ListByTenant` calls.

**Step 4: Add routes**

In `internal/router/router.go`, in the document routes section (after line 91, before `documents.GET("/:id", ...)`):

```go
	documents.GET("/review-queue", documentH.ReviewQueue)
```

After `documents.PUT("/:id/review", ...)` (line 96):

```go
	documents.PUT("/:id/assign", documentH.AssignDocument)
```

Note: `/review-queue` must come before `/:id` to avoid Gin treating "review-queue" as a document ID.

**Step 5: Run all tests**

Run: `make test`
Expected: All pass.

**Step 6: Run linter**

Run: `make lint`
Expected: No new warnings.

**Step 7: Commit**

```bash
git add internal/handler/document_handler.go internal/router/router.go tests/unit/handler/document_handler_test.go
git commit -m "feat: add AssignDocument, ReviewQueue handlers and routes; add assigned_to filter to List"
```

---

### Task 8: Update CLAUDE.md Documentation

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Update Document model section**

In the Document description, add `assigned_to`, `assigned_at`, `assigned_by` fields.

**Step 2: Update audit actions**

Add `AuditDocumentAssigned` to the audit actions list.

**Step 3: Update errors section**

Add `ErrAssigneeCannotReview`.

**Step 4: Update endpoints**

Add `PUT /documents/:id/assign` and `GET /documents/review-queue`. Add `assigned_to` query param to `GET /documents`.

**Step 5: Update key conventions**

- Note that viewer cap is removed — explicit grants are honored for viewer role
- Note that `SetPermission` validates target user exists in tenant
- Note that `RetryParse` clears assignment
- Note that `EditStructuredData` keeps assignment
- Add `assigned_to` filter documentation
- Update the "Modifying review" section with assignment info

**Step 6: Update gotchas**

Add: "Assignment is soft — anyone with editor+ can review regardless of assignment"

**Step 7: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with review assignment, viewer cap removal, tenant validation"
```

---

### Task 9: Final Verification

**Step 1: Run full test suite**

Run: `make test`
Expected: All pass.

**Step 2: Run linter**

Run: `make lint`
Expected: No warnings.

**Step 3: Verify build**

Run: `make build`
Expected: Binary compiles successfully.

**Step 4: Final commit (if any fixes needed)**

Fix any issues found and commit.

---

## Execution Order Summary

| Task | Description | Dependencies |
|------|-------------|-------------|
| 1 | Database migration | None |
| 2 | Domain layer (model, enums, errors) | None |
| 3 | Viewer cap removal | None |
| 4 | Tenant validation in SetPermission | None |
| 5 | Repository layer (new + modified methods) | Tasks 2 |
| 6 | Service layer (AssignDocument, ListReviewQueue, modified RetryParse) | Tasks 2, 3, 5 |
| 7 | Handler layer (handlers, routes, List filter) | Task 6 |
| 8 | CLAUDE.md documentation | Tasks 1-7 |
| 9 | Final verification | All |

Tasks 1-4 can be done in parallel. Task 5 depends on 2. Task 6 depends on 2, 3, 5. Task 7 depends on 6.
