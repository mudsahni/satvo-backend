package domain

import "errors"

var (
	ErrNotFound            = errors.New("resource not found")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrForbidden           = errors.New("forbidden")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrTenantInactive      = errors.New("tenant is inactive")
	ErrUserInactive        = errors.New("user is inactive")
	ErrUnsupportedFileType = errors.New("unsupported file type")
	ErrFileTooLarge        = errors.New("file exceeds maximum allowed size")
	ErrDuplicateEmail      = errors.New("email already exists for this tenant")
	ErrDuplicateTenantSlug = errors.New("tenant slug already exists")
	ErrUploadFailed        = errors.New("file upload to storage failed")
)
