# Frontend Integration Guide: Review Assignment & Review Queue

## Overview

The backend now supports assigning documents to specific users for review, a review queue endpoint, and improved permission handling. This guide covers how to integrate these features into the frontend.

### What Changed

1. **Review assignment** — Documents can be assigned to a specific user for review via `PUT /documents/:id/assign`
2. **Review queue** — Users can see documents assigned to them via `GET /documents/review-queue`
3. **Document list filtering** — `GET /documents` now supports `assigned_to` query param
4. **Viewer role fix** — Viewer-role users with explicit editor/owner permission on a collection can now review documents in that collection (previously this was silently blocked)
5. **Permission validation** — `POST /collections/:id/permissions` now verifies the target user belongs to the same tenant

### New Document Fields

Every document response now includes three new fields:

```json
{
  "id": "uuid",
  "assigned_to": "uuid | null",
  "assigned_at": "2026-02-14T10:30:00Z | null",
  "assigned_by": "uuid | null",
  ...existing fields...
}
```

- `assigned_to` — UUID of the user assigned to review this document, or null
- `assigned_at` — When the assignment was made
- `assigned_by` — Who made the assignment

---

## API Endpoints

### 1. Assign Document for Review

```
PUT /api/v1/documents/:id/assign
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "assignee_id": "uuid"
}
```

To unassign:
```json
{
  "assignee_id": null
}
```

**Requirements:**
- Caller must have editor+ permission on the document's collection
- Document must have `parsing_status: "completed"`
- Assignee must exist in the same tenant
- Assignee must have editor+ effective permission on the document's collection (they need to be able to actually review it)

**Success Response (200):**
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "assigned_to": "uuid",
    "assigned_at": "2026-02-14T10:30:00Z",
    "assigned_by": "uuid",
    "parsing_status": "completed",
    "review_status": "pending",
    ...other document fields...
  }
}
```

**Error Responses:**
- `400 DOCUMENT_NOT_PARSED` — Document not yet parsed
- `400 ASSIGNEE_CANNOT_REVIEW` — Assignee lacks editor+ permission on the collection
- `403 COLLECTION_PERM_DENIED` — Caller lacks editor+ permission
- `404 NOT_FOUND` — Document or assignee not found

### 2. Review Queue

```
GET /api/v1/documents/review-queue?offset=0&limit=20
Authorization: Bearer <access_token>
```

Returns documents assigned to the current user where:
- `parsing_status = "completed"`
- `review_status = "pending"`

Ordered by `assigned_at ASC` (oldest assignment first).

**Success Response (200):**
```json
{
  "success": true,
  "data": [
    {
      "id": "uuid",
      "name": "Invoice #123",
      "assigned_to": "current-user-uuid",
      "assigned_at": "2026-02-14T10:30:00Z",
      "assigned_by": "manager-uuid",
      "parsing_status": "completed",
      "review_status": "pending",
      "validation_status": "valid",
      "reconciliation_status": "valid",
      ...other document fields...
    }
  ],
  "meta": {
    "total": 5,
    "offset": 0,
    "limit": 20
  }
}
```

### 3. List Documents with Assignment Filter

```
GET /api/v1/documents?assigned_to=<userId>&offset=0&limit=20
Authorization: Bearer <access_token>
```

Filters documents assigned to a specific user. Can be combined with existing `collection_id` filter:

```
GET /api/v1/documents?collection_id=<uuid>&assigned_to=<userId>
```

### 4. Review a Document (Existing, Unchanged)

```
PUT /api/v1/documents/:id/review
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "status": "approved",
  "notes": "Verified against source invoice"
}
```

When a document is reviewed (approved/rejected), the assignment stays but the document disappears from the review queue (which filters by `review_status: "pending"`).

### 5. Set Collection Permission (Existing, Improved)

```
POST /api/v1/collections/:id/permissions
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "user_id": "uuid",
  "permission": "editor"
}
```

**What changed:** The backend now validates that the target user exists in the same tenant. Previously, passing an invalid or cross-tenant user ID would silently succeed at the service level (relying on DB constraints). Now it returns a clean 404.

---

## Behavioral Changes

### Viewer Role Permission Fix

Previously, viewer-role users were hard-capped at viewer-level permission even if explicitly granted editor or owner. This is now fixed:

| Scenario (Before) | Scenario (After) |
|-------------------|-------------------|
| Viewer with explicit editor grant → capped to viewer, cannot review | Viewer with explicit editor grant → gets editor, CAN review |
| Viewer with explicit owner grant → capped to viewer | Viewer with explicit owner grant → gets owner |

**Frontend impact:** If your UI hides review buttons based solely on tenant role, update the logic to also check the user's effective collection permission. The `current_user_permission` field in collection responses reflects the corrected effective permission.

### Retry Parse Clears Assignment

When a document is retried (`POST /documents/:id/retry`), the assignment is cleared (`assigned_to`, `assigned_at`, `assigned_by` all become null). The document goes back to parsing, so a new assignment should be made after parsing completes again.

### Edit Structured Data Keeps Assignment

When structured data is edited (`PUT /documents/:id`), the assignment stays. The review status resets to `pending`, so the assigned reviewer will see the document back in their review queue.

---

## Frontend Implementation

### Review Queue Page

```typescript
// pages/review-queue.tsx

interface Document {
  id: string;
  name: string;
  assigned_to: string | null;
  assigned_at: string | null;
  assigned_by: string | null;
  parsing_status: string;
  review_status: string;
  validation_status: string;
  reconciliation_status: string;
  collection_id: string;
  created_at: string;
}

interface PaginatedResponse<T> {
  success: boolean;
  data: T[];
  meta: { total: number; offset: number; limit: number };
}

async function fetchReviewQueue(offset = 0, limit = 20): Promise<PaginatedResponse<Document>> {
  const res = await apiClient(`/documents/review-queue?offset=${offset}&limit=${limit}`);
  if (!res.ok) throw new Error('Failed to fetch review queue');
  return res.json();
}
```

### Assign Document

```typescript
// lib/documents.ts

async function assignDocument(docId: string, assigneeId: string): Promise<Document> {
  const res = await apiClient(`/documents/${docId}/assign`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ assignee_id: assigneeId }),
  });

  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error?.message || 'Assignment failed');
  }

  const { data } = await res.json();
  return data;
}

async function unassignDocument(docId: string): Promise<Document> {
  const res = await apiClient(`/documents/${docId}/assign`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ assignee_id: null }),
  });

  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error?.message || 'Unassignment failed');
  }

  const { data } = await res.json();
  return data;
}
```

### Assignment UI Component

```typescript
// components/AssignReviewer.tsx

interface AssignReviewerProps {
  document: Document;
  teamMembers: User[];      // users with editor+ on this collection
  onAssigned: (doc: Document) => void;
}

function AssignReviewer({ document, teamMembers, onAssigned }: AssignReviewerProps) {
  const [selectedUser, setSelectedUser] = useState(document.assigned_to || '');
  const [loading, setLoading] = useState(false);

  // Only show for documents that are parsed and the user has editor+ permission
  if (document.parsing_status !== 'completed') return null;

  const handleAssign = async () => {
    setLoading(true);
    try {
      const updated = selectedUser
        ? await assignDocument(document.id, selectedUser)
        : await unassignDocument(document.id);
      onAssigned(updated);
    } catch (err) {
      // Handle errors:
      // - ASSIGNEE_CANNOT_REVIEW: user lacks permission
      // - NOT_FOUND: user not in tenant
      // - COLLECTION_PERM_DENIED: caller lacks permission
      showError(err.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="assign-reviewer">
      <label>Assign Reviewer</label>
      <select value={selectedUser} onChange={(e) => setSelectedUser(e.target.value)}>
        <option value="">Unassigned</option>
        {teamMembers.map((user) => (
          <option key={user.id} value={user.id}>
            {user.full_name} ({user.email})
          </option>
        ))}
      </select>
      <button onClick={handleAssign} disabled={loading}>
        {loading ? 'Saving...' : 'Assign'}
      </button>
    </div>
  );
}
```

### Review Queue Navigation

```typescript
// hooks/useAuth.ts — updated

function useAuth() {
  const user = useUser();

  return {
    ...user,
    isFree: user?.role === 'free',
    isPaid: user?.role !== 'free',
    isAdmin: user?.role === 'admin',
    canManageTeam: user?.role === 'admin' || user?.role === 'manager',
    canUpload: ['admin', 'manager', 'member', 'free'].includes(user?.role || ''),
    // Review capability now depends on collection-level permission, not just role
    // Use the review queue to determine if user has pending reviews
  };
}

// In sidebar navigation — add review queue link
function DashboardLayout({ children }) {
  const { isPaid } = useAuth();

  return (
    <div className="dashboard">
      <Sidebar>
        <NavItem href="/dashboard" label="Dashboard" />
        <NavItem href="/collections" label="Collections" />
        <NavItem href="/documents" label="Documents" />
        {isPaid && <NavItem href="/review-queue" label="Review Queue" badge={reviewCount} />}
        <NavItem href="/files" label="Files" />
      </Sidebar>
      <main>{children}</main>
    </div>
  );
}
```

### Document List with Assignment Filter

```typescript
// For managers: see what's assigned to each team member

async function fetchDocuments(params: {
  collectionId?: string;
  assignedTo?: string;
  offset?: number;
  limit?: number;
}): Promise<PaginatedResponse<Document>> {
  const query = new URLSearchParams();
  if (params.collectionId) query.set('collection_id', params.collectionId);
  if (params.assignedTo) query.set('assigned_to', params.assignedTo);
  query.set('offset', String(params.offset || 0));
  query.set('limit', String(params.limit || 20));

  const res = await apiClient(`/documents?${query}`);
  if (!res.ok) throw new Error('Failed to fetch documents');
  return res.json();
}
```

### Displaying Assignment Status on Document Cards

```typescript
function DocumentCard({ doc, users }: { doc: Document; users: Map<string, User> }) {
  const assignee = doc.assigned_to ? users.get(doc.assigned_to) : null;

  return (
    <div className="document-card">
      <h3>{doc.name}</h3>
      <StatusBadge status={doc.parsing_status} />
      <StatusBadge status={doc.review_status} />
      <StatusBadge status={doc.validation_status} />

      {assignee && (
        <div className="assignment-badge">
          Assigned to {assignee.full_name}
          <span className="assigned-date">
            {new Date(doc.assigned_at!).toLocaleDateString()}
          </span>
        </div>
      )}

      {doc.review_status === 'pending' && !doc.assigned_to && (
        <span className="unassigned-warning">Unassigned</span>
      )}
    </div>
  );
}
```

---

## Error Handling

| Error Code | HTTP Status | Meaning | Frontend Action |
|-----------|-------------|---------|-----------------|
| ASSIGNEE_CANNOT_REVIEW | 400 | Assignee lacks editor+ on collection | Show "This user doesn't have review permission on this collection" |
| DOCUMENT_NOT_PARSED | 400 | Document not yet parsed | Show "Document must be parsed before assigning a reviewer" |
| COLLECTION_PERM_DENIED | 403 | Caller lacks editor+ permission | Hide assign button / show "You don't have permission" |
| NOT_FOUND | 404 | Document or assignee not found | Show "Document or user not found" |
| INVALID_ID | 400 | Bad UUID format | Validate UUIDs on the frontend before sending |

---

## Audit Trail

Assignment creates a `document.assigned` audit entry visible in `GET /documents/:id/audit`:

```json
{
  "id": "uuid",
  "document_id": "uuid",
  "action": "document.assigned",
  "user_id": "uuid-of-caller",
  "changes": {
    "assigned_to": "uuid-of-assignee",
    "assigned_by": "uuid-of-caller"
  },
  "created_at": "2026-02-14T10:30:00Z"
}
```

Unassignment:
```json
{
  "changes": {
    "assigned_to": null,
    "assigned_by": "uuid-of-caller",
    "previous_assignee": "uuid-of-previous-assignee"
  }
}
```

---

## Gotchas

1. **Assignment doesn't prevent others from reviewing.** Any user with editor+ permission on the collection can still review the document, even if it's assigned to someone else. Assignment is a soft suggestion, not a lock.
2. **Review doesn't clear the assignment.** After approval/rejection, the `assigned_to` field stays. The document leaves the review queue because `review_status` is no longer `"pending"`.
3. **Retry clears the assignment.** When a document is retried, the assignment is cleared because the document goes back to parsing. A new assignment should be made after parsing completes.
4. **Edit keeps the assignment.** When structured data is edited, the assignment stays. The review status resets to pending, so the document reappears in the assignee's review queue.
5. **Assignee must have editor+ permission.** The backend validates this. If a user's permission is later downgraded, they'll still appear as assigned but won't be able to review (the review endpoint checks permission independently).
6. **Viewer role users can now review** if they have an explicit editor or owner grant on the collection. Previously this was silently blocked. Update any frontend logic that hides review UI based solely on tenant role.
7. **The review queue is implicitly scoped to the current user.** No need to pass a user ID — the backend uses the JWT claims.
8. **`assigned_to` filter on GET /documents respects role-based visibility.** A viewer can only see documents from collections they have permission on, even when filtering by `assigned_to`.
