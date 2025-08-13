package inmemmoryadapter

import (
	"context"
	"encoding/json"
	"sync"

	broadcaster "github.com/ostheperson/pubsub-broadcaster"
)

type Adapter struct {
	mu          sync.RWMutex
	subscribers map[string]chan *broadcaster.Message
}

func New() *Adapter {
	return &Adapter{
		subscribers: make(map[string]chan *broadcaster.Message),
	}
}

func (a *Adapter) Close() error {
	return nil
}

func (a *Adapter) Publish(ctx context.Context, topic string, msg any) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	message, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if ch, ok := a.subscribers[topic]; ok {
		ch <- &broadcaster.Message{
			Topic:   topic,
			Payload: message,
		}
	}

	return nil
}

func (a *Adapter) Subscribe(ctx context.Context, topic string) (<-chan *broadcaster.Message, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	ch := make(chan *broadcaster.Message, 10)
	a.subscribers[topic] = ch

	return ch, nil
}
