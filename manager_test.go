package broadcaster

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type mockSubscriber struct {
	ch chan *Message
}

func (m *mockSubscriber) Close() error {
	close(m.ch)
	return nil
}

func (m *mockSubscriber) Channel() <-chan *Message {
	return m.ch
}

type mockPubsub struct {
	mu          sync.Mutex
	messages    map[string][]*Message
	subscribers map[string][]*mockSubscriber
}

func newMockPubSub() *mockPubsub {
	return &mockPubsub{
		messages:    make(map[string][]*Message),
		subscribers: make(map[string][]*mockSubscriber),
	}
}

func (m *mockPubsub) Publish(ctx context.Context, channel string, message any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages[channel] = append(m.messages[channel], &Message{Payload: []byte(fmt.Sprint(message)), Channel: channel})
	for _, sub := range m.subscribers[channel] {
		sub.ch <- &Message{Payload: []byte(message.(string)), Channel: channel}
	}
	return nil
}

func (m *mockPubsub) Subscribe(ctx context.Context, channel string) (Subscriber, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sub := &mockSubscriber{ch: make(chan *Message, 1)}
	m.subscribers[channel] = append(m.subscribers[channel], sub)
	return sub, nil
}

func TestManager(t *testing.T) {
	t.Run("registration and unregistration", func(t *testing.T) {
		pubsub := newMockPubSub()
		manager := NewManager(WithPubSub(pubsub))
		defer manager.Stop()

		channel := "test-channel"
		client1ID := 1
		client2ID := 2
		t.Run("registration on same channel creates one broadcaster", func(t *testing.T) {
			manager.RegisterClient(channel, client1ID)
			if len(manager.activeBroadcasters) != 1 {
				t.Fatalf("expected 1 active broadcaster, got %d", len(manager.activeBroadcasters))
			}

			manager.RegisterClient(channel, client2ID)
			if len(manager.activeBroadcasters) != 1 {
				t.Fatalf("expected 1 active broadcaster after second client, got %d", len(manager.activeBroadcasters))
			}
		})

		t.Run("unregistration removes client from broadcaster and closes after empty", func(t *testing.T) {
			manager.UnregisterClient(channel, client1ID)
			if len(manager.activeBroadcasters) != 1 {
				t.Fatalf("expected 1 active broadcaster after one client unregisters, got %d", len(manager.activeBroadcasters))
			}

			manager.UnregisterClient(channel, client2ID)
			time.Sleep(10 * time.Millisecond) // allow time for disconnect to be processed
			if len(manager.activeBroadcasters) != 0 {
				t.Fatalf("expected 0 active broadcasters after all clients unregister, got %d", len(manager.activeBroadcasters))
			}
		})
	})

	t.Run("closes all broadcasters after manager stops", func(t *testing.T) {
		pubsub := newMockPubSub()
		manager := NewManager(WithPubSub(pubsub))

		manager.RegisterClient("channel1", 1)
		manager.RegisterClient("channel2", 1)
		if len(manager.activeBroadcasters) != 2 {
			t.Fatalf("expected 2 active broadcasters, got %d", len(manager.activeBroadcasters))
		}

		manager.Stop()
		if len(manager.activeBroadcasters) != 0 {
			t.Fatalf("expected 0 active broadcasters after stop, got %d", len(manager.activeBroadcasters))
		}
	})

	t.Run("stop single broadcaster", func(t *testing.T) {
		pubsub := newMockPubSub()
		manager := NewManager(WithPubSub(pubsub))

		manager.RegisterClient("channel1", 1)
		if len(manager.activeBroadcasters) != 1 {
			t.Fatalf("expected 1 active broadcasters, got %d", len(manager.activeBroadcasters))
		}

		manager.StopBroadcaster("channel1")
		if len(manager.activeBroadcasters) != 0 {
			t.Fatalf("expected 0 active broadcasters after stop, got %d", len(manager.activeBroadcasters))
		}
	})

	t.Run("concurrent access", func(t *testing.T) {
		pubsub := newMockPubSub()
		manager := NewManager(WithPubSub(pubsub))
		defer manager.Stop()

		var wg sync.WaitGroup
		numGoroutines := 100
		numChannels := 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				channel := "channel-" + fmt.Sprint(id%numChannels)
				clientID := id
				manager.RegisterClient(channel, clientID)
				time.Sleep(1 * time.Millisecond)
				manager.UnregisterClient(channel, clientID)
			}(i)
		}

		wg.Wait()
		time.Sleep(10 * time.Millisecond) // allow time for disconnects
		if len(manager.activeBroadcasters) != 0 {
			t.Fatalf("expected 0 active broadcasters after concurrent access, got %d", len(manager.activeBroadcasters))
		}
	})
}
