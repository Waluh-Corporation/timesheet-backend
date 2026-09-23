package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"timesheet-backend/auth"
	"timesheet-backend/config"
	"timesheet-backend/database"
	"timesheet-backend/docs"
	_ "timesheet-backend/dto/request"
	_ "timesheet-backend/dto/response"
	"timesheet-backend/handlers"
	"timesheet-backend/internal/middleware"
	"timesheet-backend/internal/observability"
	"timesheet-backend/mailer"
	"timesheet-backend/push"
	"timesheet-backend/queue"
	"timesheet-backend/scheduler"
	"timesheet-backend/storage"
)

// @title Timesheet Automation Portal API
// @version 2.0
// @description High-performance RESTful backend API for the Timesheet Automation Portal, built with Go (Gin Engine) and PostgreSQL.
// @termsOfService https://github.com/Waluh-Corporation/timesheet-backend
// @contact.name API Support
// @license.name MIT
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter JWT token with format "Bearer {token}".

func setupDatabase(cfg *config.Config, logger *slog.Logger, migrateFlag, migrateOnlyFlag bool) (*gorm.DB, bool) {
	if migrateFlag || migrateOnlyFlag {
		cfg.RunMigrations = true
	}

	db, err := database.Connect(cfg)
	if err != nil {
		logger.Error("database connection failed", slog.Any("error", err))
		os.Exit(1)
	}

	if cfg.RunMigrations {
		if err := database.Setup(db, cfg); err != nil {
			logger.Error("database setup failed", slog.Any("error", err))
			os.Exit(1)
		}
	} else {
		// Always sync AAGUID authenticator registry into memory on server boot
		if err := database.SyncAuthenticatorAAGUIDs(db); err != nil {
			logger.Warn("failed to sync authenticator aaguids into in-memory registry", slog.Any("error", err))
		}
	}

	if migrateOnlyFlag {
		logger.Info("database migrations completed successfully")
		return nil, true
	}

	return db, false
}

func registerHealthRoutes(r *gin.Engine, db *gorm.DB) {
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/readyz", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unready", "database": "disconnected"})
			return
		}

		stats := sqlDB.Stats()
		c.JSON(http.StatusOK, gin.H{
			"status":   "ready",
			"database": "connected",
			"pool": gin.H{
				"max_open_connections": stats.MaxOpenConnections,
				"open_connections":     stats.OpenConnections,
				"in_use":               stats.InUse,
				"idle":                 stats.Idle,
				"wait_count":           stats.WaitCount,
				"wait_duration_ms":     stats.WaitDuration.Milliseconds(),
				"max_idle_closed":      stats.MaxIdleClosed,
				"max_idle_time_closed": stats.MaxIdleTimeClosed,
				"max_lifetime_closed":  stats.MaxLifetimeClosed,
			},
		})
	})
}

func registerStaticRoutes(r *gin.Engine, logger *slog.Logger) {
	staticPath := filepath.Clean(os.Getenv("STATIC_FILES_PATH"))
	if staticPath == "" || staticPath == "." {
		staticPath = "./static"
	}
	//nolint:gosec // G703: staticPath is loaded from server environment variable configuration
	if _, err := os.Stat(staticPath); err == nil {
		r.NoRoute(spaHandler(staticPath))
		logger.Info("serving static frontend", slog.String("path", staticPath))
	}
}

func setupRouter(db *gorm.DB, srv *handlers.Server, logger *slog.Logger) *gin.Engine {
	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.StructuredRecovery())
	r.Use(middleware.SecurityHeaders())
	r.Use(srv.CORSMiddleware())
	r.MaxMultipartMemory = 16 << 20 // 16 MiB template uploads

	registerHealthRoutes(r, db)
	registerRoutes(r, srv)
	registerStaticRoutes(r, logger)
	return r
}

func runHTTPServer(httpServer *http.Server, db *gorm.DB, logger *slog.Logger, port string) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("server starting", slog.String("port", port))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server listen failed", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	stop()
	logger.Info("shutting down gracefully, press Ctrl+C again to force")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", slog.Any("error", err))
	}

	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}

	logger.Info("server exited cleanly")
}

func main() {
	logger := observability.Logger()

	migrateFlag := flag.Bool("migrate", false, "run database migrations before starting the server")
	migrateOnlyFlag := flag.Bool("migrate-only", false, "run database migrations and exit")
	flag.Parse()

	cfg := config.Load()
	db, exitEarly := setupDatabase(cfg, logger, *migrateFlag, *migrateOnlyFlag)
	if exitEarly {
		return
	}

	authSvc := auth.NewService(cfg.JWTSecret, cfg.JWTExpiry)
	mailSvc := mailer.New(cfg)
	pushSvc := push.New(cfg, db)

	storageSvc, err := storage.NewS3StorageService(cfg)
	if err != nil {
		logger.Warn("s3 storage service not configured or failed to initialize", slog.Any("error", err))
	}

	queueClient, err := queue.NewQueueClient(cfg)
	if err != nil {
		logger.Warn("redis queue client failed to initialize", slog.Any("error", err))
	} else {
		defer func() { _ = queueClient.Close() }()
	}

	srv, err := handlers.NewServer(db, cfg, authSvc, mailSvc, pushSvc)
	if err != nil {
		logger.Error("failed to init server", slog.Any("error", err))
		os.Exit(1)
	}
	srv.Storage = storageSvc
	srv.QueueClient = queueClient

	if queueClient != nil && storageSvc != nil {
		workerServer := queue.NewWorkerServer(cfg, srv.JobRepo, srv.UserRepo, srv.TimesheetSvc, storageSvc, mailSvc, pushSvc)
		if werr := workerServer.Start(); werr != nil {
			logger.Error("failed to start queue worker server", slog.Any("error", werr))
		} else {
			defer workerServer.Shutdown()
		}
	}

	sched := scheduler.New(db, pushSvc, cfg.Timezone, cfg.ReminderCron, cfg.CleanupCron)
	if storageSvc != nil {
		sched.SetStorage(storageSvc)
	}
	sched.Start()
	defer sched.Stop()

	r := setupRouter(db, srv, logger)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	runHTTPServer(httpServer, db, logger, cfg.Port)
}

const (
	routeApprovers   = "/approvers"
	routeCompanies   = "/companies"
	routeDepartments = "/departments"
	routeDivisions   = "/divisions"
	routeSites       = "/sites"
)

// registerRoutes wires the full Phase 2 API surface.
func registerRoutes(r *gin.Engine, s *handlers.Server) {
	// Dynamically inherit host & protocol from the accessing browser origin (OpenAPI 2.0 spec compliant)
	docs.SwaggerInfo.Host = ""
	docs.SwaggerInfo.Schemes = []string{}

	// Swagger documentation UI
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})

	api := r.Group("/api/v1")

	// --- Public setup / onboarding wizard routes ---
	setupGroup := api.Group("/setup")
	{
		setupGroup.GET("/status", s.GetSetupStatus)
		setupGroup.POST("/init", s.InitSetup)
	}

	// --- Public auth routes with rate limiting ---
	authGroup := api.Group("/auth")
	rateLimitEnabled := false
	rateLimitRequests := 10
	rateLimitWindow := 1 * time.Minute

	if s != nil && s.Cfg != nil {
		rateLimitEnabled = s.Cfg.RateLimitEnabled && s.Cfg.RateLimitRequests > 0 && s.Cfg.RateLimitWindow > 0
		if s.Cfg.RateLimitRequests > 0 {
			rateLimitRequests = s.Cfg.RateLimitRequests
		}
		if s.Cfg.RateLimitWindow > 0 {
			rateLimitWindow = s.Cfg.RateLimitWindow
		}
	}

	if rateLimitEnabled {
		authLimiter := middleware.NewIPRateLimiter(rateLimitRequests, rateLimitWindow)
		authGroup.Use(middleware.RateLimitMiddleware(authLimiter))
	}
	{
		authGroup.POST("/login", s.Login)
		authGroup.POST("/refresh", s.RefreshToken)
		authGroup.POST("/logout", s.Logout)
		authGroup.POST("/forgot-password", s.ForgotPassword)
		authGroup.POST("/reset-password", s.ResetPassword)
		authGroup.GET("/reset-password/verify", s.VerifyResetPasswordToken)
		authGroup.POST("/reset-password/verify", s.VerifyResetPasswordToken)
		authGroup.POST("/passkey/login/begin", s.BeginPasskeyLogin)
		authGroup.POST("/passkey/login/finish", s.FinishPasskeyLogin)
	}

	// Public VAPID key (needed before the user is subscribed).
	api.GET("/push/vapid-public-key", s.GetVAPIDKey)

	// WebAuthn Related Origin Requests document, served at the well-known path
	// so passkeys registered under one relying party can be used across the
	// multiple domains listed in WEBAUTHN_RP_ORIGIN.
	r.GET("/.well-known/webauthn", s.WebAuthnRelatedOrigins)

	// --- Authenticated routes (any role) ---
	authed := api.Group("")
	authed.Use(s.AuthMiddleware())
	{
		authed.GET("/me", s.Me)
		authed.POST("/users/change-password", s.ChangePassword)
		authed.POST("/profile/change", s.SubmitProfileChange)
		authed.GET("/profile/changes", s.MyProfileChanges)

		// Passkey registration + self-service management for the logged-in user.
		authed.POST("/passkey/register/begin", s.BeginPasskeyRegistration)
		authed.POST("/passkey/register/finish", s.FinishPasskeyRegistration)
		authed.GET("/passkeys", s.ListPasskeys)
		authed.PATCH("/passkeys/:id", s.UpdatePasskey)
		authed.DELETE("/passkeys/:id", s.DeletePasskey)

		// Daily activity entry + list + detail + generation.
		authed.POST("/activities", s.UpsertDailyActivity)
		authed.GET("/activities", s.ListActivities)
		authed.GET("/activities/:id", s.GetDailyActivity)
		authed.POST("/overtimes", s.UpsertOvertime)
		authed.GET("/overtimes", s.ListMonthlyOvertimes)
		authed.DELETE("/overtimes/:id", s.DeleteOvertime)
		authed.POST("/timesheet/generate", s.GenerateTimesheet)
		authed.GET("/timesheet/jobs/:id", s.GetTimesheetJob)
		authed.GET("/timesheet/jobs", s.ListTimesheetJobs)
		authed.GET("/holidays", s.GetHolidays)
		authed.GET("/holidays/all", s.ListHolidays)
		authed.POST("/holidays/sync", s.SyncHolidays)

		// Master data (normalized projects, companies, departments, activity-statuses, approvers, sites, divisions).
		authed.GET("/projects", s.ListProjects)
		authed.GET(routeCompanies, s.ListCompanies)
		authed.GET(routeDepartments, s.ListDepartments)
		authed.GET(routeSites, s.ListSites)
		authed.GET(routeDivisions, s.ListDivisions)
		authed.GET("/activity-statuses", s.ListActivityStatuses)
		authed.GET(routeApprovers, s.ListApprovers)

		// Web push subscription.
		authed.POST("/push/subscribe", s.Subscribe)
		authed.POST("/push/unsubscribe", s.Unsubscribe)
		authed.POST("/push/test", s.SendTestPush)
	}

	// --- Admin-only routes ---
	admin := api.Group("/admin")
	admin.Use(s.AuthMiddleware(), s.AdminOnly())
	{
		admin.GET("/users", s.ListUsers)
		admin.POST("/users", s.CreateUser)
		admin.PATCH("/users/:id", s.UpdateUser)
		admin.DELETE("/users/:id", s.DeleteUser)
		admin.GET("/users/:id/passkeys", s.AdminListPasskeys)
		admin.PATCH("/users/:id/passkeys/:pid", s.AdminUpdatePasskey)
		admin.DELETE("/users/:id/passkeys/:pid", s.AdminDeletePasskey)
		admin.GET("/authenticators", s.AdminListAuthenticators)
		admin.POST("/authenticators/sync", s.SyncCommunityAuthenticators)

		admin.GET("/profile-changes", s.ListProfileChanges)
		admin.POST("/profile-changes/:id/review", s.ReviewProfileChange)

		// Master data management (approvers, companies, sites, divisions, departments)
		admin.GET(routeApprovers, s.AdminListApprovers)
		admin.POST(routeApprovers, s.CreateApprover)
		admin.PATCH("/approvers/:id", s.UpdateApprover)
		admin.DELETE("/approvers/:id", s.DeleteApprover)

		admin.GET(routeCompanies, s.AdminListCompanies)
		admin.POST(routeCompanies, s.CreateCompany)
		admin.PATCH("/companies/:id", s.UpdateCompany)
		admin.DELETE("/companies/:id", s.DeleteCompany)

		admin.GET(routeSites, s.AdminListSites)
		admin.POST(routeSites, s.CreateSite)
		admin.PATCH("/sites/:id", s.UpdateSite)
		admin.DELETE("/sites/:id", s.DeleteSite)

		admin.GET(routeDivisions, s.AdminListDivisions)
		admin.POST(routeDivisions, s.CreateDivision)
		admin.PATCH("/divisions/:id", s.UpdateDivision)
		admin.DELETE("/divisions/:id", s.DeleteDivision)

		admin.GET(routeDepartments, s.AdminListDepartments)
		admin.POST(routeDepartments, s.CreateDepartment)
		admin.PATCH("/departments/:id", s.UpdateDepartment)
		admin.DELETE("/departments/:id", s.DeleteDepartment)
	}
}

const errNotFoundMsg = "not found"

// tryFiles returns the first existing, non-directory candidate.
func tryFiles(candidates ...string) (string, bool) {
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, true
		}
	}
	return "", false
}

// isPathWithinRoot checks whether target is contained within the root directory.
func isPathWithinRoot(root, target string) bool {
	return target == root || strings.HasPrefix(target, root+string(os.PathSeparator))
}

// spaHandler serves the statically-exported Next.js site (Next `output: export`)
// from the Go binary. It resolves a request path to an on-disk file, trying the
// exact file, then "<path>.html" (Next exports routes like /login -> login.html),
// then "<path>/index.html", and finally falls back to the root index.html so
// client-side routing still works. Unmatched /api/* paths return a JSON 404
// rather than HTML.
func spaHandler(staticRoot string) gin.HandlerFunc {
	root := filepath.Clean(staticRoot)

	return func(c *gin.Context) {
		// Never serve HTML for an unmatched API or Swagger route.
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.HasPrefix(c.Request.URL.Path, "/swagger/") {
			c.JSON(http.StatusNotFound, gin.H{"error": errNotFoundMsg})
			return
		}

		// filepath.Clean on a "/"-prefixed path strips any "../" traversal; the
		// subsequent prefix check is defense-in-depth against escaping the root.
		rel := filepath.Clean("/" + c.Request.URL.Path)
		target := filepath.Join(root, filepath.FromSlash(rel))
		if !isPathWithinRoot(root, target) {
			c.JSON(http.StatusNotFound, gin.H{"error": errNotFoundMsg})
			return
		}

		if file, ok := tryFiles(target, target+".html", filepath.Join(target, "index.html")); ok {
			c.File(file)
			return
		}

		// SPA fallback: hand back the root document and let the client router
		// resolve the route (or render its own 404).
		if index, ok := tryFiles(filepath.Join(root, "index.html")); ok {
			c.File(index)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": errNotFoundMsg})
	}
}
