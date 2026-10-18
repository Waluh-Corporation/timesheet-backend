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
		entries, total, err := repo.List(ctx, 0, -1)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, int64(2))
		assert.NotEmpty(t, entries)

		// Max limit cap (100)
		entriesLimited, totalLimited, err := repo.List(ctx, 200, 0)
		require.NoError(t, err)
		assert.Equal(t, total, totalLimited)
		assert.NotEmpty(t, entriesLimited)

		// Pagination with offset
		entriesPage2, _, err := repo.List(ctx, 1, 1)
		require.NoError(t, err)
		assert.Len(t, entriesPage2, 1)
	})
}
