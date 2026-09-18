package database

import (
	"testing"
	"time"

	"timesheet-backend/config"
)

func TestConnect_SuccessAndPooling(t *testing.T) {
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		t.Skip("DatabaseURL not configured")
	}

	// 1. Test with custom connection pool config
	customCfg := *cfg
	customCfg.DBMaxIdleConns = 10
	customCfg.DBMaxOpenConns = 50
	customCfg.DBConnMaxLifetime = 30 * time.Minute
	customCfg.DBConnMaxIdleTime = 10 * time.Minute

	db, err := Connect(&customCfg)
	if err != nil {
		t.Skipf("cannot connect to test postgres DB: %v", err)
	}
	if db == nil {
		t.Fatal("expected non-nil db instance")
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	// 2. Test with zero/negative pool config (triggers fallback defaults)
	zeroCfg := *cfg
	zeroCfg.DBMaxIdleConns = 0
	zeroCfg.DBMaxOpenConns = 0
	zeroCfg.DBConnMaxLifetime = 0
	zeroCfg.DBConnMaxIdleTime = 0

	db2, err := Connect(&zeroCfg)
	if err != nil {
		t.Fatalf("Connect with zero pool config failed: %v", err)
	}
	sqlDB2, _ := db2.DB()
	if sqlDB2 != nil {
		_ = sqlDB2.Close()
	}
}

func TestConnect_InvalidURL(t *testing.T) {
	badCfg := &config.Config{
		DatabaseURL: "host=invalid-host-that-does-not-exist port=5432 user=bad password=bad dbname=bad sslmode=disable connect_timeout=1",
	}
	db, err := Connect(badCfg)
	if err == nil {
		if db != nil {
			s, _ := db.DB()
			if s != nil {
				_ = s.Close()
			}
		}
	}
}
