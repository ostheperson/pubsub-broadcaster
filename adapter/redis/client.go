package redisadapter

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// RedisClient defines the interface for the Redis client functionality
// required by the adapter. This allows users to provide their own
// redis.Client instance, regardless of its specific version, as long
// as it implements these methods.
type RedisClient interface {
	Publish(ctx context.Context, channel string, message any) *redis.IntCmd
	Subscribe(ctx context.Context, channels ...string) *redis.PubSub
	Close() error
}

// Ensure that *redis.Client implements the RedisClient interface.
var _ RedisClient = (*redis.Client)(nil)
