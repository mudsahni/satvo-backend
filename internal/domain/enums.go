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

// FileStatus represents the lifecycle of an uploaded file.
type FileStatus string

const (
	FileStatusPending  FileStatus = "pending"
	FileStatusUploaded FileStatus = "uploaded"
	FileStatusFailed   FileStatus = "failed"
	FileStatusDeleted  FileStatus = "deleted"
)
