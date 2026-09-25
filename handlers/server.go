package handlers

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"timesheet-backend/auth"
	"timesheet-backend/config"
	"timesheet-backend/database"
	"timesheet-backend/internal/repository"
	"timesheet-backend/internal/service"
	"timesheet-backend/mailer"
	"timesheet-backend/models"
	"timesheet-backend/push"
	"timesheet-backend/queue"
	"timesheet-backend/storage"
)

type webAuthnSessionEntry struct {
	data      *webauthn.SessionData
	createdAt time.Time
}

type clientCooldownRecord struct {
	windowStart time.Time
	count       int
}

// Server carries the shared dependencies used by all HTTP handlers.
type Server struct {
	DB           *gorm.DB
	Cfg          *config.Config
	Auth         *auth.Service
	Mailer       *mailer.Mailer
	Push         *push.Service
	WebAuthn     *webauthn.WebAuthn
	Hasher       auth.PasswordHasher
	UserRepo     repository.UserRepository
	UserSvc      service.UserService
	TokenRepo    repository.TokenRepository
	ActivityRepo repository.ActivityRepository
	ActivitySvc  service.ActivityService
	OvertimeRepo repository.OvertimeRepository
	TimesheetSvc service.TimesheetService
	MasterRepo   repository.MasterRepository
	MasterSvc    service.MasterDataService
	JobRepo      repository.JobRepository
	QueueClient  queue.QueueClient
	Storage      storage.StorageService
	RedisClient  *redis.Client

	// webAuthnSessions holds in-flight ceremony data keyed by an opaque id
	// handed to the client for the duration of a single begin/finish exchange.
	webAuthnSessions map[string]*webAuthnSessionEntry
	sessionsMu       sync.Mutex

	resetCooldowns map[string]time.Time
	ipCooldowns    map[string]*clientCooldownRecord
	cooldownMu     sync.Mutex

	finishRegistrationFunc func(user models.User, session webauthn.SessionData, r *http.Request) (*webauthn.Credential, error)
	sendResetEmailFunc     func(toEmail, username, resetLink string) error
}

// NewServer wires up a Server and its WebAuthn relying party.
func NewServer(db *gorm.DB, cfg *config.Config, authSvc *auth.Service, m *mailer.Mailer, p *push.Service) (*Server, error) {
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: cfg.RPDisplayName,
		RPID:          cfg.RPID,
		RPOrigins:     cfg.RPOrigins,
	})
	if err != nil {
		return nil, err
	}

	var userRepo repository.UserRepository
	var userSvc service.UserService
	var tokenRepo repository.TokenRepository
	var activityRepo repository.ActivityRepository
	var activitySvc service.ActivityService
	var overtimeRepo repository.OvertimeRepository
	var timesheetSvc service.TimesheetService
	var masterRepo repository.MasterRepository
	var masterSvc service.MasterDataService

	if db != nil {
		userRepo = repository.NewUserRepository(db)
		tokenRepo = repository.NewTokenRepository(db)
		masterRepo = repository.NewMasterRepository(db)
		userSvc = service.NewUserService(userRepo, auth.DefaultHasher, m, masterRepo)
		activityRepo = repository.NewActivityRepository(db)
		activitySvc = service.NewActivityService(activityRepo)
		overtimeRepo = repository.NewOvertimeRepository(db)
		timesheetSvc = service.NewTimesheetService(userRepo, activityRepo, overtimeRepo, masterRepo, m)
		masterSvc = service.NewMasterDataService(masterRepo)
	}

	if db != nil {
		_ = database.SyncAuthenticatorAAGUIDs(db)
	}

	return &Server{
		DB:               db,
		Cfg:              cfg,
		Auth:             authSvc,
		Mailer:           m,
		Push:             p,
		WebAuthn:         wa,
		Hasher:           auth.DefaultHasher,
		UserRepo:         userRepo,
		UserSvc:          userSvc,
		TokenRepo:        tokenRepo,
		ActivityRepo:     activityRepo,
		ActivitySvc:      activitySvc,
		OvertimeRepo:     overtimeRepo,
		TimesheetSvc:     timesheetSvc,
		MasterRepo:       masterRepo,
		MasterSvc:        masterSvc,
		JobRepo:          repository.NewJobRepository(db),
		webAuthnSessions: make(map[string]*webAuthnSessionEntry),
		resetCooldowns:   make(map[string]time.Time),
		ipCooldowns:      make(map[string]*clientCooldownRecord),
	}, nil
}

func (s *Server) checkAndRecordResetCooldown(email, ip string, cooldown time.Duration) bool {
	if cooldown <= 0 {
		return true
	}
	s.cooldownMu.Lock()
	defer s.cooldownMu.Unlock()

	if s.resetCooldowns == nil {
		s.resetCooldowns = make(map[string]time.Time)
	}
	if s.ipCooldowns == nil {
		s.ipCooldowns = make(map[string]*clientCooldownRecord)
	}

	now := time.Now()

	// Proactively clean up expired entries older than 2 * cooldown
	cutoff := now.Add(-2 * cooldown)
	for k, v := range s.resetCooldowns {
		if v.Before(cutoff) {
			delete(s.resetCooldowns, k)
		}
	}
	for k, v := range s.ipCooldowns {
		if v.windowStart.Before(cutoff) {
			delete(s.ipCooldowns, k)
		}
	}

	normEmail := strings.ToLower(strings.TrimSpace(email))
	cleanIP := strings.TrimSpace(ip)

	// 1. Check email cooldown: strictly 1 request per cooldown duration to prevent email bombing
	if normEmail != "" {
		if last, exists := s.resetCooldowns[normEmail]; exists && now.Sub(last) < cooldown {
			return false
		}
	}

	// 2. Check IP rate limit: allow up to 5 requests per cooldown duration per IP to prevent spamming
	// while supporting multiple legitimate users behind shared corporate NAT / proxy.
	const maxIPRequestsPerWindow = 5
	if cleanIP != "" {
		if rec, exists := s.ipCooldowns[cleanIP]; exists && now.Sub(rec.windowStart) < cooldown {
			if rec.count >= maxIPRequestsPerWindow {
				return false
			}
		}
	}

	// Record timestamps and counts
	if normEmail != "" {
		s.resetCooldowns[normEmail] = now
	}
	if cleanIP != "" {
		if rec, exists := s.ipCooldowns[cleanIP]; exists && now.Sub(rec.windowStart) < cooldown {
			rec.count++
		} else {
			s.ipCooldowns[cleanIP] = &clientCooldownRecord{windowStart: now, count: 1}
		}
	}

	return true
}

func (s *Server) dispatchResetEmail(toEmail, username, resetLink string) {
	if s.sendResetEmailFunc != nil {
		go func() {
			if err := s.sendResetEmailFunc(toEmail, username, resetLink); err != nil {
				slog.Error("failed to send password reset email via custom func", "error", err, "email", toEmail)
			}
		}()
		return
	}
	if s.Mailer != nil {
		go func() {
			if err := s.Mailer.SendResetEmailWithUser(toEmail, username, resetLink); err != nil {
				slog.Error("failed to send password reset email", "error", err, "email", toEmail)
			}
		}()
	}
}

func (s *Server) putSession(id string, data *webauthn.SessionData) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	if s.webAuthnSessions == nil {
		s.webAuthnSessions = make(map[string]*webAuthnSessionEntry)
	}

	// Proactively clean up expired sessions (> 5 minutes old)
	cutoff := time.Now().Add(-5 * time.Minute)
	for k, v := range s.webAuthnSessions {
		if v.createdAt.Before(cutoff) {
			delete(s.webAuthnSessions, k)
		}
	}

	s.webAuthnSessions[id] = &webAuthnSessionEntry{
		data:      data,
		createdAt: time.Now(),
	}
}

func (s *Server) takeSession(id string) (*webauthn.SessionData, bool) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	if s.webAuthnSessions == nil {
		return nil, false
	}
	entry, ok := s.webAuthnSessions[id]
	if ok {
		delete(s.webAuthnSessions, id)
		if time.Since(entry.createdAt) > 5*time.Minute {
			return nil, false
		}
		return entry.data, true
	}
	return nil, false
}

// GetUserService retrieves or lazily initializes the UserService.
func (s *Server) GetUserService() service.UserService {
	if s.UserSvc != nil {
		return s.UserSvc
	}
	if s.DB != nil {
		if s.UserRepo == nil {
			s.UserRepo = repository.NewUserRepository(s.DB)
		}
		if s.MasterRepo == nil {
			s.MasterRepo = repository.NewMasterRepository(s.DB)
		}
		s.UserSvc = service.NewUserService(s.UserRepo, s.Hasher, s.Mailer, s.MasterRepo)
		return s.UserSvc
	}
	return nil
}

func (s *Server) getTimesheetService() service.TimesheetService {
	if s.TimesheetSvc != nil {
		return s.TimesheetSvc
	}
	if s.DB != nil {
		if s.OvertimeRepo == nil {
			s.OvertimeRepo = repository.NewOvertimeRepository(s.DB)
		}
		if s.UserRepo == nil {
			s.UserRepo = repository.NewUserRepository(s.DB)
		}
		if s.ActivityRepo == nil {
			s.ActivityRepo = repository.NewActivityRepository(s.DB)
		}
		if s.MasterRepo == nil {
			s.MasterRepo = repository.NewMasterRepository(s.DB)
		}
		s.TimesheetSvc = service.NewTimesheetService(s.UserRepo, s.ActivityRepo, s.OvertimeRepo, s.MasterRepo, s.Mailer)
		return s.TimesheetSvc
	}
	return nil
}

func (s *Server) getJobRepo() repository.JobRepository {
	if s.JobRepo != nil {
		return s.JobRepo
	}
	if s.DB != nil {
		s.JobRepo = repository.NewJobRepository(s.DB)
		return s.JobRepo
	}
	return nil
}

func (s *Server) getQueueClient() queue.QueueClient {
	return s.QueueClient
}

func (s *Server) getStorage() storage.StorageService {
	return s.Storage
}

func (s *Server) getRedisClient() *redis.Client {
	return s.RedisClient
}

func (s *Server) getUserRepository() repository.UserRepository {
	if s.UserRepo != nil {
		return s.UserRepo
	}
	if s.DB != nil {
		s.UserRepo = repository.NewUserRepository(s.DB)
		return s.UserRepo
	}
	return nil
}

func (s *Server) getActivityRepository() repository.ActivityRepository {
	if s.ActivityRepo != nil {
		return s.ActivityRepo
	}
	if s.DB != nil {
		s.ActivityRepo = repository.NewActivityRepository(s.DB)
		return s.ActivityRepo
	}
	return nil
}

func (s *Server) getTokenRepository() repository.TokenRepository {
	if s.TokenRepo != nil {
		return s.TokenRepo
	}
	if s.DB != nil {
		s.TokenRepo = repository.NewTokenRepository(s.DB)
		return s.TokenRepo
	}
	return nil
}

func (s *Server) getMasterService() service.MasterDataService {
	if s.MasterSvc != nil {
		return s.MasterSvc
	}
	if s.DB != nil {
		if s.MasterRepo == nil {
			s.MasterRepo = repository.NewMasterRepository(s.DB)
		}
		s.MasterSvc = service.NewMasterDataService(s.MasterRepo)
		return s.MasterSvc
	}
	return nil
}
