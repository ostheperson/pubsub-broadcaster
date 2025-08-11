package broadcaster

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"
)

type broadcaster struct {
	channel string
	pubsub  PubSub
	l       *slog.Logger

	clientChannels map[int]chan []byte
	clientMu       sync.RWMutex
	stopChan       chan struct{}
	disconnect     func(string)
	wg             sync.WaitGroup
}

const (
	initialBackoff = 1 * time.Second

	maxBackoff = 1 * time.Minute
)

func NewBroadcaster(
	channel string,
	pubsub PubSub,
	l *slog.Logger,
	disconnect func(string),
) *broadcaster {
	return &broadcaster{
		channel: channel,
		pubsub:  pubsub,
		l:       l.With(slog.String("channel", channel)),

		clientChannels: make(map[int]chan []byte),
		disconnect:     disconnect,
		stopChan:       make(chan struct{}),
	}
}

func (sb *broadcaster) Start(ctx context.Context) {
	sb.wg.Add(1)
	go sb.listen(ctx)
}

func (sb *broadcaster) AddClient(id int) <-chan []byte {
	client := make(chan []byte, 5)
	sb.clientMu.Lock()
	sb.clientChannels[id] = client
	sb.clientMu.Unlock()
	return client
}

func (sb *broadcaster) RemoveClient(id int) {
	sb.clientMu.Lock()
	if client, ok := sb.clientChannels[id]; ok {
		close(client)
	}
	delete(sb.clientChannels, id)
	sb.clientMu.Unlock()
	if len(sb.clientChannels) == 0 {
		if sb.disconnect != nil {
			sb.disconnect(sb.channel)
		}
	}
}

func (sb *broadcaster) Stop() {
	close(sb.stopChan)
	sb.wg.Wait()
}

func (sb *broadcaster) listen(ctx context.Context) {
	defer func() {
		sb.wg.Done()
		if r := recover(); r != nil {
			sb.l.Error("Panic recovered in broadcaster", "error", r, "stack", string(debug.Stack()))
		}
	}()

	backoff := initialBackoff
	for {
		select {
		case <-sb.stopChan:
			return
		case <-ctx.Done():
			return
		default:
		}

		sb.l.Info("attempting subscription...")
		pubsubClient := sb.pubsub.SubscribeWithChannel(ctx, sb.channel)
		defer pubsubClient.Close()

		if err := sb.processMessage(ctx, pubsubClient); err != nil {
			sb.l.Warn("subscription lost, attempting to reconnect", "error", err)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		} else {
			return
		}
	}
}

func (sb *broadcaster) processMessage(ctx context.Context, pubsubClient PubSubClient) error {
	for {
		select {
		case msg, ok := <-pubsubClient.Channel():
			if !ok {
				return fmt.Errorf("redis pubsub channel closed")
			}

			sb.clientMu.RLock()
			for _, clientChan := range sb.clientChannels {
				select {
				case clientChan <- []byte(msg.Payload):
				case <-time.After(100 * time.Millisecond):
					sb.l.Warn(
						"Timeout sending update to a client",
						slog.String("channel", sb.channel),
					)

				}
			}

			sb.clientMu.RUnlock()
		case <-sb.stopChan:
			return nil
		case <-ctx.Done():
			return nil
		}
	}
}
