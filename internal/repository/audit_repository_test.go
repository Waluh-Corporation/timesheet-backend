package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

func TestAuditRepository(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	repo := repository.NewAuditRepository(tx)
	ctx := context.Background()

	t.Run("Create nil entry returns nil", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.NoError(t, err)
	})

	t.Run("Create valid entry and List with pagination", func(t *testing.T) {
		assert.Equal(t, "audit_logs", models.AuditLog{}.TableName())

		entry1 := &models.AuditLog{
			Action:       "USER_LOGIN",
			ResourceType: "auth",
			ResourceID:   "1",
			IPAddress:    "127.0.0.1",
			UserAgent:    "Go-Test",
			CreatedAt:    time.Now().Add(-1 * time.Hour),
		}
		require.NoError(t, repo.Create(ctx, entry1))
		assert.NotZero(t, entry1.ID)

		entry2 := &models.AuditLog{
			Action:       "USER_DEACTIVATED",
			ResourceType: "user",
			ResourceID:   "2",
			IPAddress:    "127.0.0.1",
			UserAgent:    "Go-Test",
		}
		require.NoError(t, repo.Create(ctx, entry2))
		assert.NotZero(t, entry2.ID)
		assert.False(t, entry2.CreatedAt.IsZero())

		// Default limit & offset
		entries, total, err := repo.List(ctx, repository.AuditLogFilter{}, 0, -1)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, int64(2))
		assert.NotEmpty(t, entries)

		// Max limit cap (100)
		entriesLimited, totalLimited, err := repo.List(ctx, repository.AuditLogFilter{}, 200, 0)
		require.NoError(t, err)
		assert.Equal(t, total, totalLimited)
		assert.NotEmpty(t, entriesLimited)

		// Pagination with offset
		entriesPage2, _, err := repo.List(ctx, repository.AuditLogFilter{}, 1, 1)
		require.NoError(t, err)
		assert.Len(t, entriesPage2, 1)

		// Filter by Action
		actionEntries, actionTotal, err := repo.List(ctx, repository.AuditLogFilter{Action: "USER_LOGIN"}, 10, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, actionTotal, int64(1))
		for _, e := range actionEntries {
			assert.Equal(t, "USER_LOGIN", e.Action)
		}

		// Filter by ResourceType
		resEntries, resTotal, err := repo.List(ctx, repository.AuditLogFilter{ResourceType: "user"}, 10, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, resTotal, int64(1))
		for _, e := range resEntries {
			assert.Equal(t, "user", e.ResourceType)
		}

		// Filter by ActorUsername (empty should match none or specific)
		userEntries, userTotal, err := repo.List(ctx, repository.AuditLogFilter{ActorUsername: "nonexistent_user_123"}, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(0), userTotal)
		assert.Empty(t, userEntries)

		// Filter by date range
		now := time.Now()
		startTime := now.Add(-2 * time.Hour)
		endTime := now.Add(1 * time.Hour)
		dateEntries, dateTotal, err := repo.List(ctx, repository.AuditLogFilter{
			StartDate: &startTime,
			EndDate:   &endTime,
		}, 10, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, dateTotal, int64(2))
		assert.NotEmpty(t, dateEntries)

		// StartDate after EndDate returns 0 without executing impossible query
		invStart := now.Add(1 * time.Hour)
		invEnd := now.Add(-1 * time.Hour)
		invEntries, invTotal, err := repo.List(ctx, repository.AuditLogFilter{
			StartDate: &invStart,
			EndDate:   &invEnd,
		}, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(0), invTotal)
		assert.NotNil(t, invEntries)
		assert.Empty(t, invEntries)

		// Filter with leading/trailing whitespace is trimmed cleanly
		wsEntries, wsTotal, err := repo.List(ctx, repository.AuditLogFilter{Action: "  USER_LOGIN  "}, 10, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, wsTotal, int64(1))
		assert.NotEmpty(t, wsEntries)

		// Create nil entry safely returns nil
		assert.NoError(t, repo.Create(ctx, nil))

		// Nil repo or nil db safely returns error without panic
		nilRepo := repository.NewAuditRepository(nil)
		assert.Error(t, nilRepo.Create(ctx, &models.AuditLog{Action: "TEST"}))
		nilEntries, nilTotal, nilErr := nilRepo.List(ctx, repository.AuditLogFilter{}, 10, 0)
		assert.Error(t, nilErr)
		assert.Equal(t, int64(0), nilTotal)
		assert.Nil(t, nilEntries)
	})
}
