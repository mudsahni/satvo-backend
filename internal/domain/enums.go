package domain

// FileType represents the allowed file types for upload.
type FileType string

const (
	FileTypePDF FileType = "pdf"
	FileTypeJPG FileType = "jpg"
	FileTypePNG FileType = "png"
)

// AllowedFileTypes maps FileType to its MIME content type.
var AllowedFileTypes = map[FileType]string{
	FileTypePDF: "application/pdf",
	FileTypeJPG: "image/jpeg",
	FileTypePNG: "image/png",
}

// AllowedContentTypes maps MIME content types back to FileType.
var AllowedContentTypes = map[string]FileType{
	"application/pdf": FileTypePDF,
	"image/jpeg":      FileTypeJPG,
	"image/png":       FileTypePNG,
}

// AllowedExtensions maps file extensions (without dot) to FileType.
var AllowedExtensions = map[string]FileType{
	"pdf":  FileTypePDF,
	"jpg":  FileTypeJPG,
	"jpeg": FileTypeJPG,
	"png":  FileTypePNG,
}

// UserRole defines the role hierarchy within a tenant.
type UserRole string

const (
	RoleAdmin  UserRole = "admin"
	RoleMember UserRole = "member"
)

// CollectionPermission defines the access level for a collection.
type CollectionPermission string

const (
	CollectionPermOwner  CollectionPermission = "owner"
	CollectionPermEditor CollectionPermission = "editor"
	CollectionPermViewer CollectionPermission = "viewer"
)

// ValidCollectionPermissions maps valid permission strings.
var ValidCollectionPermissions = map[CollectionPermission]bool{
	CollectionPermOwner:  true,
	CollectionPermEditor: true,
	CollectionPermViewer: true,
}

// CollectionPermLevel returns the numeric level for permission comparison.
// Higher value = more access.
func CollectionPermLevel(p CollectionPermission) int {
	switch p {
	case CollectionPermOwner:
		return 3
	case CollectionPermEditor:
		return 2
	case CollectionPermViewer:
		return 1
	default:
		return 0
	}
}

// FileStatus represents the lifecycle of an uploaded file.
type FileStatus string

const (
	FileStatusPending  FileStatus = "pending"
	FileStatusUploaded FileStatus = "uploaded"
	FileStatusFailed   FileStatus = "failed"
	FileStatusDeleted  FileStatus = "deleted"
)
