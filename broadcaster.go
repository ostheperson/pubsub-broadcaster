package broadcaster

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type broadcaster struct {
	topic string
	wg    sync.WaitGroup

	// subscriber is the pub/sub client used for communication
	subscriber Subscriber

	// clientChannels maps client IDs to their respective data channels
	clientChannels map[string]chan []byte
	clientMu       sync.RWMutex

	// stopChan is used to signal the broadcaster's internal goroutine to stop
	stopChan chan struct{}
	// disconnectChan is used to signal the manager that this broadcaster can be removed
	disconnectChan chan<- string

	initialBackoff     time.Duration
	maxBackoff         time.Duration
	clientBufferSize   int
	channelSendTimeout time.Duration
}

func NewBroadcaster(
	topic string,
	subscriber Subscriber,
	disconnectChan chan<- string,
	initialBackoff, maxBackoff, channelSendTimeout time.Duration,
	clientBufferSize int,
) *broadcaster {
	return &broadcaster{
		topic:              topic,
		subscriber:         subscriber,
		clientChannels:     make(map[string]chan []byte),
		disconnectChan:     disconnectChan,
		stopChan:           make(chan struct{}),
		initialBackoff:     initialBackoff,
		maxBackoff:         maxBackoff,
		channelSendTimeout: channelSendTimeout,
		clientBufferSize:   clientBufferSize,
	}
}

func (sb *broadcaster) Start(ctx context.Context) {
	sb.wg.Add(1)
	go sb.listen(ctx)
}

func (sb *broadcaster) Add(id string) <-chan []byte {
	client := make(chan []byte, sb.clientBufferSize)
	sb.clientMu.Lock()
	sb.clientChannels[id] = client
	sb.clientMu.Unlock()
	return client
}

func (sb *broadcaster) Remove(id string) {
	sb.clientMu.Lock()
	if client, ok := sb.clientChannels[id]; ok {
		close(client)
		delete(sb.clientChannels, id)
	}
	sb.clientMu.Unlock()

	if len(sb.clientChannels) == 0 {
		sb.Stop()
	}
}

func (sb *broadcaster) Stop() {
	close(sb.stopChan)
	sb.wg.Wait()
	sb.clientMu.Lock()
	for _, ch := range sb.clientChannels {
		close(ch)
	}
	sb.clientMu.Unlock()
	sb.disconnectChan <- sb.topic
}

func (sb *broadcaster) listen(ctx context.Context) {
	defer sb.wg.Done()

	backoff := sb.initialBackoff
	for {
		select {
		case <-sb.stopChan:
			return
		case <-ctx.Done():
			sb.Stop()
			return
		default:
		}

		ch, err := sb.subscriber.Subscribe(ctx, sb.topic)
		if err != nil {
			time.Sleep(backoff)
			backoff *= 2
			if backoff > sb.maxBackoff {
				backoff = sb.maxBackoff
			}
			continue
		}

		if err := sb.processMessages(ctx, ch); err != nil {
			time.Sleep(backoff)
			backoff *= 2
			if backoff > sb.maxBackoff {
				backoff = sb.maxBackoff
			}
		} else {
			return
		}
	}
}

func (sb *broadcaster) processMessages(ctx context.Context, ch <-chan *Message) error {
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return fmt.Errorf("pubsub topic closed")
			}

			sb.clientMu.RLock()
			for _, clientChan := range sb.clientChannels {
				select {
				case clientChan <- msg.Payload:
				case <-time.After(sb.channelSendTimeout):
				}
			}

			sb.clientMu.RUnlock()
		case <-sb.stopChan:
			return nil
		case <-ctx.Done():
			sb.Stop()
			return nil
		}
	}
}
