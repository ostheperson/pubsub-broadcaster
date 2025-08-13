package broadcaster

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type mockPubsub struct {
	mu          sync.Mutex
	messages    map[string][]*Message
	subscribers map[string]chan *Message
}

func newMockPubSub() *mockPubsub {
	return &mockPubsub{
		messages:    make(map[string][]*Message),
		subscribers: make(map[string]chan *Message),
	}
}

func (m *mockPubsub) Publish(ctx context.Context, channel string, message any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages[channel] = append(m.messages[channel], &Message{Payload: []byte(fmt.Sprint(message)), Topic: channel})
	if ch, ok := m.subscribers[channel]; ok {
		ch <- &Message{Payload: []byte(message.(string)), Topic: channel}
	}
	return nil
}

func (m *mockPubsub) Subscribe(ctx context.Context, channel string) (<-chan *Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan *Message, 1)
	m.subscribers[channel] = ch
	return ch, nil
}

func (m *mockPubsub) Close() error {
	return nil
}

func TestManager(t *testing.T) {
	pubsub := newMockPubSub()
	t.Run("registration and unregistration", func(t *testing.T) {
		manager := NewManager(WithSubscriber(pubsub))
		defer manager.Stop()

		channel := "test-channel"
		client1ID := "1"
		client2ID := "2"
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

	t.Run("configurable options propagation", func(t *testing.T) {
		initialBackoff := 2 * time.Second
		maxBackoff := 20 * time.Second
		clientBufferSize := 20
		channelSendTimeout := 200 * time.Millisecond

		manager := NewManager(
			WithSubscriber(pubsub),
			WithInitialBackoff(initialBackoff),
			WithMaxBackoff(maxBackoff),
			WithClientBufferSize(clientBufferSize),
			WithChannelSendTimeout(channelSendTimeout),
		)
		defer manager.Stop()

		originalNewBroadcaster := newBroadcaster
		defer func() { newBroadcaster = originalNewBroadcaster }()

		var capturedInitialBackoff time.Duration
		var capturedMaxBackoff time.Duration
		var capturedClientBufferSize int
		var capturedChannelSendTimeout time.Duration

		newBroadcaster = func(topic string, subscriber Subscriber, disconnectChan chan<- string, initialBackoff, maxBackoff, channelSendTimeout time.Duration, clientBufferSize int) *broadcaster {
			capturedInitialBackoff = initialBackoff
			capturedMaxBackoff = maxBackoff
			capturedClientBufferSize = clientBufferSize
			capturedChannelSendTimeout = channelSendTimeout
			return originalNewBroadcaster(topic, subscriber, disconnectChan, initialBackoff, maxBackoff, channelSendTimeout, clientBufferSize)
		}

		manager.RegisterClient("test-channel", "1")

		if capturedInitialBackoff != initialBackoff {
			t.Errorf("expected initialBackoff to be %v, got %v", initialBackoff, capturedInitialBackoff)
		}
		if capturedMaxBackoff != maxBackoff {
			t.Errorf("expected maxBackoff to be %v, got %v", maxBackoff, capturedMaxBackoff)
		}
		if capturedClientBufferSize != clientBufferSize {
			t.Errorf("expected clientBufferSize to be %v, got %v", clientBufferSize, capturedClientBufferSize)
		}
		if capturedChannelSendTimeout != channelSendTimeout {
			t.Errorf("expected channelSendTimeout to be %v, got %v", channelSendTimeout, capturedChannelSendTimeout)
		}
	})

	t.Run("listen for disconnects", func(t *testing.T) {
		manager := NewManager(WithSubscriber(pubsub))
		defer manager.Stop()

		manager.RegisterClient("test-channel", "1")
		if len(manager.activeBroadcasters) != 1 {
			t.Fatalf("expected 1 active broadcaster, got %d", len(manager.activeBroadcasters))
		}

		manager.disconnectChan <- "test-channel"
		time.Sleep(10 * time.Millisecond) // allow time for disconnect to be processed

		if len(manager.activeBroadcasters) != 0 {
			t.Fatalf("expected 0 active broadcasters after disconnect, got %d", len(manager.activeBroadcasters))
		}
	})

	t.Run("stop single broadcaster", func(t *testing.T) {
		manager := NewManager(WithSubscriber(pubsub))

		manager.RegisterClient("channel1", "1")
		if len(manager.activeBroadcasters) != 1 {
			t.Fatalf("expected 1 active broadcasters, got %d", len(manager.activeBroadcasters))
		}

		manager.StopBroadcaster("channel1")
		if len(manager.activeBroadcasters) != 0 {
			t.Fatalf("expected 0 active broadcasters after stop, got %d", len(manager.activeBroadcasters))
		}
	})

	t.Run("closes all broadcasters after manager stops", func(t *testing.T) {
		manager := NewManager(WithSubscriber(pubsub))

		manager.RegisterClient("channel1", "1")
		manager.RegisterClient("channel2", "1")
		if len(manager.activeBroadcasters) != 2 {
			t.Fatalf("expected 2 active broadcasters, got %d", len(manager.activeBroadcasters))
		}

		manager.Stop()
		if len(manager.activeBroadcasters) != 0 {
			t.Fatalf("expected 0 active broadcasters after stop, got %d", len(manager.activeBroadcasters))
		}
	})
	t.Run("concurrent registeration and unregistration", func(t *testing.T) {
		manager := NewManager(WithSubscriber(pubsub))
		defer manager.Stop()

		var wg sync.WaitGroup
		numGoroutines := 100
		numChannels := 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				channel := "channel-" + fmt.Sprint(i%numChannels)
				clientID := fmt.Sprint(id)
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
