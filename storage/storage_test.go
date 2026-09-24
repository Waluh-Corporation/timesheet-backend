package storage

import (
	"context"
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
}

func TestS3StorageService_Validation(t *testing.T) {
	cfg := &config.Config{
		S3Region: "us-east-1",
		S3Bucket: "test-bucket",
	}
	svc, err := NewS3StorageService(cfg)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("empty key for Upload returns error", func(t *testing.T) {
		err := svc.Upload(ctx, "", strings.NewReader("content"), "text/plain")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "object key cannot be empty")
	})

	t.Run("empty key for GetPresignedDownloadURL returns error", func(t *testing.T) {
		url, err := svc.GetPresignedDownloadURL(ctx, "", 1*time.Hour)
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
}
