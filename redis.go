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
	rdb := redis.NewClient(&redis.Options{
		Addr:        addr,
		PoolTimeout: 5 * time.Second,
		Password:    pw,
		DB:          0,
		PoolSize:    10,
	})
	return &redisClient{client: rdb}
}

func (rpsc *redisClient) GetClient() *redis.Client {
	return rpsc.client
}

func (rpsc *redisClient) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return rpsc.client.Ping(ctx).Err()
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

func (rpsc *redisClient) SubscribeWithHandler(
	ctx context.Context,
	channel string,
	handler func(ctx context.Context, payload []byte) error,
) error {
	pubsub := rpsc.client.PSubscribe(ctx, channel)
	defer pubsub.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-pubsub.Channel():
			if !ok {
				return fmt.Errorf("redis subscription channel closed")
			}
			if err := handler(ctx, []byte(msg.Payload)); err != nil {
				return err
			}
		}
	}
}

func (rpsc *redisClient) SubscribeWithChannel(
	ctx context.Context,
	channel string,
) PubSubClient {
	return rpsc.client.Subscribe(ctx, channel)
}
