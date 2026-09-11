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

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"timesheet-backend/auth"
	"timesheet-backend/config"
	"timesheet-backend/database"
	_ "timesheet-backend/docs"
	"timesheet-backend/handlers"
	"timesheet-backend/internal/middleware"
	"timesheet-backend/internal/observability"
	"timesheet-backend/mailer"
	"timesheet-backend/push"
	"timesheet-backend/scheduler"
)

// @title Timesheet Automation Portal API
// @version 2.0
// @description High-performance RESTful backend API for the Timesheet Automation Portal, built with Go (Gin Engine) and PostgreSQL.
// @termsOfService https://github.com/Waluh-Corporation/timesheet-backend
// @contact.name API Support
// @license.name MIT
// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter JWT token with format "Bearer {token}".

func main() {
	logger := observability.Logger()

	migrateFlag := flag.Bool("migrate", false, "run database migrations before starting the server")
	migrateOnlyFlag := flag.Bool("migrate-only", false, "run database migrations and exit")
	flag.Parse()

	cfg := config.Load()
	if *migrateFlag || *migrateOnlyFlag {
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
	}

	if *migrateOnlyFlag {
		logger.Info("database migrations completed successfully")
		return
	}

	authSvc := auth.NewService(cfg.JWTSecret, cfg.JWTExpiry)
	mailSvc := mailer.New(cfg)
	pushSvc := push.New(cfg, db)

	srv, err := handlers.NewServer(db, cfg, authSvc, mailSvc, pushSvc)
	if err != nil {
		logger.Error("failed to init server", slog.Any("error", err))
		os.Exit(1)
	}

	// Daily 17:00 WIB reminder scheduler.
	sched := scheduler.New(db, pushSvc, cfg.Timezone)
	sched.Start()
	defer sched.Stop()

	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.StructuredRecovery())
	r.Use(middleware.SecurityHeaders())
	r.Use(handlers.CORSMiddleware())
	r.MaxMultipartMemory = 16 << 20 // 16 MiB template uploads

	// Liveness and Readiness probes for Kubernetes / container orchestration
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/readyz", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unready", "database": "disconnected"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready", "database": "connected"})
	})

	registerRoutes(r, srv)

	// Serve the exported Next.js frontend from this same binary so the whole
	// portal ships as a single image (frontend + API on one origin).
	staticPath := os.Getenv("STATIC_FILES_PATH")
	if staticPath == "" {
		staticPath = "./static"
	}
	if _, err := os.Stat(staticPath); err == nil {
		r.NoRoute(spaHandler(staticPath))
		logger.Info("serving static frontend", slog.String("path", staticPath))
	}

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("server starting", slog.String("port", cfg.Port))
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

	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}

	logger.Info("server exited cleanly")
}

// registerRoutes wires the full Phase 2 API surface.
func registerRoutes(r *gin.Engine, s *handlers.Server) {
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
	authLimiter := middleware.NewIPRateLimiter(10, 1*time.Minute)
	authGroup := api.Group("/auth")
	{
		authGroup.POST("/login", middleware.RateLimitMiddleware(authLimiter), s.Login)
		authGroup.POST("/forgot-password", middleware.RateLimitMiddleware(authLimiter), s.ForgotPassword)
		authGroup.POST("/reset-password", s.ResetPassword)
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
		authed.POST("/profile/change", s.SubmitProfileChange)
		authed.GET("/profile/changes", s.MyProfileChanges)

		// Passkey registration + self-service management for the logged-in user.
		authed.POST("/passkey/register/begin", s.BeginPasskeyRegistration)
		authed.POST("/passkey/register/finish", s.FinishPasskeyRegistration)
		authed.GET("/passkeys", s.ListPasskeys)
		authed.DELETE("/passkeys/:id", s.DeletePasskey)

		// Daily activity entry + list + detail + generation.
		authed.POST("/activities", s.UpsertDailyActivity)
		authed.GET("/activities", s.ListActivities)
		authed.GET("/activities/:id", s.GetDailyActivity)
		authed.POST("/overtimes", s.UpsertOvertime)
		authed.GET("/overtimes", s.ListMonthlyOvertimes)
		authed.DELETE("/overtimes/:id", s.DeleteOvertime)
		authed.POST("/timesheet/generate", s.GenerateTimesheet)
		authed.GET("/holidays", s.GetHolidays)
		authed.GET("/holidays/all", s.ListHolidays)
		authed.POST("/holidays/sync", s.SyncHolidays)

		// Master data (normalized projects, companies, departments, activity-statuses, approvers).
		authed.GET("/projects", s.ListProjects)
		authed.GET("/companies", s.ListCompanies)
		authed.GET("/departments", s.ListDepartments)
		authed.GET("/activity-statuses", s.ListActivityStatuses)
		authed.GET("/approvers", s.ListApprovers)

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
		admin.DELETE("/users/:id/passkeys/:pid", s.AdminDeletePasskey)

		admin.GET("/profile-changes", s.ListProfileChanges)
		admin.POST("/profile-changes/:id/review", s.ReviewProfileChange)

		// Master data management (approvers & companies)
		admin.POST("/approvers", s.CreateApprover)
		admin.PATCH("/approvers/:id", s.UpdateApprover)
		admin.DELETE("/approvers/:id", s.DeleteApprover)

		admin.POST("/companies", s.CreateCompany)
		admin.PATCH("/companies/:id", s.UpdateCompany)
		admin.DELETE("/companies/:id", s.DeleteCompany)
	}
}

// spaHandler serves the statically-exported Next.js site (Next `output: export`)
// from the Go binary. It resolves a request path to an on-disk file, trying the
// exact file, then "<path>.html" (Next exports routes like /login -> login.html),
// then "<path>/index.html", and finally falls back to the root index.html so
// client-side routing still works. Unmatched /api/* paths return a JSON 404
// rather than HTML.
func spaHandler(staticRoot string) gin.HandlerFunc {
	root := filepath.Clean(staticRoot)

	// tryFiles returns the first existing, non-directory candidate.
	tryFiles := func(candidates ...string) (string, bool) {
		for _, c := range candidates {
			if info, err := os.Stat(c); err == nil && !info.IsDir() {
				return c, true
			}
		}
		return "", false
	}

	return func(c *gin.Context) {
		// Never serve HTML for an unmatched API or Swagger route.
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.HasPrefix(c.Request.URL.Path, "/swagger/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// filepath.Clean on a "/"-prefixed path strips any "../" traversal; the
		// subsequent prefix check is defence-in-depth against escaping the root.
		rel := filepath.Clean("/" + c.Request.URL.Path)
		target := filepath.Join(root, filepath.FromSlash(rel))
		if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
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
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	}
}
