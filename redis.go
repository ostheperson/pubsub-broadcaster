package broadcaster

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisClient struct {
	client *redis.Client
}

func NewRedisClient(addr, pw string) *redisClient {
	return &redisClient{client: redis.NewClient(&redis.Options{
		Addr:        addr,
		PoolTimeout: 5 * time.Second,
		Password:    pw,
		DB:          0,
		PoolSize:    10,
	})}
}

func (rpsc *redisClient) GetClient() *redis.Client {
	return rpsc.client
}

func (r *redisClient) Close() {
	r.client.Close()
}

func (rpsc *redisClient) Publish(
	ctx context.Context,
	channel string,
	payload any,
) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	if err = rpsc.client.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("failed to publish to Redis: %w", err)
	}
	return nil
}

func (rpsc *redisClient) Subscribe(
	ctx context.Context,
	channel string,
) PubSubClient {
	return rpsc.client.Subscribe(ctx, channel)
}
