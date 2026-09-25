package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"timesheet-backend/auth"
	"timesheet-backend/dto/request"
	"timesheet-backend/models"
)

func (s *Server) resolveUserCompany(req *request.CreateUserRequest, user *models.User) (string, int) {
	if user.Role == models.RoleAdmin {
		user.CompanyID = nil
		user.Company = ""
		return "", 0
	}
	if req.CompanyID != nil && *req.CompanyID != 0 {
		var comp models.Company
		if err := s.DB.Where(queryIDAndIsActive, *req.CompanyID).First(&comp).Error; err != nil {
			return errCompanyDeptNotFound, http.StatusBadRequest
		}
		user.CompanyID = &comp.ID
		user.Company = comp.Name
	} else if req.Company != "" {
		var comp models.Company
		if err := s.DB.Where(queryCodeOrNameLikeIsActive, req.Company, "%"+req.Company+"%").First(&comp).Error; err == nil {
			user.CompanyID = &comp.ID
			user.Company = comp.Name
		} else {
			return errCompanyDeptNotFound, http.StatusBadRequest
		}
	} else {
		user.CompanyID = nil
		user.Company = ""
	}
	return "", 0
}

func (s *Server) resolveUserDepartment(req *request.CreateUserRequest, user *models.User) (string, int) {
	if req.DepartmentID != nil && *req.DepartmentID != 0 {
		var dept models.Department
		if err := s.DB.Where(queryIDAndIsActive, *req.DepartmentID).First(&dept).Error; err != nil {
			return errCompanyDeptNotFound, http.StatusBadRequest
		}
		user.DepartmentID = &dept.ID
		user.Department = dept.Name
		if user.Division == "" {
			user.Division = dept.Division
			user.DivisionID = dept.DivisionID
		}
	} else if req.Department != "" {
		var dept models.Department
		if err := s.DB.Where(queryCodeOrNameLikeIsActive, req.Department, "%"+req.Department+"%").First(&dept).Error; err == nil {
			user.DepartmentID = &dept.ID
			user.Department = dept.Name
			if user.Division == "" {
				user.Division = dept.Division
				user.DivisionID = dept.DivisionID
			}
		}
	}
	return "", 0
}

func (s *Server) resolveUserSite(req *request.CreateUserRequest, user *models.User) (string, int) {
	if req.SiteID != nil && *req.SiteID != 0 {
		var site models.Site
		if err := s.DB.Where(queryIDAndIsActive, *req.SiteID).First(&site).Error; err != nil {
			return "Site not found or inactive", http.StatusBadRequest
		}
		user.SiteID = &site.ID
		user.Site = site.Name
	} else if req.Site != "" {
		var site models.Site
		if err := s.DB.Where(queryCodeOrNameLikeIsActive, req.Site, "%"+req.Site+"%").First(&site).Error; err == nil {
			user.SiteID = &site.ID
			user.Site = site.Name
		} else {
			user.SiteID = nil
			user.Site = req.Site
		}
	} else {
		user.SiteID = nil
		user.Site = ""
	}
	return "", 0
}

func (s *Server) resolveUserDivision(req *request.CreateUserRequest, user *models.User) (string, int) {
	if req.DivisionID != nil && *req.DivisionID != 0 {
		var div models.Division
		if err := s.DB.Where(queryIDAndIsActive, *req.DivisionID).First(&div).Error; err != nil {
			return "Division not found or inactive", http.StatusBadRequest
		}
		user.DivisionID = &div.ID
		user.Division = div.Name
	} else if req.Division != "" {
		var div models.Division
		if err := s.DB.Where(queryCodeOrNameLikeIsActive, req.Division, "%"+req.Division+"%").First(&div).Error; err == nil {
			user.DivisionID = &div.ID
			user.Division = div.Name
		} else {
			user.DivisionID = nil
			user.Division = req.Division
		}
	} else if user.DivisionID == nil && user.Division != "" {
		var div models.Division
		if err := s.DB.Where(queryCodeOrNameLikeIsActive, user.Division, "%"+user.Division+"%").First(&div).Error; err == nil {
			user.DivisionID = &div.ID
			user.Division = div.Name
		}
	}
	return "", 0
}

func (s *Server) resolveUserMasterData(req *request.CreateUserRequest, user *models.User) (string, int) {
	if errMsg, code := s.resolveUserCompany(req, user); code != 0 {
		return errMsg, code
	}
	if errMsg, code := s.resolveUserDepartment(req, user); code != 0 {
		return errMsg, code
	}
	if errMsg, code := s.resolveUserDivision(req, user); code != 0 {
		return errMsg, code
	}
	return s.resolveUserSite(req, user)
}

func (s *Server) handleInitialPassword(user *models.User) (string, string, int) {
	plain, err := auth.GenerateSecurePassword(auth.GeneratedPasswordLength)
	if err != nil {
		return "", "failed to generate initial password", http.StatusInternalServerError
	}

	hasher := s.Hasher
	if hasher == nil {
		hasher = auth.DefaultHasher
	}
	hash, err := hasher.Hash(plain)
	if err != nil {
		return "", "could not hash password", http.StatusInternalServerError
	}
	user.PasswordHash = hash
	return plain, "", 0
}

func (s *Server) checkUserExistence(username, email string) (string, int) {
	var existing models.User
	if err := s.DB.Where("LOWER(username) = LOWER(?)", username).First(&existing).Error; err == nil {
		return "username already exists", http.StatusConflict
	}
	if err := s.DB.Where("LOWER(email) = LOWER(?)", email).First(&existing).Error; err == nil {
		return "email already exists", http.StatusConflict
	}
	return "", 0
}

func handleCreateUserDBError(err error) (string, int) {
	if strings.Contains(err.Error(), "23503") || strings.Contains(err.Error(), "foreign key") {
		return errCompanyDeptNotFound, http.StatusBadRequest
	}
	errLower := strings.ToLower(err.Error())
	if strings.Contains(errLower, "username") {
		return "username already exists", http.StatusConflict
	}
	if strings.Contains(errLower, "email") {
		return "email already exists", http.StatusConflict
	}
	return "username or email already exists", http.StatusConflict
}

func validateSelfUpdate(c *gin.Context, id uint, req *request.UpdateUserRequest) (string, int) {
	if !isSelf(c, id) {
		return "", 0
	}
	if req.IsActive != nil && !*req.IsActive {
		return "you cannot deactivate your own account", http.StatusForbidden
	}
	if req.Role != nil && *req.Role != models.RoleAdmin {
		return "you cannot remove your own admin role", http.StatusForbidden
	}
	return "", 0
}

func updateUserDepartment(db *gorm.DB, user *models.User, req *request.UpdateUserRequest) (string, int) {
	if req.Department != nil {
		user.Department = *req.Department
	}
	if req.DepartmentID != nil {
		if *req.DepartmentID != 0 {
			var dept models.Department
			if err := db.Where(queryIDAndIsActive, *req.DepartmentID).First(&dept).Error; err != nil {
				return errCompanyDeptNotFound, http.StatusBadRequest
			}
			user.DepartmentID = &dept.ID
			user.Department = dept.Name
			if user.Division == "" {
				user.Division = dept.Division
			}
		} else {
			user.DepartmentID = nil
		}
	}
	return "", 0
}

func updateUserCompany(db *gorm.DB, user *models.User, req *request.UpdateUserRequest) (string, int) {
	if req.CompanyID != nil {
		if *req.CompanyID != 0 {
			var comp models.Company
			if err := db.Where(queryIDAndIsActive, *req.CompanyID).First(&comp).Error; err != nil {
				return errCompanyDeptNotFound, http.StatusBadRequest
			}
			user.CompanyID = &comp.ID
			user.Company = comp.Name
		} else {
			user.CompanyID = nil
			user.Company = ""
		}
	} else if req.Company != nil {
		if *req.Company != "" {
			var comp models.Company
			if err := db.Where(queryCodeOrNameLikeIsActive, *req.Company, "%"+*req.Company+"%").First(&comp).Error; err == nil {
				user.CompanyID = &comp.ID
				user.Company = comp.Name
			} else {
				return errCompanyDeptNotFound, http.StatusBadRequest
			}
		} else {
			user.CompanyID = nil
			user.Company = ""
		}
	}
	return "", 0
}

func applyUserUpdates(db *gorm.DB, user *models.User, req *request.UpdateUserRequest) (string, int) {
	if req.Role != nil {
		user.Role = *req.Role
	}
	if req.Email != nil {
		newEmail := strings.TrimSpace(*req.Email)
		if newEmail != "" && newEmail != user.Email {
			var existing models.User
			if err := db.Where("email = ? AND id != ?", newEmail, user.ID).First(&existing).Error; err == nil {
				return "email is already registered", http.StatusBadRequest
			}
			user.Email = newEmail
		}
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.BniID != nil {
		user.BniID = *req.BniID
	}
	if req.EmployeeID != nil {
		user.EmployeeID = *req.EmployeeID
	}
	if req.Division != nil {
		user.Division = *req.Division
	}
	if req.Site != nil {
		user.Site = *req.Site
	}

	if msg, code := updateUserDepartment(db, user, req); code != 0 {
		return msg, code
	}
	if msg, code := updateUserCompany(db, user, req); code != 0 {
		return msg, code
	}
	if msg, code := updateUserDivision(db, user, req); code != 0 {
		return msg, code
	}
	if msg, code := updateUserSite(db, user, req); code != 0 {
		return msg, code
	}

	if user.Role == models.RoleAdmin {
		user.Company = ""
		user.CompanyID = nil
	}
	return "", 0
}

func updateUserSite(db *gorm.DB, user *models.User, req *request.UpdateUserRequest) (string, int) {
	if req.SiteID != nil {
		if *req.SiteID != 0 {
			var site models.Site
			if err := db.Where(queryIDAndIsActive, *req.SiteID).First(&site).Error; err != nil {
				return "Site not found or inactive", http.StatusBadRequest
			}
			user.SiteID = &site.ID
			user.Site = site.Name
		} else {
			user.SiteID = nil
			user.Site = ""
		}
	} else if req.Site != nil {
		user.Site = *req.Site
		if *req.Site != "" {
			var site models.Site
			if err := db.Where(queryCodeOrNameLikeIsActive, *req.Site, "%"+*req.Site+"%").First(&site).Error; err == nil {
				user.SiteID = &site.ID
				user.Site = site.Name
			} else {
				user.SiteID = nil
			}
		} else {
			user.SiteID = nil
		}
	}
	return "", 0
}

func updateUserDivision(db *gorm.DB, user *models.User, req *request.UpdateUserRequest) (string, int) {
	if req.DivisionID != nil {
		if *req.DivisionID != 0 {
			var div models.Division
			if err := db.Where(queryIDAndIsActive, *req.DivisionID).First(&div).Error; err != nil {
				return "Division not found or inactive", http.StatusBadRequest
			}
			user.DivisionID = &div.ID
			user.Division = div.Name
		} else {
			user.DivisionID = nil
			user.Division = ""
		}
	} else if req.Division != nil {
		user.Division = *req.Division
		if *req.Division != "" {
			var div models.Division
			if err := db.Where(queryCodeOrNameLikeIsActive, *req.Division, "%"+*req.Division+"%").First(&div).Error; err == nil {
				user.DivisionID = &div.ID
				user.Division = div.Name
			} else {
				user.DivisionID = nil
			}
		} else {
			user.DivisionID = nil
		}
	}
	return "", 0
}
