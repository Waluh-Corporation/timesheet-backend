package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"timesheet-backend/dto/request"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/internal/service"
	"timesheet-backend/models"
	"timesheet-backend/services"
)

const (
	errSignatureSvcNotInit = "Signature service not initialized"
	errInvalidParamPrefix  = "Parameter tidak valid: "
)

func extractAuditMeta(c *gin.Context) service.AuditMeta {
	return service.AuditMeta{
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	}
}

// UploadSignature godoc
// @Summary Upload or draw user profile signature
// @Description Uploads a Base64-encoded PNG/JPEG signature image to storage and updates user profile.
// @Tags Signatures
// @Accept json
// @Produce json
// @Param request body request.UploadSignatureRequest true "Signature base64 image"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/v1/user/signature [post]
func (s *Server) UploadSignature(c *gin.Context) {
	if s.SignatureSvc == nil {
		RespondError(c, http.StatusInternalServerError, errSignatureSvcNotInit)
		return
	}

	var req request.UploadSignatureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "Payload tidak valid: "+err.Error())
		return
	}

	userID := currentUserID(c)
	resp, err := s.SignatureSvc.UploadSignature(c.Request.Context(), userID, req.SignatureBase64)
	if err != nil {
		var userErr *domain.UserError
		if errors.As(err, &userErr) {
			RespondError(c, domain.MapErrorToHTTPStatus(userErr.Err), userErr.Msg)
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, http.StatusOK, resp)
}

// GetSignature godoc
// @Summary Get user profile signature
// @Description Retrieves the signature presigned URL and status for the authenticated user.
// @Tags Signatures
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/v1/user/signature [get]
func (s *Server) GetSignature(c *gin.Context) {
	if s.SignatureSvc == nil {
		RespondError(c, http.StatusInternalServerError, errSignatureSvcNotInit)
		return
	}

	userID := currentUserID(c)
	resp, err := s.SignatureSvc.GetSignature(c.Request.Context(), userID)
	if err != nil {
		var userErr *domain.UserError
		if errors.As(err, &userErr) {
			RespondError(c, domain.MapErrorToHTTPStatus(userErr.Err), userErr.Msg)
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, http.StatusOK, resp)
}

// DeleteSignature godoc
// @Summary Delete user profile signature
// @Description Removes the saved signature for the authenticated user.
// @Tags Signatures
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/v1/user/signature [delete]
func (s *Server) DeleteSignature(c *gin.Context) {
	if s.SignatureSvc == nil {
		RespondError(c, http.StatusInternalServerError, errSignatureSvcNotInit)
		return
	}

	userID := currentUserID(c)
	if err := s.SignatureSvc.DeleteSignature(c.Request.Context(), userID); err != nil {
		var userErr *domain.UserError
		if errors.As(err, &userErr) {
			RespondError(c, domain.MapErrorToHTTPStatus(userErr.Err), userErr.Msg)
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondMessage(c, http.StatusOK, "Tanda tangan profil berhasil dihapus")
}

// SignTimesheet godoc
// @Summary Sign monthly timesheet
// @Description Signs the user's monthly timesheet, locking activity edits and issuing verification hash.
// @Tags Signatures
// @Accept json
// @Produce json
// @Param request body request.SignTimesheetRequest true "Timesheet period"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /api/v1/timesheets/sign [post]
func (s *Server) SignTimesheet(c *gin.Context) {
	if s.SignatureSvc == nil {
		RespondError(c, http.StatusInternalServerError, errSignatureSvcNotInit)
		return
	}

	var req request.SignTimesheetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, errInvalidParamPrefix+err.Error())
		return
	}

	userID := currentUserID(c)
	meta := extractAuditMeta(c)
	resp, err := s.SignatureSvc.SignTimesheet(c.Request.Context(), userID, &req, meta)
	if err != nil {
		var userErr *domain.UserError
		if errors.As(err, &userErr) {
			RespondError(c, domain.MapErrorToHTTPStatus(userErr.Err), userErr.Msg)
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, http.StatusOK, resp)
}

// RevokeTimesheet godoc
// @Summary Revoke timesheet signature
// @Description Revokes a signed timesheet, unlocking daily activity mutations.
// @Tags Signatures
// @Accept json
// @Produce json
// @Param request body request.RevokeTimesheetRequest true "Timesheet period"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/timesheets/revoke [post]
func (s *Server) RevokeTimesheet(c *gin.Context) {
	if s.SignatureSvc == nil {
		RespondError(c, http.StatusInternalServerError, errSignatureSvcNotInit)
		return
	}

	var req request.RevokeTimesheetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, errInvalidParamPrefix+err.Error())
		return
	}

	userID := currentUserID(c)
	meta := extractAuditMeta(c)
	if err := s.SignatureSvc.RevokeTimesheet(c.Request.Context(), userID, &req, meta); err != nil {
		var userErr *domain.UserError
		if errors.As(err, &userErr) {
			RespondError(c, domain.MapErrorToHTTPStatus(userErr.Err), userErr.Msg)
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondMessage(c, http.StatusOK, "Tanda tangan timesheet berhasil dibatalkan")
}

// GetTimesheetSubmissionStatus godoc
// @Summary Get timesheet sign-off status
// @Description Retrieves the sign-off lifecycle state of a monthly timesheet.
// @Tags Signatures
// @Produce json
// @Param month query int true "Month (1-12)"
// @Param year query int true "Year"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/timesheets/status [get]
func (s *Server) GetTimesheetSubmissionStatus(c *gin.Context) {
	if s.SignatureSvc == nil {
		RespondError(c, http.StatusInternalServerError, errSignatureSvcNotInit)
		return
	}

	month, _ := strconv.Atoi(c.Query("month"))
	year, _ := strconv.Atoi(c.Query("year"))
	if month < 1 || month > 12 || year < 2000 {
		RespondError(c, http.StatusBadRequest, "Bulan atau tahun tidak valid")
		return
	}

	userID := currentUserID(c)
	resp, err := s.SignatureSvc.GetTimesheetSubmission(c.Request.Context(), userID, month, year)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, http.StatusOK, resp)
}

// SubmitBeritaAcara godoc
// @Summary Submit Berita Acara for witness sign-off
// @Description Submits monthly Berita Acara with employee signature and assigns a witness for approval.
// @Tags Signatures
// @Accept json
// @Produce json
// @Param request body request.SubmitBeritaAcaraRequest true "Submission payload"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/berita-acara/submit [post]
func (s *Server) SubmitBeritaAcara(c *gin.Context) {
	if s.SignatureSvc == nil {
		RespondError(c, http.StatusInternalServerError, errSignatureSvcNotInit)
		return
	}

	var req request.SubmitBeritaAcaraRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, errInvalidParamPrefix+err.Error())
		return
	}

	userID := currentUserID(c)
	meta := extractAuditMeta(c)
	resp, err := s.SignatureSvc.SubmitBeritaAcara(c.Request.Context(), userID, &req, meta)
	if err != nil {
		var userErr *domain.UserError
		if errors.As(err, &userErr) {
			RespondError(c, domain.MapErrorToHTTPStatus(userErr.Err), userErr.Msg)
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, http.StatusOK, resp)
}

// ReviewBeritaAcara godoc
// @Summary Review Berita Acara submission context for designated witness
// @Description Provides full verification context for witness review, records idempotent first-view tracking, and returns cryptographic signing challenge.
// @Tags Signatures
// @Produce json
// @Param id path string true "Submission ID"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/berita-acara/{id}/review [get]
func (s *Server) ReviewBeritaAcara(c *gin.Context) {
	if s.SignatureSvc == nil {
		RespondError(c, http.StatusInternalServerError, errSignatureSvcNotInit)
		return
	}

	submissionID := c.Param("id")
	if submissionID == "" {
		RespondError(c, http.StatusBadRequest, "Submission ID wajib diisi")
		return
	}

	witnessID := currentUserID(c)
	meta := extractAuditMeta(c)
	resp, err := s.SignatureSvc.GetBeritaAcaraReview(c.Request.Context(), witnessID, submissionID, meta)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			RespondError(c, http.StatusForbidden, "unauthorized access to witness review")
			return
		}
		var userErr *domain.UserError
		if errors.As(err, &userErr) {
			RespondError(c, domain.MapErrorToHTTPStatus(userErr.Err), userErr.Msg)
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, err.Error())
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, http.StatusOK, resp)
}

// WitnessAction godoc
// @Summary Approve or reject Berita Acara as witness
// @Description Allows the designated witness to sign/approve or reject with revision notes.
// @Tags Signatures
// @Accept json
// @Produce json
// @Param id path string true "Submission ID"
// @Param request body request.WitnessActionRequest true "Witness decision"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/berita-acara/{id}/witness-action [post]
func (s *Server) WitnessAction(c *gin.Context) {
	if s.SignatureSvc == nil {
		RespondError(c, http.StatusInternalServerError, errSignatureSvcNotInit)
		return
	}

	submissionID := c.Param("id")
	if submissionID == "" {
		RespondError(c, http.StatusBadRequest, "Submission ID wajib diisi")
		return
	}

	var req request.WitnessActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, errInvalidParamPrefix+err.Error())
		return
	}

	witnessID := currentUserID(c)
	meta := extractAuditMeta(c)
	resp, err := s.SignatureSvc.WitnessAction(c.Request.Context(), witnessID, submissionID, &req, meta)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			RespondError(c, http.StatusForbidden, "unauthorized witness action")
			return
		}
		var userErr *domain.UserError
		if errors.As(err, &userErr) {
			RespondError(c, domain.MapErrorToHTTPStatus(userErr.Err), userErr.Msg)
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, http.StatusOK, resp)
}

// RevokeBeritaAcara godoc
// @Summary Revoke Berita Acara submission
// @Description Revokes a pending or signed Berita Acara back to draft.
// @Tags Signatures
// @Param id path string true "Submission ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/berita-acara/{id}/revoke [post]
func (s *Server) RevokeBeritaAcara(c *gin.Context) {
	if s.SignatureSvc == nil {
		RespondError(c, http.StatusInternalServerError, errSignatureSvcNotInit)
		return
	}

	submissionID := c.Param("id")
	if submissionID == "" {
		RespondError(c, http.StatusBadRequest, "Submission ID wajib diisi")
		return
	}

	userID := currentUserID(c)
	meta := extractAuditMeta(c)
	if err := s.SignatureSvc.RevokeBeritaAcara(c.Request.Context(), userID, submissionID, meta); err != nil {
		var userErr *domain.UserError
		if errors.As(err, &userErr) {
			RespondError(c, domain.MapErrorToHTTPStatus(userErr.Err), userErr.Msg)
			return
		}
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondMessage(c, http.StatusOK, "Pengajuan Berita Acara berhasil dibatalkan")
}

// GetBeritaAcaraSubmissionStatus godoc
// @Summary Get Berita Acara sign-off status
// @Description Retrieves the sign-off lifecycle state of a monthly Berita Acara.
// @Tags Signatures
// @Produce json
// @Param month query int true "Month (1-12)"
// @Param year query int true "Year"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/berita-acara/status [get]
func (s *Server) GetBeritaAcaraSubmissionStatus(c *gin.Context) {
	if s.SignatureSvc == nil {
		RespondError(c, http.StatusInternalServerError, errSignatureSvcNotInit)
		return
	}

	month, _ := strconv.Atoi(c.Query("month"))
	year, _ := strconv.Atoi(c.Query("year"))
	if month < 1 || month > 12 || year < 2000 {
		RespondError(c, http.StatusBadRequest, "Bulan atau tahun tidak valid")
		return
	}

	userID := currentUserID(c)
	resp, err := s.SignatureSvc.GetBeritaAcaraSubmission(c.Request.Context(), userID, month, year)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, http.StatusOK, resp)
}

// ListPendingWitnessRequests godoc
// @Summary List pending witness review requests
// @Description Lists all Berita Acara submissions awaiting the authenticated user's signature as a witness.
// @Tags Signatures
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/berita-acara/witness-requests [get]
func (s *Server) ListPendingWitnessRequests(c *gin.Context) {
	if s.SignatureSvc == nil {
		RespondError(c, http.StatusInternalServerError, errSignatureSvcNotInit)
		return
	}

	witnessID := currentUserID(c)
	list, err := s.SignatureSvc.ListPendingWitnessRequests(c.Request.Context(), witnessID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(c, http.StatusOK, list)
}

// PreviewBeritaAcaraPDF godoc
// @Summary Stream instant PDF preview of Berita Acara
// @Description Renders and streams on-the-fly Berita Acara PDF for witness/applicant review modal.
// @Tags Signatures
// @Produce application/pdf
// @Param id path string true "Berita Acara Submission ID"
// @Success 200 {file} binary "PDF document stream"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/berita-acara/{id}/preview/pdf [get]
func (s *Server) PreviewBeritaAcaraPDF(c *gin.Context) {
	submissionID := c.Param("id")
	if submissionID == "" {
		RespondError(c, http.StatusBadRequest, "Submission ID wajib diisi")
		return
	}

	if s.SubmissionRepo == nil {
		RespondError(c, http.StatusInternalServerError, "Submission repository not initialized")
		return
	}

	sub, err := s.SubmissionRepo.FindBeritaAcaraByID(c.Request.Context(), submissionID)
	if err != nil || sub == nil {
		RespondError(c, http.StatusNotFound, "Pengajuan Berita Acara tidak ditemukan")
		return
	}

	callerID := currentUserID(c)
	callerRole := currentUserRole(c)
	isWitness := sub.WitnessID == callerID
	isApplicant := sub.UserID == callerID
	isAdmin := callerRole == models.RoleAdmin

	if !isWitness && !isApplicant && !isAdmin {
		RespondError(c, http.StatusForbidden, "Akses ditolak: Anda tidak memiliki wewenang untuk menelaah dokumen ini")
		return
	}

	empName := ""
	empID := ""
	deptName := ""
	groupName := ""

	user := sub.User
	if user == nil && s.UserRepo != nil {
		user, _ = s.UserRepo.FindByIDWithDetails(c.Request.Context(), sub.UserID)
	}
	if user != nil {
		empName = user.Name
		if empName == "" {
			empName = user.Username
		}
		empID = user.BniID
		deptName = user.Department
		if user.DepartmentRel != nil && user.DepartmentRel.Name != "" {
			deptName = user.DepartmentRel.Name
		}
		groupName = user.GroupName
	}

	witName := ""
	if sub.Witness != nil && sub.Witness.Name != "" {
		witName = sub.Witness.Name
	} else if sub.WitnessID != 0 && s.UserRepo != nil {
		if wUser, wErr := s.UserRepo.FindByID(c.Request.Context(), sub.WitnessID); wErr == nil && wUser != nil {
			witName = wUser.Name
			if witName == "" {
				witName = wUser.Username
			}
		}
	}

	var items []services.BeritaAcaraItem
	if s.ActivityRepo != nil {
		filter := repository.ActivityFilter{
			Month:     &sub.Month,
			Year:      &sub.Year,
			SortOrder: "asc",
			IsAll:     true,
		}
		activities, _, aErr := s.ActivityRepo.ListActiveByUser(c.Request.Context(), sub.UserID, filter)
		if aErr == nil {
			for _, a := range activities {
				if a.IsBeritaAcara {
					items = append(items, services.BeritaAcaraItem{
						Date:      a.Date,
						Day:       services.IndonesianDayName(a.Date.Weekday()),
						StartTime: a.StartTime,
						EndTime:   a.EndTime,
						Notes:     a.BeritaAcaraNotes,
					})
				}
			}
		}
	}

	var empSigBytes, witSigBytes []byte
	if s.Storage != nil {
		if user != nil && user.SignatureFileKey != "" {
			empSigBytes, _ = s.Storage.Download(c.Request.Context(), user.SignatureFileKey)
		}
		if sub.Witness != nil && sub.Witness.SignatureFileKey != "" {
			witSigBytes, _ = s.Storage.Download(c.Request.Context(), sub.Witness.SignatureFileKey)
		} else if sub.WitnessID != 0 && s.UserRepo != nil {
			if wUser, wErr := s.UserRepo.FindByID(c.Request.Context(), sub.WitnessID); wErr == nil && wUser != nil && wUser.SignatureFileKey != "" {
				witSigBytes, _ = s.Storage.Download(c.Request.Context(), wUser.SignatureFileKey)
			}
		}
	}

	var qrBytes []byte
	if sub.VerificationToken != "" {
		baseURL := "https://timesheet.local"
		if s.Cfg != nil && s.Cfg.AppBaseURL != "" {
			baseURL = strings.TrimRight(s.Cfg.AppBaseURL, "/")
		}
		qrURL := fmt.Sprintf("%s/verify/doc/%s", baseURL, sub.VerificationToken)
		qrBytes, _ = services.GenerateQRCodePNG(qrURL, 120)
	}

	payload := services.BeritaAcaraPayload{
		EmployeeName:           empName,
		NPP:                    empID,
		Department:             deptName,
		GroupName:              groupName,
		WitnessName:            witName,
		TeamLeadName:           "",
		Items:                  items,
		EmployeeSignatureBytes: empSigBytes,
		WitnessSignatureBytes:  witSigBytes,
		QRCodeBytes:            qrBytes,
		IsSigned:               sub.Status == models.SubmissionStatusSigned,
	}

	pdfBytes, err := services.BuildBeritaAcaraPDF(c.Request.Context(), &payload)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "Gagal membuat dokumen PDF: "+err.Error())
		return
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", `inline; filename="Berita_Acara_Preview.pdf"`)
	c.Header("Content-Length", strconv.Itoa(len(pdfBytes)))
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}
