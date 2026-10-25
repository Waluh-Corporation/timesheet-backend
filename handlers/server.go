package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/redis/go-redis/v9"
	"gorm.io/datatypes"
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
	"timesheet-backend/services"
	"timesheet-backend/storage"
)

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
	AuditRepo         repository.AuditRepository
	SubmissionRepo    repository.SubmissionRepository
	SignatureSvc      service.SignatureService
	QueueClient       queue.QueueClient
	Storage           storage.StorageService
	RedisClient       *redis.Client
	GeoIP             *services.GeoIPService

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

	var geoSvc *services.GeoIPService
	if cfg != nil {
		var gErr error
		geoSvc, gErr = services.NewGeoIPService(cfg.IP2LocationDBPath)
		if gErr != nil {
			slog.Warn("failed to initialize IP2Location service", "error", gErr)
		}
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
	var auditRepo repository.AuditRepository
	var submissionRepo repository.SubmissionRepository
	var signatureSvc service.SignatureService
	var storageSvc storage.StorageService
	if cfg != nil {
		var sErr error
		storageSvc, sErr = storage.NewS3StorageService(cfg)
		if sErr != nil {
			slog.Warn("failed to initialize storage service in server", "error", sErr)
		}
	}

	if db != nil {
		userRepo = repository.NewUserRepository(db)
		tokenRepo = repository.NewTokenRepository(db)
		masterRepo = repository.NewMasterRepository(db)
		userSvc = service.NewUserService(userRepo, auth.DefaultHasher, m, masterRepo)
		activityRepo = repository.NewActivityRepository(db)
		submissionRepo = repository.NewSubmissionRepository(db)
		activitySvc = service.NewActivityService(activityRepo, submissionRepo)
		overtimeRepo = repository.NewOvertimeRepository(db)
		timesheetSvc = service.NewTimesheetService(userRepo, activityRepo, overtimeRepo, masterRepo, m)
		baseURL := ""
		if cfg != nil {
			baseURL = cfg.AppBaseURL
		}
		signatureSvc = service.NewSignatureService(userRepo, activityRepo, submissionRepo, storageSvc, m, p, baseURL)
		if sigCfg, ok := timesheetSvc.(interface {
			SetSignatureDependencies(repository.SubmissionRepository, storage.StorageService, string)
		}); ok {
			sigCfg.SetSignatureDependencies(submissionRepo, storageSvc, baseURL)
		}
		masterSvc = service.NewMasterDataService(masterRepo)
		jobRepo = repository.NewJobRepository(db)
		pushRepo = repository.NewPushRepository(db)
		pushSvc = service.NewPushService(pushRepo, p)
		authRepo = repository.NewAuthenticatorRepository(db)
		authenticatorSvc = service.NewAuthenticatorService(authRepo)
		setupRepo = repository.NewSetupRepository(db)
		setupSvc = service.NewSetupService(setupRepo, authSvc)
		auditRepo = repository.NewAuditRepository(db)

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
		AuditRepo:         auditRepo,
		SubmissionRepo:    submissionRepo,
		SignatureSvc:      signatureSvc,
		Storage:           storageSvc,
		GeoIP:             geoSvc,
	}, nil
}

func (s *Server) checkAndRecordResetCooldownCtx(ctx context.Context, email, ip string, cooldown time.Duration) bool {
	if cooldown <= 0 {
		return true
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	normEmail := strings.ToLower(strings.TrimSpace(email))
	cleanIP := strings.TrimSpace(ip)
	if commaIdx := strings.Index(cleanIP, ","); commaIdx != -1 {
		cleanIP = strings.TrimSpace(cleanIP[:commaIdx])
	}
	if host, _, err := net.SplitHostPort(cleanIP); err == nil {
		cleanIP = host
	}
	cleanIP = strings.Trim(cleanIP, "[]")
	if zoneIdx := strings.Index(cleanIP, "%"); zoneIdx != -1 {
		cleanIP = cleanIP[:zoneIdx]
	}
	if parsed := net.ParseIP(cleanIP); parsed != nil {
		cleanIP = parsed.String()
	}

	if normEmail == "" && cleanIP == "" {
		return true
	}

	rdb := s.getRedisClient()
	if rdb == nil {
		slog.Warn("redis client unavailable for reset cooldown check; bypassing fast-path")
		return true
	}

	const maxIPRequestsPerWindow = 5

	effectiveCooldown := cooldown
	if effectiveCooldown < time.Millisecond {
		effectiveCooldown = time.Millisecond
	}

	// 1. Check IP rate limit window if client IP is present
	var ipKey string
	if cleanIP != "" {
		ipKey = fmt.Sprintf("cooldown:reset:ip:%s", cleanIP)
		if currCount, err := rdb.Get(ctx, ipKey).Int(); err == nil && currCount >= maxIPRequestsPerWindow {
			return false
		}
	}

	// 2. Atomic email cooldown using SET NX EX pattern
	var emailKey string
	if normEmail != "" {
		emailKey = fmt.Sprintf("cooldown:reset:email:%s", normEmail)
		set, err := rdb.SetNX(ctx, emailKey, "1", effectiveCooldown).Result()
		if err != nil {
			slog.Warn("redis error during email reset cooldown check; bypassing fast-path", "error", err, "email", normEmail)
			return true
		}
		if !set {
			return false
		}
	}

	// 3. Atomically track and increment client IP request count with TTL preservation
	if ipKey != "" {
		const ipIncrScript = `
			local count = redis.call('INCR', KEYS[1])
			if count == 1 or redis.call('PTTL', KEYS[1]) < 0 then
				redis.call('PEXPIRE', KEYS[1], ARGV[1])
			end
			return count
		`
		var newCount int64
		res, err := rdb.Eval(ctx, ipIncrScript, []string{ipKey}, effectiveCooldown.Milliseconds()).Result()
		if err == nil {
			switch n := res.(type) {
			case int64:
				newCount = n
			case int:
				newCount = int64(n)
			case int32:
				newCount = int64(n)
			case uint64:
				if n > math.MaxInt64 {
					newCount = math.MaxInt64
				} else {
					newCount = int64(n)
				}
			case float64:
				if n > float64(math.MaxInt64) {
					newCount = math.MaxInt64
				} else {
					newCount = int64(n)
				}
			}
		} else {
			newCount, err = rdb.Incr(ctx, ipKey).Result()
			if err == nil && newCount == 1 {
				_ = rdb.PExpire(ctx, ipKey, effectiveCooldown).Err()
			}
		}

		if err == nil && newCount > maxIPRequestsPerWindow {
			// Rollback email reservation if IP limit was exceeded concurrently
			if emailKey != "" {
				_ = rdb.Del(ctx, emailKey).Err()
			}
			return false
		}
	}

	return true
}

func (s *Server) checkAndRecordResetCooldown(email, ip string, cooldown time.Duration) bool {
	return s.checkAndRecordResetCooldownCtx(context.Background(), email, ip, cooldown)
}

func (s *Server) dispatchResetEmail(toEmail, username, resetLink, clientIP, location string) {
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
			if err := s.Mailer.SendResetEmailWithDetails(toEmail, username, resetLink, clientIP, location); err != nil {
				slog.Error("failed to send password reset email", "error", err, "email", toEmail)
			}
		}()
	}
}

const webAuthnSessionTTL = 5 * time.Minute

func (s *Server) putSessionCtx(ctx context.Context, id string, data *webauthn.SessionData) {
	cleanID := strings.TrimSpace(id)
	if cleanID == "" || data == nil {
		return
	}
	rdb := s.getRedisClient()
	if rdb == nil {
		slog.Warn("cannot store webauthn session: redis client is unavailable", "session_id", cleanID)
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	b, err := json.Marshal(data)
	if err != nil {
		slog.Error("failed to marshal webauthn session data", "error", err, "session_id", cleanID)
		return
	}

	key := fmt.Sprintf("webauthn:session:%s", cleanID)
	if err := rdb.Set(ctx, key, b, webAuthnSessionTTL).Err(); err != nil {
		slog.Error("failed to store webauthn session in redis", "error", err, "session_id", cleanID)
	}
}

func (s *Server) putSession(id string, data *webauthn.SessionData) {
	s.putSessionCtx(context.Background(), id, data)
}

func (s *Server) takeSessionCtx(ctx context.Context, id string) (*webauthn.SessionData, bool) {
	cleanID := strings.TrimSpace(id)
	if cleanID == "" {
		return nil, false
	}
	rdb := s.getRedisClient()
	if rdb == nil {
		slog.Warn("cannot retrieve webauthn session: redis client is unavailable", "session_id", cleanID)
		return nil, false
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	key := fmt.Sprintf("webauthn:session:%s", cleanID)
	val, err := rdb.GetDel(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false
		}
		// Fallback for older Redis versions (< 6.2) or test doubles / proxies not supporting GETDEL
		errLower := strings.ToLower(err.Error())
		if strings.Contains(errLower, "unknown") || strings.Contains(errLower, "unsupported") || strings.Contains(errLower, "not supported") {
			const getDelScript = `
				local v = redis.call('GET', KEYS[1])
				if not v or v == false then
					return nil
				end
				redis.call('DEL', KEYS[1])
				return v
			`
			res, scriptErr := rdb.Eval(ctx, getDelScript, []string{key}).Result()
			if scriptErr != nil {
				if errors.Is(scriptErr, redis.Nil) {
					return nil, false
				}
				slog.Error("failed to execute fallback getdel script", "error", scriptErr, "session_id", cleanID)
				return nil, false
			}
			var strVal string
			switch s := res.(type) {
			case string:
				strVal = s
			case []byte:
				strVal = string(s)
			}
			if strVal == "" {
				return nil, false
			}
			val = strVal
		} else {
			slog.Error("failed to retrieve webauthn session from redis", "error", err, "session_id", cleanID)
			return nil, false
		}
	}

	if strings.TrimSpace(val) == "" {
		return nil, false
	}

	var data webauthn.SessionData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		slog.Error("failed to unmarshal webauthn session data", "error", err, "session_id", cleanID)
		return nil, false
	}

	return &data, true
}

func (s *Server) takeSession(id string) (*webauthn.SessionData, bool) {
	return s.takeSessionCtx(context.Background(), id)
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
	if s == nil {
		return nil
	}
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

func (s *Server) getAuditRepository() repository.AuditRepository {
	if s == nil {
		return nil
	}
	return s.AuditRepo
}

func (s *Server) getSubmissionRepository() repository.SubmissionRepository {
	if s == nil {
		return nil
	}
	return s.SubmissionRepo
}

// AuditLogParams encapsulates parameters for recording an administrative audit event.
type AuditLogParams struct {
	ActorID      *uint
	Action       string
	ResourceType string
	ResourceID   string
	IP           string
	UserAgent    string
	Details      interface{}
}

func (s *Server) recordAuditLog(ctx context.Context, p AuditLogParams) {
	repo := s.getAuditRepository()
	if repo == nil {
		return
	}
	actorID := p.ActorID
	if actorID != nil && *actorID == 0 {
		actorID = nil
	}
	actorUsername := ""
	if actorID != nil && s != nil && s.UserRepo != nil {
		if u, err := s.UserRepo.FindByID(ctx, *actorID); err == nil && u != nil {
			actorUsername = u.Username
		}
	}
	var detailsBytes datatypes.JSON
	if p.Details != nil {
		if b, err := json.Marshal(p.Details); err == nil {
			detailsBytes = datatypes.JSON(b)
		}
	}
	entry := &models.AuditLog{
		ActorID:       actorID,
		ActorUsername: actorUsername,
		Action:        p.Action,
		ResourceType:  p.ResourceType,
		ResourceID:    p.ResourceID,
		IPAddress:     p.IP,
		UserAgent:     p.UserAgent,
		Details:       detailsBytes,
	}
	_ = repo.Create(ctx, entry)
}
