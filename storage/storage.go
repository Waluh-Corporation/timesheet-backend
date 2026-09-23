package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"timesheet-backend/config"
)

// StorageService provides high-level object storage operations for generated artifacts.
type StorageService interface {
	Upload(ctx context.Context, key string, body io.Reader, contentType string) error
	GetPresignedDownloadURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	Delete(ctx context.Context, key string) error
}

// S3StorageService implements StorageService backed by AWS SDK for Go v2.
type S3StorageService struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
}

// NewS3StorageService initializes an S3StorageService configured with AWS SDK Go v2.
// It supports standard AWS S3 as well as S3-compatible alternatives (MinIO, Cloudflare R2, Wasabi).
func NewS3StorageService(cfg *config.Config) (*S3StorageService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	region := cfg.S3Region
	if strings.TrimSpace(region) == "" {
		region = "us-east-1"
	}

	var optFns []func(*awsconfig.LoadOptions) error
	optFns = append(optFns, awsconfig.WithRegion(region))

	if strings.TrimSpace(cfg.S3AccessKey) != "" && strings.TrimSpace(cfg.S3SecretKey) != "" {
		optFns = append(optFns, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if strings.TrimSpace(cfg.S3Endpoint) != "" {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
		}
		o.UsePathStyle = cfg.S3UsePathStyle
	})

	presignClient := s3.NewPresignClient(s3Client)

	bucket := cfg.S3Bucket
	if strings.TrimSpace(bucket) == "" {
		bucket = "timesheets"
	}

	return &S3StorageService{
		client:        s3Client,
		presignClient: presignClient,
		bucket:        bucket,
	}, nil
}

// Upload uploads an object with the specified content type into S3.
func (s *S3StorageService) Upload(ctx context.Context, key string, body io.Reader, contentType string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("object key cannot be empty")
	}
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	}
	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to upload object %s to bucket %s: %w", key, s.bucket, err)
	}
	return nil
}

// GetPresignedDownloadURL generates a presigned GET URL valid for the given duration.
func (s *S3StorageService) GetPresignedDownloadURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("object key cannot be empty")
	}
	if expiry <= 0 {
		expiry = 7 * 24 * time.Hour
	}

	req, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("failed to presign get object %s: %w", key, err)
	}

	return req.URL, nil
}

// Delete removes an object from S3.
func (s *S3StorageService) Delete(ctx context.Context, key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("object key cannot be empty")
	}
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}
	_, err := s.client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete object %s from bucket %s: %w", key, s.bucket, err)
	}
	return nil
}
