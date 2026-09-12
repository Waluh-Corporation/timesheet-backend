package database

import (
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"timesheet-backend/config"
)

func TestParseMigrationFileName(t *testing.T) {
	tests := []struct {
		filename    string
		wantVersion int64
		wantName    string
		wantDir     string
		wantErr     bool
	}{
		{
			filename:    "000001_create_initial_schema.up.sql",
			wantVersion: 1,
			wantName:    "create_initial_schema",
			wantDir:     "up",
			wantErr:     false,
		},
		{
			filename:    "migrations/000003_normalize_departments_statuses_holidays.down.sql",
			wantVersion: 3,
			wantName:    "normalize_departments_statuses_holidays",
			wantDir:     "down",
			wantErr:     false,
		},
		{
			filename:    "migrations/000004_enforce_referential_integrity_and_indexes.up.sql",
			wantVersion: 4,
			wantName:    "enforce_referential_integrity_and_indexes",
			wantDir:     "up",
			wantErr:     false,
		},
		{
			filename: "invalid_name.sql",
			wantErr:  true,
		},
		{
			filename: "000001_initial.foo.sql",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		got, err := parseMigrationFileName(tt.filename)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseMigrationFileName(%q) error = %v, wantErr %v", tt.filename, err, tt.wantErr)
			continue
		}
		if !tt.wantErr {
			if got.Version != tt.wantVersion || got.Name != tt.wantName || got.Direction != tt.wantDir {
				t.Errorf("parseMigrationFileName(%q) = %+v, want version=%d name=%s dir=%s",
					tt.filename, got, tt.wantVersion, tt.wantName, tt.wantDir)
			}
		}
	}
}

func TestEmbeddedMigrationsAvailable(t *testing.T) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		t.Fatalf("failed to read embedded migrations dir: %v", err)
	}
	if len(entries) < 24 {
		t.Errorf("expected at least 24 migration files (12 up, 12 down), got %d", len(entries))
	}
}

func TestRunMigrationsOnDB(t *testing.T) {
	cfg := config.Load()
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		t.Skipf("cannot connect to db: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}
	var records []MigrationRecord
	if err := db.Table("schema_migrations").Order("version asc").Find(&records).Error; err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	for _, r := range records {
		t.Logf("Migration: version=%d name=%s applied_at=%v", r.Version, r.Name, r.AppliedAt)
	}

	// 1. Verify dropped tables do not exist
	for _, tbl := range []string{"templates", "cell_mappings"} {
		var exists bool
		_ = db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = ?)`, tbl).Scan(&exists)
		if exists {
			t.Errorf("table %s should have been dropped by migration 000006", tbl)
		}
	}

	// 2. Verify password_reset_tokens columns
	var hasUsed, hasUsedAt, hasTokenType bool
	_ = db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'password_reset_tokens' AND column_name = 'used')`).Scan(&hasUsed)
	_ = db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'password_reset_tokens' AND column_name = 'used_at')`).Scan(&hasUsedAt)
	_ = db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'password_reset_tokens' AND column_name = 'token_type')`).Scan(&hasTokenType)
	if hasUsed {
		t.Errorf("password_reset_tokens.used should have been removed")
	}
	if !hasUsedAt {
		t.Errorf("password_reset_tokens.used_at should exist")
	}
	if !hasTokenType {
		t.Errorf("password_reset_tokens.token_type should exist")
	}

	// 3. Verify overtime_entries normalized columns
	var hasTL, hasTLID bool
	_ = db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'overtime_entries' AND column_name = 'team_leader')`).Scan(&hasTL)
	_ = db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'overtime_entries' AND column_name = 'team_leader_id')`).Scan(&hasTLID)
	if hasTL {
		t.Errorf("overtime_entries.team_leader string column should have been removed")
	}
	if !hasTLID {
		t.Errorf("overtime_entries.team_leader_id FK column should exist")
	}

	// 4. Verify approvers table exists after migration 000008 and company_id is removed after 000011
	var hasApproversTable, hasApproverRoleType, hasApproverCompanyID bool
	_ = db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = ?)`, "approvers").Scan(&hasApproversTable)
	if !hasApproversTable {
		t.Errorf("table approvers should exist after migration 000008")
	}
	_ = db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'approvers' AND column_name = 'role_type')`).Scan(&hasApproverRoleType)
	if !hasApproverRoleType {
		t.Errorf("approvers.role_type column should exist")
	}
	_ = db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'approvers' AND column_name = 'company_id')`).Scan(&hasApproverCompanyID)
	if hasApproverCompanyID {
		t.Errorf("approvers.company_id column should have been removed by migration 000011")
	}

	// 5. Verify daily_activities user foreign key constraint after migration 000009
	var hasFKUser bool
	_ = db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_constraint 
			WHERE conrelid = 'daily_activities'::regclass 
			  AND (conname = 'fk_daily_activities_user' OR pg_get_constraintdef(oid) LIKE '%FOREIGN KEY (user_id) REFERENCES users(id)%')
		)
	`).Scan(&hasFKUser)
	if !hasFKUser {
		t.Errorf("expected foreign key from daily_activities(user_id) to users(id) to exist")
	}

	// 6. Verify profile_change_requests has exactly 1 foreign key constraint referencing companies(id)
	var pcrCompanyFKCount int64
	_ = db.Raw(`
		SELECT count(*)
		FROM information_schema.table_constraints tc
		JOIN pg_constraint c ON c.conname = tc.constraint_name
		WHERE tc.table_name = 'profile_change_requests'
		  AND tc.constraint_type = 'FOREIGN KEY'
		  AND pg_get_constraintdef(c.oid) LIKE '%FOREIGN KEY (company_id) REFERENCES companies(id)%'
	`).Scan(&pcrCompanyFKCount)
	if pcrCompanyFKCount != 1 {
		t.Errorf("expected exactly 1 foreign key from profile_change_requests(company_id) to companies(id), got %d", pcrCompanyFKCount)
	}

	// 7. Verify users table has bni_id and mii_id is dropped/renamed
	var hasBniID, hasMiiID bool
	_ = db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'bni_id')`).Scan(&hasBniID)
	_ = db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'mii_id')`).Scan(&hasMiiID)
	if !hasBniID {
		t.Errorf("users.bni_id should exist after migration 000015")
	}
	if hasMiiID {
		t.Errorf("users.mii_id should not exist after migration 000015")
	}

	// 8. Verify users.bni_id comment
	var bniIDComment string
	_ = db.Raw(`
		SELECT col_description('users'::regclass, ordinal_position)
		FROM information_schema.columns
		WHERE table_name = 'users' AND column_name = 'bni_id'
	`).Scan(&bniIDComment)
	if bniIDComment != "NPP BNI" {
		t.Errorf("expected users.bni_id comment to be 'NPP BNI', got %q", bniIDComment)
	}

	// 9. Verify projects.company_id is removed
	var hasProjectCompanyID bool
	_ = db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'projects' AND column_name = 'company_id')`).Scan(&hasProjectCompanyID)
	if hasProjectCompanyID {
		t.Errorf("projects.company_id should be removed after migration 000015")
	}

	// 10. Verify departments.company_id foreign key constraint is dropped after migration 000019
	var hasDeptCompanyFK bool
	_ = db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.table_constraints tc
			JOIN pg_constraint c ON c.conname = tc.constraint_name
			WHERE tc.table_name = 'departments'
			  AND tc.constraint_type = 'FOREIGN KEY'
			  AND pg_get_constraintdef(c.oid) LIKE '%FOREIGN KEY (company_id) REFERENCES companies(id)%'
		)
	`).Scan(&hasDeptCompanyFK)
	if hasDeptCompanyFK {
		t.Errorf("departments.company_id foreign key constraint referencing companies(id) should have been dropped by migration 000019")
	}

	// 11. Verify chk_users_admin_no_company constraint exists after migration 000019
	var hasAdminNoCompanyCheck bool
	_ = db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_constraint
			WHERE conname = 'chk_users_admin_no_company'
		)
	`).Scan(&hasAdminNoCompanyCheck)
	if !hasAdminNoCompanyCheck {
		t.Errorf("chk_users_admin_no_company constraint should exist on users table after migration 000019")
	}
}
