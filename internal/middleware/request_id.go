package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"timesheet-backend/internal/observability"
)

const HeaderXRequestID = "X-Request-ID"

// RequestID injects or generates a unique correlation ID for each HTTP request.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader(HeaderXRequestID)
		if reqID == "" {
			reqID = uuid.New().String()
		}

		c.Header(HeaderXRequestID, reqID)
		c.Set(string(observability.RequestIDKey), reqID)

		// Also attach to context.Context
		ctx := context.WithValue(c.Request.Context(), observability.RequestIDKey, reqID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
