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
	stopChan    chan struct{}
	stopOnce    sync.Once
}

// NewIPRateLimiter creates a new IPRateLimiter with background garbage collection.
func NewIPRateLimiter(limit int, window time.Duration) *IPRateLimiter {
	limiter := &IPRateLimiter{
		clients:     make(map[string]*clientRecord),
		limit:       limit,
		window:      window,
		cleanupRate: window * 2,
		stopChan:    make(chan struct{}),
	}

	if limit > 0 && window > 0 {
		go limiter.cleanupLoop()
	}
	return limiter
}

// Stop gracefully stops the background cleanup loop.
func (l *IPRateLimiter) Stop() {
	if l == nil {
		return
	}
	l.stopOnce.Do(func() {
		if l.stopChan != nil {
			close(l.stopChan)
		}
	})
}

func (l *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(l.cleanupRate)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.mu.Lock()
			cutoff := time.Now().Add(-l.window)
			for ip, rec := range l.clients {
				if rec.lastSeen.Before(cutoff) {
					delete(l.clients, ip)
				}
			}
			l.mu.Unlock()
		case <-l.stopChan:
			return
		}
	}
}

// Allow checks if the given IP is within the rate limit.
func (l *IPRateLimiter) Allow(ip string) bool {
	if l == nil || l.limit <= 0 || l.window <= 0 {
		return true
	}

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

// RateLimiter defines the contract for request rate limiters, allowing
// in-memory token bucket implementations or distributed backends (e.g. Redis).
type RateLimiter interface {
	Allow(key string) bool
	Stop()
}

// RateLimitMiddleware returns a Gin middleware restricting requests per client IP.
func RateLimitMiddleware(limiter RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limiter == nil {
			c.Next()
			return
		}
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
