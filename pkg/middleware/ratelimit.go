package middleware

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/yourusername/ecom-backend/pkg/response"
)

// RateLimitConfig configures the rate limiter.
type RateLimitConfig struct {
	// Max allowed requests in Window duration
	Max int
	// Window is the time window for rate limiting
	Window time.Duration
	// KeyFunc extracts the rate limit key from the request.
	// Default: IP address
	KeyFunc func(c *fiber.Ctx) string
	// Message shown when rate limit is exceeded
	Message string
}

// RateLimit returns a Redis-backed sliding window rate limiter middleware.
func RateLimit(rdb *redis.Client, cfg RateLimitConfig) fiber.Handler {
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = func(c *fiber.Ctx) string {
			return c.IP()
		}
	}
	if cfg.Message == "" {
		cfg.Message = "Too many requests. Please slow down."
	}

	return func(c *fiber.Ctx) error {
		key := fmt.Sprintf("ratelimit:%s:%s", c.Path(), cfg.KeyFunc(c))

		ctx := context.Background()
		now := time.Now().UnixMilli()
		windowMs := cfg.Window.Milliseconds()

		// Sliding window using Redis sorted set
		pipe := rdb.Pipeline()

		// Remove expired entries
		pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(now-windowMs, 10))

		// Count current requests in window
		countCmd := pipe.ZCard(ctx, key)

		// Add current request
		pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: now})

		// Set TTL
		pipe.Expire(ctx, key, cfg.Window+time.Second)

		_, err := pipe.Exec(ctx)
		if err != nil {
			// On Redis error: allow request (fail open)
			return c.Next()
		}

		count := countCmd.Val()

		// Set rate limit headers
		remaining := int64(cfg.Max) - count
		if remaining < 0 {
			remaining = 0
		}

		c.Set("X-RateLimit-Limit", strconv.Itoa(cfg.Max))
		c.Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		c.Set("X-RateLimit-Reset", strconv.FormatInt(now/1000+int64(cfg.Window.Seconds()), 10))

		if count >= int64(cfg.Max) {
			retryAfter := int(cfg.Window.Seconds())
			c.Set("Retry-After", strconv.Itoa(retryAfter))
			return response.TooManyRequests(c, cfg.Message)
		}

		return c.Next()
	}
}

// ── Pre-configured rate limiters for specific endpoints ────────

// OTPRateLimit: 5 OTP sends per phone per hour
func OTPRateLimit(rdb *redis.Client) fiber.Handler {
	return RateLimit(rdb, RateLimitConfig{
		Max:    5,
		Window: time.Hour,
		KeyFunc: func(c *fiber.Ctx) string {
			// Rate limit by phone number from request body
			// Parsed phone is stored in context by handler
			phone := c.Locals("rate_limit_phone")
			if phone == nil {
				return c.IP()
			}
			return fmt.Sprintf("phone:%s", phone)
		},
		Message: "Too many OTP requests. Please wait before requesting another.",
	})
}

// CheckoutRateLimit: 10 checkout attempts per IP per hour
func CheckoutRateLimit(rdb *redis.Client) fiber.Handler {
	return RateLimit(rdb, RateLimitConfig{
		Max:     10,
		Window:  time.Hour,
		Message: "Too many checkout attempts. Please wait before trying again.",
	})
}

// SearchRateLimit: 30 searches per IP per minute
func SearchRateLimit(rdb *redis.Client) fiber.Handler {
	return RateLimit(rdb, RateLimitConfig{
		Max:     30,
		Window:  time.Minute,
		Message: "Search rate limit exceeded. Please slow down.",
	})
}

// LoginRateLimit: 10 login attempts per IP per 15 minutes
func LoginRateLimit(rdb *redis.Client) fiber.Handler {
	return RateLimit(rdb, RateLimitConfig{
		Max:     10,
		Window:  15 * time.Minute,
		Message: "Too many login attempts. Please wait 15 minutes.",
	})
}

// APIRateLimit: General API rate limit - 200 requests per IP per minute
func APIRateLimit(rdb *redis.Client) fiber.Handler {
	return RateLimit(rdb, RateLimitConfig{
		Max:     200,
		Window:  time.Minute,
		Message: "API rate limit exceeded.",
	})
}
