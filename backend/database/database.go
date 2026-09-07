package database

import (
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"timesheet-backend/assets"
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

	if err := seedMasterData(db); err != nil {
		log.Printf("[database] could not seed master data: %v", err)
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

// seedDefaultTemplate installs the two bundled company templates on first boot
// (when no template exists yet): the MII timesheet as the default, and the
// BNI DEV timesheet as a second, non-default template. Each ships with a
// representative cell mapping so the admin builder reflects its structure;
// generation for both uses a dedicated strict-typed path keyed off
// Template.Builtin.
func seedDefaultTemplate(db *gorm.DB) error {
	// MII timesheet — the default template.
	if len(assets.MIITemplate) > 0 {
		var count int64
		_ = db.Model(&models.Template{}).Where("builtin = ?", "mii").Count(&count)
		if count == 0 {
			mii := models.Template{
				Name:        "MII Timesheet",
				Description: "Built-in default MII timesheet template (working hours, total-hour column, status matrix, and hardcoded project/division/department metadata).",
				Company:     "MII",
				SheetName:   "Sheet1",
				FileData:    assets.MIITemplate,
				IsDefault:   true,
				Builtin:     "mii",
				CellMappings: []models.CellMapping{
					{Field: models.FieldMetaDivision, Scope: models.ScopeCell, CellRef: "C2", Fillable: false},
					{Field: models.FieldMetaName, Scope: models.ScopeCell, CellRef: "C3", Fillable: false},
					{Field: models.FieldMetaMiiID, Scope: models.ScopeCell, CellRef: "C4", Fillable: false},
					{Field: models.FieldMetaSite, Scope: models.ScopeCell, CellRef: "C5", Fillable: false},
					{Field: models.FieldMetaMonth, Scope: models.ScopeCell, CellRef: "C6", Fillable: false},
					{Field: models.FieldDate, Scope: models.ScopeDailyColumn, Column: "A", StartRow: 9, Fillable: false},
					{Field: models.FieldTimeIn, Scope: models.ScopeDailyColumn, Column: "B", StartRow: 9, Fillable: true},
					{Field: models.FieldTimeOut, Scope: models.ScopeDailyColumn, Column: "C", StartRow: 9, Fillable: true},
					{Field: models.FieldTotalHour, Scope: models.ScopeDailyColumn, Column: "D", StartRow: 9, Fillable: false},
					{Field: models.FieldStatus, Scope: models.ScopeDailyColumn, Column: "E", StartRow: 9, Fillable: true},
					{Field: models.FieldActivity, Scope: models.ScopeDailyColumn, Column: "K", StartRow: 9, Fillable: true},
					{Field: models.FieldProjectName, Scope: models.ScopeDailyColumn, Column: "L", StartRow: 9, Fillable: false},
					{Field: models.FieldProjectID, Scope: models.ScopeDailyColumn, Column: "M", StartRow: 9, Fillable: false},
					{Field: models.FieldAppImpacted, Scope: models.ScopeDailyColumn, Column: "N", StartRow: 9, Fillable: true},
					{Field: models.FieldAIPFitur, Scope: models.ScopeDailyColumn, Column: "O", StartRow: 9, Fillable: false},
					{Field: models.FieldDivision, Scope: models.ScopeDailyColumn, Column: "P", StartRow: 9, Fillable: false},
					{Field: models.FieldDepartment, Scope: models.ScopeDailyColumn, Column: "Q", StartRow: 9, Fillable: false},
					{Field: models.FieldSubDept, Scope: models.ScopeDailyColumn, Column: "R", StartRow: 9, Fillable: false},
				},
			}
			if err := db.Create(&mii).Error; err != nil {
				return err
			}
			log.Printf("[database] seeded default template '%s' (builtin mii)", mii.Name)
		}
	}

	// SDD timesheet template.
	if len(assets.SDDTemplate) > 0 {
		var count int64
		_ = db.Model(&models.Template{}).Where("builtin = ?", "sdd").Count(&count)
		if count == 0 {
			sdd := models.Template{
				Name:        "SDD Timesheet",
				Description: "Built-in SDD manual attendance timesheet template (BNI NPP, Hadir/Cuti/Izin/Sakit/Lembur checklist, working hours).",
				Company:     "SDD",
				SheetName:   "Juni",
				FileData:    assets.SDDTemplate,
				IsDefault:   false,
				Builtin:     "sdd",
				CellMappings: []models.CellMapping{
					{Field: models.FieldMetaName, Scope: models.ScopeCell, CellRef: "G2", Fillable: false},
					{Field: models.FieldMetaMiiID, Scope: models.ScopeCell, CellRef: "G3", Fillable: false},
					{Field: models.FieldMetaDivision, Scope: models.ScopeCell, CellRef: "G4", Fillable: false},
					{Field: models.FieldMetaMonth, Scope: models.ScopeCell, CellRef: "G7", Fillable: false},
					{Field: models.FieldDate, Scope: models.ScopeDailyColumn, Column: "B", StartRow: 12, Fillable: false},
					{Field: models.FieldTimeIn, Scope: models.ScopeDailyColumn, Column: "C", StartRow: 12, Fillable: true},
					{Field: models.FieldTimeOut, Scope: models.ScopeDailyColumn, Column: "D", StartRow: 12, Fillable: true},
					{Field: models.FieldTotalHour, Scope: models.ScopeDailyColumn, Column: "E", StartRow: 12, Fillable: false},
					{Field: models.FieldStatus, Scope: models.ScopeDailyColumn, Column: "F", StartRow: 12, Fillable: true},
					{Field: models.FieldProjectName, Scope: models.ScopeDailyColumn, Column: "K", StartRow: 12, Fillable: true},
					{Field: models.FieldProjectID, Scope: models.ScopeDailyColumn, Column: "L", StartRow: 12, Fillable: true},
					{Field: models.FieldActivity, Scope: models.ScopeDailyColumn, Column: "N", StartRow: 12, Fillable: true},
				},
			}
			if err := db.Create(&sdd).Error; err != nil {
				return err
			}
			log.Printf("[database] seeded template '%s' (builtin sdd)", sdd.Name)
		}
	}

	// Adidata timesheet template.
	if len(assets.AdidataTemplate) > 0 {
		var count int64
		_ = db.Model(&models.Template{}).Where("builtin = ?", "adidata").Count(&count)
		if count == 0 {
			adidata := models.Template{
				Name:        "Adidata Timesheet",
				Description: "Built-in Adidata timesheet template (TIMESHEET and SPL overtime sheets).",
				Company:     "Adidata",
				SheetName:   "TIMESHEET",
				FileData:    assets.AdidataTemplate,
				IsDefault:   false,
				Builtin:     "adidata",
				CellMappings: []models.CellMapping{
					{Field: models.FieldMetaDivision, Scope: models.ScopeCell, CellRef: "C2", Fillable: false},
					{Field: models.FieldMetaName, Scope: models.ScopeCell, CellRef: "C3", Fillable: false},
					{Field: models.FieldMetaMiiID, Scope: models.ScopeCell, CellRef: "C4", Fillable: false},
					{Field: models.FieldMetaMonth, Scope: models.ScopeCell, CellRef: "C5", Fillable: false},
					{Field: models.FieldDate, Scope: models.ScopeDailyColumn, Column: "A", StartRow: 9, Fillable: false},
					{Field: models.FieldTimeIn, Scope: models.ScopeDailyColumn, Column: "B", StartRow: 9, Fillable: true},
					{Field: models.FieldTimeOut, Scope: models.ScopeDailyColumn, Column: "C", StartRow: 9, Fillable: true},
					{Field: models.FieldTotalHour, Scope: models.ScopeDailyColumn, Column: "D", StartRow: 9, Fillable: false},
					{Field: models.FieldStatus, Scope: models.ScopeDailyColumn, Column: "E", StartRow: 9, Fillable: true},
					{Field: models.FieldActivity, Scope: models.ScopeDailyColumn, Column: "K", StartRow: 9, Fillable: true},
					{Field: models.FieldProjectName, Scope: models.ScopeDailyColumn, Column: "L", StartRow: 9, Fillable: true},
					{Field: models.FieldProjectID, Scope: models.ScopeDailyColumn, Column: "M", StartRow: 9, Fillable: true},
					{Field: models.FieldAppImpacted, Scope: models.ScopeDailyColumn, Column: "N", StartRow: 9, Fillable: true},
				},
			}
			if err := db.Create(&adidata).Error; err != nil {
				return err
			}
			log.Printf("[database] seeded template '%s' (builtin adidata)", adidata.Name)
		}
	}

	// BNI DEV timesheet — existing fallback.
	if len(assets.BNITemplate) > 0 {
		var count int64
		_ = db.Model(&models.Template{}).Where("builtin = ?", "bni_dev").Count(&count)
		if count == 0 {
			bni := models.Template{
				Name:        "BNI DEV Timesheet",
				Description: "Built-in BNI DEV timesheet template (strict date/time typing + status matrix).",
				SheetName:   "Sheet1",
				FileData:    assets.BNITemplate,
				IsDefault:   false,
				Builtin:     "bni_dev",
				CellMappings: []models.CellMapping{
					{Field: models.FieldMetaDivision, Scope: models.ScopeCell, CellRef: "C2", Fillable: false},
					{Field: models.FieldMetaName, Scope: models.ScopeCell, CellRef: "C3", Fillable: false},
					{Field: models.FieldMetaMiiID, Scope: models.ScopeCell, CellRef: "C4", Fillable: false},
					{Field: models.FieldMetaSite, Scope: models.ScopeCell, CellRef: "C5", Fillable: false},
					{Field: models.FieldMetaMonth, Scope: models.ScopeCell, CellRef: "C6", Fillable: false},
					{Field: models.FieldDate, Scope: models.ScopeDailyColumn, Column: "A", StartRow: 9, Fillable: false},
					{Field: models.FieldTimeIn, Scope: models.ScopeDailyColumn, Column: "B", StartRow: 9, Fillable: true},
					{Field: models.FieldTimeOut, Scope: models.ScopeDailyColumn, Column: "C", StartRow: 9, Fillable: true},
					{Field: models.FieldStatus, Scope: models.ScopeDailyColumn, Column: "E", StartRow: 9, Fillable: true},
					{Field: models.FieldActivity, Scope: models.ScopeDailyColumn, Column: "K", StartRow: 9, Fillable: true},
					{Field: models.FieldProjectName, Scope: models.ScopeDailyColumn, Column: "L", StartRow: 9, Fillable: true},
					{Field: models.FieldProjectID, Scope: models.ScopeDailyColumn, Column: "M", StartRow: 9, Fillable: true},
					{Field: models.FieldAppImpacted, Scope: models.ScopeDailyColumn, Column: "N", StartRow: 9, Fillable: true},
				},
			}
			if err := db.Create(&bni).Error; err != nil {
				return err
			}
			log.Printf("[database] seeded template '%s' (builtin bni_dev)", bni.Name)
		}
	}
	return nil
}

// AutoMigrate runs GORM migrations for every entity.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Company{},
		&models.Division{},
		&models.Site{},
		&models.User{},
		&models.WebAuthnCredential{},
		&models.Template{},
		&models.CellMapping{},
		&models.DailyActivity{},
		&models.PushSubscription{},
		&models.ProfileChangeRequest{},
		&models.PasswordResetToken{},
	)
}

// seedMasterData populates standard companies, divisions, and sites if no company exists.
// It also backfills existing users with matching company_id, division_id, and site_id.
func seedMasterData(db *gorm.DB) error {
	var count int64
	if err := db.Model(&models.Company{}).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		initialCompanies := []models.Company{
			{
				Code:     "MII",
				Name:     "PT Mitra Integrasi Informatika",
				IsActive: true,
				Divisions: []models.Division{
					{Code: "ADD", Name: "Application Development Division", IsActive: true},
					{Code: "WDD", Name: "Wholesale Digital Delivery", IsActive: true},
					{Code: "WCSD", Name: "Wholesale Channel and Service Delivery", IsActive: true},
				},
				Sites: []models.Site{
					{Name: "Jakarta", IsActive: true},
					{Name: "Client On-Site", IsActive: true},
				},
			},
			{
				Code:     "SDD",
				Name:     "PT Sinergi Digital Digital",
				IsActive: true,
				Divisions: []models.Division{
					{Code: "DEV", Name: "Software Development", IsActive: true},
				},
				Sites: []models.Site{
					{Name: "Jakarta", IsActive: true},
				},
			},
			{
				Code:     "NTT",
				Name:     "PT NTT Data Indonesia",
				IsActive: true,
				Divisions: []models.Division{
					{Code: "ENG", Name: "Engineering Division", IsActive: true},
				},
				Sites: []models.Site{
					{Name: "Jakarta", IsActive: true},
				},
			},
			{
				Code:     "Adidata",
				Name:     "PT Adidata Informatika",
				IsActive: true,
				Divisions: []models.Division{
					{Code: "CONS", Name: "Consulting & Delivery", IsActive: true},
				},
				Sites: []models.Site{
					{Name: "Jakarta", IsActive: true},
				},
			},
		}

		for _, comp := range initialCompanies {
			if err := db.Create(&comp).Error; err != nil {
				log.Printf("[database] warning: failed to seed company %s: %v", comp.Code, err)
			}
		}
		log.Printf("[database] seeded default master companies, divisions, and sites")
	}

	// Backfill existing users if they have string company/division/site but no foreign key set.
	var usersWithoutMaster []models.User
	if err := db.Where("company_id IS NULL AND company != ''").Find(&usersWithoutMaster).Error; err == nil {
		for _, u := range usersWithoutMaster {
			var comp models.Company
			if err := db.Where("LOWER(code) = LOWER(?) OR LOWER(name) LIKE LOWER(?)", u.Company, "%"+u.Company+"%").First(&comp).Error; err == nil {
				updates := map[string]interface{}{"company_id": comp.ID}
				if u.Division != "" {
					var div models.Division
					if err := db.Where("company_id = ? AND (LOWER(name) = LOWER(?) OR LOWER(code) = LOWER(?))", comp.ID, u.Division, u.Division).First(&div).Error; err == nil {
						updates["division_id"] = div.ID
					}
				}
				if u.Site != "" {
					var site models.Site
					if err := db.Where("company_id = ? AND LOWER(name) = LOWER(?)", comp.ID, u.Site).First(&site).Error; err == nil {
						updates["site_id"] = site.ID
					}
				}
				_ = db.Model(&models.User{}).Where("id = ?", u.ID).Updates(updates)
			}
		}
	}

	return nil
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
