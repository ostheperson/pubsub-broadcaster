package inmemmoryadapter

import (
	"context"
	"encoding/json"
	"sync"

	broadcaster "github.com/ostheperson/pubsub-broadcaster"
)

type client struct {
	mu          sync.RWMutex
	subscribers map[string][]*subscriber
}

func New() *client {
	return &client{
		subscribers: make(map[string][]*subscriber),
	}
}

func (a *client) Publish(ctx context.Context, channel string, msg any) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	message, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	for _, sub := range a.subscribers[channel] {
		sub.ch <- &broadcaster.Message{
			Channel: channel,
			Payload: message,
		}
	}

	return nil
}

func (a *client) Subscribe(ctx context.Context, channel string) (broadcaster.Subscriber, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	sub := &subscriber{
		ch:      make(chan *broadcaster.Message, 10),
		client:  a,
		channel: channel,
	}

	a.subscribers[channel] = append(a.subscribers[channel], sub)

	return sub, nil
}

type subscriber struct {
	ch      chan *broadcaster.Message
	client  *client
	channel string
}

func (s *subscriber) Channel() <-chan *broadcaster.Message {
	return s.ch
}

func (s *subscriber) Close() error {
	s.client.mu.Lock()
	defer s.client.mu.Unlock()

	subs := s.client.subscribers[s.channel]
	for i, sub := range subs {
		if sub == s {
			s.client.subscribers[s.channel] = append(subs[:i], subs[i+1:]...)
			break
		}
	}

	close(s.ch)
	return nil
}
