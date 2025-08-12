package broadcaster

import (
	"context"
)

// Message is the generic message format used by the broadcaster.
// Any pub/sub implementation must adapt its specific message format to this one.
type Message struct {
	Channel string
	Payload []byte
}

// Interface for a subscription to a channel.
type Subscriber interface {
	// Channel returns a read-only channel that delivers messages
	Channel() <-chan *Message
	// Close closes the subscription
	Close() error
}

// PubSub is the interface for a pub/sub ststem
type PubSub interface {
	Publish(ctx context.Context, channel string, payload any) error
	Subscribe(ctx context.Context, channel string) (Subscriber, error)
}
