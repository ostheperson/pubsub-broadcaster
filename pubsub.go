package broadcaster

import (
	"context"
)

type Message struct {
	Channel string
	Payload []byte
}

type Subscriber interface {
	Channel() <-chan *Message
	Close() error
}

type PubSub interface {
	Publish(ctx context.Context, channel string, payload any) error
	Subscribe(ctx context.Context, channel string) (Subscriber, error)
}
