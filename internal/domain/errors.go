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
	ErrUploadFailed            = errors.New("file upload to storage failed")
	ErrCollectionNotFound      = errors.New("collection not found")
	ErrCollectionPermDenied    = errors.New("insufficient collection permission")
	ErrDuplicateCollectionFile = errors.New("file already exists in collection")
	ErrSelfPermissionRemoval   = errors.New("cannot remove own permission")
	ErrInvalidPermission       = errors.New("invalid collection permission")
	ErrDocumentNotFound        = errors.New("document not found")
	ErrDocumentAlreadyExists   = errors.New("document already exists for this file")
	ErrDocumentNotParsed       = errors.New("document has not been parsed yet")
	ErrValidationRuleNotFound  = errors.New("validation rule not found")
	ErrInsufficientRole        = errors.New("insufficient role for this action")
	ErrInvalidStructuredData   = errors.New("invalid structured data format")
	ErrQuotaExceeded           = errors.New("monthly document quota exceeded")
)
