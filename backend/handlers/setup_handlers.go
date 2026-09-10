package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"timesheet-backend/auth"
	"timesheet-backend/database"
	"timesheet-backend/models"
)

// SetupStatusResponse describes the system initialization state.
type SetupStatusResponse struct {
	IsNew         string `json:"is_new" example:"Y"`
	IsInitialized bool   `json:"is_initialized"`
	RequiresSetup bool   `json:"requires_setup"`
	AdminCount    int64  `json:"admin_count"`
}

// InitSetupAdminRequest carries administrator account details for setup.
type InitSetupAdminRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64" example:"admin"`
	Email    string `json:"email" binding:"required,email" example:"admin@example.com"`
	Name     string `json:"name" binding:"required" example:"Super Administrator"`
	Password string `json:"password" binding:"required,min=8" example:"SuperSecretPass2026!"`
}

// InitSetupCompanyRequest carries initial company details.
type InitSetupCompanyRequest struct {
	Code string `json:"code" binding:"required" example:"mii"`
	Name string `json:"name" binding:"required" example:"PT Mitra Integrasi Informatika"`
}

// InitSetupApproverRequest carries initial approver supervisor details.
type InitSetupApproverRequest struct {
	Name        string                  `json:"name" binding:"required" example:"Supervisor Name"`
	RoleType    models.ApproverRoleType `json:"role_type" binding:"required,oneof=team_leader department_head" example:"team_leader"`
	Title       string                  `json:"title" example:"Team Leader"`
	CompanyCode string                  `json:"company_code" example:"mii"`
}

// InitSetupDepartmentRequest carries optional initial department details.
type InitSetupDepartmentRequest struct {
	Code        string `json:"code" binding:"required" example:"WCSD"`
	Name        string `json:"name" binding:"required" example:"Wholesale Channel and Service Delivery"`
	Division    string `json:"division" example:"Wholesale Digital Delivery"`
	CompanyCode string `json:"company_code" example:"mii"`
}

// InitSetupRequest is the full payload for the one-time system initialization wizard.
type InitSetupRequest struct {
	Admin       InitSetupAdminRequest        `json:"admin" binding:"required"`
	Companies   []InitSetupCompanyRequest    `json:"companies"`
	Approvers   []InitSetupApproverRequest   `json:"approvers"`
	Departments []InitSetupDepartmentRequest `json:"departments"`
}

// GetSetupStatus godoc
// @Summary Check system onboarding initialization status
// @Description Returns whether the system is already configured with an administrator or requires initial onboarding setup.
// @Tags Setup
// @Produce json
// @Success 200 {object} handlers.SetupStatusResponse
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/setup/status [get]
func (s *Server) GetSetupStatus(c *gin.Context) {
	var adminCount int64
	if err := s.DB.Model(&models.User{}).Where("role = ? AND deleted_at IS NULL", models.RoleAdmin).Count(&adminCount).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to check setup status: "+err.Error())
		return
	}

	var setting models.SystemSetting
	isNew := "Y"
	if err := s.DB.Where("key = ?", "is_new").First(&setting).Error; err == nil {
		isNew = strings.ToUpper(strings.TrimSpace(setting.Value))
	} else if adminCount > 0 {
		isNew = "N"
	}

	resp := SetupStatusResponse{
		IsNew:         isNew,
		IsInitialized: isNew == "N",
		RequiresSetup: isNew == "Y",
		AdminCount:    adminCount,
	}
	RespondSuccess(c, http.StatusOK, resp)
}

// InitSetup godoc
// @Summary Perform initial system onboarding setup
// @Description One-time setup endpoint to create the primary Super Administrator, initial companies, approvers, and departments. Automatically seeds ActivityStatus and sets is_new = N. Fails with 403 if system is already initialized.
// @Tags Setup
// @Accept json
// @Produce json
// @Param request body handlers.InitSetupRequest true "Initialization payload"
// @Success 200 {object} map[string]interface{} "Setup success response with admin token"
// @Failure 400 {object} models.ErrorResponse "Bad request or validation error"
// @Failure 403 {object} models.ErrorResponse "System already initialized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /api/v1/setup/init [post]
func (s *Server) InitSetup(c *gin.Context) {
	var adminCount int64
	if err := s.DB.Model(&models.User{}).Where("role = ? AND deleted_at IS NULL", models.RoleAdmin).Count(&adminCount).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to check setup status: "+err.Error())
		return
	}

	var setting models.SystemSetting
	isNew := "Y"
	if err := s.DB.Where("key = ?", "is_new").First(&setting).Error; err == nil {
		isNew = strings.ToUpper(strings.TrimSpace(setting.Value))
	} else if adminCount > 0 {
		isNew = "N"
	}

	if isNew == "N" || adminCount > 0 {
		RespondError(c, http.StatusForbidden, "system is already initialized")
		return
	}

	var req InitSetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Validate admin password policy
	if err := auth.ValidatePassword(req.Admin.Password, req.Admin.Username, req.Admin.Email); err != nil {
		RespondError(c, http.StatusBadRequest, "admin password policy violation: "+err.Error())
		return
	}

	hash, err := auth.HashPassword(req.Admin.Password)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to hash password: "+err.Error())
		return
	}

	var adminUser models.User

	err = s.DB.Transaction(func(tx *gorm.DB) error {
		// 1. Companies
		compMap := make(map[string]uint)
		var existingComps []models.Company
		if err := tx.Find(&existingComps).Error; err != nil {
			return err
		}
		for _, comp := range existingComps {
			compMap[strings.ToLower(comp.Code)] = comp.ID
		}

		for _, cr := range req.Companies {
			code := strings.ToLower(strings.TrimSpace(cr.Code))
			if code == "" {
				continue
			}
			if _, exists := compMap[code]; !exists {
				newComp := models.Company{
					Code: code,
					Name: strings.TrimSpace(cr.Name),
				}
				if err := tx.Create(&newComp).Error; err != nil {
					return err
				}
				compMap[code] = newComp.ID
			}
		}

		// 2. Departments
		for _, dr := range req.Departments {
			code := strings.TrimSpace(dr.Code)
			if code == "" {
				continue
			}
			var compID *uint
			if cCode := strings.ToLower(strings.TrimSpace(dr.CompanyCode)); cCode != "" {
				if id, ok := compMap[cCode]; ok {
					compID = &id
				}
			}

			var count int64
			_ = tx.Model(&models.Department{}).Where("code = ?", code).Count(&count).Error
			if count == 0 {
				newDept := models.Department{
					Code:      code,
					Name:      strings.TrimSpace(dr.Name),
					Division:  strings.TrimSpace(dr.Division),
					CompanyID: compID,
					IsActive:  true,
				}
				if err := tx.Create(&newDept).Error; err != nil {
					return err
				}
			}
		}

		// 3. Approvers
		for _, ar := range req.Approvers {
			name := strings.TrimSpace(ar.Name)
			if name == "" {
				continue
			}
			var compID *uint
			if cCode := strings.ToLower(strings.TrimSpace(ar.CompanyCode)); cCode != "" {
				if id, ok := compMap[cCode]; ok {
					compID = &id
				}
			}

			newAppr := models.Approver{
				CompanyID: compID,
				Name:      name,
				RoleType:  ar.RoleType,
				Title:     strings.TrimSpace(ar.Title),
				IsActive:  true,
			}
			if err := tx.Create(&newAppr).Error; err != nil {
				return err
			}
		}

		// 4. Create Super Admin user
		adminUser = models.User{
			Username:     strings.TrimSpace(req.Admin.Username),
			Email:        strings.TrimSpace(req.Admin.Email),
			Name:         strings.TrimSpace(req.Admin.Name),
			PasswordHash: string(hash),
			Role:         models.RoleAdmin,
			IsActive:     true,
		}
		if err := tx.Create(&adminUser).Error; err != nil {
			return err
		}

		// 5. Ensure ActivityStatuses are automatically seeded
		if err := database.SeedActivityStatuses(tx); err != nil {
			return err
		}

		// 6. Mark system as initialized (is_new = N)
		if err := tx.Save(&models.SystemSetting{
			Key:       "is_new",
			Value:     "N",
			UpdatedAt: time.Now(),
		}).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		RespondError(c, http.StatusInternalServerError, "initialization transaction failed: "+err.Error())
		return
	}

	token, err := s.Auth.GenerateToken(&adminUser)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to issue session token: "+err.Error())
		return
	}

	RespondSuccess(c, http.StatusOK, gin.H{
		"message": "system setup completed successfully",
		"token":   token,
		"user":    adminUser,
	})
}
