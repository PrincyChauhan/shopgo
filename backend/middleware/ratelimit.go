package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type bucket struct {
	count int
	reset time.Time
}

// RateLimit allows `limit` requests per `window` for each client IP.
// Used on login/register to slow down password guessing.
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	buckets := map[string]*bucket{}

	// Housekeeping: forget old IPs so the map can't grow forever.
	go func() {
		for range time.Tick(window) {
			mu.Lock()
			now := time.Now()
			for ip, b := range buckets {
				if now.After(b.reset) {
					delete(buckets, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		mu.Lock()
		b := buckets[ip]
		if b == nil || now.After(b.reset) {
			b = &bucket{reset: now.Add(window)}
			buckets[ip] = b
		}
		b.count++
		blocked := b.count > limit
		mu.Unlock()

		if blocked {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many attempts, please wait a minute"})
			return
		}
		c.Next()
	}
}
