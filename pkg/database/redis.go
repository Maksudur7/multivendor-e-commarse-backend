package database

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates and verifies a Redis client connection.
func NewRedisClient(ctx context.Context, redisURL string, maxRetries int) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	opts.MaxRetries = maxRetries
	opts.DialTimeout = 5e9   // 5 seconds
	opts.ReadTimeout  = 3e9  // 3 seconds
	opts.WriteTimeout = 3e9  // 3 seconds
	opts.PoolSize = 20
	opts.MinIdleConns = 5

	client := redis.NewClient(opts)

	// Verify connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return client, nil
}
