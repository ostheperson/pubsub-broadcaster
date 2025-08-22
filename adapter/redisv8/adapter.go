package redisv8adapter

import (
	"context"

	"github.com/go-redis/redis/v8"
	broadcaster "github.com/ostheperson/pubsub-broadcaster"
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

func (a *Adapter) Publish(ctx context.Context, channel string, payload []byte) error {
	return a.client.Publish(ctx, channel, payload).Err()
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
