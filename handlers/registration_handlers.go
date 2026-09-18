package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"timesheet-backend/auth"
	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/models"
)

// Register godoc
// @Summary User self-registration
// @Description Creates a new user account with status pending administrator approval.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body request.RegisterRequest true "Self-registration payload"
// @Success 201 {object} response.RegisterResponse
// @Failure 400 {object} response.ErrorResponse "Invalid payload or password policy failure"
// @Failure 409 {object} response.ErrorResponse "Username or email already exists"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/auth/register [post]
func (s *Server) Register(c *gin.Context) {
	var req request.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Name = strings.TrimSpace(req.Name)

	// 1. Password policy verification (NIST SP 800-63B)
	if err := auth.ValidatePassword(req.Password, req.Username, req.Email, req.Name); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	// 2. Check username & email uniqueness
	if errMsg, code := s.checkUserExistence(req.Username, req.Email); code != 0 {
		RespondError(c, code, errMsg)
		return
	}

	// 3. Resolve master data references
	resolvedCompany, resolvedCompanyID, errStr, errCode := s.resolveRegistrationCompany(&req)
	if errCode != 0 {
		RespondError(c, errCode, errStr)
		return
	}

	resolvedDept, resolvedDeptID, resolvedDiv, resolvedDivID, errStr, errCode := s.resolveRegistrationDepartment(&req)
	if errCode != 0 {
		RespondError(c, errCode, errStr)
		return
	}

	if resolvedDiv == "" {
		divName, divID, eStr, eCode := s.resolveRegistrationDivision(&req)
		if eCode != 0 {
			RespondError(c, eCode, eStr)
			return
		}
		resolvedDiv = divName
		resolvedDivID = divID
	}

	resolvedSite, resolvedSiteID, errStr, errCode := s.resolveRegistrationSite(&req)
	if errCode != 0 {
		RespondError(c, errCode, errStr)
		return
	}

	// 4. Hash password with Argon2id
	hasher := s.Hasher
	if hasher == nil {
		hasher = auth.DefaultHasher
	}
	hash, err := hasher.Hash(req.Password)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "could not hash password")
		return
	}

	// 5. Database transaction: create inactive User and pending UserRegistration
	var user models.User
	var reg models.UserRegistration

	txErr := s.DB.Transaction(func(tx *gorm.DB) error {
		user = models.User{
			Username:     req.Username,
			Email:        req.Email,
			PasswordHash: hash,
			Role:         models.RoleUser,
			IsActive:     false, // strictly inactive until approved by admin
			Name:         req.Name,
			BniID:        strings.TrimSpace(req.BniID),
			EmployeeID:   strings.TrimSpace(req.EmployeeID),
			Division:     resolvedDiv,
			DivisionID:   resolvedDivID,
			Department:   resolvedDept,
			DepartmentID: resolvedDeptID,
			Site:         resolvedSite,
			SiteID:       resolvedSiteID,
			Company:      resolvedCompany,
			CompanyID:    resolvedCompanyID,
			Position:     strings.TrimSpace(req.Position),
			GroupName:    strings.TrimSpace(req.GroupName),
		}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.User{}).Where(queryID, user.ID).Update("is_active", false).Error; err != nil {
			return err
		}
		user.IsActive = false

		reg = models.UserRegistration{
			UserID:        user.ID,
			Status:        models.RegistrationPending,
			Name:          req.Name,
			BniID:         strings.TrimSpace(req.BniID),
			EmployeeID:    strings.TrimSpace(req.EmployeeID),
			Division:      resolvedDiv,
			DivisionID:    resolvedDivID,
			Department:    resolvedDept,
			DepartmentID:  resolvedDeptID,
			Site:          resolvedSite,
			SiteID:        resolvedSiteID,
			Company:       resolvedCompany,
			CompanyID:     resolvedCompanyID,
			Position:      strings.TrimSpace(req.Position),
			GroupName:     strings.TrimSpace(req.GroupName),
		}
		if err := tx.Create(&reg).Error; err != nil {
			return err
		}
		return nil
	})

	if txErr != nil {
		errMsg, code := handleCreateUserDBError(txErr)
		RespondError(c, code, errMsg)
		return
	}

	RespondSuccess(c, http.StatusCreated, response.RegisterResultData{
		RegistrationID: reg.ID,
		UserID:         user.ID,
		Username:       user.Username,
		Email:          user.Email,
		Status:         reg.Status,
		Message:        "Registration submitted successfully. Your account is pending administrator approval before you can log in.",
	})
}

// ListRegistrations godoc
// @Summary List user self-registrations (Admin)
// @Description Returns a list of user registrations, optionally filtered by status (Admin only).
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param status query string false "Filter by registration status" Enums(pending, approved, rejected)
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} response.RegistrationListResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin only"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/registrations [get]
func (s *Server) ListRegistrations(c *gin.Context) {
	if s.DB == nil {
		RespondError(c, http.StatusInternalServerError, "database not available")
		return
	}

	query := s.DB.Model(&models.UserRegistration{}).
		Preload("User").
		Preload("Reviewer").
		Preload("CompanyRel").
		Preload("DepartmentRel").
		Preload("DivisionRel").
		Preload("SiteRel")

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))

	var totalRows int64
	if err := query.Count(&totalRows).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if page > 0 && limit > 0 {
		offset := (page - 1) * limit
		query = query.Offset(offset).Limit(limit)
	}

	var registrations []models.UserRegistration
	if err := query.Order("created_at DESC").Find(&registrations).Error; err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	resp := make([]response.UserRegistrationResponse, len(registrations))
	for i := range registrations {
		resp[i] = response.ToUserRegistrationResponse(&registrations[i])
	}

	if page > 0 && limit > 0 {
		RespondPaginated(c, http.StatusOK, resp, page, limit, totalRows)
		return
	}

	RespondSuccess(c, http.StatusOK, resp)
}

// GetRegistration godoc
// @Summary Get registration details (Admin)
// @Description Returns the detailed record of a registration submission (Admin only).
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param id path int true "Registration ID"
// @Success 200 {object} response.RegistrationDetailResponse
// @Failure 400 {object} response.ErrorResponse "Invalid ID"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin only"
// @Failure 404 {object} response.ErrorResponse "Registration not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/registrations/{id} [get]
func (s *Server) GetRegistration(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid registration ID")
		return
	}

	var reg models.UserRegistration
	if err := s.DB.Preload("User").
		Preload("Reviewer").
		Preload("CompanyRel").
		Preload("DepartmentRel").
		Preload("DivisionRel").
		Preload("SiteRel").
		Where(queryID, id).First(&reg).Error; err != nil {
		RespondError(c, http.StatusNotFound, "registration not found")
		return
	}

	RespondSuccess(c, http.StatusOK, response.ToUserRegistrationResponse(&reg))
}

// ReviewRegistration godoc
// @Summary Review user registration (Admin)
// @Description Approves or rejects a submitted user registration. Approving activates the user account (Admin only).
// @Tags Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "Registration ID"
// @Param action query string false "Review action (approve or reject)" Enums(approve, reject)
// @Param request body request.ReviewRegistrationRequest false "Review notes and action payload"
// @Success 200 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Invalid action or ID"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin only"
// @Failure 404 {object} response.ErrorResponse "Registration not found"
// @Failure 409 {object} response.ErrorResponse "Registration already reviewed"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/admin/registrations/{id}/review [post]
func (s *Server) ReviewRegistration(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		RespondError(c, http.StatusBadRequest, "invalid registration ID")
		return
	}

	action := strings.ToLower(strings.TrimSpace(c.Query("action")))
	adminNotes := ""

	// Optionally parse JSON payload if provided
	var req request.ReviewRegistrationRequest
	if c.Request.ContentLength > 0 {
		if bindErr := c.ShouldBindJSON(&req); bindErr == nil {
			if req.Action != "" {
				action = strings.ToLower(strings.TrimSpace(req.Action))
			}
			adminNotes = strings.TrimSpace(req.AdminNotes)
		}
	}

	if action != "approve" && action != "reject" {
		RespondError(c, http.StatusBadRequest, "invalid action: must be 'approve' or 'reject'")
		return
	}

	if s.DB == nil {
		RespondError(c, http.StatusInternalServerError, "database not available")
		return
	}

	var reg models.UserRegistration
	if err := s.DB.Where(queryID, id).First(&reg).Error; err != nil {
		RespondError(c, http.StatusNotFound, "registration not found")
		return
	}
	if reg.Status != models.RegistrationPending {
		RespondError(c, http.StatusConflict, "registration already reviewed")
		return
	}

	reviewerID := currentUserID(c)
	now := time.Now()

	txErr := s.DB.Transaction(func(tx *gorm.DB) error {
		if action == "approve" {
			reg.Status = models.RegistrationApproved
			if err := tx.Model(&models.User{}).Where(queryID, reg.UserID).Update("is_active", true).Error; err != nil {
				return err
			}
		} else {
			reg.Status = models.RegistrationRejected
			if err := tx.Model(&models.User{}).Where(queryID, reg.UserID).Update("is_active", false).Error; err != nil {
				return err
			}
		}

		reg.ReviewedBy = &reviewerID
		reg.ReviewedAt = &now
		if adminNotes != "" {
			reg.AdminNotes = adminNotes
		}

		return tx.Save(&reg).Error
	})

	if txErr != nil {
		RespondError(c, http.StatusInternalServerError, txErr.Error())
		return
	}

	// Dispatch notification email if approved and mailer is available
	if action == "approve" && s.Mailer != nil {
		var user models.User
		if err := s.DB.First(&user, reg.UserID).Error; err == nil && user.Email != "" {
			loginURL := s.publicBaseURL(c) + "/login"
			go func(to, username, url string) {
				_ = s.Mailer.SendAccountWelcomeEmail(to, username, "(password as registered)", url)
			}(user.Email, user.Username, loginURL)
		}
	}

	RespondMessage(c, http.StatusOK, "registration "+action+"d successfully")
}

// Master data resolution helpers for self-registration

func (s *Server) resolveRegistrationCompany(req *request.RegisterRequest) (string, *uint, string, int) {
	if req.CompanyID != nil && *req.CompanyID != 0 {
		var comp models.Company
		if err := s.DB.Where(queryIDAndIsActive, *req.CompanyID).First(&comp).Error; err != nil {
			return "", nil, errCompanyDeptNotFound, http.StatusBadRequest
		}
		return comp.Name, &comp.ID, "", 0
	} else if req.Company != "" {
		var comp models.Company
		if err := s.DB.Where(queryCodeOrNameLikeIsActive, req.Company, "%"+req.Company+"%").First(&comp).Error; err == nil {
			return comp.Name, &comp.ID, "", 0
		}
		return "", nil, errCompanyDeptNotFound, http.StatusBadRequest
	}
	return "", nil, "", 0
}

func (s *Server) resolveRegistrationDepartment(req *request.RegisterRequest) (string, *uint, string, *uint, string, int) {
	if req.DepartmentID != nil && *req.DepartmentID != 0 {
		var dept models.Department
		if err := s.DB.Where(queryIDAndIsActive, *req.DepartmentID).First(&dept).Error; err != nil {
			return "", nil, "", nil, errCompanyDeptNotFound, http.StatusBadRequest
		}
		return dept.Name, &dept.ID, dept.Division, dept.DivisionID, "", 0
	} else if req.Department != "" {
		var dept models.Department
		if err := s.DB.Where(queryCodeOrNameLikeIsActive, req.Department, "%"+req.Department+"%").First(&dept).Error; err == nil {
			return dept.Name, &dept.ID, dept.Division, dept.DivisionID, "", 0
		}
	}
	return req.Department, nil, "", nil, "", 0
}

func (s *Server) resolveRegistrationDivision(req *request.RegisterRequest) (string, *uint, string, int) {
	if req.DivisionID != nil && *req.DivisionID != 0 {
		var div models.Division
		if err := s.DB.Where(queryIDAndIsActive, *req.DivisionID).First(&div).Error; err != nil {
			return "", nil, "Division not found or inactive", http.StatusBadRequest
		}
		return div.Name, &div.ID, "", 0
	} else if req.Division != "" {
		var div models.Division
		if err := s.DB.Where(queryCodeOrNameLikeIsActive, req.Division, "%"+req.Division+"%").First(&div).Error; err == nil {
			return div.Name, &div.ID, "", 0
		}
		return req.Division, nil, "", 0
	}
	return "", nil, "", 0
}

func (s *Server) resolveRegistrationSite(req *request.RegisterRequest) (string, *uint, string, int) {
	if req.SiteID != nil && *req.SiteID != 0 {
		var site models.Site
		if err := s.DB.Where(queryIDAndIsActive, *req.SiteID).First(&site).Error; err != nil {
			return "", nil, "Site not found or inactive", http.StatusBadRequest
		}
		return site.Name, &site.ID, "", 0
	} else if req.Site != "" {
		var site models.Site
		if err := s.DB.Where(queryCodeOrNameLikeIsActive, req.Site, "%"+req.Site+"%").First(&site).Error; err == nil {
			return site.Name, &site.ID, "", 0
		}
		return req.Site, nil, "", 0
	}
	return "", nil, "", 0
}
