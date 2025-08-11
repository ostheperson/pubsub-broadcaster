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
	SubscribeWithHandler(
		ctx context.Context,
		channel string,
		handler func(ctx context.Context, payload []byte) error,
	) error
	SubscribeWithChannel(ctx context.Context, channel string) PubSubClient
}

type PubSubClient interface {
	Channel(...redis.ChannelOption) <-chan *redis.Message
	Close() error
}
