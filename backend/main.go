package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"timesheet-backend/auth"
	"timesheet-backend/config"
	"timesheet-backend/database"
	"timesheet-backend/handlers"
	"timesheet-backend/mailer"
	"timesheet-backend/push"
	"timesheet-backend/scheduler"
)

// @title Timesheet Automation Portal API
// @version 2.0
// @description Phase 2 portal: RBAC, passkeys, dynamic templates, web push, SMTP delivery.
// @BasePath /

func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	authSvc := auth.NewService(cfg.JWTSecret, cfg.JWTExpiry)
	mailSvc := mailer.New(cfg)
	pushSvc := push.New(cfg, db)

	srv, err := handlers.NewServer(db, cfg, authSvc, mailSvc, pushSvc)
	if err != nil {
		log.Fatalf("failed to init server: %v", err)
	}

	// Daily 17:00 WIB reminder scheduler.
	sched := scheduler.New(db, pushSvc, cfg.Timezone)
	sched.Start()
	defer sched.Stop()

	r := gin.Default()
	r.Use(handlers.CORSMiddleware())
	r.MaxMultipartMemory = 16 << 20 // 16 MiB template uploads

	registerRoutes(r, srv)

	// Serve the exported Next.js frontend from this same binary so the whole
	// portal ships as a single image (frontend + API on one origin).
	staticPath := os.Getenv("STATIC_FILES_PATH")
	if staticPath == "" {
		staticPath = "./static"
	}
	if _, err := os.Stat(staticPath); err == nil {
		r.NoRoute(spaHandler(staticPath))
		log.Printf("serving static frontend from %s", staticPath)
	}

	log.Printf("server starting on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}

// registerRoutes wires the full Phase 2 API surface.
func registerRoutes(r *gin.Engine, s *handlers.Server) {
	api := r.Group("/api")

	// --- Public auth routes (NO public sign-up) ---
	authGroup := api.Group("/auth")
	{
		authGroup.POST("/login", s.Login)
		authGroup.POST("/forgot-password", s.ForgotPassword)
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

		// Daily activity entry + monthly view + generation.
		authed.POST("/activities", s.UpsertDailyActivity)
		authed.GET("/activities", s.ListMonthlyActivities)
		authed.POST("/overtimes", s.UpsertOvertime)
		authed.GET("/overtimes", s.ListMonthlyOvertimes)
		authed.DELETE("/overtimes/:id", s.DeleteOvertime)
		authed.POST("/timesheet/generate", s.GenerateTimesheet)
		authed.GET("/holidays", s.GetHolidays)

		// Templates are readable by users (to fill the grid), writable by admins.
		authed.GET("/templates", s.ListTemplates)
		authed.GET("/templates/:id/grid", s.GetTemplateGrid)

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

		admin.POST("/templates", s.UploadTemplate)
		admin.POST("/templates/:id/mappings", s.SaveTemplateMappings)
		admin.POST("/templates/:id/default", s.SetDefaultTemplate)
		admin.DELETE("/templates/:id", s.DeleteTemplate)
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
		// Never serve HTML for an unmatched API route.
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
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
