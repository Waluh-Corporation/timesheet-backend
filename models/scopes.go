package models

import (
	"gorm.io/gorm"
)

// ActiveOnly is a reusable GORM query scope that filters records where is_active is true.
// It ensures that soft-deleted or inactive records are never returned in public or operational queries.
func ActiveOnly(db *gorm.DB) *gorm.DB {
	return db.Where("is_active = ?", true)
}

// FilterActiveTable returns a GORM query scope that qualifies the is_active column with a specific table name.
// This prevents ambiguous column errors during JOIN operations.
func FilterActiveTable(tableName string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(tableName+".is_active = ?", true)
	}
}
