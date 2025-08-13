package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	broadcaster "github.com/ostheperson/pubsub-broadcaster"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	client *redis.Client
}

func New(addr, password string) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:        addr,
		PoolTimeout: 5 * time.Second,
		Password:    password,
		DB:          0,
		PoolSize:    10,
	})
	return &Client{client: rdb}
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) Publish(ctx context.Context, topic string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	if err = c.client.Publish(ctx, topic, data).Err(); err != nil {
		return fmt.Errorf("failed to publish to Redis: %w", err)
	}
	return nil
}

func (c *Client) Subscribe(ctx context.Context, topic string) (<-chan *broadcaster.Message, error) {
	pubsub := c.client.Subscribe(ctx, topic)
	ch := make(chan *broadcaster.Message, 10)
	go func() {
		defer close(ch)
		redisCh := pubsub.Channel()
		for msg := range redisCh {
			ch <- &broadcaster.Message{
				Topic:   msg.Channel,
				Payload: []byte(msg.Payload),
			}
		}
	}()
	return ch, nil
}
