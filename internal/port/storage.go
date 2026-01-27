package port

import (
	"context"
	"io"
)

// UploadInput encapsulates the parameters needed to upload an object.
type UploadInput struct {
	Bucket      string
	Key         string
	Body        io.Reader
	ContentType string
	Size        int64
}

// UploadOutput contains the result of a successful upload.
type UploadOutput struct {
	Location string
	ETag     string
}

// ObjectStorage abstracts cloud object storage operations.
type ObjectStorage interface {
	Upload(ctx context.Context, input UploadInput) (*UploadOutput, error)
	Delete(ctx context.Context, bucket, key string) error
	GetPresignedURL(ctx context.Context, bucket, key string, expirySeconds int64) (string, error)
}
