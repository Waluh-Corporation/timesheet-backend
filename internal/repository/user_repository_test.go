package repository_test

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"timesheet-backend/config"
	"timesheet-backend/internal/repository"
	"timesheet-backend/models"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	cfg := config.Load()
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		t.Skipf("cannot connect to postgres db: %v", err)
	}
	return db
}

func TestUserRepository(t *testing.T) {
	db := setupTestDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	repo := repository.NewUserRepository(tx)
	ctx := context.Background()

	// 1. Create user
	user := &models.User{
		Username:     "repo_test_user_unique",
		Email:        "repo_test_unique@example.com",
		Name:         "Repo Test User",
		Role:         models.RoleUser,
		PasswordHash: "dummyhash123",
		IsActive:     true,
	}

	err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("repo.Create failed: %v", err)
	}
	if user.ID == 0 {
		t.Fatal("expected user ID to be assigned, got 0")
	}

	// 2. FindByID - Found
	found, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("repo.FindByID failed: %v", err)
	}
	if found.Username != user.Username {
		t.Fatalf("expected username %q, got %q", user.Username, found.Username)
	}
	if found.Email != user.Email {
		t.Fatalf("expected email %q, got %q", user.Email, found.Email)
	}

	// 3. FindByID - Not Found
	notFound, err := repo.FindByID(ctx, 99999999)
	if err == nil || notFound != nil {
		t.Fatalf("expected error and nil user for non-existent ID, got user=%v, err=%v", notFound, err)
	}

	// 4. FindByUsernameOrEmail - by Username
	byUser, err := repo.FindByUsernameOrEmail(ctx, "repo_test_user_unique")
	if err != nil {
		t.Fatalf("repo.FindByUsernameOrEmail(username) failed: %v", err)
	}
	if byUser.ID != user.ID {
		t.Fatalf("expected user ID %d, got %d", user.ID, byUser.ID)
	}

	// 5. FindByUsernameOrEmail - by Email
	byEmail, err := repo.FindByUsernameOrEmail(ctx, "repo_test_unique@example.com")
	if err != nil {
		t.Fatalf("repo.FindByUsernameOrEmail(email) failed: %v", err)
	}
	if byEmail.ID != user.ID {
		t.Fatalf("expected user ID %d, got %d", user.ID, byEmail.ID)
	}

	// 6. FindByUsernameOrEmail - Not Found
	notByIdent, err := repo.FindByUsernameOrEmail(ctx, "nonexistent_ident@example.com")
	if err == nil || notByIdent != nil {
		t.Fatalf("expected error and nil user for non-existent identifier, got user=%v, err=%v", notByIdent, err)
	}

	// 7. UpdatePassword
	newHash := "updatedhash123"
	updatedAt := time.Now().Truncate(time.Second)
	err = repo.UpdatePassword(ctx, user.ID, newHash, updatedAt)
	if err != nil {
		t.Fatalf("repo.UpdatePassword failed: %v", err)
	}

	// Verify updated password
	refreshed, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("repo.FindByID failed after password update: %v", err)
	}
	if refreshed.PasswordHash != newHash {
		t.Fatalf("expected password hash %q, got %q", newHash, refreshed.PasswordHash)
	}

	// 8. FindByIDWithDetails
	withDetails, err := repo.FindByIDWithDetails(ctx, user.ID)
	if err != nil {
		t.Fatalf("repo.FindByIDWithDetails failed: %v", err)
	}
	if withDetails == nil || withDetails.ID != user.ID {
		t.Fatalf("expected user with ID %d, got %v", user.ID, withDetails)
	}

	// 9. FindByIDWithCredentials
	withCreds, err := repo.FindByIDWithCredentials(ctx, user.ID)
	if err != nil {
		t.Fatalf("repo.FindByIDWithCredentials failed: %v", err)
	}
	if withCreds == nil || withCreds.ID != user.ID {
		t.Fatalf("expected user with ID %d, got %v", user.ID, withCreds)
	}

	// 10. FindByUsernameOrEmailWithCredentials
	byCreds, err := repo.FindByUsernameOrEmailWithCredentials(ctx, user.Username)
	if err != nil {
		t.Fatalf("repo.FindByUsernameOrEmailWithCredentials failed: %v", err)
	}
	if byCreds.ID != user.ID {
		t.Fatalf("expected user ID %d, got %d", user.ID, byCreds.ID)
	}

	// 11. FindByEmail
	byEmailFound, err := repo.FindByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("repo.FindByEmail failed: %v", err)
	}
	if byEmailFound.ID != user.ID {
		t.Fatalf("expected user ID %d, got %d", user.ID, byEmailFound.ID)
	}

	// 12. Update
	user.Name = "Updated Name"
	if err := repo.Update(ctx, user); err != nil {
		t.Fatalf("repo.Update failed: %v", err)
	}

	// 13. Passkey Operations
	cred := &models.WebAuthnCredential{
		UserID:          user.ID,
		CredentialID:    []byte("test-credential-id-123"),
		PublicKey:       []byte("test-public-key-bytes"),
		AttestationType: "none",
		AAGUID:          []byte("00000000-0000-0000-0000-000000000000"),
		SignCount:       0,
		FriendlyName:    "My Security Key",
	}
	if err := repo.CreatePasskeyCredential(ctx, cred); err != nil {
		t.Fatalf("repo.CreatePasskeyCredential failed: %v", err)
	}

	// UpdatePasskeySignCount
	if err := repo.UpdatePasskeySignCount(ctx, cred.CredentialID, 5, true); err != nil {
		t.Fatalf("repo.UpdatePasskeySignCount failed: %v", err)
	}

	// ListPasskeysByUserID
	keys, err := repo.ListPasskeysByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("repo.ListPasskeysByUserID failed: %v", err)
	}
	if len(keys) != 1 || keys[0].SignCount != 5 {
		t.Fatalf("expected 1 passkey with sign count 5, got %v", keys)
	}

	// Test ListPasskeysByUserID with Authenticator preloaded & in-memory fallback
	authAAGUID := "fa264024-4a24-4e2b-a489-3224b1263d95" // gitleaks:allow
	_ = tx.Create(&models.AuthenticatorAAGUID{
		AAGUID: authAAGUID,
		Name:   "Preloaded Authenticator",
		Icon:   "data:image/svg+xml;base64,bGlnaHQ=",
	}).Error

	credPreload := &models.WebAuthnCredential{
		UserID:              user.ID,
		CredentialID:        []byte("test-credential-preload"),
		PublicKey:           []byte("test-public-key-bytes"),
		AttestationType:     "none",
		AAGUID:              []byte{0xfa, 0x26, 0x40, 0x24, 0x4a, 0x24, 0x4e, 0x2b, 0xa4, 0x89, 0x32, 0x24, 0xb1, 0x26, 0x3d, 0x95},
		AuthenticatorAAGUID: &authAAGUID,
		FriendlyName:        "Preloaded Key",
	}
	_ = repo.CreatePasskeyCredential(ctx, credPreload)

	inMemAAGUID := "fa264024-4a24-4e2b-a489-3224b1263d96" // gitleaks:allow
	models.RegisterAuthenticator(inMemAAGUID, "InMem Authenticator", "icon_mem")
	credInMem := &models.WebAuthnCredential{
		UserID:          user.ID,
		CredentialID:    []byte("test-credential-inmem"),
		PublicKey:       []byte("test-public-key-bytes"),
		AttestationType: "none",
		AAGUID:          []byte{0xfa, 0x26, 0x40, 0x24, 0x4a, 0x24, 0x4e, 0x2b, 0xa4, 0x89, 0x32, 0x24, 0xb1, 0x26, 0x3d, 0x96},
		FriendlyName:    "InMem Key",
	}
	_ = repo.CreatePasskeyCredential(ctx, credInMem)

	keysWithIcons, err := repo.ListPasskeysByUserID(ctx, user.ID)
	if err != nil || len(keysWithIcons) < 3 {
		t.Fatalf("expected at least 3 passkeys, got %d (err: %v)", len(keysWithIcons), err)
	}

	// UpdatePasskeyName tests
	updated, err := repo.UpdatePasskeyName(ctx, cred.ID, &user.ID, "Renamed Key")
	if err != nil || !updated {
		t.Fatalf("repo.UpdatePasskeyName failed: updated=%v, err=%v", updated, err)
	}

	wrongUID := user.ID + 999
	updatedWrong, err := repo.UpdatePasskeyName(ctx, cred.ID, &wrongUID, "Should Not Work")
	if err != nil || updatedWrong {
		t.Fatalf("expected update to fail for wrong user ID: updated=%v, err=%v", updatedWrong, err)
	}

	// DeletePasskey with matching userID
	deleted, err := repo.DeletePasskey(ctx, cred.ID, &user.ID)
	if err != nil || !deleted {
		t.Fatalf("repo.DeletePasskey failed: deleted=%v, err=%v", deleted, err)
	}

	// Re-create and delete without userID (admin delete)
	cred2 := &models.WebAuthnCredential{
		UserID:          user.ID,
		CredentialID:    []byte("test-credential-id-456"),
		PublicKey:       []byte("test-public-key-bytes-2"),
		AttestationType: "none",
		AAGUID:          []byte("00000000-0000-0000-0000-000000000000"),
		SignCount:       1,
		FriendlyName:    "Admin Deleted Key",
	}
	_ = repo.CreatePasskeyCredential(ctx, cred2)

	// Admin update (nil userID)
	updatedAdmin, err := repo.UpdatePasskeyName(ctx, cred2.ID, nil, "Admin Renamed Key")
	if err != nil || !updatedAdmin {
		t.Fatalf("repo.UpdatePasskeyName(admin) failed: updated=%v, err=%v", updatedAdmin, err)
	}

	deleted2, err := repo.DeletePasskey(ctx, cred2.ID, nil)
	if err != nil || !deleted2 {
		t.Fatalf("repo.DeletePasskey(admin) failed: deleted=%v, err=%v", deleted2, err)
	}

	// Test FindByEmailExcludingUser
	otherUser, err := repo.FindByEmailExcludingUser(ctx, user.Email, user.ID)
	if err == nil || otherUser != nil {
		t.Fatalf("expected not found when excluding self: %v", err)
	}
	foundOther, err := repo.FindByEmailExcludingUser(ctx, user.Email, 999999)
	if err != nil || foundOther == nil || foundOther.ID != user.ID {
		t.Fatalf("expected to find user when excluding different ID: %v", err)
	}

	// Test ListUsers
	allUsers, err := repo.ListUsers(ctx, nil)
	if err != nil || len(allUsers) == 0 {
		t.Fatalf("repo.ListUsers(nil) failed: %v", err)
	}
	activeFlag := true
	activeUsers, err := repo.ListUsers(ctx, &activeFlag)
	if err != nil || len(activeUsers) == 0 {
		t.Fatalf("repo.ListUsers(active) failed: %v", err)
	}

	// Test ProfileChange CRUD
	pChange := &models.ProfileChangeRequest{
		UserID: user.ID,
		Status: models.ProfilePending,
		Name:   "Pending New Name",
		Notes:  "Test notes",
	}
	if err := repo.CreateProfileChange(ctx, pChange); err != nil {
		t.Fatalf("repo.CreateProfileChange failed: %v", err)
	}
	if pChange.ID == 0 {
		t.Fatal("expected assigned ID for profile change")
	}

	foundChange, err := repo.FindProfileChangeByID(ctx, pChange.ID)
	if err != nil || foundChange == nil {
		t.Fatalf("repo.FindProfileChangeByID failed: %v", err)
	}
	if foundChange.Name != "Pending New Name" {
		t.Fatalf("expected name %s, got %s", "Pending New Name", foundChange.Name)
	}

	// ListProfileChanges with user and status filters
	userChanges, err := repo.ListProfileChanges(ctx, &user.ID, string(models.ProfilePending))
	if err != nil || len(userChanges) == 0 {
		t.Fatalf("repo.ListProfileChanges with filters failed: %v", err)
	}

	allPendingChanges, err := repo.ListProfileChanges(ctx, nil, string(models.ProfilePending))
	if err != nil || len(allPendingChanges) == 0 {
		t.Fatalf("repo.ListProfileChanges status only failed: %v", err)
	}

	// UpdateProfileChange
	pChange.Status = models.ProfileApproved
	if err := repo.UpdateProfileChange(ctx, pChange); err != nil {
		t.Fatalf("repo.UpdateProfileChange failed: %v", err)
	}
}
