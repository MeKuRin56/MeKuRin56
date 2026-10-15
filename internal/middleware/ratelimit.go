package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type clientLimit struct {
	count     int
	lastReset time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]*clientLimit
	limit   int
	window  time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		clients: make(map[string]*clientLimit),
		limit:   limit,
		window:  window,
	}

	// Background cleanup of expired IP records every 5 minutes
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			rl.mu.Lock()
			now := time.Now()
			for ip, entry := range rl.clients {
				if now.Sub(entry.lastReset) > rl.window*2 {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()

	return rl
}

// Middleware creates a Gin handler to enforce the rate limit
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		rl.mu.Lock()
		client, exists := rl.clients[ip]
		if !exists || now.Sub(client.lastReset) > rl.window {
			rl.clients[ip] = &clientLimit{
				count:     1,
				lastReset: now,
			}
			rl.mu.Unlock()
			c.Next()
			return
		}

		if client.count >= rl.limit {
			rl.mu.Unlock()
			c.Header("Retry-After", "60")
			if c.ContentType() == "application/json" || c.GetHeader("Accept") == "application/json" || gin.Mode() == gin.TestMode {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error": "Too many requests. Please try again after 1 minute.",
				})
			} else {
				c.HTML(http.StatusTooManyRequests, "error.html", gin.H{
					"Title":   "Rate Limit Exceeded",
					"Message": "คุณทำรายการบ่อยเกินไป กรุณารอ 1 นาทีแล้วลองใหม่อีกครั้ง (Too many requests)",
				})
			}
			c.Abort()
			return
		}

		client.count++
		rl.mu.Unlock()
		c.Next()
	}
}
