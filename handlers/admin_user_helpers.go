package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"timesheet-backend/dto/request"
	"timesheet-backend/models"
)

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
