package middleware

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
)

// failClosedRoutes are buckets where, if Redis is unreachable, we DENY rather
// than ALLOW. Login and signup endpoints — better to error than to expose
// brute-force/spam attack surface.
var failClosedRoutes = map[string]bool{
	"login":    true,
	"register": true,
}

type RateLimiter struct {
	limiter *redis_rate.Limiter
}

// NewRateLimiter returns a limiter. If rdb is nil, all calls fail-open (or
// fail-closed for sensitive routes — see failClosedRoutes).
func NewRateLimiter(rdb *redis.Client) *RateLimiter {
	if rdb == nil {
		return &RateLimiter{limiter: nil}
	}
	return &RateLimiter{limiter: redis_rate.NewLimiter(rdb)}
}

// PerIP rate-limits using the client IP as the key.
func (r *RateLimiter) PerIP(bucket string, limit int, windowSeconds int) gin.HandlerFunc {
	return r.middleware(bucket, limit, windowSeconds, func(c *gin.Context) string {
		return "ip:" + c.ClientIP()
	})
}

// PerUser rate-limits using the JWT's user_id as the key. Falls back to IP if
// no token is present (e.g., unauthenticated request hits an authed route — the
// downstream service will reject it on JWT, but we still want to limit the spam).
func (r *RateLimiter) PerUser(bucket string, limit int, windowSeconds int) gin.HandlerFunc {
	return r.middleware(bucket, limit, windowSeconds, func(c *gin.Context) string {
		if uid := peekUserID(c.GetHeader("Authorization")); uid != "" {
			return "user:" + uid
		}
		return "ip:" + c.ClientIP()
	})
}

func (r *RateLimiter) middleware(bucket string, limit, windowSeconds int, keyFn func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if r.limiter == nil {
			if failClosedRoutes[bucket] {
				abort429(c, windowSeconds)
				return
			}
			c.Next()
			return
		}

		key := "ratelimit:" + bucket + ":" + keyFn(c)
		ctx, cancel := context.WithTimeout(c.Request.Context(), 200*time.Millisecond)
		defer cancel()

		res, err := r.limiter.Allow(ctx, key, redis_rate.Limit{
			Rate:   limit,
			Burst:  limit,
			Period: time.Duration(windowSeconds) * time.Second,
		})
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				if failClosedRoutes[bucket] {
					abort429(c, windowSeconds)
					return
				}
				c.Next()
				return
			}
			if failClosedRoutes[bucket] {
				abort429(c, windowSeconds)
				return
			}
			c.Next()
			return
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))

		if res.Allowed == 0 {
			retryAfter := int(res.RetryAfter.Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			abort429(c, retryAfter)
			return
		}
		c.Next()
	}
}

func abort429(c *gin.Context, retryAfterSeconds int) {
	c.Header("Retry-After", strconv.Itoa(retryAfterSeconds))
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"error":               "rate_limit_exceeded",
		"retry_after_seconds": retryAfterSeconds,
	})
}
