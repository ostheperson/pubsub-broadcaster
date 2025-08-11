package broadcaster

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type PubSub interface {
	Publish(
		ctx context.Context,
		channel string,
		payload any,
	) error
	Subscribe(ctx context.Context, channel string) PubSubClient
}

type PubSubClient interface {
	Channel(...redis.ChannelOption) <-chan *redis.Message
	Close() error
}
