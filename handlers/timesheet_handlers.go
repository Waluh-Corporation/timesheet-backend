package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"timesheet-backend/dto/request"
	"timesheet-backend/dto/response"
	"timesheet-backend/internal/domain"
	"timesheet-backend/internal/repository"
	"timesheet-backend/mailer"
	"timesheet-backend/models"
	"timesheet-backend/push"
	"timesheet-backend/queue"
	"timesheet-backend/storage"
)

// GenerateTimesheet godoc
// @Summary Generate timesheet
// @Description Generates monthly timesheet Excel file. If background worker and queue are configured, enqueues an asynchronous generation job and sends download link to email upon completion. Otherwise, generates synchronously and streams the file.
// @Tags Timesheet
// @Security BearerAuth
// @Accept json
// @Produce application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/json
// @Param request body request.GenerateRequest true "Generation parameters"
// @Success 200 {file} file "Excel timesheet (.xlsx) (synchronous mode)"
// @Success 202 {object} response.TimesheetJobResponse "Job enqueued asynchronously"
// @Failure 400 {object} response.ErrorResponse "Template or company mapping missing"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Failure 409 {object} response.ErrorResponse "Generation request in-flight or debounce active"
// @Failure 500 {object} response.ErrorResponse "Generation failed"
// @Router /api/v1/timesheet/generate [post]
func (s *Server) GenerateTimesheet(c *gin.Context) {
	req, ok := s.bindAndValidateGenerateRequest(c)
	if !ok {
		return
	}

	uid := currentUserID(c)
	user, ok := s.validateUserWithCompany(c, uid)
	if !ok {
		return
	}

	if !s.checkUserHasActivities(c, uid, req.Month, req.Year) {
		return
	}

	if s.handleAsyncGeneration(c, user, req) {
		return
	}

	s.handleSyncGenerationFallback(c, uid, req)
}

func (s *Server) bindAndValidateGenerateRequest(c *gin.Context) (*request.GenerateRequest, bool) {
	var req request.GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return nil, false
	}

	if req.Month < 1 || req.Month > 12 || req.Year < 2000 || req.Year > 2100 {
		RespondError(c, http.StatusBadRequest, "Invalid month or year")
		return nil, false
	}

	if currentUserRole(c) != models.RoleAdmin {
		if err := domain.ValidateTimesheetPeriod(req.Month, req.Year, time.Now(), jakarta()); err != nil {
			RespondError(c, http.StatusBadRequest, err.Error())
			return nil, false
		}
	}
	return &req, true
}

func (s *Server) validateUserWithCompany(c *gin.Context, uid uint) (*models.User, bool) {
	userRepo := s.getUserRepository()
	if userRepo == nil {
		RespondError(c, http.StatusInternalServerError, "user repository not initialized")
		return nil, false
	}

	user, err := userRepo.FindByIDWithDetails(reqContext(c), uid)
	if err != nil || user == nil {
		RespondError(c, http.StatusNotFound, "user not found")
		return nil, false
	}

	if !hasUserCompany(user) {
		RespondError(c, http.StatusBadRequest, "User has no company assigned")
		return nil, false
	}
	return user, true
}

func hasUserCompany(user *models.User) bool {
	if user.CompanyRel != nil && user.CompanyRel.Code != "" {
		return true
	}
	if user.CompanyID != nil && *user.CompanyID != 0 {
		return true
	}
	return strings.TrimSpace(user.Company) != ""
}

func (s *Server) checkUserHasActivities(c *gin.Context, uid uint, month, year int) bool {
	actRepo := s.getActivityRepository()
	if actRepo == nil {
		return true
	}
	filter := repository.ActivityFilter{
		Month: &month,
		Year:  &year,
		IsAll: true,
	}
	activities, _, aerr := actRepo.ListActiveByUser(reqContext(c), uid, filter)
	if aerr != nil {
		RespondError(c, http.StatusInternalServerError, "failed to check activities: "+aerr.Error())
		return false
	}
	if len(activities) == 0 {
		RespondError(c, http.StatusBadRequest, "No activities recorded for this period")
		return false
	}
	return true
}

func (s *Server) handleAsyncGeneration(c *gin.Context, user *models.User, req *request.GenerateRequest) bool {
	jobRepo := s.getJobRepo()
	queueClient := s.getQueueClient()
	if queueClient == nil || jobRepo == nil {
		return false
	}

	if !s.acquireGenerationLock(c, user.ID, req.Year, req.Month) {
		return true
	}

	if inFlight, err := jobRepo.HasInFlightJob(reqContext(c), user.ID, req.Month, req.Year); err == nil && inFlight {
		RespondError(c, http.StatusConflict, "A generation request for this period is already queued or processing. Please check your email or wait a moment.")
		return true
	}

	if s.tryHandleCachedJob(c, jobRepo, user, req.Month, req.Year) {
		return true
	}

	return s.enqueueNewGenerationJob(c, jobRepo, queueClient, user.ID, req.Month, req.Year)
}

func (s *Server) acquireGenerationLock(c *gin.Context, uid uint, year, month int) bool {
	rdb := s.getRedisClient()
	if rdb == nil {
		return true
	}
	lockKey := fmt.Sprintf("timesheet:lock:%d:%d:%02d", uid, year, month)
	acquired, lerr := rdb.SetNX(reqContext(c), lockKey, "locked", 10*time.Second).Result()
	if lerr == nil && !acquired {
		RespondError(c, http.StatusConflict, "A generation request for this period is currently being processed. Please wait a moment.")
		return false
	}
	return true
}

func (s *Server) tryHandleCachedJob(c *gin.Context, jobRepo repository.JobRepository, user *models.User, month, year int) bool {
	activeJob, err := jobRepo.FindActiveCompletedJob(reqContext(c), user.ID, month, year)
	if err != nil || activeJob == nil || activeJob.FileKey == "" {
		return false
	}

	if !isJobCacheValid(reqContext(c), activeJob, user, s.getActivityRepository(), s.getStorage(), month, year) {
		return false
	}

	return s.reissueCachedJob(c, jobRepo, activeJob, user, month, year)
}

func isJobCacheValid(ctx context.Context, job *models.TimesheetJob, user *models.User, actRepo repository.ActivityRepository, storageSvc storage.StorageService, month, year int) bool {
	if actRepo != nil {
		latestActUpdate, aerr := actRepo.GetLatestActivityUpdateTime(ctx, user.ID, month, year)
		if aerr == nil && !latestActUpdate.IsZero() && latestActUpdate.After(job.CreatedAt) {
			return false
		}
	}

	if user.UpdatedAt.After(job.CreatedAt) {
		return false
	}

	if storageSvc != nil {
		exists, serr := storageSvc.FileExists(ctx, job.FileKey)
		if serr != nil || !exists {
			return false
		}
	}
	return true
}

func (s *Server) reissueCachedJob(c *gin.Context, jobRepo repository.JobRepository, activeJob *models.TimesheetJob, user *models.User, month, year int) bool {
	newToken := uuid.New().String()
	maxDownloads := 3
	if s.Cfg != nil && s.Cfg.TimesheetDownloadMaxQuota > 0 {
		maxDownloads = s.Cfg.TimesheetDownloadMaxQuota
	}
	retentionDays := 7
	if s.Cfg != nil && s.Cfg.S3RetentionDays > 0 {
		retentionDays = s.Cfg.S3RetentionDays
	}
	tokenExpiry := time.Now().Add(time.Duration(retentionDays) * 24 * time.Hour)

	if rerr := jobRepo.RenewDownloadToken(reqContext(c), activeJob.ID, newToken, maxDownloads, tokenExpiry); rerr != nil {
		return false
	}

	if rdb := s.getRedisClient(); rdb != nil {
		tokenKey := fmt.Sprintf("timesheet:token:%s:count", newToken)
		_ = rdb.Set(reqContext(c), tokenKey, 0, time.Duration(retentionDays)*24*time.Hour).Err()
	}

	baseURL := s.publicBaseURL(c)
	if s.Cfg != nil && s.Cfg.AppBaseURL != "" {
		baseURL = s.Cfg.AppBaseURL
	}
	downloadURL := fmt.Sprintf("%s/api/v1/timesheet/downloads/%s", baseURL, newToken)

	s.dispatchReissueNotifications(user, downloadURL, tokenExpiry, month, year)

	activeJob.DownloadURL = downloadURL
	activeJob.DownloadToken = &newToken
	activeJob.DownloadCount = 0
	activeJob.MaxDownloads = maxDownloads
	activeJob.TokenExpiresAt = &tokenExpiry

	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusOK,
		"status":  "success",
		"message": "Timesheet retrieved from cache and new download link re-issued. Please check your email.",
		"data":    response.ToTimesheetJobResponse(activeJob),
	})
	return true
}

func (s *Server) dispatchReissueNotifications(user *models.User, downloadURL string, tokenExpiry time.Time, month, year int) {
	if s.Mailer != nil && user.Email != "" {
		compName := user.Company
		if user.CompanyRel != nil {
			compName = user.CompanyRel.Name
		}
		period := mailer.FormatMonthYearIndonesian(month, year)
		filename := fmt.Sprintf("Timesheet_%s_%02d_%04d.xlsx", user.Username, month, year)
		toEmail := user.Email
		toUsername := user.Username
		m := s.Mailer
		go func() {
			if merr := m.SendTimesheetReadyEmail(toEmail, toUsername, compName, period, filename, downloadURL, tokenExpiry); merr != nil {
				slog.Error("failed to send re-issued timesheet ready email", "error", merr, "email", toEmail)
			}
		}()
	}

	if s.Push != nil {
		pushTitle := "Timesheet Telah Siap"
		pushBody := fmt.Sprintf("Timesheet periode %02d/%04d Anda telah siap diunduh.", month, year)
		s.Push.SendToUser(user.ID, push.Payload{
			Title: pushTitle,
			Body:  pushBody,
			URL:   downloadURL,
		})
	}
}

func (s *Server) enqueueNewGenerationJob(c *gin.Context, jobRepo repository.JobRepository, queueClient queue.QueueClient, uid uint, month, year int) bool {
	jobID := uuid.New().String()
	job := &models.TimesheetJob{
		ID:        jobID,
		UserID:    uid,
		Month:     month,
		Year:      year,
		Status:    models.JobStatusQueued,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := jobRepo.Create(reqContext(c), job); err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to create generation job: "+err.Error())
		return true
	}

	if err := queueClient.EnqueueTimesheetJob(reqContext(c), jobID, uid, month, year); err != nil {
		_ = jobRepo.UpdateStatus(reqContext(c), jobID, models.JobStatusFailed, "", "", "failed to enqueue task: "+err.Error(), nil)
		RespondError(c, http.StatusInternalServerError, "failed to enqueue generation task: "+err.Error())
		return true
	}

	c.JSON(http.StatusAccepted, gin.H{
		"code":    http.StatusAccepted,
		"status":  "success",
		"message": "Timesheet generation job queued successfully. The download link will be sent to your email once ready.",
		"data":    response.ToTimesheetJobResponse(job),
	})
	return true
}

func (s *Server) handleSyncGenerationFallback(c *gin.Context, uid uint, req *request.GenerateRequest) {
	tsSvc := s.getTimesheetService()
	if tsSvc == nil {
		RespondError(c, http.StatusInternalServerError, "timesheet service not initialized")
		return
	}

	out, filename, err := tsSvc.GenerateWorkbook(reqContext(c), uid, req.Month, req.Year)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(c, http.StatusNotFound, "user not found")
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		RespondError(c, http.StatusInternalServerError, "generation failed: "+err.Error())
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", out)
}

// GetTimesheetJob godoc
// @Summary Get timesheet generation job status
// @Description Checks status of an asynchronous timesheet generation job.
// @Tags Timesheet
// @Security BearerAuth
// @Produce json
// @Param id path string true "Job ID (UUID)"
// @Success 200 {object} response.TimesheetJobResponse
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "Job not found"
// @Router /api/v1/timesheet/jobs/{id} [get]
func (s *Server) GetTimesheetJob(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		RespondError(c, http.StatusBadRequest, "job id is required")
		return
	}

	jobRepo := s.getJobRepo()
	if jobRepo == nil {
		RespondError(c, http.StatusInternalServerError, "job repository not initialized")
		return
	}

	job, err := jobRepo.FindActiveByIDAndUser(reqContext(c), id, currentUserID(c))
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to query job: "+err.Error())
		return
	}
	if job == nil {
		RespondError(c, http.StatusNotFound, "job not found")
		return
	}

	RespondSuccess(c, http.StatusOK, response.ToTimesheetJobResponse(job))
}

// ListTimesheetJobs godoc
// @Summary List user's timesheet generation jobs
// @Description Returns recent timesheet generation jobs for the authenticated user.
// @Tags Timesheet
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page number (default 1)"
// @Param limit query int false "Page size (default 10)"
// @Success 200 {object} response.PaginatedResponse
// @Router /api/v1/timesheet/jobs [get]
func (s *Server) ListTimesheetJobs(c *gin.Context) {
	jobRepo := s.getJobRepo()
	if jobRepo == nil {
		RespondError(c, http.StatusInternalServerError, "job repository not initialized")
		return
	}

	page := queryIntDefault(c, "page", 1)
	if page < 1 {
		page = 1
	}
	limit := queryIntDefault(c, "limit", 10)
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	jobs, total, err := jobRepo.ListByUser(reqContext(c), currentUserID(c), limit, offset)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to list jobs: "+err.Error())
		return
	}

	resps := make([]response.TimesheetJobResponse, 0, len(jobs))
	for i := range jobs {
		resps = append(resps, response.ToTimesheetJobResponse(&jobs[i]))
	}

	RespondPaginated(c, http.StatusOK, resps, page, limit, total)
}

// DownloadTimesheetByToken godoc
// @Summary Download generated timesheet via secure token
// @Description Validates magic download token, enforces download limits, and redirects directly to S3 storage. Supports HEAD for link verifiers.
// @Tags Timesheet
// @Produce octet-stream
// @Param token path string true "Magic Download Token"
// @Success 200 "Link verified successfully (HEAD request)"
// @Success 302 {string} string "Redirects directly to S3 Presigned URL"
// @Failure 400 {object} response.ErrorResponse "Invalid download token"
// @Failure 404 {object} response.ErrorResponse "Download token not found"
// @Failure 410 {object} response.ErrorResponse "Download token expired or quota exceeded"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /api/v1/timesheet/downloads/{token} [get]
func (s *Server) DownloadTimesheetByToken(c *gin.Context) {
	token := strings.TrimSpace(c.Param("token"))
	if len(token) < 10 {
		RespondError(c, http.StatusBadRequest, "Invalid download token")
		return
	}

	jobRepo := s.getJobRepo()
	if jobRepo == nil {
		RespondError(c, http.StatusInternalServerError, "Job repository not initialized")
		return
	}

	ctx := reqContext(c)
	job, err := jobRepo.FindByDownloadToken(ctx, token)
	if err != nil || job == nil {
		RespondError(c, http.StatusNotFound, "Download token not found or invalid")
		return
	}

	maxQuota := s.configuredDownloadQuota()
	if !validateDownloadTokenQuota(c, job, maxQuota) {
		return
	}

	if c.Request.Method == http.MethodHead {
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Status(http.StatusOK)
		return
	}

	if !s.recordRedisDownloadQuota(ctx, token, job, maxQuota) {
		RespondError(c, http.StatusGone, "Download quota exceeded. Please generate a new timesheet to get an updated link.")
		return
	}

	if err := jobRepo.IncrementDownloadCount(ctx, job.ID); err != nil {
		slog.Error("failed to increment download count in db", "error", err, "job_id", job.ID)
	}

	s.redirectPresignedS3Download(c, ctx, job.FileKey)
}

func (s *Server) configuredDownloadQuota() int {
	if s.Cfg != nil && s.Cfg.TimesheetDownloadMaxQuota > 0 {
		return s.Cfg.TimesheetDownloadMaxQuota
	}
	return 3
}

func validateDownloadTokenQuota(c *gin.Context, job *models.TimesheetJob, maxQuota int) bool {
	if job.TokenExpiresAt != nil && job.TokenExpiresAt.Before(time.Now()) {
		RespondError(c, http.StatusGone, "Download token has expired. Please generate a new timesheet to get an updated link.")
		return false
	}

	maxDownloads := job.MaxDownloads
	if maxDownloads <= 0 {
		maxDownloads = maxQuota
	}

	if job.DownloadCount >= maxDownloads {
		RespondError(c, http.StatusGone, "Download quota exceeded. Please generate a new timesheet to get an updated link.")
		return false
	}
	return true
}

func (s *Server) recordRedisDownloadQuota(ctx context.Context, token string, job *models.TimesheetJob, maxQuota int) bool {
	rdb := s.getRedisClient()
	if rdb == nil {
		return true
	}

	maxDownloads := job.MaxDownloads
	if maxDownloads <= 0 {
		maxDownloads = maxQuota
	}

	tokenKey := fmt.Sprintf("timesheet:token:%s:count", token)
	exists, _ := rdb.Exists(ctx, tokenKey).Result()
	if exists == 0 {
		ttl := 7 * 24 * time.Hour
		if job.TokenExpiresAt != nil && job.TokenExpiresAt.After(time.Now()) {
			ttl = time.Until(*job.TokenExpiresAt)
		}
		_ = rdb.Set(ctx, tokenKey, job.DownloadCount, ttl).Err()
	}

	newCount, err := rdb.Incr(ctx, tokenKey).Result()
	return err != nil || newCount <= int64(maxDownloads)
}

func (s *Server) redirectPresignedS3Download(c *gin.Context, ctx context.Context, fileKey string) {
	storageSvc := s.getStorage()
	if storageSvc == nil {
		RespondError(c, http.StatusInternalServerError, "Storage service not initialized")
		return
	}

	filename := extractAttachmentFilename(fileKey)
	presignedURL, err := storageSvc.GetPresignedDownloadURLWithFilename(ctx, fileKey, filename, 15*time.Minute)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "Failed to generate download link: "+err.Error())
		return
	}

	c.Redirect(http.StatusFound, presignedURL)
}

func extractAttachmentFilename(fileKey string) string {
	filename := path.Base(fileKey)
	if idx := strings.Index(filename, "_"); idx != -1 && idx+1 < len(filename) {
		filename = filename[idx+1:]
	}
	if filename == "" || filename == "." {
		return "timesheet.xlsx"
	}
	return filename
}
