package queue

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
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
	userRepo    repository.UserAccountRepository
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
	userRepo repository.UserAccountRepository,
	tsSvc service.TimesheetService,
	storage storage.StorageService,
	m *mailer.Mailer,
	pushSvc *push.Service,
) *WorkerServer {
	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Username: cfg.RedisUsername,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}
	if cfg.RedisTLSEnabled {
		redisOpt.TLSConfig = &tls.Config{
			InsecureSkipVerify: cfg.RedisTLSSkipVerify, //nolint:gosec // configurable for internal TLS
		}
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

	// 2. Acquire semaphore to throttle CPU & RAM intensive generation
	release, err := w.acquireSlot(ctx, p.JobID)
	if err != nil {
		return err
	}
	defer release()

	// 3. Generate workbook
	out, filename, err := w.tsSvc.GenerateWorkbook(ctx, p.UserID, p.Month, p.Year)
	if err != nil {
		errMsg := fmt.Sprintf("generation error: %v", err)
		_ = w.jobRepo.UpdateStatus(ctx, p.JobID, models.JobStatusFailed, "", "", errMsg, nil)
		return err
	}

	// 3b. Package timesheet and optional Berita Acara
	finalBytes, finalFilename, contentType := w.packageTimesheetOutput(ctx, p.UserID, p.Month, p.Year, out, filename)

	// 4. Upload to S3
	fileKey := fmt.Sprintf("timesheets/%d/%s_%s", p.UserID, p.JobID, finalFilename)
	if err := w.storage.Upload(ctx, fileKey, bytes.NewReader(finalBytes), contentType); err != nil {
		errMsg := fmt.Sprintf("s3 upload error: %v", err)
		_ = w.jobRepo.UpdateStatus(ctx, p.JobID, models.JobStatusFailed, "", "", errMsg, nil)
		return err
	}

	// 5. Generate magic download token and update job
	downloadToken, maxDownloads, expiresAt, downloadURL := w.resolveDownloadParams(ctx)
	if err := w.jobRepo.UpdateStatusWithToken(ctx, p.JobID, models.JobStatusCompleted, fileKey, downloadURL, "", &expiresAt, downloadToken, maxDownloads, &expiresAt); err != nil {
		log.Printf("[worker] failed to update job %s to completed: %v", p.JobID, err)
	}

	// 6. Dispatch notifications
	w.dispatchNotifications(ctx, p.UserID, p.Month, p.Year, downloadURL, finalFilename, expiresAt)
	return nil
}

func (w *WorkerServer) acquireSlot(ctx context.Context, jobID string) (func(), error) {
	select {
	case w.sem <- struct{}{}:
		return func() { <-w.sem }, nil
	case <-ctx.Done():
		errMsg := "context canceled waiting for generator slot"
		_ = w.jobRepo.UpdateStatus(ctx, jobID, models.JobStatusFailed, "", "", errMsg, nil)
		return nil, ctx.Err()
	}
}

func (w *WorkerServer) packageTimesheetOutput(ctx context.Context, userID uint, month, year int, out []byte, filename string) ([]byte, string, string) {
	finalBytes := out
	finalFilename := filename
	contentType := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

	baBytes, baFilename, baErr := w.tsSvc.GenerateBeritaAcara(ctx, userID, month, year)
	if baErr != nil {
		log.Printf("[worker] error checking/generating berita acara for user %d: %v", userID, baErr)
	}

	if len(baBytes) == 0 {
		return finalBytes, finalFilename, contentType
	}

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	if fw1, err := zw.Create(filename); err == nil {
		_, _ = fw1.Write(out)
	}
	if fw2, err := zw.Create(baFilename); err == nil {
		_, _ = fw2.Write(baBytes)
	}
	if err := zw.Close(); err == nil {
		finalBytes = zipBuf.Bytes()
		baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
		finalFilename = fmt.Sprintf("%s_and_Berita_Acara.zip", baseName)
		contentType = "application/zip"
	}
	return finalBytes, finalFilename, contentType
}

func (w *WorkerServer) resolveDownloadParams(ctx context.Context) (string, int, time.Time, string) {
	downloadToken := uuid.New().String()
	maxDownloads := 3
	if w.cfg != nil && w.cfg.TimesheetDownloadMaxQuota > 0 {
		maxDownloads = w.cfg.TimesheetDownloadMaxQuota
	}
	retentionDays := 7
	if w.cfg != nil && w.cfg.S3RetentionDays > 0 {
		retentionDays = w.cfg.S3RetentionDays
	}
	expiryDuration := time.Duration(retentionDays) * 24 * time.Hour
	expiresAt := time.Now().Add(expiryDuration)

	baseURL := "http://localhost:8080"
	if w.cfg != nil && w.cfg.AppBaseURL != "" {
		baseURL = w.cfg.AppBaseURL
	}
	downloadURL := fmt.Sprintf("%s/api/v1/timesheet/downloads/%s", baseURL, downloadToken)

	if w.redisClient != nil {
		tokenKey := fmt.Sprintf("timesheet:token:%s:count", downloadToken)
		_ = w.redisClient.Set(ctx, tokenKey, 0, expiryDuration).Err()
	}
	return downloadToken, maxDownloads, expiresAt, downloadURL
}

func (w *WorkerServer) dispatchNotifications(ctx context.Context, userID uint, month, year int, downloadURL, finalFilename string, expiresAt time.Time) {
	if w.pushSvc != nil {
		w.pushSvc.SendToUser(userID, push.Payload{
			Title: "Timesheet Telah Siap",
			Body:  fmt.Sprintf("Timesheet periode %02d/%04d Anda telah selesai diproses dan siap diunduh.", month, year),
			URL:   downloadURL,
		})
	}

	if w.mailer == nil {
		return
	}
	user, uerr := w.userRepo.FindByIDWithDetails(ctx, userID)
	if uerr != nil || user == nil || user.Email == "" {
		return
	}
	compName := user.Company
	if user.CompanyRel != nil {
		compName = user.CompanyRel.Name
	}
	period := mailer.FormatMonthYearIndonesian(month, year)
	if mErr := w.mailer.SendTimesheetReadyEmail(user.Email, user.Username, compName, period, finalFilename, downloadURL, expiresAt); mErr != nil {
		log.Printf("[worker] failed to send timesheet ready email to %s: %v", user.Email, mErr)
	}
}
