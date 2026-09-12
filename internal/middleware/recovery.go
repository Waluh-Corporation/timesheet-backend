package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"timesheet-backend/internal/observability"
)

// StructuredRecovery recovers from any panics and writes a 500 while logging via slog.
func StructuredRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger := observability.FromContext(c.Request.Context())
				logger.ErrorContext(c.Request.Context(), "panic recovered in HTTP request",
					slog.Any("panic", r),
					slog.String("path", c.Request.URL.Path),
					slog.String("method", c.Request.Method),
					slog.String("stack", string(debug.Stack())),
				)

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":    http.StatusInternalServerError,
					"status":  "error",
					"error":   "internal server error",
					"message": "an unexpected error occurred",
				})
			}
		}()
		c.Next()
	}
}
