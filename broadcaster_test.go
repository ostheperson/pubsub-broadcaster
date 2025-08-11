package broadcaster

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type mockPubSubClient struct {
	ch chan *redis.Message
	mu sync.Mutex
}

func (m *mockPubSubClient) Channel(...redis.ChannelOption) <-chan *redis.Message {
	return m.ch
}

func (m *mockPubSubClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ch != nil {
		close(m.ch)
		m.ch = nil
	}
	return nil
}

type mockPubSub struct {
	mu      sync.Mutex
	clients map[string]*mockPubSubClient
}

func (t *mockPubSub) Publish(ctx context.Context, channel string, payload any) error {
	return nil
}

func (t *mockPubSub) SubscribeWithHandler(ctx context.Context, channel string, handler func(ctx context.Context, payload []byte) error) error {
	return nil
}

func (m *mockPubSub) SubscribeWithChannel(ctx context.Context, channel string) PubSubClient {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.clients == nil {
		m.clients = make(map[string]*mockPubSubClient)
	}

	if m.clients[channel] != nil {
		m.clients[channel].Close()
	}

	client := &mockPubSubClient{
		ch: make(chan *redis.Message, 10),
	}
	m.clients[channel] = client
	return client
}

func TestBroadcaster_HandleRedisDisconnectAndReconnect(t *testing.T) {
	mockPbs := &mockPubSub{}
	logger := slog.New(slog.Default().Handler())
	channel := "test_channel"
	onEmpty := func(s string) {}

	b := NewBroadcaster(channel, mockPbs, logger, onEmpty)
	clientChan := b.AddClient(1)
	b.Start(context.Background())

	time.Sleep(100 * time.Millisecond)

	t.Run("Initial messages are received", func(t *testing.T) {
		mockClient, ok := mockPbs.clients[channel]
		if !ok {
			t.Fatalf("Expected mock client for channel %s to exist", channel)
		}

		testMessage := &redis.Message{Payload: "message 1"}
		mockClient.ch <- testMessage

		select {
		case msg := <-clientChan:
			if string(msg) != "message 1" {
				t.Errorf("Expected 'message 1', got %s", string(msg))
			}
		case <-time.After(50 * time.Millisecond):
			t.Fatal("Timeout waiting for message 1")
		}
	})

	t.Run("Disconnection is handled", func(t *testing.T) {

		mockPbs.clients[channel].Close()
		time.Sleep(200 * time.Millisecond)
		select {
		case <-clientChan:
			t.Fatal("Did not expect to receive a message after disconnection")
		case <-time.After(50 * time.Millisecond):
		}
	})

	t.Run("Reconnection succeeds and new messages are received", func(t *testing.T) {
		time.Sleep(1 * time.Second)

		mockClient, ok := mockPbs.clients[channel]
		if !ok {
			t.Fatal("Expected a new mock client after reconnection attempt")
		}

		testMessage := &redis.Message{Payload: "message 3"}
		mockClient.ch <- testMessage

		select {
		case msg := <-clientChan:
			if string(msg) != "message 3" {
				t.Errorf("Expected 'message 3', got %s", string(msg))
			}
		case <-time.After(50 * time.Millisecond):
			t.Fatal("Timeout waiting for message 3 after reconnection")
		}
	})

	b.Stop()
}
