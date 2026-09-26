package repository_test

import (
	"context"
	"testing"

	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

func TestPushRepository(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	repo := repository.NewPushRepository(tx)
	ctx := context.Background()

	// Ensure test user
	user := &models.User{
		Username:     "push_repo_user_unique",
		Email:        "push_repo_user@example.com",
		Name:         "Push Repo User",
		Role:         models.RoleUser,
		PasswordHash: "dummyhash",
		IsActive:     true,
	}
	if err := tx.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// 1. Subscribe
	sub := &models.PushSubscription{
		UserID:   user.ID,
		Endpoint: "https://fcm.googleapis.com/fcm/send/unique-test-token-1",
		P256dh:   "test-p256dh-key-1",
		Auth:     "test-auth-secret-1",
	}
	if err := repo.Subscribe(ctx, sub); err != nil {
		t.Fatalf("repo.Subscribe failed: %v", err)
	}

	// Upsert with new keys
	subUpdated := &models.PushSubscription{
		UserID:   user.ID,
		Endpoint: "https://fcm.googleapis.com/fcm/send/unique-test-token-1",
		P256dh:   "test-p256dh-key-updated",
		Auth:     "test-auth-secret-updated",
	}
	if err := repo.Subscribe(ctx, subUpdated); err != nil {
		t.Fatalf("repo.Subscribe upsert failed: %v", err)
	}

	// Second subscription
	sub2 := &models.PushSubscription{
		UserID:   user.ID,
		Endpoint: "https://fcm.googleapis.com/fcm/send/unique-test-token-2",
		P256dh:   "test-p256dh-key-2",
		Auth:     "test-auth-secret-2",
	}
	if err := repo.Subscribe(ctx, sub2); err != nil {
		t.Fatalf("repo.Subscribe sub2 failed: %v", err)
	}

	// 2. ListByUserID
	subs, err := repo.ListByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("repo.ListByUserID failed: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 subscriptions, got %d", len(subs))
	}

	// 3. Unsubscribe by endpoint
	if err := repo.Unsubscribe(ctx, user.ID, "https://fcm.googleapis.com/fcm/send/unique-test-token-1"); err != nil {
		t.Fatalf("repo.Unsubscribe by endpoint failed: %v", err)
	}

	subs, err = repo.ListByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("repo.ListByUserID after unsubscribe failed: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 remaining subscription, got %d", len(subs))
	}

	// 4. DeleteByID
	if err := repo.DeleteByID(ctx, subs[0].ID); err != nil {
		t.Fatalf("repo.DeleteByID failed: %v", err)
	}

	subs, err = repo.ListByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("repo.ListByUserID after deleteByID failed: %v", err)
	}
	if len(subs) != 0 {
		t.Fatalf("expected 0 remaining subscriptions, got %d", len(subs))
	}

	// 5. Unsubscribe all for user
	sub3 := &models.PushSubscription{
		UserID:   user.ID,
		Endpoint: "https://fcm.googleapis.com/fcm/send/unique-test-token-3",
		P256dh:   "test-p256dh-key-3",
		Auth:     "test-auth-secret-3",
	}
	_ = repo.Subscribe(ctx, sub3)
	if err := repo.Unsubscribe(ctx, user.ID, ""); err != nil {
		t.Fatalf("repo.Unsubscribe all failed: %v", err)
	}
	subs, _ = repo.ListByUserID(ctx, user.ID)
	if len(subs) != 0 {
		t.Fatalf("expected 0 subscriptions after unsubscribe all, got %d", len(subs))
	}
}
