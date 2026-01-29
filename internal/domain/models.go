package domain

import (
	"time"

	"github.com/google/uuid"
)

// Tenant represents an isolated organizational tenant.
type Tenant struct {
	ID        uuid.UUID `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	Slug      string    `db:"slug" json:"slug"`
	IsActive  bool      `db:"is_active" json:"is_active"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// User represents an authenticated user belonging to a tenant.
type User struct {
	ID           uuid.UUID `db:"id" json:"id"`
	TenantID     uuid.UUID `db:"tenant_id" json:"tenant_id"`
	Email        string    `db:"email" json:"email"`
	PasswordHash string    `db:"password_hash" json:"-"`
	FullName     string    `db:"full_name" json:"full_name"`
	Role         UserRole  `db:"role" json:"role"`
	IsActive     bool      `db:"is_active" json:"is_active"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

// Collection represents a grouping of files within a tenant.
type Collection struct {
	ID          uuid.UUID `db:"id" json:"id"`
	TenantID    uuid.UUID `db:"tenant_id" json:"tenant_id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	CreatedBy   uuid.UUID `db:"created_by" json:"created_by"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// CollectionPermissionEntry represents a user's permission on a collection.
type CollectionPermissionEntry struct {
	ID           uuid.UUID            `db:"id" json:"id"`
	CollectionID uuid.UUID            `db:"collection_id" json:"collection_id"`
	TenantID     uuid.UUID            `db:"tenant_id" json:"tenant_id"`
	UserID       uuid.UUID            `db:"user_id" json:"user_id"`
	Permission   CollectionPermission `db:"permission" json:"permission"`
	GrantedBy    uuid.UUID            `db:"granted_by" json:"granted_by"`
	CreatedAt    time.Time            `db:"created_at" json:"created_at"`
}

// CollectionFile represents the association between a collection and a file.
type CollectionFile struct {
	CollectionID uuid.UUID `db:"collection_id" json:"collection_id"`
	FileID       uuid.UUID `db:"file_id" json:"file_id"`
	TenantID     uuid.UUID `db:"tenant_id" json:"tenant_id"`
	AddedBy      uuid.UUID `db:"added_by" json:"added_by"`
	AddedAt      time.Time `db:"added_at" json:"added_at"`
}

// FileMeta stores metadata about an uploaded file.
type FileMeta struct {
	ID           uuid.UUID  `db:"id" json:"id"`
	TenantID     uuid.UUID  `db:"tenant_id" json:"tenant_id"`
	UploadedBy   uuid.UUID  `db:"uploaded_by" json:"uploaded_by"`
	FileName     string     `db:"file_name" json:"file_name"`
	OriginalName string     `db:"original_name" json:"original_name"`
	FileType     FileType   `db:"file_type" json:"file_type"`
	FileSize     int64      `db:"file_size" json:"file_size"`
	S3Bucket     string     `db:"s3_bucket" json:"s3_bucket"`
	S3Key        string     `db:"s3_key" json:"s3_key"`
	ContentType  string     `db:"content_type" json:"content_type"`
	Status       FileStatus `db:"status" json:"status"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
}
