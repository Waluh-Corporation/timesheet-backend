package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"timesheet-backend/auth"
	"timesheet-backend/models"
)

// isSelf reports whether the :id path param refers to the authenticated caller.
func isSelf(c *gin.Context, id string) bool {
	target, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return false
	}
	return uint(target) == currentUserID(c)
}

// createUserRequest is the admin-only account creation payload.
type createUserRequest struct {
	Username   string      `json:"username" binding:"required,min=3,max=64"`
	Email      string      `json:"email" binding:"required,email"`
	Role       models.Role `json:"role" binding:"required,oneof=admin user"`
	Name       string      `json:"name"`
	MiiID      string      `json:"mii_id"`
	Division   string      `json:"division"`
	Site       string      `json:"site"`
	Company    string      `json:"company"`
	CompanyID  *uint       `json:"company_id"`
	DivisionID *uint       `json:"division_id"`
	SiteID     *uint       `json:"site_id"`
	// Password is optional; when omitted the user completes setup via email link.
	Password string `json:"password"`
}

// ListUsers returns all users (admin only).
func (s *Server) ListUsers(c *gin.Context) {
	var users []models.User
	if err := s.DB.Preload("CompanyRef").Preload("DivisionRef").Preload("SiteRef").
		Order("created_at desc").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

// CreateUser provisions a new account (admin only) and emails a setup link.
// This is the sole registration path — there is no public sign-up.
func (s *Server) CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := models.User{
		Username: req.Username,
		Email:    req.Email,
		Role:     req.Role,
		Name:     req.Name,
		MiiID:    req.MiiID,
		Division: req.Division,
		Site:     req.Site,
		Company:  req.Company,
		IsActive: true,
	}

	if req.CompanyID != nil && *req.CompanyID > 0 {
		var comp models.Company
		if err := s.DB.First(&comp, *req.CompanyID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "selected company not found"})
			return
		}
		user.CompanyID = &comp.ID
		user.Company = comp.Code
	}

	if req.DivisionID != nil && *req.DivisionID > 0 {
		var div models.Division
		if err := s.DB.First(&div, *req.DivisionID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "selected division not found"})
			return
		}
		if user.CompanyID != nil && div.CompanyID != *user.CompanyID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "division does not belong to selected company"})
			return
		}
		user.DivisionID = &div.ID
		user.Division = div.Name
	}

	if req.SiteID != nil && *req.SiteID > 0 {
		var site models.Site
		if err := s.DB.First(&site, *req.SiteID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "selected site not found"})
			return
		}
		if user.CompanyID != nil && site.CompanyID != *user.CompanyID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "site does not belong to selected company"})
			return
		}
		user.SiteID = &site.ID
		user.Site = site.Name
	}

	if req.Password != "" {
		// Enforce the NIST SP 800-63B policy on any admin-supplied initial
		// password (length + blocklist + context-specific terms).
		if err := auth.ValidatePassword(req.Password, req.Username, req.Email); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not hash password"})
			return
		}
		user.PasswordHash = hash
	}

	if err := s.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username or email already exists"})
		return
	}

	// Issue a setup token so the user can set their own password / passkey.
	raw, hash, err := auth.GenerateResetToken()
	if err == nil {
		s.DB.Create(&models.PasswordResetToken{
			UserID:    user.ID,
			TokenHash: hash,
			ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		})
		link := s.publicBaseURL(c) + "/reset-password?token=" + raw
		_ = s.Mailer.SendSetupEmail(user.Email, user.Username, link)
	}

	s.DB.Preload("CompanyRef").Preload("DivisionRef").Preload("SiteRef").First(&user, user.ID)
	c.JSON(http.StatusCreated, user)
}

// updateUserRequest lets admins toggle role/active state.
type updateUserRequest struct {
	Role       *models.Role `json:"role"`
	IsActive   *bool        `json:"is_active"`
	Name       *string      `json:"name"`
	MiiID      *string      `json:"mii_id"`
	Division   *string      `json:"division"`
	Site       *string      `json:"site"`
	Company    *string      `json:"company"`
	CompanyID  *uint        `json:"company_id"`
	DivisionID *uint        `json:"division_id"`
	SiteID     *uint        `json:"site_id"`
}

// UpdateUser edits a user directly (admin only). Admin edits are applied
// immediately, bypassing the approval flow that governs self-service edits.
func (s *Server) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	if err := s.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// An admin may never deactivate or demote their own account through the
	// update path either — both are self-lockout vectors.
	if isSelf(c, id) {
		if req.IsActive != nil && !*req.IsActive {
			c.JSON(http.StatusForbidden, gin.H{"error": "you cannot deactivate your own account"})
			return
		}
		if req.Role != nil && *req.Role != models.RoleAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "you cannot remove your own admin role"})
			return
		}
	}

	updates := map[string]interface{}{}
	if req.Role != nil {
		updates["role"] = *req.Role
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.MiiID != nil {
		updates["mii_id"] = *req.MiiID
	}
	if req.Division != nil {
		updates["division"] = *req.Division
	}
	if req.Site != nil {
		updates["site"] = *req.Site
	}
	if req.Company != nil {
		updates["company"] = *req.Company
	}

	targetCompanyID := user.CompanyID
	if req.CompanyID != nil {
		if *req.CompanyID > 0 {
			var comp models.Company
			if err := s.DB.First(&comp, *req.CompanyID).Error; err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "selected company not found"})
				return
			}
			updates["company_id"] = comp.ID
			updates["company"] = comp.Code
			targetCompanyID = &comp.ID
		} else {
			updates["company_id"] = nil
			updates["company"] = ""
			targetCompanyID = nil
		}
	}

	if req.DivisionID != nil {
		if *req.DivisionID > 0 {
			var div models.Division
			if err := s.DB.First(&div, *req.DivisionID).Error; err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "selected division not found"})
				return
			}
			if targetCompanyID != nil && div.CompanyID != *targetCompanyID {
				c.JSON(http.StatusBadRequest, gin.H{"error": "division does not belong to selected company"})
				return
			}
			updates["division_id"] = div.ID
			updates["division"] = div.Name
		} else {
			updates["division_id"] = nil
			updates["division"] = ""
		}
	}

	if req.SiteID != nil {
		if *req.SiteID > 0 {
			var site models.Site
			if err := s.DB.First(&site, *req.SiteID).Error; err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "selected site not found"})
				return
			}
			if targetCompanyID != nil && site.CompanyID != *targetCompanyID {
				c.JSON(http.StatusBadRequest, gin.H{"error": "site does not belong to selected company"})
				return
			}
			updates["site_id"] = site.ID
			updates["site"] = site.Name
		} else {
			updates["site_id"] = nil
			updates["site"] = ""
		}
	}

	if len(updates) > 0 {
		s.DB.Model(&user).Updates(updates)
	}
	s.DB.Preload("CompanyRef").Preload("DivisionRef").Preload("SiteRef").First(&user, id)
	c.JSON(http.StatusOK, user)
}

// DeleteUser deactivates a user (admin only). This is a soft action — the
// account is set inactive but stays in the users list and can be reactivated;
// it is never hard-deleted, preserving their timesheet history.
func (s *Server) DeleteUser(c *gin.Context) {
	id := c.Param("id")
	// An admin may never deactivate/delete their own account — doing so could
	// lock the last administrator out of the portal.
	if isSelf(c, id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "you cannot deactivate your own account"})
		return
	}
	var user models.User
	if err := s.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if err := s.DB.Model(&user).Update("is_active", false).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.DB.First(&user, id)
	c.JSON(http.StatusOK, user)
}

// --- Profile approval flow ---

// profileChangeRequest is a user's self-service profile edit request.
type profileChangeRequest struct {
	Name       string `json:"name"`
	MiiID      string `json:"mii_id"`
	Division   string `json:"division"`
	Site       string `json:"site"`
	Company    string `json:"company"`
	CompanyID  *uint  `json:"company_id"`
	DivisionID *uint  `json:"division_id"`
	SiteID     *uint  `json:"site_id"`
}

// SubmitProfileChange records a pending profile change for the current user.
// The live profile is not modified until an admin approves the request.
func (s *Server) SubmitProfileChange(c *gin.Context) {
	var req profileChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	change := models.ProfileChangeRequest{
		UserID:     currentUserID(c),
		Status:     models.ProfilePending,
		Name:       req.Name,
		MiiID:      req.MiiID,
		Division:   req.Division,
		Site:       req.Site,
		Company:    req.Company,
		CompanyID:  req.CompanyID,
		DivisionID: req.DivisionID,
		SiteID:     req.SiteID,
	}

	if req.CompanyID != nil && *req.CompanyID > 0 {
		var comp models.Company
		if err := s.DB.First(&comp, *req.CompanyID).Error; err == nil {
			change.Company = comp.Code
		}
	}
	if req.DivisionID != nil && *req.DivisionID > 0 {
		var div models.Division
		if err := s.DB.First(&div, *req.DivisionID).Error; err == nil {
			change.Division = div.Name
		}
	}
	if req.SiteID != nil && *req.SiteID > 0 {
		var site models.Site
		if err := s.DB.First(&site, *req.SiteID).Error; err == nil {
			change.Site = site.Name
		}
	}

	if err := s.DB.Create(&change).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, change)
}

// MyProfileChanges returns the current user's own profile change requests so
// the profile page can show pending/approved/rejected status.
func (s *Server) MyProfileChanges(c *gin.Context) {
	var changes []models.ProfileChangeRequest
	if err := s.DB.Where("user_id = ?", currentUserID(c)).
		Order("created_at desc").Find(&changes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, changes)
}

// ListProfileChanges returns pending profile change requests (admin only).
func (s *Server) ListProfileChanges(c *gin.Context) {
	var changes []models.ProfileChangeRequest
	q := s.DB.Preload("User").Order("created_at desc")
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Find(&changes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, changes)
}

// ReviewProfileChange approves or rejects a pending profile change (admin only).
// On approval the requested values are copied onto the live user record.
func (s *Server) ReviewProfileChange(c *gin.Context) {
	id := c.Param("id")
	action := c.Query("action") // "approve" or "reject"

	var change models.ProfileChangeRequest
	if err := s.DB.First(&change, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "request not found"})
		return
	}
	if change.Status != models.ProfilePending {
		c.JSON(http.StatusConflict, gin.H{"error": "request already reviewed"})
		return
	}

	reviewer := currentUserID(c)
	now := time.Now()

	if action == "approve" {
		updates := map[string]interface{}{
			"name":     change.Name,
			"mii_id":   change.MiiID,
			"division": change.Division,
			"site":     change.Site,
		}
		if change.Company != "" {
			updates["company"] = change.Company
		}
		if change.CompanyID != nil {
			updates["company_id"] = change.CompanyID
		}
		if change.DivisionID != nil {
			updates["division_id"] = change.DivisionID
		}
		if change.SiteID != nil {
			updates["site_id"] = change.SiteID
		}
		s.DB.Model(&models.User{}).Where("id = ?", change.UserID).Updates(updates)
		change.Status = models.ProfileApproved
	} else {
		change.Status = "rejected"
	}
	change.ReviewedBy = &reviewer
	change.ReviewedAt = &now
	s.DB.Save(&change)

	c.JSON(http.StatusOK, change)
}
