package handlers

import (
	"math"

	"github.com/gin-gonic/gin"
)

// RespondSuccess sends a 2xx JSON response wrapped in the unified envelope.
func RespondSuccess(c *gin.Context, code int, data interface{}) {
	c.JSON(code, gin.H{
		"code":   code,
		"status": "success",
		"data":   data,
	})
}

// RespondMessage sends a 2xx JSON message response wrapped in the unified envelope.
func RespondMessage(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{
		"code":    code,
		"status":  "success",
		"message": message,
	})
}

// RespondDelete sends a 2xx JSON deletion confirmation wrapped in the unified envelope.
func RespondDelete(c *gin.Context, code int) {
	c.JSON(code, gin.H{
		"code":    code,
		"status":  "success",
		"deleted": true,
	})
}

// RespondError sends a 4xx/5xx JSON error response with status code and error message.
func RespondError(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{
		"code":    code,
		"status":  "error",
		"error":   message,
		"message": message,
	})
}

// RespondAbortError aborts the context with a 4xx/5xx JSON error response.
func RespondAbortError(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(code, gin.H{
		"code":    code,
		"status":  "error",
		"error":   message,
		"message": message,
	})
}

// RespondPaginated sends a 2xx JSON response wrapped with pagination metadata.
func RespondPaginated(c *gin.Context, code int, data interface{}, page int, limit int, totalRows int64) {
	totalPages := 0
	if limit > 0 {
		totalPages = int(math.Ceil(float64(totalRows) / float64(limit)))
	}
	c.JSON(code, gin.H{
		"code":   code,
		"status": "success",
		"data":   data,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total_rows":  totalRows,
			"total_pages": totalPages,
		},
	})
}

// Convenience methods on Server
func (s *Server) RespondSuccess(c *gin.Context, code int, data interface{}) {
	RespondSuccess(c, code, data)
}
func (s *Server) RespondPaginated(c *gin.Context, code int, data interface{}, page int, limit int, totalRows int64) {
	RespondPaginated(c, code, data, page, limit, totalRows)
}
func (s *Server) RespondMessage(c *gin.Context, code int, message string) {
	RespondMessage(c, code, message)
}
func (s *Server) RespondDelete(c *gin.Context, code int) { RespondDelete(c, code) }
func (s *Server) RespondError(c *gin.Context, code int, message string) {
	RespondError(c, code, message)
}
func (s *Server) RespondAbortError(c *gin.Context, code int, message string) {
	RespondAbortError(c, code, message)
}
