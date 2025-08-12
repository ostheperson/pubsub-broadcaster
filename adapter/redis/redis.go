package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	broadcaster "github.com/ostheperson/pubsub-broadcaster"
	"github.com/redis/go-redis/v9"
)

// Client is a Redis adapter that implements the broadcaster.PubSub interface.
type Client struct {
	client *redis.Client
}

// New creates a new Redis client adapter.
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

// GetClient returns the underlying Redis client.
func (c *Client) GetClient() *redis.Client {
	return c.client
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	return c.client.Close()
}

// Publish publishes a message to a Redis channel.
func (c *Client) Publish(ctx context.Context, channel string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	if err = c.client.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("failed to publish to Redis: %w", err)
	}
	return nil
}

// Subscribe creates a subscription to a Redis channel.
func (c *Client) Subscribe(ctx context.Context, channel string) (broadcaster.Subscriber, error) {
	pubsub := c.client.Subscribe(ctx, channel)
	return &redisSubscriber{
		pubsub:  pubsub,
		channel: channel,
		msgChan: make(chan *broadcaster.Message, 10),
		done:    make(chan struct{}),
	}, nil
}

// redisSubscriber implements the broadcaster.Subscriber interface for Redis.
type redisSubscriber struct {
	pubsub  *redis.PubSub
	channel string
	msgChan chan *broadcaster.Message
	done    chan struct{}
}

// Channel returns the message channel.
func (s *redisSubscriber) Channel() <-chan *broadcaster.Message {
	// Start the message forwarding goroutine if not already started
	go s.forwardMessages()
	return s.msgChan
}

// Close closes the subscription.
func (s *redisSubscriber) Close() error {
	close(s.done)
	close(s.msgChan)
	return s.pubsub.Close()
}

// forwardMessages forwards Redis messages to our generic message format.
func (s *redisSubscriber) forwardMessages() {
	redisChan := s.pubsub.Channel()
	for {
		select {
		case <-s.done:
			return
		case msg, ok := <-redisChan:
			if !ok {
				return
			}
			// Convert Redis message to our generic format
			genericMsg := &broadcaster.Message{
				Channel: msg.Channel,
				Payload: []byte(msg.Payload),
			}
			select {
			case s.msgChan <- genericMsg:
			case <-s.done:
				return
			}
		}
	}
}
