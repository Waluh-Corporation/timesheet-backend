package queue

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"timesheet-backend/config"
)

// QueueClient provides an interface for dispatching background timesheet generation jobs.
type QueueClient interface {
	EnqueueTimesheetJob(ctx context.Context, jobID string, userID uint, month, year int) error
	Close() error
}

// PDFQueueClient extends QueueClient with support for specifying whether to render and include PDFs.
type PDFQueueClient interface {
	QueueClient
	EnqueueTimesheetJobWithPDF(ctx context.Context, jobID string, userID uint, month, year int, includePDF bool) error
}

type asynqClient struct {
	client *asynq.Client
}

// NewQueueClient creates an Asynq-backed task queue client connected to Redis.
func NewQueueClient(cfg *config.Config) (QueueClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Username: cfg.RedisUsername,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}
	if cfg.RedisTLSEnabled {
		redisOpt.TLSConfig = &tls.Config{
			InsecureSkipVerify: cfg.RedisTLSSkipVerify, //nolint:gosec // configurable for internal TLS
		}
	}
	client := asynq.NewClient(redisOpt)
	return &asynqClient{client: client}, nil
}

// EnqueueTimesheetJob dispatches a generation job into the Redis task queue with default PDF settings.
func (c *asynqClient) EnqueueTimesheetJob(ctx context.Context, jobID string, userID uint, month, year int) error {
	return c.EnqueueTimesheetJobWithPDF(ctx, jobID, userID, month, year, false)
}

// EnqueueTimesheetJobWithPDF dispatches a generation job with the specified PDF inclusion option into the Redis task queue.
func (c *asynqClient) EnqueueTimesheetJobWithPDF(ctx context.Context, jobID string, userID uint, month, year int, includePDF bool) error {
	payload, err := json.Marshal(GenerateTimesheetPayload{
		JobID:      jobID,
		UserID:     userID,
		Month:      month,
		Year:       year,
		IncludePDF: includePDF,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal task payload: %w", err)
	}

	task := asynq.NewTask(TypeTimesheetGenerate, payload)
	_, err = c.client.EnqueueContext(ctx, task,
		asynq.MaxRetry(3),
		asynq.Timeout(5*time.Minute),
		asynq.Retention(24*time.Hour),
	)
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}
	return nil
}

// EnqueueJob dispatches a timesheet generation job using either PDFQueueClient or standard QueueClient.
func EnqueueJob(ctx context.Context, qc QueueClient, jobID string, userID uint, month, year int, includePDF bool) error {
	if pqc, ok := qc.(PDFQueueClient); ok {
		return pqc.EnqueueTimesheetJobWithPDF(ctx, jobID, userID, month, year, includePDF)
	}
	return qc.EnqueueTimesheetJob(ctx, jobID, userID, month, year)
}

// Close closes the underlying Redis client connection.
func (c *asynqClient) Close() error {
	return c.client.Close()
}
