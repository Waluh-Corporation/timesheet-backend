package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"timesheet-backend/config"
)

func TestNewS3StorageService(t *testing.T) {
	t.Run("nil config returns error", func(t *testing.T) {
		svc, err := NewS3StorageService(nil)
		assert.Error(t, err)
		assert.Nil(t, svc)
	})

	t.Run("valid config creates service with defaults", func(t *testing.T) {
		cfg := &config.Config{
			S3Region:       "ap-southeast-1",
			S3Bucket:       "my-timesheets",
			S3AccessKey:    "AKIAIOSFODNN7EXAMPLE",
			S3SecretKey:    "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			S3Endpoint:     "https://s3.ap-southeast-1.amazonaws.com",
			S3UsePathStyle: true,
		}
		svc, err := NewS3StorageService(cfg)
		require.NoError(t, err)
		require.NotNil(t, svc)
		assert.Equal(t, "my-timesheets", svc.bucket)
	})

	t.Run("empty region bucket and credentials uses defaults", func(t *testing.T) {
		cfg := &config.Config{
			S3Region: "",
			S3Bucket: "",
		}
		svc, err := NewS3StorageService(cfg)
		require.NoError(t, err)
		require.NotNil(t, svc)
		assert.Equal(t, "timesheets", svc.bucket)
	})
}

func TestS3StorageService_Validation(t *testing.T) {
	cfg := &config.Config{
		S3Region:    "us-east-1",
		S3Bucket:    "test-bucket",
		S3AccessKey: "AKIAIOSFODNN7EXAMPLE",
		S3SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}
	svc, err := NewS3StorageService(cfg)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("empty key for Upload returns error", func(t *testing.T) {
		err := svc.Upload(ctx, "", strings.NewReader("content"), "text/plain")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "object key cannot be empty")
	})

	t.Run("empty key for Download returns error", func(t *testing.T) {
		b, err := svc.Download(ctx, "")
		assert.Error(t, err)
		assert.Nil(t, b)
		assert.Contains(t, err.Error(), "object key cannot be empty")
	})

	t.Run("empty key for GetPresignedDownloadURL returns error", func(t *testing.T) {
		url, err := svc.GetPresignedDownloadURL(ctx, "", 1*time.Hour)
		assert.Error(t, err)
		assert.Empty(t, url)
		assert.Contains(t, err.Error(), "object key cannot be empty")
	})

	t.Run("empty key for GetPresignedDownloadURLWithFilename returns error", func(t *testing.T) {
		url, err := svc.GetPresignedDownloadURLWithFilename(ctx, "", "file.xlsx", 1*time.Hour)
		assert.Error(t, err)
		assert.Empty(t, url)
		assert.Contains(t, err.Error(), "object key cannot be empty")
	})

	t.Run("empty key for Delete returns error", func(t *testing.T) {
		err := svc.Delete(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "object key cannot be empty")
	})

	t.Run("empty key for FileExists returns error", func(t *testing.T) {
		exists, err := svc.FileExists(ctx, "")
		assert.Error(t, err)
		assert.False(t, exists)
		assert.Contains(t, err.Error(), "object key cannot be empty")
	})

	t.Run("valid key for GetPresignedDownloadURL succeeds offline", func(t *testing.T) {
		url, err := svc.GetPresignedDownloadURL(ctx, "exports/test.xlsx", 1*time.Hour)
		assert.NoError(t, err)
		assert.NotEmpty(t, url)
		assert.Contains(t, url, "exports/test.xlsx")
	})

	t.Run("valid key with filename and default expiry succeeds offline", func(t *testing.T) {
		url, err := svc.GetPresignedDownloadURLWithFilename(ctx, "exports/test.xlsx", "my-timesheet.xlsx", 0)
		assert.NoError(t, err)
		assert.NotEmpty(t, url)
		assert.Contains(t, url, "my-timesheet.xlsx")
	})
}

func TestS3StorageService_OperationsWithMockServer(t *testing.T) {
	ctx := context.Background()

	t.Run("Upload success and failure", func(t *testing.T) {
		statusCode := http.StatusOK
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(statusCode)
		}))
		defer server.Close()

		cfg := &config.Config{
			S3Region:       "us-east-1",
			S3Bucket:       "test-bucket",
			S3Endpoint:     server.URL,
			S3AccessKey:    "test-key",
			S3SecretKey:    "test-secret",
			S3UsePathStyle: true,
		}
		svc, err := NewS3StorageService(cfg)
		require.NoError(t, err)

		err = svc.Upload(ctx, "test-key.txt", strings.NewReader("hello"), "text/plain")
		assert.NoError(t, err)

		statusCode = http.StatusInternalServerError
		err = svc.Upload(ctx, "test-key.txt", strings.NewReader("hello"), "text/plain")
		assert.Error(t, err)
	})

	t.Run("Download success and failure", func(t *testing.T) {
		statusCode := http.StatusOK
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if statusCode != http.StatusOK {
				w.WriteHeader(statusCode)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "mock-file-content")
		}))
		defer server.Close()

		cfg := &config.Config{
			S3Region:       "us-east-1",
			S3Bucket:       "test-bucket",
			S3Endpoint:     server.URL,
			S3AccessKey:    "test-key",
			S3SecretKey:    "test-secret",
			S3UsePathStyle: true,
		}
		svc, err := NewS3StorageService(cfg)
		require.NoError(t, err)

		data, err := svc.Download(ctx, "test-key.txt")
		assert.NoError(t, err)
		assert.Equal(t, "mock-file-content", string(data))

		statusCode = http.StatusNotFound
		_, err = svc.Download(ctx, "nonexistent.txt")
		assert.Error(t, err)
	})

	t.Run("Delete success and failure", func(t *testing.T) {
		statusCode := http.StatusNoContent
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(statusCode)
		}))
		defer server.Close()

		cfg := &config.Config{
			S3Region:       "us-east-1",
			S3Bucket:       "test-bucket",
			S3Endpoint:     server.URL,
			S3AccessKey:    "test-key",
			S3SecretKey:    "test-secret",
			S3UsePathStyle: true,
		}
		svc, err := NewS3StorageService(cfg)
		require.NoError(t, err)

		err = svc.Delete(ctx, "test-key.txt")
		assert.NoError(t, err)

		statusCode = http.StatusInternalServerError
		err = svc.Delete(ctx, "test-key.txt")
		assert.Error(t, err)
	})

	t.Run("FileExists 200, 404, and 500", func(t *testing.T) {
		statusCode := http.StatusOK
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(statusCode)
		}))
		defer server.Close()

		cfg := &config.Config{
			S3Region:       "us-east-1",
			S3Bucket:       "test-bucket",
			S3Endpoint:     server.URL,
			S3AccessKey:    "test-key",
			S3SecretKey:    "test-secret",
			S3UsePathStyle: true,
		}
		svc, err := NewS3StorageService(cfg)
		require.NoError(t, err)

		// 200 OK -> exists
		exists, err := svc.FileExists(ctx, "test-key.txt")
		assert.NoError(t, err)
		assert.True(t, exists)

		// 404 Not Found -> does not exist
		statusCode = http.StatusNotFound
		exists, err = svc.FileExists(ctx, "test-key.txt")
		assert.NoError(t, err)
		assert.False(t, exists)

		// 500 Internal Server Error -> error
		statusCode = http.StatusInternalServerError
		exists, err = svc.FileExists(ctx, "test-key.txt")
		assert.Error(t, err)
		assert.False(t, exists)
	})
}
