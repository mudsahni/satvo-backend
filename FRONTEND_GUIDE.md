# Frontend Development Guide

A comprehensive guide for building a frontend application for the SATVOS document processing service.

## Table of Contents

- [Overview](#overview)
- [Tech Stack Recommendations](#tech-stack-recommendations)
- [Authentication](#authentication)
- [Core Features](#core-features)
- [UI Components](#ui-components)
- [State Management](#state-management)
- [API Integration](#api-integration)
- [Real-time Updates](#real-time-updates)
- [Error Handling](#error-handling)
- [File Handling](#file-handling)
- [Validation Display](#validation-display)
- [Role-Based Access](#role-based-access)
- [Recommended Libraries](#recommended-libraries)
- [Page Structure](#page-structure)
- [Design Considerations](#design-considerations)

---

## Overview

SATVOS is a multi-tenant document processing service with these core capabilities:

1. **File Management**: Upload, organize, and manage PDF/image files
2. **Collections**: Group files with permission-based access control
3. **AI Document Parsing**: Extract structured data from invoices using LLM
4. **Validation**: Automated GST compliance validation with 50+ rules
5. **Review Workflow**: Human approval/rejection of parsed documents
6. **Tagging**: Searchable user and auto-generated tags

The frontend should provide an intuitive interface for all these workflows.

---

## Tech Stack Recommendations

### Primary Stack

| Component | Recommendation | Alternatives |
|-----------|---------------|--------------|
| Framework | Next.js 14+ (App Router) | React + Vite, Remix |
| Language | TypeScript | - |
| Styling | Tailwind CSS | styled-components, CSS Modules |
| UI Components | shadcn/ui | Radix UI, Headless UI, Ant Design |
| State | Zustand or Jotai | Redux Toolkit, React Query |
| Data Fetching | TanStack Query (React Query) | SWR |
| Forms | React Hook Form + Zod | Formik + Yup |
| Tables | TanStack Table | AG Grid, react-table |
| File Upload | react-dropzone | Uppy |
| PDF Viewer | react-pdf | PDF.js |
| Charts | Recharts | Chart.js, Visx |
| Date Handling | date-fns | Day.js, Luxon |
| Icons | Lucide React | Heroicons, Phosphor |

### Development Tools

- ESLint + Prettier for code quality
- Husky for git hooks
- Storybook for component development
- Playwright or Cypress for E2E testing
- MSW for API mocking

---

## Authentication

### Token Management

```typescript
// lib/auth.ts
import { jwtDecode } from "jwt-decode";

interface TokenPayload {
  tenant_id: string;
  user_id: string;
  email: string;
  role: "admin" | "manager" | "member" | "viewer";
  exp: number;
}

class AuthManager {
  private accessToken: string | null = null;
  private refreshToken: string | null = null;

  setTokens(access: string, refresh: string) {
    this.accessToken = access;
    this.refreshToken = refresh;
    // Store refresh token securely (httpOnly cookie preferred)
    localStorage.setItem("refreshToken", refresh);
  }

  getAccessToken(): string | null {
    return this.accessToken;
  }

  isTokenExpired(): boolean {
    if (!this.accessToken) return true;
    const payload = jwtDecode<TokenPayload>(this.accessToken);
    // Add 30s buffer for network latency
    return payload.exp * 1000 < Date.now() + 30000;
  }

  async refreshAccessToken(): Promise<string> {
    const response = await fetch("/api/v1/auth/refresh", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: this.refreshToken }),
    });

    if (!response.ok) {
      this.logout();
      throw new Error("Session expired");
    }

    const { data } = await response.json();
    this.setTokens(data.access_token, data.refresh_token);
    return data.access_token;
  }

  getUserInfo(): TokenPayload | null {
    if (!this.accessToken) return null;
    return jwtDecode<TokenPayload>(this.accessToken);
  }

  logout() {
    this.accessToken = null;
    this.refreshToken = null;
    localStorage.removeItem("refreshToken");
    window.location.href = "/login";
  }
}

export const auth = new AuthManager();
```

### Login Flow

```typescript
// components/LoginForm.tsx
interface LoginFormData {
  tenantSlug: string;
  email: string;
  password: string;
}

async function handleLogin(data: LoginFormData) {
  const response = await fetch("/api/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      tenant_slug: data.tenantSlug,
      email: data.email,
      password: data.password,
    }),
  });

  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.error?.message || "Login failed");
  }

  const { data: tokens } = await response.json();
  auth.setTokens(tokens.access_token, tokens.refresh_token);

  // Redirect to dashboard
  router.push("/dashboard");
}
```

### Protected Routes

```typescript
// middleware.ts (Next.js)
import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

export function middleware(request: NextRequest) {
  const token = request.cookies.get("accessToken");

  if (!token && !request.nextUrl.pathname.startsWith("/login")) {
    return NextResponse.redirect(new URL("/login", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!api|_next/static|_next/image|favicon.ico).*)"],
};
```

---

## Core Features

### 1. Dashboard

Display key metrics and recent activity:

```typescript
interface DashboardData {
  stats: {
    totalDocuments: number;
    pendingReview: number;
    validationIssues: number;
    recentUploads: number;
  };
  recentDocuments: Document[];
  validationSummary: {
    valid: number;
    warning: number;
    invalid: number;
  };
}
```

**Components needed**:
- Stats cards (total docs, pending review, validation issues)
- Recent documents list
- Validation status pie chart
- Quick upload button
- Collection overview

### 2. File Management

**Upload Component**:
```typescript
interface UploadConfig {
  acceptedTypes: ["application/pdf", "image/jpeg", "image/png"];
  maxSize: 50 * 1024 * 1024; // 50MB
  multiple: true;
  collectionId?: string;
}
```

**Features**:
- Drag-and-drop upload zone
- Upload progress indicators
- Batch upload support
- File type validation (client-side)
- File size validation
- Thumbnail previews for images

### 3. Collections

**Collection List View**:
- Grid/list toggle
- Sort by name, date, file count
- Search/filter
- Create collection modal
- Collection cards with file counts

**Collection Detail View**:
- File list with pagination
- Drag-and-drop to add files
- Batch file upload
- Permission management (for owners)
- Edit collection metadata

### 4. Document Processing

**Document Creation Flow**:
```
1. Select file(s) from uploads or new upload
2. Choose/create collection
3. Set document type (invoice)
4. Optionally add tags
5. Choose parse mode (single/dual)
6. Submit for processing
```

**Processing Status Display**:
```typescript
const statusConfig = {
  pending: { color: "gray", icon: "clock", label: "Queued" },
  processing: { color: "blue", icon: "spinner", label: "Parsing..." },
  completed: { color: "green", icon: "check", label: "Complete" },
  failed: { color: "red", icon: "x", label: "Failed" },
};
```

### 5. Document Viewer

**Layout**:
```
+------------------+-------------------+
|                  |                   |
|   PDF/Image      |   Extracted Data  |
|   Viewer         |   (structured)    |
|                  |                   |
+------------------+-------------------+
|                                      |
|        Validation Results            |
|                                      |
+--------------------------------------+
```

**Features**:
- Side-by-side PDF viewer and extracted data
- Field highlighting with confidence scores
- Validation result display
- Review actions (approve/reject)
- Tag management
- Navigation to specific fields

### 6. Validation Review

**Validation Display**:
```typescript
interface ValidationUI {
  // Summary bar
  summary: {
    total: number;
    passed: number;
    errors: number;
    warnings: number;
  };

  // Filter options
  filters: {
    status: "all" | "passed" | "failed";
    severity: "all" | "error" | "warning";
    category: string; // required, format, math, etc.
  };

  // Grouped results
  groupedResults: Map<string, ValidationResultEntry[]>;
}
```

**Features**:
- Summary statistics
- Filter by severity/category
- Expandable rule details
- Field highlighting on hover
- Reconciliation status indicator
- Critical vs non-critical distinction

### 7. User Management (Admin)

**User List**:
- Table with sorting/filtering
- Role badges
- Active/inactive status
- Bulk actions

**User Form**:
- Create/edit user modal
- Role selection dropdown
- Email validation
- Password requirements display

---

## UI Components

### Essential Components

```typescript
// Component library structure
components/
├── ui/                    # Base UI components (shadcn/ui)
│   ├── button.tsx
│   ├── input.tsx
│   ├── select.tsx
│   ├── dialog.tsx
│   ├── dropdown-menu.tsx
│   ├── table.tsx
│   ├── badge.tsx
│   ├── card.tsx
│   ├── tabs.tsx
│   └── toast.tsx
├── layout/               # Layout components
│   ├── sidebar.tsx
│   ├── header.tsx
│   ├── page-header.tsx
│   └── breadcrumbs.tsx
├── auth/                 # Auth components
│   ├── login-form.tsx
│   ├── protected-route.tsx
│   └── role-guard.tsx
├── files/                # File components
│   ├── file-uploader.tsx
│   ├── file-list.tsx
│   ├── file-card.tsx
│   ├── file-preview.tsx
│   └── upload-progress.tsx
├── collections/          # Collection components
│   ├── collection-list.tsx
│   ├── collection-card.tsx
│   ├── collection-form.tsx
│   └── permission-manager.tsx
├── documents/            # Document components
│   ├── document-list.tsx
│   ├── document-card.tsx
│   ├── document-viewer.tsx
│   ├── parsing-status.tsx
│   ├── structured-data-view.tsx
│   └── review-actions.tsx
├── validation/           # Validation components
│   ├── validation-summary.tsx
│   ├── validation-results.tsx
│   ├── field-status-badge.tsx
│   ├── rule-detail.tsx
│   └── reconciliation-badge.tsx
├── tags/                 # Tag components
│   ├── tag-input.tsx
│   ├── tag-list.tsx
│   └── tag-search.tsx
└── common/               # Shared components
    ├── data-table.tsx
    ├── pagination.tsx
    ├── search-input.tsx
    ├── loading-spinner.tsx
    ├── empty-state.tsx
    └── error-boundary.tsx
```

### Status Badges

```typescript
// components/common/status-badge.tsx
const parsingStatusStyles = {
  pending: "bg-gray-100 text-gray-700",
  processing: "bg-blue-100 text-blue-700",
  completed: "bg-green-100 text-green-700",
  failed: "bg-red-100 text-red-700",
};

const validationStatusStyles = {
  pending: "bg-gray-100 text-gray-700",
  valid: "bg-green-100 text-green-700",
  warning: "bg-yellow-100 text-yellow-700",
  invalid: "bg-red-100 text-red-700",
};

const reviewStatusStyles = {
  pending: "bg-gray-100 text-gray-700",
  approved: "bg-green-100 text-green-700",
  rejected: "bg-red-100 text-red-700",
};
```

### Field Status Indicators

```typescript
// components/validation/field-status.tsx
const fieldStatusConfig = {
  valid: {
    icon: CheckCircle,
    color: "text-green-500",
    bg: "bg-green-50",
  },
  invalid: {
    icon: XCircle,
    color: "text-red-500",
    bg: "bg-red-50",
  },
  unsure: {
    icon: AlertCircle,
    color: "text-yellow-500",
    bg: "bg-yellow-50",
  },
};
```

---

## State Management

### Recommended Store Structure

```typescript
// stores/auth-store.ts
interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (credentials: LoginCredentials) => Promise<void>;
  logout: () => void;
  refreshToken: () => Promise<void>;
}

// stores/documents-store.ts
interface DocumentsState {
  documents: Document[];
  selectedDocument: Document | null;
  filters: DocumentFilters;
  isLoading: boolean;
  fetchDocuments: () => Promise<void>;
  selectDocument: (id: string) => void;
  setFilters: (filters: Partial<DocumentFilters>) => void;
}

// stores/collections-store.ts
interface CollectionsState {
  collections: Collection[];
  selectedCollection: Collection | null;
  permissions: Map<string, CollectionPermission>;
  fetchCollections: () => Promise<void>;
  createCollection: (data: CreateCollectionInput) => Promise<Collection>;
  deleteCollection: (id: string) => Promise<void>;
}

// stores/upload-store.ts
interface UploadState {
  uploads: Map<string, UploadProgress>;
  addUpload: (file: File) => string;
  updateProgress: (id: string, progress: number) => void;
  completeUpload: (id: string, file: FileMeta) => void;
  failUpload: (id: string, error: string) => void;
  removeUpload: (id: string) => void;
}
```

### React Query Setup

```typescript
// lib/query-client.ts
import { QueryClient } from "@tanstack/react-query";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30 * 1000, // 30 seconds
      retry: (failureCount, error) => {
        if (error instanceof UnauthorizedError) return false;
        return failureCount < 3;
      },
    },
  },
});

// hooks/use-documents.ts
export function useDocuments(filters?: DocumentFilters) {
  return useQuery({
    queryKey: ["documents", filters],
    queryFn: () => api.documents.list(filters),
  });
}

export function useDocument(id: string) {
  return useQuery({
    queryKey: ["documents", id],
    queryFn: () => api.documents.get(id),
    refetchInterval: (data) =>
      data?.parsing_status === "processing" ? 2000 : false,
  });
}

export function useCreateDocument() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: api.documents.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["documents"] });
    },
  });
}
```

---

## API Integration

### API Client Setup

```typescript
// lib/api-client.ts
class APIClient {
  private baseUrl: string;

  constructor(baseUrl: string = "/api/v1") {
    this.baseUrl = baseUrl;
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const token = auth.getAccessToken();

    // Refresh token if expired
    if (token && auth.isTokenExpired()) {
      await auth.refreshAccessToken();
    }

    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      ...options,
      headers: {
        "Content-Type": "application/json",
        ...(token && { Authorization: `Bearer ${token}` }),
        ...options.headers,
      },
    });

    const data = await response.json();

    if (!response.ok) {
      throw new APIError(data.error?.code, data.error?.message, response.status);
    }

    return data;
  }

  // GET request
  get<T>(endpoint: string, params?: Record<string, any>): Promise<T> {
    const url = params
      ? `${endpoint}?${new URLSearchParams(params)}`
      : endpoint;
    return this.request<T>(url);
  }

  // POST request
  post<T>(endpoint: string, body?: any): Promise<T> {
    return this.request<T>(endpoint, {
      method: "POST",
      body: body ? JSON.stringify(body) : undefined,
    });
  }

  // PUT request
  put<T>(endpoint: string, body?: any): Promise<T> {
    return this.request<T>(endpoint, {
      method: "PUT",
      body: body ? JSON.stringify(body) : undefined,
    });
  }

  // DELETE request
  delete<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, { method: "DELETE" });
  }

  // File upload
  async upload<T>(endpoint: string, formData: FormData): Promise<T> {
    const token = auth.getAccessToken();

    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      method: "POST",
      headers: {
        ...(token && { Authorization: `Bearer ${token}` }),
        // Don't set Content-Type for FormData
      },
      body: formData,
    });

    const data = await response.json();

    if (!response.ok) {
      throw new APIError(data.error?.code, data.error?.message, response.status);
    }

    return data;
  }
}

export const apiClient = new APIClient();
```

### API Modules

```typescript
// lib/api/documents.ts
export const documentsApi = {
  list: (params?: { offset?: number; limit?: number; collection_id?: string }) =>
    apiClient.get<APIResponse<Document[]>>("/documents", params),

  get: (id: string) =>
    apiClient.get<APIResponse<Document>>(`/documents/${id}`),

  create: (data: CreateDocumentRequest) =>
    apiClient.post<APIResponse<Document>>("/documents", data),

  retry: (id: string) =>
    apiClient.post<APIResponse<Document>>(`/documents/${id}/retry`),

  review: (id: string, data: ReviewDocumentRequest) =>
    apiClient.put<APIResponse<Document>>(`/documents/${id}/review`, data),

  validate: (id: string) =>
    apiClient.post<APIResponse<{ message: string }>>(`/documents/${id}/validate`),

  getValidation: (id: string) =>
    apiClient.get<APIResponse<ValidationResponse>>(`/documents/${id}/validation`),

  listTags: (id: string) =>
    apiClient.get<APIResponse<DocumentTag[]>>(`/documents/${id}/tags`),

  addTags: (id: string, tags: Record<string, string>) =>
    apiClient.post<APIResponse<DocumentTag[]>>(`/documents/${id}/tags`, { tags }),

  deleteTag: (docId: string, tagId: string) =>
    apiClient.delete<APIResponse<{ message: string }>>(`/documents/${docId}/tags/${tagId}`),

  searchByTag: (key: string, value: string, params?: { offset?: number; limit?: number }) =>
    apiClient.get<APIResponse<Document[]>>("/documents/search/tags", { key, value, ...params }),

  delete: (id: string) =>
    apiClient.delete<APIResponse<{ message: string }>>(`/documents/${id}`),
};

// lib/api/files.ts
export const filesApi = {
  list: (params?: { offset?: number; limit?: number }) =>
    apiClient.get<APIResponse<FileMeta[]>>("/files", params),

  get: (id: string) =>
    apiClient.get<APIResponse<FileMeta>>(`/files/${id}`),

  upload: async (file: File, collectionId?: string, onProgress?: (progress: number) => void) => {
    const formData = new FormData();
    formData.append("file", file);
    if (collectionId) {
      formData.append("collection_id", collectionId);
    }
    return apiClient.upload<APIResponse<FileMeta>>("/files/upload", formData);
  },

  delete: (id: string) =>
    apiClient.delete<APIResponse<{ message: string }>>(`/files/${id}`),
};

// lib/api/collections.ts
export const collectionsApi = {
  list: (params?: { offset?: number; limit?: number }) =>
    apiClient.get<APIResponse<Collection[]>>("/collections", params),

  get: (id: string, params?: { offset?: number; limit?: number }) =>
    apiClient.get<APIResponse<CollectionWithFiles>>(`/collections/${id}`, params),

  create: (data: CreateCollectionRequest) =>
    apiClient.post<APIResponse<Collection>>("/collections", data),

  update: (id: string, data: UpdateCollectionRequest) =>
    apiClient.put<APIResponse<Collection>>(`/collections/${id}`, data),

  delete: (id: string) =>
    apiClient.delete<APIResponse<{ message: string }>>(`/collections/${id}`),

  uploadFiles: async (id: string, files: File[]) => {
    const formData = new FormData();
    files.forEach((file) => formData.append("files", file));
    return apiClient.upload<APIResponse<BatchUploadResult[]>>(`/collections/${id}/files`, formData);
  },

  removeFile: (collectionId: string, fileId: string) =>
    apiClient.delete<APIResponse<{ message: string }>>(`/collections/${collectionId}/files/${fileId}`),

  listPermissions: (id: string, params?: { offset?: number; limit?: number }) =>
    apiClient.get<APIResponse<CollectionPermissionEntry[]>>(`/collections/${id}/permissions`, params),

  setPermission: (id: string, userId: string, permission: CollectionPermission) =>
    apiClient.post<APIResponse<{ message: string }>>(`/collections/${id}/permissions`, {
      user_id: userId,
      permission,
    }),

  removePermission: (collectionId: string, userId: string) =>
    apiClient.delete<APIResponse<{ message: string }>>(`/collections/${collectionId}/permissions/${userId}`),
};
```

---

## Real-time Updates

### Document Parsing Polling

```typescript
// hooks/use-document-polling.ts
export function useDocumentPolling(documentId: string) {
  const [document, setDocument] = useState<Document | null>(null);
  const [isPolling, setIsPolling] = useState(false);

  useEffect(() => {
    let intervalId: NodeJS.Timeout;

    const poll = async () => {
      try {
        const response = await documentsApi.get(documentId);
        setDocument(response.data);

        const status = response.data.parsing_status;
        if (status === "completed" || status === "failed") {
          setIsPolling(false);
          clearInterval(intervalId);
        }
      } catch (error) {
        console.error("Polling error:", error);
      }
    };

    if (documentId) {
      poll(); // Initial fetch
      setIsPolling(true);
      intervalId = setInterval(poll, 2000);
    }

    return () => {
      if (intervalId) clearInterval(intervalId);
    };
  }, [documentId]);

  return { document, isPolling };
}
```

### Upload Progress Tracking

```typescript
// hooks/use-upload.ts
export function useUpload() {
  const [uploads, setUploads] = useState<Map<string, UploadProgress>>(new Map());

  const upload = async (file: File, collectionId?: string) => {
    const uploadId = crypto.randomUUID();

    setUploads((prev) =>
      new Map(prev).set(uploadId, {
        file,
        progress: 0,
        status: "uploading",
      })
    );

    try {
      // Use XMLHttpRequest for progress tracking
      const formData = new FormData();
      formData.append("file", file);
      if (collectionId) formData.append("collection_id", collectionId);

      const result = await new Promise<FileMeta>((resolve, reject) => {
        const xhr = new XMLHttpRequest();

        xhr.upload.onprogress = (event) => {
          if (event.lengthComputable) {
            const progress = (event.loaded / event.total) * 100;
            setUploads((prev) =>
              new Map(prev).set(uploadId, {
                ...prev.get(uploadId)!,
                progress,
              })
            );
          }
        };

        xhr.onload = () => {
          if (xhr.status >= 200 && xhr.status < 300) {
            const response = JSON.parse(xhr.responseText);
            resolve(response.data);
          } else {
            reject(new Error("Upload failed"));
          }
        };

        xhr.onerror = () => reject(new Error("Network error"));

        xhr.open("POST", "/api/v1/files/upload");
        xhr.setRequestHeader("Authorization", `Bearer ${auth.getAccessToken()}`);
        xhr.send(formData);
      });

      setUploads((prev) =>
        new Map(prev).set(uploadId, {
          ...prev.get(uploadId)!,
          status: "completed",
          result,
        })
      );

      return result;
    } catch (error) {
      setUploads((prev) =>
        new Map(prev).set(uploadId, {
          ...prev.get(uploadId)!,
          status: "failed",
          error: error instanceof Error ? error.message : "Upload failed",
        })
      );
      throw error;
    }
  };

  return { uploads, upload };
}
```

---

## Error Handling

### Error Types

```typescript
// lib/errors.ts
export class APIError extends Error {
  constructor(
    public code: string,
    message: string,
    public status: number
  ) {
    super(message);
    this.name = "APIError";
  }
}

export class UnauthorizedError extends APIError {
  constructor(message = "Unauthorized") {
    super("UNAUTHORIZED", message, 401);
  }
}

export class ForbiddenError extends APIError {
  constructor(message = "Forbidden") {
    super("FORBIDDEN", message, 403);
  }
}

export class NotFoundError extends APIError {
  constructor(message = "Not found") {
    super("NOT_FOUND", message, 404);
  }
}
```

### Error Display

```typescript
// components/common/error-display.tsx
const errorMessages: Record<string, string> = {
  UNAUTHORIZED: "Please log in to continue",
  FORBIDDEN: "You don't have permission to perform this action",
  INSUFFICIENT_ROLE: "Your role doesn't allow this action",
  NOT_FOUND: "The requested resource was not found",
  COLLECTION_NOT_FOUND: "Collection not found",
  DOCUMENT_NOT_FOUND: "Document not found",
  DOCUMENT_NOT_PARSED: "Document hasn't finished parsing yet",
  DOCUMENT_ALREADY_EXISTS: "A document already exists for this file",
  UNSUPPORTED_FILE_TYPE: "File type not supported. Use PDF, JPG, or PNG",
  FILE_TOO_LARGE: "File exceeds the 50MB size limit",
  DUPLICATE_EMAIL: "This email is already registered",
  INVALID_REQUEST: "Invalid request. Please check your input",
  INTERNAL_ERROR: "Something went wrong. Please try again later",
};

export function getErrorMessage(code: string, fallback?: string): string {
  return errorMessages[code] || fallback || "An error occurred";
}
```

### Global Error Boundary

```typescript
// components/common/error-boundary.tsx
export class ErrorBoundary extends Component<Props, State> {
  state = { hasError: false, error: null };

  static getDerivedStateFromError(error: Error) {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error("Error boundary caught:", error, errorInfo);
    // Report to error tracking service
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="flex flex-col items-center justify-center min-h-screen">
          <h1 className="text-2xl font-bold">Something went wrong</h1>
          <p className="text-gray-600 mt-2">Please refresh the page</p>
          <button
            onClick={() => window.location.reload()}
            className="mt-4 px-4 py-2 bg-blue-600 text-white rounded"
          >
            Refresh
          </button>
        </div>
      );
    }

    return this.props.children;
  }
}
```

---

## File Handling

### File Type Validation

```typescript
// lib/file-utils.ts
export const ALLOWED_TYPES = {
  "application/pdf": "pdf",
  "image/jpeg": "jpg",
  "image/png": "png",
} as const;

export const MAX_FILE_SIZE = 50 * 1024 * 1024; // 50MB

export function validateFile(file: File): { valid: boolean; error?: string } {
  if (!Object.keys(ALLOWED_TYPES).includes(file.type)) {
    return {
      valid: false,
      error: "File type not supported. Please upload PDF, JPG, or PNG files.",
    };
  }

  if (file.size > MAX_FILE_SIZE) {
    return {
      valid: false,
      error: `File too large. Maximum size is ${MAX_FILE_SIZE / 1024 / 1024}MB.`,
    };
  }

  return { valid: true };
}

export function formatFileSize(bytes: number): string {
  if (bytes === 0) return "0 Bytes";
  const k = 1024;
  const sizes = ["Bytes", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
}
```

### File Uploader Component

```typescript
// components/files/file-uploader.tsx
import { useDropzone } from "react-dropzone";

interface FileUploaderProps {
  onUpload: (files: File[]) => void;
  collectionId?: string;
  multiple?: boolean;
}

export function FileUploader({ onUpload, collectionId, multiple = true }: FileUploaderProps) {
  const { getRootProps, getInputProps, isDragActive, fileRejections } = useDropzone({
    accept: {
      "application/pdf": [".pdf"],
      "image/jpeg": [".jpg", ".jpeg"],
      "image/png": [".png"],
    },
    maxSize: MAX_FILE_SIZE,
    multiple,
    onDrop: (acceptedFiles) => {
      if (acceptedFiles.length > 0) {
        onUpload(acceptedFiles);
      }
    },
  });

  return (
    <div
      {...getRootProps()}
      className={cn(
        "border-2 border-dashed rounded-lg p-8 text-center cursor-pointer transition-colors",
        isDragActive ? "border-blue-500 bg-blue-50" : "border-gray-300 hover:border-gray-400"
      )}
    >
      <input {...getInputProps()} />
      <UploadCloud className="mx-auto h-12 w-12 text-gray-400" />
      <p className="mt-2 text-sm text-gray-600">
        {isDragActive
          ? "Drop files here..."
          : "Drag and drop files here, or click to select"}
      </p>
      <p className="mt-1 text-xs text-gray-500">
        PDF, JPG, PNG up to 50MB
      </p>
      {fileRejections.length > 0 && (
        <div className="mt-4 text-sm text-red-600">
          {fileRejections.map(({ file, errors }) => (
            <p key={file.name}>
              {file.name}: {errors.map((e) => e.message).join(", ")}
            </p>
          ))}
        </div>
      )}
    </div>
  );
}
```

### PDF Viewer

```typescript
// components/documents/pdf-viewer.tsx
import { Document, Page, pdfjs } from "react-pdf";

pdfjs.GlobalWorkerOptions.workerSrc = `//cdnjs.cloudflare.com/ajax/libs/pdf.js/${pdfjs.version}/pdf.worker.min.js`;

interface PDFViewerProps {
  url: string;
  highlightFields?: string[];
}

export function PDFViewer({ url, highlightFields }: PDFViewerProps) {
  const [numPages, setNumPages] = useState<number>(0);
  const [pageNumber, setPageNumber] = useState(1);
  const [scale, setScale] = useState(1.0);

  return (
    <div className="pdf-viewer">
      <div className="toolbar flex items-center gap-2 p-2 border-b">
        <Button onClick={() => setPageNumber((p) => Math.max(1, p - 1))}>
          Previous
        </Button>
        <span>
          Page {pageNumber} of {numPages}
        </span>
        <Button onClick={() => setPageNumber((p) => Math.min(numPages, p + 1))}>
          Next
        </Button>
        <div className="flex-1" />
        <Button onClick={() => setScale((s) => s - 0.1)}>-</Button>
        <span>{Math.round(scale * 100)}%</span>
        <Button onClick={() => setScale((s) => s + 0.1)}>+</Button>
      </div>

      <div className="viewer-content overflow-auto p-4">
        <Document
          file={url}
          onLoadSuccess={({ numPages }) => setNumPages(numPages)}
        >
          <Page pageNumber={pageNumber} scale={scale} />
        </Document>
      </div>
    </div>
  );
}
```

---

## Validation Display

### Validation Results Component

```typescript
// components/validation/validation-results.tsx
interface ValidationResultsProps {
  results: ValidationResultEntry[];
  fieldStatuses: Record<string, FieldStatus>;
}

export function ValidationResults({ results, fieldStatuses }: ValidationResultsProps) {
  const [filter, setFilter] = useState<"all" | "failed" | "passed">("all");
  const [severityFilter, setSeverityFilter] = useState<"all" | "error" | "warning">("all");

  const filteredResults = useMemo(() => {
    return results.filter((r) => {
      if (filter === "failed" && r.passed) return false;
      if (filter === "passed" && !r.passed) return false;
      if (severityFilter !== "all" && r.severity !== severityFilter) return false;
      return true;
    });
  }, [results, filter, severityFilter]);

  const groupedResults = useMemo(() => {
    return filteredResults.reduce((acc, result) => {
      const category = result.rule_type;
      if (!acc[category]) acc[category] = [];
      acc[category].push(result);
      return acc;
    }, {} as Record<string, ValidationResultEntry[]>);
  }, [filteredResults]);

  return (
    <div className="space-y-4">
      {/* Filters */}
      <div className="flex gap-4">
        <Select value={filter} onValueChange={setFilter}>
          <SelectItem value="all">All Results</SelectItem>
          <SelectItem value="failed">Failed Only</SelectItem>
          <SelectItem value="passed">Passed Only</SelectItem>
        </Select>
        <Select value={severityFilter} onValueChange={setSeverityFilter}>
          <SelectItem value="all">All Severities</SelectItem>
          <SelectItem value="error">Errors</SelectItem>
          <SelectItem value="warning">Warnings</SelectItem>
        </Select>
      </div>

      {/* Grouped Results */}
      {Object.entries(groupedResults).map(([category, categoryResults]) => (
        <div key={category} className="border rounded-lg">
          <div className="p-3 bg-gray-50 font-medium">
            {formatCategory(category)} ({categoryResults.length})
          </div>
          <div className="divide-y">
            {categoryResults.map((result, idx) => (
              <ValidationResultRow key={idx} result={result} />
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

function ValidationResultRow({ result }: { result: ValidationResultEntry }) {
  return (
    <div className={cn("p-3", result.passed ? "bg-white" : "bg-red-50")}>
      <div className="flex items-center gap-2">
        {result.passed ? (
          <CheckCircle className="h-4 w-4 text-green-500" />
        ) : (
          <XCircle className="h-4 w-4 text-red-500" />
        )}
        <span className="font-medium">{result.rule_name}</span>
        {result.reconciliation_critical && (
          <Badge variant="outline" className="text-xs">
            Reconciliation Critical
          </Badge>
        )}
        <Badge variant={result.severity === "error" ? "destructive" : "warning"}>
          {result.severity}
        </Badge>
      </div>
      <div className="mt-1 text-sm text-gray-600 ml-6">
        {result.message}
      </div>
      <div className="mt-1 text-xs text-gray-500 ml-6">
        Field: <code>{result.field_path}</code>
        {result.actual_value && (
          <> | Actual: <code>{result.actual_value}</code></>
        )}
      </div>
    </div>
  );
}
```

### Structured Data Viewer

```typescript
// components/documents/structured-data-view.tsx
interface StructuredDataViewProps {
  data: GSTInvoice;
  confidenceScores: ConfidenceScores;
  fieldStatuses: Record<string, FieldStatus>;
  onFieldClick?: (fieldPath: string) => void;
}

export function StructuredDataView({
  data,
  confidenceScores,
  fieldStatuses,
  onFieldClick,
}: StructuredDataViewProps) {
  const renderField = (
    label: string,
    value: any,
    fieldPath: string,
    confidence?: number
  ) => {
    const status = fieldStatuses[fieldPath];

    return (
      <div
        key={fieldPath}
        className={cn(
          "flex justify-between items-center py-2 px-3 rounded cursor-pointer hover:bg-gray-50",
          status?.status === "invalid" && "bg-red-50",
          status?.status === "unsure" && "bg-yellow-50"
        )}
        onClick={() => onFieldClick?.(fieldPath)}
      >
        <span className="text-gray-600">{label}</span>
        <div className="flex items-center gap-2">
          <span className="font-medium">{value?.toString() || "-"}</span>
          {confidence !== undefined && (
            <ConfidenceBadge confidence={confidence} />
          )}
          {status && <FieldStatusIcon status={status.status} />}
        </div>
      </div>
    );
  };

  return (
    <div className="space-y-6">
      {/* Invoice Header */}
      <Section title="Invoice Details">
        {renderField("Invoice Number", data.invoice.invoice_number, "invoice.invoice_number", confidenceScores.invoice.invoice_number)}
        {renderField("Invoice Date", data.invoice.invoice_date, "invoice.invoice_date", confidenceScores.invoice.invoice_date)}
        {renderField("Due Date", data.invoice.due_date, "invoice.due_date", confidenceScores.invoice.due_date)}
        {renderField("Currency", data.invoice.currency, "invoice.currency", confidenceScores.invoice.currency)}
        {renderField("Place of Supply", data.invoice.place_of_supply, "invoice.place_of_supply", confidenceScores.invoice.place_of_supply)}
      </Section>

      {/* Seller */}
      <Section title="Seller">
        {renderField("Name", data.seller.name, "seller.name", confidenceScores.seller.name)}
        {renderField("GSTIN", data.seller.gstin, "seller.gstin", confidenceScores.seller.gstin)}
        {renderField("PAN", data.seller.pan, "seller.pan", confidenceScores.seller.pan)}
        {renderField("State", data.seller.state, "seller.state", confidenceScores.seller.state)}
        {renderField("State Code", data.seller.state_code, "seller.state_code", confidenceScores.seller.state_code)}
      </Section>

      {/* Buyer */}
      <Section title="Buyer">
        {renderField("Name", data.buyer.name, "buyer.name", confidenceScores.buyer.name)}
        {renderField("GSTIN", data.buyer.gstin, "buyer.gstin", confidenceScores.buyer.gstin)}
        {renderField("PAN", data.buyer.pan, "buyer.pan", confidenceScores.buyer.pan)}
        {renderField("State", data.buyer.state, "buyer.state", confidenceScores.buyer.state)}
        {renderField("State Code", data.buyer.state_code, "buyer.state_code", confidenceScores.buyer.state_code)}
      </Section>

      {/* Line Items */}
      <Section title="Line Items">
        <LineItemsTable items={data.line_items} confidenceScores={confidenceScores.line_items} fieldStatuses={fieldStatuses} />
      </Section>

      {/* Totals */}
      <Section title="Totals">
        {renderField("Subtotal", formatCurrency(data.totals.subtotal), "totals.subtotal", confidenceScores.totals.subtotal)}
        {renderField("Taxable Amount", formatCurrency(data.totals.taxable_amount), "totals.taxable_amount", confidenceScores.totals.taxable_amount)}
        {renderField("CGST", formatCurrency(data.totals.cgst), "totals.cgst", confidenceScores.totals.cgst)}
        {renderField("SGST", formatCurrency(data.totals.sgst), "totals.sgst", confidenceScores.totals.sgst)}
        {renderField("IGST", formatCurrency(data.totals.igst), "totals.igst", confidenceScores.totals.igst)}
        {renderField("Total", formatCurrency(data.totals.total), "totals.total", confidenceScores.totals.total)}
      </Section>
    </div>
  );
}
```

---

## Role-Based Access

### Permission Checks

```typescript
// lib/permissions.ts
type Action =
  | "upload_files"
  | "create_collections"
  | "delete_collections"
  | "manage_users"
  | "manage_tenants";

const rolePermissions: Record<UserRole, Action[]> = {
  admin: ["upload_files", "create_collections", "delete_collections", "manage_users", "manage_tenants"],
  manager: ["upload_files", "create_collections"],
  member: ["upload_files", "create_collections"],
  viewer: [],
};

export function canPerform(role: UserRole, action: Action): boolean {
  return rolePermissions[role]?.includes(action) ?? false;
}

export function getEffectiveCollectionPermission(
  userRole: UserRole,
  explicitPermission?: CollectionPermission
): CollectionPermission | null {
  const implicit: Record<UserRole, CollectionPermission | null> = {
    admin: "owner",
    manager: "editor",
    member: "viewer",
    viewer: null,
  };

  const implicitPerm = implicit[userRole];
  if (!implicitPerm && !explicitPermission) return null;
  if (!implicitPerm) return explicitPermission || null;
  if (!explicitPermission) return implicitPerm;

  // Return higher permission
  const levels = { owner: 3, editor: 2, viewer: 1 };
  return levels[implicitPerm] >= levels[explicitPermission] ? implicitPerm : explicitPermission;
}
```

### Role Guard Component

```typescript
// components/auth/role-guard.tsx
interface RoleGuardProps {
  requiredRole?: UserRole;
  requiredAction?: Action;
  children: React.ReactNode;
  fallback?: React.ReactNode;
}

export function RoleGuard({
  requiredRole,
  requiredAction,
  children,
  fallback = null,
}: RoleGuardProps) {
  const user = useAuthStore((s) => s.user);

  if (!user) return null;

  if (requiredRole) {
    const roleLevel = { admin: 4, manager: 3, member: 2, viewer: 1 };
    if (roleLevel[user.role] < roleLevel[requiredRole]) {
      return <>{fallback}</>;
    }
  }

  if (requiredAction && !canPerform(user.role, requiredAction)) {
    return <>{fallback}</>;
  }

  return <>{children}</>;
}

// Usage
<RoleGuard requiredRole="admin">
  <Button>Manage Users</Button>
</RoleGuard>

<RoleGuard requiredAction="delete_collections" fallback={<span>Read Only</span>}>
  <Button variant="destructive">Delete Collection</Button>
</RoleGuard>
```

---

## Recommended Libraries

```json
{
  "dependencies": {
    "@tanstack/react-query": "^5.x",
    "@tanstack/react-table": "^8.x",
    "react-hook-form": "^7.x",
    "zod": "^3.x",
    "@hookform/resolvers": "^3.x",
    "zustand": "^4.x",
    "react-dropzone": "^14.x",
    "react-pdf": "^7.x",
    "date-fns": "^3.x",
    "lucide-react": "^0.x",
    "recharts": "^2.x",
    "jwt-decode": "^4.x",
    "class-variance-authority": "^0.x",
    "clsx": "^2.x",
    "tailwind-merge": "^2.x"
  },
  "devDependencies": {
    "@types/react": "^18.x",
    "typescript": "^5.x",
    "tailwindcss": "^3.x",
    "eslint": "^8.x",
    "prettier": "^3.x",
    "@playwright/test": "^1.x",
    "msw": "^2.x",
    "storybook": "^7.x"
  }
}
```

---

## Page Structure

```
app/
├── (auth)/
│   ├── login/
│   │   └── page.tsx
│   └── layout.tsx
├── (dashboard)/
│   ├── layout.tsx                # Authenticated layout with sidebar
│   ├── page.tsx                  # Dashboard/home
│   ├── files/
│   │   ├── page.tsx              # File list
│   │   └── [id]/
│   │       └── page.tsx          # File details
│   ├── collections/
│   │   ├── page.tsx              # Collection list
│   │   ├── new/
│   │   │   └── page.tsx          # Create collection
│   │   └── [id]/
│   │       ├── page.tsx          # Collection details
│   │       └── permissions/
│   │           └── page.tsx      # Manage permissions
│   ├── documents/
│   │   ├── page.tsx              # Document list
│   │   ├── new/
│   │   │   └── page.tsx          # Create document wizard
│   │   └── [id]/
│   │       ├── page.tsx          # Document viewer
│   │       └── validation/
│   │           └── page.tsx      # Validation details
│   ├── users/
│   │   ├── page.tsx              # User list (admin)
│   │   └── [id]/
│   │       └── page.tsx          # User details
│   └── settings/
│       └── page.tsx              # User settings
└── api/                          # API routes (if using Next.js API routes)
```

---

## Design Considerations

### Color Palette

```css
:root {
  /* Status colors */
  --status-success: #10b981; /* green-500 */
  --status-warning: #f59e0b; /* amber-500 */
  --status-error: #ef4444;   /* red-500 */
  --status-info: #3b82f6;    /* blue-500 */
  --status-pending: #6b7280; /* gray-500 */

  /* Brand colors */
  --primary: #2563eb;        /* blue-600 */
  --primary-hover: #1d4ed8;  /* blue-700 */

  /* Backgrounds */
  --bg-success: #d1fae5;     /* green-100 */
  --bg-warning: #fef3c7;     /* amber-100 */
  --bg-error: #fee2e2;       /* red-100 */
  --bg-info: #dbeafe;        /* blue-100 */
}
```

### Accessibility

- Use semantic HTML elements
- Ensure sufficient color contrast (WCAG AA minimum)
- Support keyboard navigation
- Add ARIA labels to interactive elements
- Test with screen readers
- Support reduced motion preferences

### Responsive Design

- Mobile-first approach
- Collapsible sidebar on mobile
- Stacked layouts for document viewer on small screens
- Touch-friendly targets (44x44px minimum)

### Performance

- Lazy load routes and heavy components
- Use React.memo for expensive components
- Virtualize long lists (react-window or @tanstack/react-virtual)
- Optimize images with next/image
- Implement proper caching strategies
- Use skeleton loaders for better perceived performance
