package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	_ "timesheet-backend/models"
)

// SetupStatusResponse describes the system initialization state.
type SetupStatusResponse = response.SetupStatusResponse

// InitSetupAdminRequest carries administrator account details for setup.
type InitSetupAdminRequest = request.InitSetupAdminRequest

// InitSetupCompanyRequest carries initial company details.
type InitSetupCompanyRequest = request.InitSetupCompanyRequest

// InitSetupApproverRequest carries initial approver supervisor details.
type InitSetupApproverRequest = request.InitSetupApproverRequest

// InitSetupDepartmentRequest carries optional initial department details.
type InitSetupDepartmentRequest = request.InitSetupDepartmentRequest

// InitSetupRequest is the full payload for the one-time system initialization wizard.
type InitSetupRequest = request.InitSetupRequest

// GetSetupStatus godoc
// @Summary Check system onboarding initialization status
// @Description Returns whether the system is already configured with an administrator or requires initial onboarding setup.
// @Tags Setup
// @Produce json
// @Success 200 {object} response.SetupStatusResponse
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/setup/status [get]
func (s *Server) GetSetupStatus(c *gin.Context) {
	svc := s.getSetupService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "setup service unavailable")
		return
	}
	resp, err := svc.GetSetupStatus(c.Request.Context())
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to check setup status: "+err.Error())
		return
	}
	RespondSuccess(c, http.StatusOK, resp)
}

// InitSetup godoc
// @Summary Perform initial system onboarding setup
// @Description One-time setup endpoint to create the primary Super Administrator, initial companies, approvers, and departments. Automatically seeds ActivityStatus and sets is_new = N. Fails with 403 if system is already initialized.
// @Tags Setup
// @Accept json
// @Produce json
// @Param request body request.InitSetupRequest true "Initialization payload"
// @Success 200 {object} map[string]interface{} "Setup success response with admin token"
// @Failure 400 {object} response.ErrorResponse "Bad request or validation error"
// @Failure 403 {object} response.ErrorResponse "System already initialized"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/setup/init [post]
func (s *Server) InitSetup(c *gin.Context) {
	svc := s.getSetupService()
	if svc == nil {
		RespondError(c, http.StatusInternalServerError, "setup service unavailable")
		return
	}

	status, err := svc.GetSetupStatus(c.Request.Context())
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to check setup status: "+err.Error())
		return
	}
	if status.IsInitialized {
		RespondError(c, http.StatusForbidden, "system is already initialized")
		return
	}

	var req request.InitSetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	adminUser, token, err := svc.InitSetup(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			RespondError(c, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, http.StatusOK, gin.H{
		"message": "system setup completed successfully",
		"token":   token,
		"user":    adminUser,
	})
}
