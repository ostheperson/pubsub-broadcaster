package broadcaster

import (
	"context"
	"sync"
	"testing"
	"time"
)

type mockSubscriber struct {
	mu      sync.Mutex
	closed  bool
	clients map[string]chan *Message
}

func newMockSubscriber() *mockSubscriber {
	return &mockSubscriber{
		clients: make(map[string]chan *Message),
	}
}

func (m *mockSubscriber) Subscribe(ctx context.Context, channel string) (<-chan *Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan *Message, 10)
	m.clients[channel] = ch
	return ch, nil
}

func (m *mockSubscriber) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		for _, ch := range m.clients {
			close(ch)
		}
		m.closed = true
	}
	return nil
}

func (m *mockSubscriber) Publish(ctx context.Context, channel string, msg *Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ch, ok := m.clients[channel]; ok {
		ch <- msg
	}
}

func TestBroadcaster_HandleRedisDisconnectAndReconnect(t *testing.T) {
	mockPbs := newMockSubscriber()
	channel := "test_channel"
	disconnect := make(chan string, 1)
	backoff := 100 * time.Millisecond

	b := NewBroadcaster(channel, mockPbs, disconnect, backoff, 1*time.Second, 100*time.Millisecond, 1)
	clientChan := b.Add("player1")

	b.Start(context.Background())

	go func() {
		<-disconnect
	}()

	time.Sleep(50 * time.Millisecond)

	t.Run("Initial messages are received", func(t *testing.T) {
		testMessage := &Message{Payload: []byte("message 1")}
		mockPbs.Publish(context.Background(), channel, testMessage)
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
		mockPbs.Close()
		time.Sleep(backoff)
		select {
		case <-clientChan:
			t.Fatal("Did not expect to receive a message after disconnection")
		case <-time.After(50 * time.Millisecond):
		}
	})

	t.Run("Reconnection succeeds and new messages are received", func(t *testing.T) {
		testMessage := &Message{Payload: []byte("message 3")}
		mockPbs.Publish(context.Background(), channel, testMessage)

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

func TestBroadcaster_ClientRemoval(t *testing.T) {
	mockPbs := newMockSubscriber()
	channel := "test_channel"
	disconnect := make(chan string, 1)

	b := NewBroadcaster(channel, mockPbs, disconnect, time.Second, 1*time.Second, 100*time.Millisecond, 1)
	b.Start(context.Background())
	time.Sleep(50 * time.Millisecond)
	defer b.Stop()

	go func() {
		<-disconnect
	}()

	client1Chan := b.Add("player1")
	client2Chan := b.Add("player2")

	b.Remove("player1")
	testMessage := &Message{Payload: []byte("message 1")}
	mockPbs.Publish(context.Background(), channel, testMessage)

	select {
	case _, ok := <-client1Chan:
		if ok {
			t.Fatal("Did not expect an open channel")
		}
	case <-time.After(50 * time.Millisecond):
	}

	select {
	case msg := <-client2Chan:
		if string(msg) != "message 1" {
			t.Errorf("Expected 'message 1', got %s", string(msg))
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatal("Timeout waiting for message")
	}
}

func TestBroadcaster_MultipleClients(t *testing.T) {
	mockPbs := newMockSubscriber()
	channel := "test_channel"
	disconnect := make(chan string, 1)

	b := NewBroadcaster(channel, mockPbs, disconnect, time.Second, 1*time.Second, 100*time.Millisecond, 1)
	b.Start(context.Background())
	time.Sleep(50 * time.Millisecond)

	defer b.Stop()

	go func() {
		<-disconnect
	}()

	client1Chan := b.Add("player1")
	client2Chan := b.Add("player2")

	testMessage := &Message{Payload: []byte("message 1")}
	mockPbs.Publish(context.Background(), channel, testMessage)

	for i := 0; i < 2; i++ {
		select {
		case msg := <-client1Chan:
			if string(msg) != "message 1" {
				t.Errorf("Expected 'message 1', got %s", string(msg))
			}
		case msg := <-client2Chan:
			if string(msg) != "message 1" {
				t.Errorf("Expected 'message 1', got %s", string(msg))
			}
		case <-time.After(50 * time.Millisecond):
			t.Fatal("Timeout waiting for message")
		}
	}
}
