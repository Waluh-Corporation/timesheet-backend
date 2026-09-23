package queue

import (
	"context"
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
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}
	client := asynq.NewClient(redisOpt)
	return &asynqClient{client: client}, nil
}

// EnqueueTimesheetJob dispatches a generation job into the Redis task queue.
func (c *asynqClient) EnqueueTimesheetJob(ctx context.Context, jobID string, userID uint, month, year int) error {
	payload, err := json.Marshal(GenerateTimesheetPayload{
		JobID:  jobID,
		UserID: userID,
		Month:  month,
		Year:   year,
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

// Close closes the underlying Redis client connection.
func (c *asynqClient) Close() error {
	return c.client.Close()
}
