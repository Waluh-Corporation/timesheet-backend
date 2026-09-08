package database

import (
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"timesheet-backend/auth"
	"timesheet-backend/config"
	"timesheet-backend/models"
)

// Connect opens the PostgreSQL connection, runs migrations, and seeds the
// bootstrap admin account.
func Connect(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return nil, err
	}

	if err := AutoMigrate(db); err != nil {
		return nil, err
	}

	if err := seedAdmin(db, cfg); err != nil {
		return nil, err
	}
	if err := seedDefaultTemplate(db); err != nil {
		// Non-fatal: the portal still runs, admins can upload a template.
		log.Printf("[database] could not seed default template: %v", err)
	}
	return db, nil
}

// seedDefaultTemplate installs the bundled company templates on first boot
// (when no template exists yet): MII, SDD, Adidata, and NTT.
func seedDefaultTemplate(db *gorm.DB) error {
	// Seed companies first
	companies := []models.Company{
		{Code: "mii", Name: "PT Mitra Integrasi Informatika"},
		{Code: "sdd", Name: "PT Swadharma Duta Data"},
		{Code: "adidata", Name: "PT Adidata Informatics"},
		{Code: "ntt", Name: "PT NTT Data Indonesia"},
	}
	for _, c := range companies {
		var cnt int64
		_ = db.Model(&models.Company{}).Where("code = ?", c.Code).Count(&cnt).Error
		if cnt == 0 {
			_ = db.Create(&c).Error
		}
	}

	var count int64
	if err := db.Model(&models.Template{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	// Helper to find company ID
	findCompanyID := func(code string) *uint {
		var comp models.Company
		if err := db.Where("code = ?", code).Limit(1).Find(&comp).Error; err == nil && comp.ID != 0 {
			return &comp.ID
		}
		return nil
	}

	// 1. MII timesheet — the default template.
	mii := models.Template{
		Name:        "MII Timesheet",
		Description: "Built-in MII timesheet template (pure programmatic builder with K1 header logo).",
		SheetName:   "Sheet1",
		Company:     "MII",
		CompanyID:   findCompanyID("mii"),
		IsDefault:   true,
		Builtin:     "mii",
	}
	_ = db.Create(&mii)
	log.Printf("[database] seeded default template '%s' (builtin mii)", mii.Name)

	// 2. SDD timesheet — pure programmatic builder.
	sdd := models.Template{
		Name:        "SDD Timesheet",
		Description: "Built-in SDD timesheet template (pure programmatic builder with A2 header logo).",
		SheetName:   "Juni",
		Company:     "SDD",
		CompanyID:   findCompanyID("sdd"),
		IsDefault:   false,
		Builtin:     "sdd",
	}
	_ = db.Create(&sdd)
	log.Printf("[database] seeded template '%s' (builtin sdd)", sdd.Name)

	// 3. Adidata timesheet — pure programmatic builder with SPL.
	adidata := models.Template{
		Name:        "Adidata Timesheet",
		Description: "Built-in Adidata timesheet template (pure programmatic builder with N2 header logo & SPL support).",
		SheetName:   "TIMESHEET",
		Company:     "Adidata",
		CompanyID:   findCompanyID("adidata"),
		IsDefault:   false,
		Builtin:     "adidata",
	}
	_ = db.Create(&adidata)
	log.Printf("[database] seeded template '%s' (builtin adidata)", adidata.Name)

	// 4. NTT timesheet — pure programmatic builder.
	ntt := models.Template{
		Name:        "NTT Timesheet",
		Description: "Built-in NTT timesheet template (pure programmatic builder with N2 header logo).",
		SheetName:   "Timesheet",
		Company:     "NTT",
		CompanyID:   findCompanyID("ntt"),
		IsDefault:   false,
		Builtin:     "ntt",
	}
	_ = db.Create(&ntt)
	log.Printf("[database] seeded template '%s' (builtin ntt)", ntt.Name)

	return nil
}

// AutoMigrate runs GORM migrations for every entity.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Company{},
		&models.User{},
		&models.WebAuthnCredential{},
		&models.Template{},
		&models.CellMapping{},
		&models.DailyActivity{},
		&models.OvertimeEntry{},
		&models.PushSubscription{},
		&models.ProfileChangeRequest{},
		&models.PasswordResetToken{},
	)
}

// seedAdmin creates the bootstrap admin the first time the portal boots with an
// empty users table. Public sign-up is disabled, so this is the only way an
// initial administrator can come into existence.
func seedAdmin(db *gorm.DB, cfg *config.Config) error {
	var count int64
	if err := db.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	// Never seed a known/guessable admin password. When BOOTSTRAP_ADMIN_PASSWORD
	// is not explicitly provided, generate a strong, NIST SP 800-63B-compliant
	// random one and print it once so an operator can capture it from the logs
	// and rotate it. When it IS provided, validate it against the same policy so
	// a weak default can never enter the system through the seeder.
	adminPassword := cfg.AdminPassword
	generated := false
	if adminPassword == "" {
		pw, err := auth.GeneratePassword(auth.GeneratedPasswordLength)
		if err != nil {
			return err
		}
		adminPassword = pw
		generated = true
	} else if err := auth.ValidatePassword(adminPassword, cfg.AdminUsername, cfg.AdminEmail); err != nil {
		log.Fatalf("[database] BOOTSTRAP_ADMIN_PASSWORD rejected by password policy: %v", err)
	}

	hash, err := auth.HashPassword(adminPassword)
	if err != nil {
		return err
	}
	admin := models.User{
		Username:     cfg.AdminUsername,
		Email:        cfg.AdminEmail,
		PasswordHash: string(hash),
		Role:         models.RoleAdmin,
		IsActive:     true,
		Name:         "Portal Administrator",
	}
	if err := db.Create(&admin).Error; err != nil {
		return err
	}
	if generated {
		log.Printf("[database] seeded bootstrap admin '%s' (%s) with a GENERATED password: %s", cfg.AdminUsername, cfg.AdminEmail, adminPassword)
		log.Printf("[database] ^ capture this now and change it after first login; it will not be shown again")
	} else {
		log.Printf("[database] seeded bootstrap admin '%s' (%s) using BOOTSTRAP_ADMIN_PASSWORD", cfg.AdminUsername, cfg.AdminEmail)
	}
	return nil
}
