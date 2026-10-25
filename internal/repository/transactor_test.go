package repository_test

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"timesheet-backend/auth"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/internal/service"
	"timesheet-backend/models"
)

func TestTransactor_InjectAndExtractTx(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// When context has no tx, ExtractTx returns fallback (configured with context)
	extracted := repository.ExtractTx(ctx, db)
	if extracted == nil {
		t.Fatal("expected non-nil db from ExtractTx fallback")
	}

	// When fallback is nil and no tx in context, returns nil safely
	if nilExtracted := repository.ExtractTx(ctx, nil); nilExtracted != nil {
		t.Fatal("expected nil db when both context tx and fallback are nil")
	}

	// Create a transaction and test injection and extraction
	tx := db.Begin()
	defer tx.Rollback()

	ctxWithTx := repository.InjectTx(ctx, tx)
	extractedTx := repository.ExtractTx(ctxWithTx, db)
	if extractedTx == nil {
		t.Fatal("expected non-nil db from ExtractTx with injected tx")
	}
}

func TestGormTransactor_WithinTransaction(t *testing.T) {
	db := setupTestDB(t)

	transactor := repository.NewGormTransactor(db)
	ctx := context.Background()

	t.Run("successful transaction commits", func(t *testing.T) {
		var createdID uint
		err := transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
			txDB := repository.ExtractTx(txCtx, db)
			user := &models.User{
				Username:     "tx_commit_user",
				Email:        "tx_commit@example.com",
				Role:         models.RoleUser,
				PasswordHash: "hash123",
				IsActive:     true,
			}
			if err := txDB.Create(user).Error; err != nil {
				return err
			}
			createdID = user.ID
			return nil
		})
		if err != nil {
			t.Fatalf("expected nil error on commit, got %v", err)
		}
		defer func() {
			if createdID != 0 {
				db.Unscoped().Delete(&models.User{}, createdID)
			}
		}()

		// Verify user exists in database
		var found models.User
		if err := db.First(&found, createdID).Error; err != nil {
			t.Fatalf("expected user to exist after commit, got %v", err)
		}
	})

	t.Run("failing transaction rolls back", func(t *testing.T) {
		var createdID uint
		errSimulated := errors.New("simulated transaction failure")

		err := transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
			txDB := repository.ExtractTx(txCtx, db)
			user := &models.User{
				Username:     "tx_rollback_user",
				Email:        "tx_rollback@example.com",
				Role:         models.RoleUser,
				PasswordHash: "hash123",
				IsActive:     true,
			}
			if err := txDB.Create(user).Error; err != nil {
				return err
			}
			createdID = user.ID
			return errSimulated
		})

		if !errors.Is(err, errSimulated) {
			t.Fatalf("expected simulated error, got %v", err)
		}

		// Verify user does not exist in database after rollback
		var found models.User
		if err := db.First(&found, createdID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
			db.Unscoped().Delete(&models.User{}, createdID)
			t.Fatalf("expected ErrRecordNotFound after rollback, got %v", err)
		}
	})

	t.Run("nested transaction commits outer and inner", func(t *testing.T) {
		var u1ID, u2ID uint
		err := transactor.WithinTransaction(ctx, func(outerCtx context.Context) error {
			txDB := repository.ExtractTx(outerCtx, db)
			u1 := &models.User{
				Username:     "tx_nested_u1",
				Email:        "tx_nested_u1@example.com",
				Role:         models.RoleUser,
				PasswordHash: "hash123",
				IsActive:     true,
			}
			if err := txDB.Create(u1).Error; err != nil {
				return err
			}
			u1ID = u1.ID

			return transactor.WithinTransaction(outerCtx, func(innerCtx context.Context) error {
				innerDB := repository.ExtractTx(innerCtx, db)
				u2 := &models.User{
					Username:     "tx_nested_u2",
					Email:        "tx_nested_u2@example.com",
					Role:         models.RoleUser,
					PasswordHash: "hash123",
					IsActive:     true,
				}
				if err := innerDB.Create(u2).Error; err != nil {
					return err
				}
				u2ID = u2.ID
				return nil
			})
		})
		if err != nil {
			t.Fatalf("expected nil error on nested transaction, got %v", err)
		}
		defer func() {
			if u1ID != 0 {
				db.Unscoped().Delete(&models.User{}, u1ID)
			}
			if u2ID != 0 {
				db.Unscoped().Delete(&models.User{}, u2ID)
			}
		}()

		var found1, found2 models.User
		if err := db.First(&found1, u1ID).Error; err != nil {
			t.Errorf("expected u1 to exist, got %v", err)
		}
		if err := db.First(&found2, u2ID).Error; err != nil {
			t.Errorf("expected u2 to exist, got %v", err)
		}
	})

	t.Run("nested transaction rollback to savepoint rolls back inner only", func(t *testing.T) {
		var u1ID, u2ID uint
		errSimulated := errors.New("inner rollback error")

		err := transactor.WithinTransaction(ctx, func(outerCtx context.Context) error {
			txDB := repository.ExtractTx(outerCtx, db)
			u1 := &models.User{
				Username:     "tx_savepoint_u1",
				Email:        "tx_savepoint_u1@example.com",
				Role:         models.RoleUser,
				PasswordHash: "hash123",
				IsActive:     true,
			}
			if err := txDB.Create(u1).Error; err != nil {
				return err
			}
			u1ID = u1.ID

			innerErr := transactor.WithinTransaction(outerCtx, func(innerCtx context.Context) error {
				innerDB := repository.ExtractTx(innerCtx, db)
				u2 := &models.User{
					Username:     "tx_savepoint_u2",
					Email:        "tx_savepoint_u2@example.com",
					Role:         models.RoleUser,
					PasswordHash: "hash123",
					IsActive:     true,
				}
				if err := innerDB.Create(u2).Error; err != nil {
					return err
				}
				u2ID = u2.ID
				return errSimulated
			})
			if !errors.Is(innerErr, errSimulated) {
				t.Errorf("expected innerErr to match errSimulated, got %v", innerErr)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("expected outer transaction to commit successfully, got %v", err)
		}
		defer func() {
			if u1ID != 0 {
				db.Unscoped().Delete(&models.User{}, u1ID)
			}
			if u2ID != 0 {
				db.Unscoped().Delete(&models.User{}, u2ID)
			}
		}()

		var found1, found2 models.User
		if err := db.First(&found1, u1ID).Error; err != nil {
			t.Errorf("expected outer u1 to exist after commit, got %v", err)
		}
		if err := db.First(&found2, u2ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("expected inner u2 to be rolled back to savepoint, got err=%v", err)
		}
	})
}

func TestTransactor_EdgeCasesAndSafety(t *testing.T) {
	db := setupTestDB(t)

	t.Run("HasTx safety checks", func(t *testing.T) {
		var nilCtx context.Context
		if repository.HasTx(nilCtx) { //nolint:staticcheck // testing defensive nil context safety
			t.Error("expected HasTx(nil) to be false")
		}
		if repository.HasTx(context.Background()) {
			t.Error("expected HasTx(bg) to be false")
		}

		tx := db.Begin()
		defer tx.Rollback()

		ctxWithTx := repository.InjectTx(context.Background(), tx)
		if !repository.HasTx(ctxWithTx) {
			t.Error("expected HasTx(ctxWithTx) to be true")
		}
	})

	t.Run("InjectTx with nil context", func(t *testing.T) {
		tx := db.Begin()
		defer tx.Rollback()

		var nilCtx context.Context
		ctx := repository.InjectTx(nilCtx, tx) //nolint:staticcheck // testing defensive nil context safety
		if ctx == nil {
			t.Fatal("expected non-nil context from InjectTx(nil, tx)")
		}
		if !repository.HasTx(ctx) {
			t.Error("expected HasTx to be true on returned context")
		}
	})

	t.Run("ExtractTx with nil ctx or nil fallback", func(t *testing.T) {
		var nilCtx context.Context
		if res := repository.ExtractTx(nilCtx, nil); res != nil { //nolint:staticcheck // testing defensive nil context safety
			t.Errorf("expected nil from ExtractTx(nil, nil), got %v", res)
		}
		if res := repository.ExtractTx(nilCtx, db); res == nil { //nolint:staticcheck // testing defensive nil context safety
			t.Error("expected non-nil fallback from ExtractTx(nil, db)")
		}
	})

	t.Run("WithinTransaction nil transactor or nil db", func(t *testing.T) {
		var nilT *repository.GormTransactor
		err := nilT.WithinTransaction(context.Background(), func(ctx context.Context) error { return nil })
		if err == nil {
			t.Error("expected error when transactor is nil, got nil")
		}

		emptyT := repository.NewGormTransactor(nil)
		err = emptyT.WithinTransaction(context.Background(), func(ctx context.Context) error { return nil })
		if err == nil {
			t.Error("expected error when transactor db is nil, got nil")
		}
	})

	t.Run("WithinTransaction nil fn", func(t *testing.T) {
		transactor := repository.NewGormTransactor(db)
		if err := transactor.WithinTransaction(context.Background(), nil); err != nil {
			t.Errorf("expected nil error when fn is nil, got %v", err)
		}
	})

	t.Run("WithinTransaction canceled context", func(t *testing.T) {
		transactor := repository.NewGormTransactor(db)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
			t.Error("callback should not be executed when context is already canceled")
			return nil
		})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})
}

func TestRepositories_WithinTransactionImplementation(t *testing.T) {
	db := setupTestDB(t)

	t.Run("userRepository satisfies domain.Transactor", func(t *testing.T) {
		userRepo := repository.NewUserRepository(db)
		txRepo, ok := userRepo.(interface {
			WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
		})
		if !ok {
			t.Fatal("expected userRepository to satisfy Transactor interface")
		}

		var createdID uint
		err := txRepo.WithinTransaction(context.Background(), func(ctx context.Context) error {
			u := &models.User{
				Username:     "tx_user_repo_test",
				Email:        "tx_user_repo@example.com",
				Role:         models.RoleUser,
				PasswordHash: "hash123",
				IsActive:     true,
			}
			if err := userRepo.Create(ctx, u); err != nil {
				return err
			}
			createdID = u.ID
			return nil
		})
		if err != nil {
			t.Fatalf("expected WithinTransaction to succeed, got %v", err)
		}
		defer func() {
			if createdID != 0 {
				db.Unscoped().Delete(&models.User{}, createdID)
			}
		}()

		found, err := userRepo.FindByID(context.Background(), createdID)
		if err != nil || found == nil {
			t.Fatalf("expected user to be created and committed, got %v", err)
		}
	})

	t.Run("masterRepository satisfies domain.Transactor", func(t *testing.T) {
		masterRepo := repository.NewMasterRepository(db)
		txRepo, ok := masterRepo.(interface {
			WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
		})
		if !ok {
			t.Fatal("expected masterRepository to satisfy Transactor interface")
		}

		err := txRepo.WithinTransaction(context.Background(), func(ctx context.Context) error {
			return nil
		})
		if err != nil {
			t.Fatalf("expected WithinTransaction to succeed, got %v", err)
		}
	})
}

func TestReviewProfileChange_AtomicRollbackAndCommit_RealDB(t *testing.T) {
	db := setupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	transactor := repository.NewGormTransactor(db)
	svc := service.NewUserService(userRepo, userRepo, auth.DefaultHasher, nil, nil, transactor)
	ctx := context.Background()

	t.Run("ReviewProfileChange commit updates user and profile change atomically", func(t *testing.T) {
		u := &models.User{
			Username:     "tx_real_u1",
			Email:        "tx_real_u1@example.com",
			Name:         "Original Real Name",
			Role:         models.RoleUser,
			PasswordHash: "hash123",
			IsActive:     true,
		}
		if err := userRepo.Create(ctx, u); err != nil {
			t.Fatalf("failed to create user: %v", err)
		}
		defer db.Unscoped().Delete(&models.User{}, u.ID)

		change := &models.ProfileChangeRequest{
			UserID: u.ID,
			Name:   "Updated Real Name",
			Status: models.ProfilePending,
		}
		if err := userRepo.CreateProfileChange(ctx, change); err != nil {
			t.Fatalf("failed to create profile change: %v", err)
		}
		defer db.Unscoped().Delete(&models.ProfileChangeRequest{}, change.ID)

		if err := svc.ReviewProfileChange(ctx, change.ID, u.ID, "approve"); err != nil {
			t.Fatalf("ReviewProfileChange failed: %v", err)
		}

		updatedU, err := userRepo.FindByID(ctx, u.ID)
		if err != nil || updatedU.Name != "Updated Real Name" {
			t.Errorf("expected user name to be 'Updated Real Name', got %v", updatedU)
		}

		updatedChange, err := userRepo.FindProfileChangeByID(ctx, change.ID)
		if err != nil || updatedChange.Status != models.ProfileApproved {
			t.Errorf("expected status to be approved, got %v", updatedChange)
		}
	})

	t.Run("ReviewProfileChange rollback on second step error", func(t *testing.T) {
		u1 := &models.User{
			Username:     "tx_real_existing",
			Email:        "existing_email@example.com",
			Name:         "Existing User",
			Role:         models.RoleUser,
			PasswordHash: "hash123",
			IsActive:     true,
		}
		if err := userRepo.Create(ctx, u1); err != nil {
			t.Fatalf("failed to create user 1: %v", err)
		}
		defer db.Unscoped().Delete(&models.User{}, u1.ID)

		u2 := &models.User{
			Username:     "tx_real_u2",
			Email:        "tx_real_u2@example.com",
			Name:         "Original U2 Name",
			Role:         models.RoleUser,
			PasswordHash: "hash123",
			IsActive:     true,
		}
		if err := userRepo.Create(ctx, u2); err != nil {
			t.Fatalf("failed to create user 2: %v", err)
		}
		defer db.Unscoped().Delete(&models.User{}, u2.ID)

		// Request change to conflicting email
		change := &models.ProfileChangeRequest{
			UserID: u2.ID,
			Name:   "Attempted New Name",
			Email:  "existing_email@example.com",
			Status: models.ProfilePending,
		}
		if err := userRepo.CreateProfileChange(ctx, change); err != nil {
			t.Fatalf("failed to create profile change: %v", err)
		}
		defer db.Unscoped().Delete(&models.ProfileChangeRequest{}, change.ID)

		err := svc.ReviewProfileChange(ctx, change.ID, u1.ID, "approve")
		if !errors.Is(err, domain.ErrEmailConflict) {
			t.Fatalf("expected ErrEmailConflict, got %v", err)
		}

		// Verify database rollback: u2 unchanged, change remains pending
		foundU2, err := userRepo.FindByID(ctx, u2.ID)
		if err != nil || foundU2.Name != "Original U2 Name" {
			t.Errorf("expected u2 name to remain 'Original U2 Name' after rollback, got %v", foundU2)
		}

		foundChange, err := userRepo.FindProfileChangeByID(ctx, change.ID)
		if err != nil || foundChange.Status != models.ProfilePending {
			t.Errorf("expected change status to remain pending after rollback, got %v", foundChange)
		}
	})

	t.Run("ReviewProfileChange rollback on second table error after user table update", func(t *testing.T) {
		u := &models.User{
			Username:     "tx_real_step2",
			Email:        "tx_real_step2@example.com",
			Name:         "Original Step2 Name",
			Role:         models.RoleUser,
			PasswordHash: "hash123",
			IsActive:     true,
		}
		if err := userRepo.Create(ctx, u); err != nil {
			t.Fatalf("failed to create user: %v", err)
		}
		defer db.Unscoped().Delete(&models.User{}, u.ID)

		change := &models.ProfileChangeRequest{
			UserID: u.ID,
			Name:   "Updated Step2 Name",
			Status: models.ProfilePending,
		}
		if err := userRepo.CreateProfileChange(ctx, change); err != nil {
			t.Fatalf("failed to create profile change: %v", err)
		}
		defer db.Unscoped().Delete(&models.ProfileChangeRequest{}, change.ID)

		errSecondStep := errors.New("simulated error on updating profile_change_requests table")
		failingRepo := &failingProfileChangeRepo{
			ProfileChangeRepository: userRepo,
			err:                     errSecondStep,
		}
		svcWithFailingChangeRepo := service.NewUserService(userRepo, failingRepo, auth.DefaultHasher, nil, nil, transactor)

		err := svcWithFailingChangeRepo.ReviewProfileChange(ctx, change.ID, u.ID, "approve")
		if !errors.Is(err, errSecondStep) {
			t.Fatalf("expected errSecondStep, got %v", err)
		}

		// Verify database rollback: user was updated in tx, but rolled back after second step failed!
		foundU, err := userRepo.FindByID(ctx, u.ID)
		if err != nil || foundU.Name != "Original Step2 Name" {
			t.Errorf("expected user name to remain 'Original Step2 Name' after rollback, got %v", foundU)
		}

		foundChange, err := userRepo.FindProfileChangeByID(ctx, change.ID)
		if err != nil || foundChange.Status != models.ProfilePending {
			t.Errorf("expected profile change status to remain pending after rollback, got %v", foundChange)
		}
	})
}

type failingProfileChangeRepo struct {
	repository.ProfileChangeRepository
	err error
}

func (f *failingProfileChangeRepo) UpdateProfileChange(ctx context.Context, change *models.ProfileChangeRequest) error {
	return f.err
}
