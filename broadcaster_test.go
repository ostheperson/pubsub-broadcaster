package broadcaster

import (
	"context"
	"sync"
	"testing"
	"time"
)

type mockPubSubClient struct {
	ch chan *Message
	mu sync.Mutex
}

func (m *mockPubSubClient) Channel() <-chan *Message {
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

func (m *mockPubSub) Subscribe(ctx context.Context, channel string) (Subscriber, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.clients == nil {
		m.clients = make(map[string]*mockPubSubClient)
	}

	if m.clients[channel] != nil {
		m.clients[channel].Close()
	}

	client := &mockPubSubClient{
		ch: make(chan *Message, 10),
	}
	m.clients[channel] = client
	return client, nil
}

func TestBroadcaster_HandleRedisDisconnectAndReconnect(t *testing.T) {
	mockPbs := &mockPubSub{}
	channel := "test_channel"

	b := NewBroadcaster(channel, mockPbs, make(chan<- string), time.Second, 2*time.Second, 100*time.Millisecond, 10)
	clientChan := b.AddClient(1)

	b.Start(context.Background())

	time.Sleep(100 * time.Millisecond)

	t.Run("Initial messages are received", func(t *testing.T) {
		mockClient, ok := mockPbs.clients[channel]
		if !ok {
			t.Fatalf("Expected mock client for channel %s to exist", channel)
		}

		testMessage := &Message{Payload: []byte("message 1")}
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

		testMessage := &Message{Payload: []byte("message 3")}
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
