package database

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// MigrationRecord tracks an applied migration in schema_migrations.
type MigrationRecord struct {
	Version   int64     `gorm:"primaryKey" json:"version"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	AppliedAt time.Time `gorm:"not null" json:"applied_at"`
}

// migrationFile models a parsed migration script file.
type migrationFile struct {
	Version   int64
	Name      string
	Direction string // "up" or "down"
	FileName  string
}

// parseMigrationFileName extracts version, descriptive name, and direction from a filename.
// Expected format: <version>_<name>.<direction>.sql, e.g. 000001_create_initial_schema.up.sql.
func parseMigrationFileName(filename string) (*migrationFile, error) {
	base := filepath.Base(filename)
	parts := strings.Split(base, ".")
	if len(parts) != 3 || parts[2] != "sql" {
		return nil, fmt.Errorf("invalid migration filename: %s", filename)
	}

	direction := parts[1]
	if direction != "up" && direction != "down" {
		return nil, fmt.Errorf("invalid migration direction: %s", direction)
	}

	nameParts := strings.SplitN(parts[0], "_", 2)
	if len(nameParts) != 2 {
		return nil, fmt.Errorf("invalid migration version prefix: %s", parts[0])
	}

	version, err := strconv.ParseInt(nameParts[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid migration version number: %w", err)
	}

	return &migrationFile{
		Version:   version,
		Name:      nameParts[1],
		Direction: direction,
		FileName:  filename,
	}, nil
}

// RunMigrations discovers embedded up-migrations, applies any that have not
// yet run in numerical order within a database transaction, and records them
// in the schema_migrations table.
func RunMigrations(db *gorm.DB) error {
	// 1. Ensure schema_migrations exists
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	// 2. Query already applied migration versions
	var applied []int64
	if err := db.Table("schema_migrations").Pluck("version", &applied).Error; err != nil {
		return fmt.Errorf("failed to query applied migrations: %w", err)
	}
	appliedMap := make(map[int64]bool, len(applied))
	for _, v := range applied {
		appliedMap[v] = true
	}

	// 3. Scan embedded migration files
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to read embedded migrations dir: %w", err)
	}

	var upMigrations []*migrationFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		mf, err := parseMigrationFileName("migrations/" + entry.Name())
		if err != nil {
			continue
		}
		if mf.Direction == "up" {
			upMigrations = append(upMigrations, mf)
		}
	}

	// Sort up-migrations by version ascending
	sort.Slice(upMigrations, func(i, j int) bool {
		return upMigrations[i].Version < upMigrations[j].Version
	})

	// 4. Apply unapplied migrations sequentially
	for _, m := range upMigrations {
		if appliedMap[m.Version] {
			continue
		}

		content, err := migrationFS.ReadFile(m.FileName)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", m.FileName, err)
		}

		log.Printf("[migrator] applying migration %06d_%s...", m.Version, m.Name)
		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(string(content)).Error; err != nil {
				return fmt.Errorf("error executing %s: %w", m.FileName, err)
			}
			record := MigrationRecord{
				Version:   m.Version,
				Name:      m.Name,
				AppliedAt: time.Now(),
			}
			if err := tx.Table("schema_migrations").Create(&record).Error; err != nil {
				return fmt.Errorf("error recording migration %d: %w", m.Version, err)
			}
			return nil
		})
		if err != nil {
			return err
		}
		log.Printf("[migrator] applied migration %06d_%s successfully", m.Version, m.Name)
	}

	return nil
}
