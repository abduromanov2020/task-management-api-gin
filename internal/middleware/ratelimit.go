package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/abduromanov2020/tasks-api/internal/apperr"
)

// IPRateLimiter is a small in-memory per-IP token-bucket limiter. Intended
// for auth endpoints only; for cross-replica enforcement a Redis-backed
// limiter would be required (noted in README).
type IPRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	rps      rate.Limit
	burst    int
}

func NewIPRateLimiter(perMinute int) *IPRateLimiter {
	rps := rate.Limit(float64(perMinute) / 60.0)
	return &IPRateLimiter{
		limiters: map[string]*rate.Limiter{},
		rps:      rps,
		burst:    perMinute,
	}
}

func (r *IPRateLimiter) get(ip string) *rate.Limiter {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.limiters[ip]
	if !ok {
		l = rate.NewLimiter(r.rps, r.burst)
		r.limiters[ip] = l
	}
	return l
}

func (r *IPRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !r.get(c.ClientIP()).AllowN(time.Now(), 1) {
			_ = c.Error(apperr.New(429, "RATE_LIMITED", "Too many requests; slow down"))
			c.Abort()
			return
		}
		c.Next()
	}
}
