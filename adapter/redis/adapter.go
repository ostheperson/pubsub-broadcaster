package redisadapter

import (
	"context"
	"encoding/json"
	"fmt"

	broadcaster "github.com/ostheperson/pubsub-broadcaster"
	"github.com/redis/go-redis/v9"
)

type Adapter struct {
	client *redis.Client
}

func New(client *redis.Client) *Adapter {
	return &Adapter{client: client}
}

func (a *Adapter) Close() error {
	return a.client.Close()
}

func (a *Adapter) Publish(ctx context.Context, channel string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	if err = a.client.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("failed to publish to Redis: %w", err)
	}
	return nil
}

func (a *Adapter) Subscribe(ctx context.Context, channel string) (<-chan *broadcaster.Message, error) {
	pubsub := a.client.Subscribe(ctx, channel)
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
