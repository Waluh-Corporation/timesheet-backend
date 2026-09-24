package queue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"timesheet-backend/config"
	"timesheet-backend/internal/repository"
	"timesheet-backend/internal/service"
	"timesheet-backend/mailer"
	"timesheet-backend/models"
	"timesheet-backend/push"
	"timesheet-backend/storage"
)

// WorkerServer processes asynchronous timesheet generation jobs from the Redis task queue.
type WorkerServer struct {
	server      *asynq.Server
	mux         *asynq.ServeMux
	cfg         *config.Config
	jobRepo     repository.JobRepository
	userRepo    repository.UserRepository
	tsSvc       service.TimesheetService
	storage     storage.StorageService
	mailer      *mailer.Mailer
	pushSvc     *push.Service
	redisClient *redis.Client
	sem         chan struct{}
}

// NewWorkerServer constructs a WorkerServer with configured concurrency and resource limiters.
func NewWorkerServer(
	cfg *config.Config,
	jobRepo repository.JobRepository,
	userRepo repository.UserRepository,
	tsSvc service.TimesheetService,
	storage storage.StorageService,
	m *mailer.Mailer,
	pushSvc *push.Service,
) *WorkerServer {
	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}

	concurrency := cfg.MailerWorkerCount
	if concurrency <= 0 {
		concurrency = 5
	}

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				"default": 1,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				log.Printf("[worker] task %s failed: %v", task.Type(), err)
			}),
		},
	)

	maxConcurrent := cfg.ExcelMaxConcurrentJobs
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}

	w := &WorkerServer{
		server:   srv,
		mux:      asynq.NewServeMux(),
		cfg:      cfg,
		jobRepo:  jobRepo,
		userRepo: userRepo,
		tsSvc:    tsSvc,
		storage:  storage,
		mailer:   m,
		pushSvc:  pushSvc,
		sem:      make(chan struct{}, maxConcurrent),
	}

	w.mux.HandleFunc(TypeTimesheetGenerate, w.handleTimesheetGenerate)
	return w
}

// Start begins processing tasks in the background.
func (w *WorkerServer) Start() error {
	log.Printf("[worker] starting Asynq worker server on Redis %s (concurrency: %d, max excel: %d)",
		w.cfg.RedisAddr, w.cfg.MailerWorkerCount, w.cfg.ExcelMaxConcurrentJobs)
	return w.server.Start(w.mux)
}

// Shutdown gracefully stops the background task worker.
func (w *WorkerServer) Shutdown() {
	w.server.Shutdown()
}

// SetRedisClient attaches a redis.Client for managing fast-path counters and locks.
func (w *WorkerServer) SetRedisClient(client *redis.Client) {
	w.redisClient = client
}

// ProcessTaskDirect directly handles a task payload synchronously (useful for isolated unit testing).
func (w *WorkerServer) ProcessTaskDirect(ctx context.Context, payload GenerateTimesheetPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	task := asynq.NewTask(TypeTimesheetGenerate, body)
	return w.handleTimesheetGenerate(ctx, task)
}

func (w *WorkerServer) handleTimesheetGenerate(ctx context.Context, t *asynq.Task) error {
	var p GenerateTimesheetPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	// 1. Mark job as processing
	_ = w.jobRepo.UpdateStatus(ctx, p.JobID, models.JobStatusProcessing, "", "", "", nil)

	// 2. Acquire semaphore to throttle CPU & RAM intensive Excelize generation
	select {
	case w.sem <- struct{}{}:
		defer func() { <-w.sem }()
	case <-ctx.Done():
		errMsg := "context canceled waiting for generator slot"
		_ = w.jobRepo.UpdateStatus(ctx, p.JobID, models.JobStatusFailed, "", "", errMsg, nil)
		return ctx.Err()
	}

	// 3. Generate workbook
	out, filename, err := w.tsSvc.GenerateWorkbook(ctx, p.UserID, p.Month, p.Year)
	if err != nil {
		errMsg := fmt.Sprintf("generation error: %v", err)
		_ = w.jobRepo.UpdateStatus(ctx, p.JobID, models.JobStatusFailed, "", "", errMsg, nil)
		return err
	}

	// 4. Upload to S3
	fileKey := fmt.Sprintf("timesheets/%d/%s_%s", p.UserID, p.JobID, filename)
	contentType := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	if err := w.storage.Upload(ctx, fileKey, bytes.NewReader(out), contentType); err != nil {
		errMsg := fmt.Sprintf("s3 upload error: %v", err)
		_ = w.jobRepo.UpdateStatus(ctx, p.JobID, models.JobStatusFailed, "", "", errMsg, nil)
		return err
	}

	// 5. Generate magic download token with retention period expiration and max 3 downloads
	downloadToken := uuid.New().String()
	maxDownloads := 3
	retentionDays := w.cfg.S3RetentionDays
	if retentionDays <= 0 {
		retentionDays = 7
	}
	expiryDuration := time.Duration(retentionDays) * 24 * time.Hour
	expiresAt := time.Now().Add(expiryDuration)

	baseURL := "http://localhost:8080"
	if w.cfg != nil && w.cfg.AppBaseURL != "" {
		baseURL = w.cfg.AppBaseURL
	}
	downloadURL := fmt.Sprintf("%s/api/v1/timesheet/downloads/%s", baseURL, downloadToken)

	// Initialize fast-path counter in Redis
	if w.redisClient != nil {
		tokenKey := fmt.Sprintf("timesheet:token:%s:count", downloadToken)
		_ = w.redisClient.Set(ctx, tokenKey, 0, expiryDuration).Err()
	}

	// 6. Update job status to completed with magic download token
	if err := w.jobRepo.UpdateStatusWithToken(ctx, p.JobID, models.JobStatusCompleted, fileKey, downloadURL, "", &expiresAt, downloadToken, maxDownloads, &expiresAt); err != nil {
		log.Printf("[worker] failed to update job %s to completed: %v", p.JobID, err)
	}

	// 7. Dispatch Web Push notification
	if w.pushSvc != nil {
		pushTitle := "Timesheet Telah Siap"
		pushBody := fmt.Sprintf("Timesheet periode %02d/%04d Anda telah selesai diproses dan siap diunduh.", p.Month, p.Year)
		w.pushSvc.SendToUser(p.UserID, push.Payload{
			Title: pushTitle,
			Body:  pushBody,
			URL:   downloadURL,
		})
	}

	// 8. Dispatch Email with presigned download link
	if w.mailer != nil {
		user, uerr := w.userRepo.FindByIDWithDetails(ctx, p.UserID)
		if uerr == nil && user != nil && user.Email != "" {
			compName := ""
			if user.CompanyRel != nil {
				compName = user.CompanyRel.Name
			} else {
				compName = user.Company
			}
			period := mailer.FormatMonthYearIndonesian(p.Month, p.Year)
			if mErr := w.mailer.SendTimesheetReadyEmail(user.Email, user.Username, compName, period, filename, downloadURL, expiresAt); mErr != nil {
				log.Printf("[worker] failed to send timesheet ready email to %s: %v", user.Email, mErr)
			}
		}
	}

	return nil
}
