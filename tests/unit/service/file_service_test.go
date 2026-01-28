package service_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/textproto"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"satvos/internal/config"
	"satvos/internal/domain"
	"satvos/internal/port"
	"satvos/internal/service"
	"satvos/mocks"
)

func testS3Config() config.S3Config {
	return config.S3Config{
		Region:        "us-east-1",
		Bucket:        "test-bucket",
		MaxFileSizeMB: 50,
		PresignExpiry: 3600,
	}
}

// createMultipartFile creates a fake multipart file header and content for testing.
func createMultipartFile(filename string, content []byte, contentType string) (multipart.File, *multipart.FileHeader) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	h.Set("Content-Type", contentType)

	part, _ := writer.CreatePart(h)
	_, _ = part.Write(content)
	writer.Close()

	reader := multipart.NewReader(body, writer.Boundary())
	form, _ := reader.ReadForm(int64(len(content) + 1024))
	file, _ := form.File["file"][0].Open()
	return file, form.File["file"][0]
}

// pdfContent returns minimal valid PDF bytes.
func pdfContent() []byte {
	return []byte("%PDF-1.4 test content that is at least a few bytes long for detection purposes")
}

// pngContent returns minimal valid PNG bytes (magic bytes).
func pngContent() []byte {
	// PNG magic bytes: 137 80 78 71 13 10 26 10
	header := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	return append(header, bytes.Repeat([]byte{0x00}, 100)...)
}

func TestFileService_Upload_Success_PDF(t *testing.T) {
	fileRepo := new(mocks.MockFileMetaRepo)
	storage := new(mocks.MockObjectStorage)
	cfg := testS3Config()
	svc := service.NewFileService(fileRepo, storage, &cfg)

	tenantID := uuid.New()
	userID := uuid.New()

	file, header := createMultipartFile("document.pdf", pdfContent(), "application/pdf")
	defer file.Close()

	fileRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.FileMeta")).Return(nil)
	storage.On("Upload", mock.Anything, mock.AnythingOfType("port.UploadInput")).
		Return(&port.UploadOutput{Location: "https://test-bucket.s3.amazonaws.com/test", ETag: "abc"}, nil)
	fileRepo.On("UpdateStatus", mock.Anything, tenantID, mock.AnythingOfType("uuid.UUID"), domain.FileStatusUploaded).Return(nil)

	result, err := svc.Upload(context.Background(), service.FileUploadInput{
		TenantID:   tenantID,
		UploadedBy: userID,
		File:       file,
		Header:     header,
	})

	assert.NoError(t, err)
	assert.Equal(t, domain.FileStatusUploaded, result.Status)
	assert.Equal(t, domain.FileTypePDF, result.FileType)
	assert.Equal(t, "document.pdf", result.OriginalName)

	fileRepo.AssertExpectations(t)
	storage.AssertExpectations(t)
}

func TestFileService_Upload_Success_PNG(t *testing.T) {
	fileRepo := new(mocks.MockFileMetaRepo)
	storage := new(mocks.MockObjectStorage)
	cfg := testS3Config()
	svc := service.NewFileService(fileRepo, storage, &cfg)

	tenantID := uuid.New()
	userID := uuid.New()

	file, header := createMultipartFile("image.png", pngContent(), "image/png")
	defer file.Close()

	fileRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.FileMeta")).Return(nil)
	storage.On("Upload", mock.Anything, mock.AnythingOfType("port.UploadInput")).
		Return(&port.UploadOutput{Location: "https://s3/test", ETag: "abc"}, nil)
	fileRepo.On("UpdateStatus", mock.Anything, tenantID, mock.AnythingOfType("uuid.UUID"), domain.FileStatusUploaded).Return(nil)

	result, err := svc.Upload(context.Background(), service.FileUploadInput{
		TenantID:   tenantID,
		UploadedBy: userID,
		File:       file,
		Header:     header,
	})

	assert.NoError(t, err)
	assert.Equal(t, domain.FileTypePNG, result.FileType)
}

func TestFileService_Upload_UnsupportedExtension(t *testing.T) {
	fileRepo := new(mocks.MockFileMetaRepo)
	storage := new(mocks.MockObjectStorage)
	cfg := testS3Config()
	svc := service.NewFileService(fileRepo, storage, &cfg)

	file, header := createMultipartFile("malware.exe", []byte("MZ fake exe content"), "application/octet-stream")
	defer file.Close()

	result, err := svc.Upload(context.Background(), service.FileUploadInput{
		TenantID:   uuid.New(),
		UploadedBy: uuid.New(),
		File:       file,
		Header:     header,
	})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrUnsupportedFileType)
}

func TestFileService_Upload_FileTooLarge(t *testing.T) {
	fileRepo := new(mocks.MockFileMetaRepo)
	storage := new(mocks.MockObjectStorage)
	cfg := testS3Config()
	cfg.MaxFileSizeMB = 1 // 1MB limit
	svc := service.NewFileService(fileRepo, storage, &cfg)

	// Create a file header with size exceeding limit
	file, header := createMultipartFile("large.pdf", pdfContent(), "application/pdf")
	defer file.Close()
	header.Size = 2 * 1024 * 1024 // 2MB

	result, err := svc.Upload(context.Background(), service.FileUploadInput{
		TenantID:   uuid.New(),
		UploadedBy: uuid.New(),
		File:       file,
		Header:     header,
	})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrFileTooLarge)
}

func TestFileService_Upload_StorageFailure(t *testing.T) {
	fileRepo := new(mocks.MockFileMetaRepo)
	storage := new(mocks.MockObjectStorage)
	cfg := testS3Config()
	svc := service.NewFileService(fileRepo, storage, &cfg)

	tenantID := uuid.New()
	file, header := createMultipartFile("document.pdf", pdfContent(), "application/pdf")
	defer file.Close()

	fileRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.FileMeta")).Return(nil)
	storage.On("Upload", mock.Anything, mock.AnythingOfType("port.UploadInput")).
		Return(nil, io.ErrUnexpectedEOF)
	fileRepo.On("UpdateStatus", mock.Anything, tenantID, mock.AnythingOfType("uuid.UUID"), domain.FileStatusFailed).Return(nil)

	result, err := svc.Upload(context.Background(), service.FileUploadInput{
		TenantID:   tenantID,
		UploadedBy: uuid.New(),
		File:       file,
		Header:     header,
	})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrUploadFailed)

	fileRepo.AssertExpectations(t)
	storage.AssertExpectations(t)
}

func TestFileService_GetByID_Success(t *testing.T) {
	fileRepo := new(mocks.MockFileMetaRepo)
	storage := new(mocks.MockObjectStorage)
	cfg := testS3Config()
	svc := service.NewFileService(fileRepo, storage, &cfg)

	tenantID := uuid.New()
	fileID := uuid.New()
	expected := &domain.FileMeta{
		ID:       fileID,
		TenantID: tenantID,
		Status:   domain.FileStatusUploaded,
	}

	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(expected, nil)

	result, err := svc.GetByID(context.Background(), tenantID, fileID)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestFileService_GetByID_NotFound(t *testing.T) {
	fileRepo := new(mocks.MockFileMetaRepo)
	storage := new(mocks.MockObjectStorage)
	cfg := testS3Config()
	svc := service.NewFileService(fileRepo, storage, &cfg)

	tenantID := uuid.New()
	fileID := uuid.New()

	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(nil, domain.ErrNotFound)

	result, err := svc.GetByID(context.Background(), tenantID, fileID)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestFileService_Delete_Success(t *testing.T) {
	fileRepo := new(mocks.MockFileMetaRepo)
	storage := new(mocks.MockObjectStorage)
	cfg := testS3Config()
	svc := service.NewFileService(fileRepo, storage, &cfg)

	tenantID := uuid.New()
	fileID := uuid.New()
	meta := &domain.FileMeta{
		ID:       fileID,
		TenantID: tenantID,
		S3Bucket: "test-bucket",
		S3Key:    "tenants/test/files/test.pdf",
		Status:   domain.FileStatusUploaded,
	}

	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(meta, nil)
	storage.On("Delete", mock.Anything, "test-bucket", "tenants/test/files/test.pdf").Return(nil)
	fileRepo.On("Delete", mock.Anything, tenantID, fileID).Return(nil)

	err := svc.Delete(context.Background(), tenantID, fileID)

	assert.NoError(t, err)
	fileRepo.AssertExpectations(t)
	storage.AssertExpectations(t)
}

func TestFileService_List_Success(t *testing.T) {
	fileRepo := new(mocks.MockFileMetaRepo)
	storage := new(mocks.MockObjectStorage)
	cfg := testS3Config()
	svc := service.NewFileService(fileRepo, storage, &cfg)

	tenantID := uuid.New()
	expected := []domain.FileMeta{
		{ID: uuid.New(), TenantID: tenantID, Status: domain.FileStatusUploaded},
		{ID: uuid.New(), TenantID: tenantID, Status: domain.FileStatusUploaded},
	}

	fileRepo.On("ListByTenant", mock.Anything, tenantID, 0, 20).Return(expected, 2, nil)

	files, total, err := svc.List(context.Background(), tenantID, 0, 20)

	assert.NoError(t, err)
	assert.Len(t, files, 2)
	assert.Equal(t, 2, total)
}

func TestFileService_GetDownloadURL_Success(t *testing.T) {
	fileRepo := new(mocks.MockFileMetaRepo)
	storage := new(mocks.MockObjectStorage)
	cfg := testS3Config()
	svc := service.NewFileService(fileRepo, storage, &cfg)

	tenantID := uuid.New()
	fileID := uuid.New()
	meta := &domain.FileMeta{
		ID:       fileID,
		TenantID: tenantID,
		S3Bucket: "test-bucket",
		S3Key:    "tenants/test/files/test.pdf",
	}

	fileRepo.On("GetByID", mock.Anything, tenantID, fileID).Return(meta, nil)
	storage.On("GetPresignedURL", mock.Anything, "test-bucket", "tenants/test/files/test.pdf", int64(3600)).
		Return("https://presigned-url.example.com/test", nil)

	url, err := svc.GetDownloadURL(context.Background(), tenantID, fileID)

	assert.NoError(t, err)
	assert.Equal(t, "https://presigned-url.example.com/test", url)
}
