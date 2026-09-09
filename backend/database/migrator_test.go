package database

import (
	"testing"
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
	if len(entries) < 8 {
		t.Errorf("expected at least 8 migration files (4 up, 4 down), got %d", len(entries))
	}
}
