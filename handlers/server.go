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
	Cfg               *config.Config
	Auth              *auth.Service
	Mailer            *mailer.Mailer
	Push              *push.Service
	WebAuthn          *webauthn.WebAuthn
	Hasher            auth.PasswordHasher
	UserRepo          repository.UserRepository
	UserSvc           service.UserService
	TokenRepo         repository.TokenRepository
	ActivityRepo      repository.ActivityRepository
	ActivitySvc       service.ActivityService
	OvertimeRepo      repository.OvertimeRepository
	TimesheetSvc      service.TimesheetService
	MasterRepo        repository.MasterRepository
	MasterSvc         service.MasterDataService
	JobRepo           repository.JobRepository
	PushRepo          repository.PushRepository
	PushSvc           service.PushService
	AuthenticatorRepo repository.AuthenticatorRepository
	AuthenticatorSvc  service.AuthenticatorService
	SetupRepo         repository.SetupRepository
	SetupSvc          service.SetupService
	QueueClient       queue.QueueClient
	Storage           storage.StorageService
	RedisClient       *redis.Client

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

// NewServer wires up a Server and its WebAuthn relying party without exposing DB to handlers.
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
	var jobRepo repository.JobRepository
	var pushRepo repository.PushRepository
	var pushSvc service.PushService
	var authRepo repository.AuthenticatorRepository
	var authenticatorSvc service.AuthenticatorService
	var setupRepo repository.SetupRepository
	var setupSvc service.SetupService

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
		jobRepo = repository.NewJobRepository(db)
		pushRepo = repository.NewPushRepository(db)
		pushSvc = service.NewPushService(pushRepo, p)
		authRepo = repository.NewAuthenticatorRepository(db)
		authenticatorSvc = service.NewAuthenticatorService(authRepo)
		setupRepo = repository.NewSetupRepository(db)
		setupSvc = service.NewSetupService(setupRepo, authSvc)

		_ = database.SyncAuthenticatorAAGUIDs(db)
	}

	return &Server{
		Cfg:               cfg,
		Auth:              authSvc,
		Mailer:            m,
		Push:              p,
		WebAuthn:          wa,
		Hasher:            auth.DefaultHasher,
		UserRepo:          userRepo,
		UserSvc:           userSvc,
		TokenRepo:         tokenRepo,
		ActivityRepo:      activityRepo,
		ActivitySvc:       activitySvc,
		OvertimeRepo:      overtimeRepo,
		TimesheetSvc:      timesheetSvc,
		MasterRepo:        masterRepo,
		MasterSvc:         masterSvc,
		JobRepo:           jobRepo,
		PushRepo:          pushRepo,
		PushSvc:           pushSvc,
		AuthenticatorRepo: authRepo,
		AuthenticatorSvc:  authenticatorSvc,
		SetupRepo:         setupRepo,
		SetupSvc:          setupSvc,
		webAuthnSessions:  make(map[string]*webAuthnSessionEntry),
		resetCooldowns:    make(map[string]time.Time),
		ipCooldowns:       make(map[string]*clientCooldownRecord),
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

// GetUserService retrieves the UserService.
func (s *Server) GetUserService() service.UserService {
	return s.UserSvc
}

func (s *Server) getTimesheetService() service.TimesheetService {
	return s.TimesheetSvc
}

func (s *Server) getJobRepo() repository.JobRepository {
	return s.JobRepo
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
	return s.UserRepo
}

func (s *Server) getActivityRepository() repository.ActivityRepository {
	return s.ActivityRepo
}

func (s *Server) getTokenRepository() repository.TokenRepository {
	return s.TokenRepo
}

func (s *Server) getMasterService() service.MasterDataService {
	return s.MasterSvc
}

func (s *Server) getPushService() service.PushService {
	if s.PushSvc != nil {
		return s.PushSvc
	}
	if s.Push != nil {
		return service.NewPushService(nil, s.Push)
	}
	return nil
}

func (s *Server) getAuthenticatorService() service.AuthenticatorService {
	return s.AuthenticatorSvc
}

func (s *Server) getSetupService() service.SetupService {
	return s.SetupSvc
}
