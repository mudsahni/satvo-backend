# Frontend Integration Guide: Free Tier

## Overview

The backend now supports a free tier that allows unregistered users to try the invoice parsing service. Free users self-register on a shared "satvos" tenant, receive 5 document parses per month, and can only see their own files/collections.

This guide covers how to integrate the free tier into a Next.js frontend.

---

## UX Flow

### Landing Page → Try It Free

```
Landing Page
  └─ "Try for Free" CTA
       └─ Registration Form (email + password + optional full name)
            ├─ If full_name not provided, derive from email prefix ("user" from "user@email.com")
            └─ POST /api/v1/auth/register
                 ├─ Success (201) → Store tokens → Redirect to Dashboard
                 └─ Error (409 duplicate email) → Show "Already registered, log in" link
```

### Dashboard (Same for Free & Paid, Scoped Down)

Free users see the same dashboard as paid users, but:
- Only their personal collection is visible
- Only their own files appear in file list
- Admin/team features (user management, tenant settings) are hidden
- Quota indicator shows usage (e.g., "3/5 documents used this month")
- When quota is hit (429 response), show an upgrade modal with pricing

---

## API Endpoints

### 1. Registration (Public)

```
POST /api/v1/auth/register
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "user-chosen-password-min-8-chars",
  "full_name": "User"
}
```

**Success Response (201):**
```json
{
  "success": true,
  "data": {
    "user": {
      "id": "uuid",
      "tenant_id": "uuid",
      "email": "user@example.com",
      "full_name": "User",
      "role": "free",
      "monthly_document_limit": 5,
      "documents_used_this_period": 0,
      "current_period_start": "2025-01-15T10:30:00Z",
      "is_active": true,
      "created_at": "...",
      "updated_at": "..."
    },
    "collection": {
      "id": "uuid",
      "tenant_id": "uuid",
      "name": "User's Invoices",
      "description": "Personal invoice collection",
      "document_count": 0,
      "created_by": "uuid",
      "created_at": "...",
      "updated_at": "..."
    },
    "tokens": {
      "access_token": "eyJ...",
      "refresh_token": "eyJ...",
      "expires_at": "2025-01-15T10:45:00Z"
    }
  }
}
```

**Error Responses:**
- `400` — Validation error (missing fields, invalid email, password < 8 chars)
- `409` — Email already exists (user already registered)

### 2. Login (Public)

For returning free users:

```
POST /api/v1/auth/login
Content-Type: application/json

{
  "tenant_slug": "satvos",
  "email": "user@example.com",
  "password": "user-chosen-password"
}
```

### 3. Token Refresh (Public)

```
POST /api/v1/auth/refresh
Content-Type: application/json

{
  "refresh_token": "eyJ..."
}
```

### 4. Upload File (Authenticated, Free Allowed)

```
POST /api/v1/files/upload
Authorization: Bearer <access_token>
Content-Type: multipart/form-data

file: <binary>
collection_id: <uuid>  (optional — use the personal collection ID from registration)
```

**Returns 201** with file metadata.

### 5. Create Document (Authenticated)

```
POST /api/v1/documents
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "file_id": "uuid",
  "collection_id": "uuid",
  "name": "Invoice #123"
}
```

**Success (201)** — Document created, parsing starts in background.

**Quota Exceeded (429):**
```json
{
  "success": false,
  "error": {
    "code": "QUOTA_EXCEEDED",
    "message": "monthly document quota exceeded; upgrade for more"
  }
}
```

### 6. List Files (Authenticated)

```
GET /api/v1/files?offset=0&limit=20
Authorization: Bearer <access_token>
```

For `free` users, the backend automatically filters to only the user's own files. No frontend logic needed.

### 7. List Collections (Authenticated)

```
GET /api/v1/collections?offset=0&limit=20
Authorization: Bearer <access_token>
```

For `free` users, only collections they have explicit access to are returned (i.e., their personal collection). No special filtering needed.

### 8. List Documents (Authenticated)

```
GET /api/v1/documents?offset=0&limit=20
Authorization: Bearer <access_token>
```

### 9. Get Document (Authenticated)

```
GET /api/v1/documents/:id
Authorization: Bearer <access_token>
```

Returns full document with `parsing_status`, `structured_data`, `validation_status`, etc.

### 10. Get Validation Results (Authenticated)

```
GET /api/v1/documents/:id/validation
Authorization: Bearer <access_token>
```

Returns per-field validation status, reconciliation summary, and rule results.

---

## Frontend Implementation Details

### Registration Flow

```typescript
// lib/auth.ts

interface RegisterResponse {
  success: boolean;
  data: {
    user: User;
    collection: Collection;
    tokens: TokenPair;
  };
}

interface RegisterInput {
  email: string;
  password: string;
  fullName?: string;
}

async function registerFreeUser({ email, password, fullName }: RegisterInput): Promise<RegisterResponse> {
  // Derive name from email prefix if not provided
  const name = fullName || email.split('@')[0] || 'User';

  const res = await fetch('/api/v1/auth/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password, full_name: name }),
  });

  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error?.message || 'Registration failed');
  }

  const data = await res.json();

  // Store tokens
  localStorage.setItem('satvos_access_token', data.data.tokens.access_token);
  localStorage.setItem('satvos_refresh_token', data.data.tokens.refresh_token);
  localStorage.setItem('satvos_token_expires', data.data.tokens.expires_at);

  // Store personal collection ID for uploads
  localStorage.setItem('satvos_personal_collection_id', data.data.collection.id);

  return data;
}
```

### Token Management

```typescript
// lib/api.ts

async function apiClient(path: string, options: RequestInit = {}): Promise<Response> {
  let token = localStorage.getItem('satvos_access_token');
  const expires = localStorage.getItem('satvos_token_expires');

  // Auto-refresh if token is expired or about to expire (< 1 min)
  if (expires && new Date(expires).getTime() - Date.now() < 60_000) {
    token = await refreshToken();
  }

  const res = await fetch(`/api/v1${path}`, {
    ...options,
    headers: {
      ...options.headers,
      'Authorization': `Bearer ${token}`,
    },
  });

  // Handle 401 — try refresh once
  if (res.status === 401) {
    token = await refreshToken();
    return fetch(`/api/v1${path}`, {
      ...options,
      headers: {
        ...options.headers,
        'Authorization': `Bearer ${token}`,
      },
    });
  }

  return res;
}

async function refreshToken(): Promise<string> {
  const refreshToken = localStorage.getItem('satvos_refresh_token');
  if (!refreshToken) throw new Error('No refresh token');

  const res = await fetch('/api/v1/auth/refresh', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refreshToken }),
  });

  if (!res.ok) {
    // Refresh failed — redirect to login page
    clearTokens();
    window.location.href = '/login';
    throw new Error('Session expired — please log in again');
  }

  const data = await res.json();
  localStorage.setItem('satvos_access_token', data.data.access_token);
  localStorage.setItem('satvos_refresh_token', data.data.refresh_token);
  localStorage.setItem('satvos_token_expires', data.data.expires_at);
  return data.data.access_token;
}

function clearTokens() {
  localStorage.removeItem('satvos_access_token');
  localStorage.removeItem('satvos_refresh_token');
  localStorage.removeItem('satvos_token_expires');
}
```

### Quota Display

The user object from registration (and from `/users/:id`) includes quota fields:

```typescript
interface User {
  id: string;
  role: 'admin' | 'manager' | 'member' | 'viewer' | 'free';
  monthly_document_limit: number;    // 5 for free, 0 = unlimited
  documents_used_this_period: number; // current usage
  current_period_start: string;       // ISO timestamp
  // ... other fields
}

function QuotaIndicator({ user }: { user: User }) {
  if (user.monthly_document_limit === 0) return null; // Paid user, no quota

  const used = user.documents_used_this_period;
  const limit = user.monthly_document_limit;
  const pct = (used / limit) * 100;

  return (
    <div className="quota-bar">
      <div className="progress" style={{ width: `${pct}%` }} />
      <span>{used}/{limit} documents this month</span>
    </div>
  );
}
```

### Handling 429 Quota Exceeded

```typescript
async function createDocument(fileId: string, collectionId: string, name?: string) {
  const res = await apiClient('/documents', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      file_id: fileId,
      collection_id: collectionId,
      name: name,
    }),
  });

  if (res.status === 429) {
    const data = await res.json();
    if (data.error?.code === 'QUOTA_EXCEEDED') {
      showUpgradeModal(); // Show upgrade modal with pricing
      return null;
    }
  }

  if (!res.ok) {
    throw new Error('Failed to create document');
  }

  return res.json();
}
```

### Upgrade Modal

```typescript
function UpgradeModal({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) {
  return (
    <Modal isOpen={isOpen} onClose={onClose}>
      <h2>Monthly Limit Reached</h2>
      <p>You've used all 5 free document parses this month.</p>

      <div className="pricing-cards">
        <PricingCard
          name="Free"
          price="$0"
          features={["5 documents/month", "1 collection", "GST validation"]}
          current
        />
        <PricingCard
          name="Pro"
          price="Contact Us"
          features={["Unlimited documents", "Unlimited collections", "Team access", "API access", "Priority support"]}
          cta="Contact Sales"
          onSelect={() => window.open('mailto:sales@satvos.com')}
        />
      </div>
    </Modal>
  );
}
```

### Role-Based UI Scoping

```typescript
// hooks/useAuth.ts
function useAuth() {
  const user = useUser(); // from your auth context

  return {
    ...user,
    isFree: user?.role === 'free',
    isPaid: user?.role !== 'free',
    isAdmin: user?.role === 'admin',
    canManageTeam: user?.role === 'admin' || user?.role === 'manager',
    canUpload: ['admin', 'manager', 'member', 'free'].includes(user?.role || ''),
    hasQuota: (user?.monthly_document_limit ?? 0) > 0,
  };
}

// In dashboard layout
function DashboardLayout({ children }: { children: React.ReactNode }) {
  const { isFree, canManageTeam } = useAuth();

  return (
    <div className="dashboard">
      <Sidebar>
        <NavItem href="/dashboard" label="Dashboard" />
        <NavItem href="/collections" label="Collections" />
        <NavItem href="/documents" label="Documents" />
        <NavItem href="/files" label="Files" />
        {!isFree && <NavItem href="/stats" label="Statistics" />}
        {canManageTeam && <NavItem href="/users" label="Team" />}
        {canManageTeam && <NavItem href="/settings" label="Settings" />}
      </Sidebar>
      <main>{children}</main>
    </div>
  );
}
```

### Upload + Parse Flow (Combined)

For free users, the simplest UX is: upload file → auto-create document → show results.

```typescript
async function uploadAndParse(file: File) {
  const personalCollectionId = localStorage.getItem('satvos_personal_collection_id');

  // Step 1: Upload file
  const formData = new FormData();
  formData.append('file', file);
  if (personalCollectionId) {
    formData.append('collection_id', personalCollectionId);
  }

  const uploadRes = await apiClient('/files/upload', {
    method: 'POST',
    body: formData,
  });

  if (!uploadRes.ok) throw new Error('Upload failed');
  const { data: uploadData } = await uploadRes.json();
  const fileId = uploadData.file?.id || uploadData.id;

  // Step 2: Create document (triggers parsing + validation)
  const docRes = await apiClient('/documents', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      file_id: fileId,
      collection_id: personalCollectionId,
      name: file.name,
    }),
  });

  if (docRes.status === 429) {
    showUpgradeModal();
    return null;
  }

  if (!docRes.ok) throw new Error('Document creation failed');
  const { data: doc } = await docRes.json();

  // Step 3: Poll for parsing completion
  return await pollDocumentStatus(doc.id);
}

async function pollDocumentStatus(docId: string, maxAttempts = 60): Promise<Document> {
  for (let i = 0; i < maxAttempts; i++) {
    await new Promise(resolve => setTimeout(resolve, 2000)); // 2s interval

    const res = await apiClient(`/documents/${docId}`);
    if (!res.ok) throw new Error('Failed to fetch document');

    const { data: doc } = await res.json();

    if (doc.parsing_status === 'completed') return doc;
    if (doc.parsing_status === 'failed') throw new Error('Parsing failed');
    // 'pending', 'processing', 'queued' → keep polling
  }

  throw new Error('Parsing timed out');
}
```

---

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| UX friction | Email + password registration | Standard sign-up form. User picks their own password, can log in from any device. |
| Password handling | User-provided (min 8 chars) | User controls their credentials. Standard login flow on return visits. Future: add "forgot password" email reset. |
| Dashboard approach | Same dashboard, scoped down | Less code to maintain. Free users see the same UI but fewer features/data. |
| Quota UX | Upgrade modal with pricing | Clear call-to-action. Shows exact usage and what they'd get by upgrading. |
| Framework | Next.js | User's choice for the frontend. |

## Gotchas

1. **No "forgot password" yet**: If a user forgets their password, there's no reset flow. Plan for email-based password reset in v2.
2. **Refresh token expiry**: Refresh tokens last 7 days. If a free user doesn't visit for 7+ days, they'll need to log in again with their email + password.
3. **Access token is 15 minutes**: Implement auto-refresh before API calls.
4. **Quota is checked on document creation, not file upload**: Users can upload files freely. The 429 only happens on `POST /documents`. This is intentional — uploaded files without documents are just stored files with no LLM cost.
5. **Collection ID from registration**: The personal collection ID is returned once at registration. Store it — the user will need it for every upload.
6. **Polling for parse status**: Parsing happens in the background. The `POST /documents` returns immediately with `parsing_status: "pending"`. Poll `GET /documents/:id` until `completed` or `failed`.
7. **Free users cannot create collections**: The `POST /collections` route requires admin/manager/member role. Free users only have their auto-created personal collection.
8. **The `tenant_slug` for login is always `"satvos"`** for free-tier users. Hardcode this in the login form or store it after registration.
9. **`full_name` is required by the backend**: If your registration form doesn't include a name field, derive it from the email prefix before sending the request.
