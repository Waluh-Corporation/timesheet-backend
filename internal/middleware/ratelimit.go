package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type clientRecord struct {
	lastSeen time.Time
	tokens   int
}

// IPRateLimiter provides a simple, standard-library in-memory token bucket rate limiter per client IP.
type IPRateLimiter struct {
	mu          sync.Mutex
	clients     map[string]*clientRecord
	limit       int           // max requests per window
	window      time.Duration // window duration
	cleanupRate time.Duration
}

// NewIPRateLimiter creates a new IPRateLimiter with background garbage collection.
func NewIPRateLimiter(limit int, window time.Duration) *IPRateLimiter {
	limiter := &IPRateLimiter{
		clients:     make(map[string]*clientRecord),
		limit:       limit,
		window:      window,
		cleanupRate: window * 2,
	}

	go limiter.cleanupLoop()
	return limiter
}

func (l *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(l.cleanupRate)
	for range ticker.C {
		l.mu.Lock()
		cutoff := time.Now().Add(-l.window)
		for ip, rec := range l.clients {
			if rec.lastSeen.Before(cutoff) {
				delete(l.clients, ip)
			}
		}
		l.mu.Unlock()
	}
}

// Allow checks if the given IP is within the rate limit.
func (l *IPRateLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	rec, exists := l.clients[ip]
	if !exists || now.Sub(rec.lastSeen) > l.window {
		l.clients[ip] = &clientRecord{lastSeen: now, tokens: l.limit - 1}
		return true
	}

	rec.lastSeen = now
	if rec.tokens > 0 {
		rec.tokens--
		return true
	}

	return false
}

// RateLimitMiddleware returns a Gin middleware restricting requests per client IP.
func RateLimitMiddleware(limiter *IPRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !limiter.Allow(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    http.StatusTooManyRequests,
				"status":  "error",
				"error":   "rate limit exceeded",
				"message": "Too many requests. Please try again later.",
			})
			return
		}
		c.Next()
	}
}
