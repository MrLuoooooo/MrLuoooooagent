package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter 按 IP 限流，token bucket 实现。
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*tokenBucket
	rate     float64 // tokens per second
	capacity float64 // max burst
	done     chan struct{}
}

type tokenBucket struct {
	tokens   float64
	lastTime time.Time
}

// NewRateLimiter — rate 是每秒请求数，burst 是突发容量。
func NewRateLimiter(rate, capacity float64) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*tokenBucket),
		rate:     rate,
		capacity: capacity,
		done:     make(chan struct{}),
	}
	// Clean up stale buckets every 5 minutes.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rl.mu.Lock()
				for ip, b := range rl.buckets {
					if time.Since(b.lastTime) > 10*time.Minute {
						delete(rl.buckets, ip)
					}
				}
				rl.mu.Unlock()
			case <-rl.done:
				return
			}
		}
	}()
	return rl
}

// Stop signals the cleanup goroutine to exit. Safe to call multiple times.
func (rl *RateLimiter) Stop() {
	select {
	case <-rl.done:
		// already closed
	default:
		close(rl.done)
	}
}

func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[ip]
	if !ok {
		b = &tokenBucket{tokens: rl.capacity, lastTime: time.Now()}
		rl.buckets[ip] = b
	}

	elapsed := time.Since(b.lastTime).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > rl.capacity {
		b.tokens = rl.capacity
	}
	b.lastTime = time.Now()

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// Middleware 返回 Gin 中间件，按 IP 限流。
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !rl.allow(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "too many requests",
			})
			return
		}
		c.Next()
	}
}
