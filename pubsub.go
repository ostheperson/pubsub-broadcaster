package broadcaster

import (
	"context"
)

type Message struct {
	Topic   string
	Payload []byte
}

type Subscriber interface {
	Subscribe(ctx context.Context, channel string) (<-chan *Message, error)
	Close() error
}

type Publisher interface {
	Publish(ctx context.Context, channel string, payload []byte) error
	Close() error
}
