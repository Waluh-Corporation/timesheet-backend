package queue_test

import (
	"context"
	"testing"

	"timesheet-backend/config"
	"timesheet-backend/queue"
)

func TestNewQueueClient_NilConfig(t *testing.T) {
	client, err := queue.NewQueueClient(nil)
	if err == nil {
		t.Fatal("expected error with nil config")
	}
	if client != nil {
		t.Fatal("expected nil client with nil config")
	}
}

func TestNewQueueClient_ValidConfig(t *testing.T) {
	cfg := &config.Config{
		RedisAddr:     "127.0.0.1:6379",
		RedisPassword: "",
		RedisDB:       0,
	}

	client, err := queue.NewQueueClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}

	// Try enqueuing; if Redis is not running locally, it should return an error gracefully
	ctx := context.Background()
	_ = client.EnqueueTimesheetJob(ctx, "job-123", 1, 6, 2026)

	// Close client
	if err := client.Close(); err != nil {
		t.Fatalf("failed to close queue client: %v", err)
	}
}
