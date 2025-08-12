package broadcaster

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type broadcaster struct {
	channel string
	wg      sync.WaitGroup

	// pubsub is the pub/sub client used for communication
	pubsub PubSub

	// clientChannels maps client IDs to their respective data channels
	clientChannels map[int]chan []byte
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
	channel string,
	pubsub PubSub,
	disconnectChan chan<- string,
	initialBackoff, maxBackoff, channelSendTimeout time.Duration,
	clientBufferSize int,
) *broadcaster {
	return &broadcaster{
		channel:            channel,
		pubsub:             pubsub,
		clientChannels:     make(map[int]chan []byte),
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

func (sb *broadcaster) AddClient(id int) <-chan []byte {
	client := make(chan []byte, sb.clientBufferSize)
	sb.clientMu.Lock()
	sb.clientChannels[id] = client
	sb.clientMu.Unlock()
	return client
}

func (sb *broadcaster) RemoveClient(id int) {
	sb.clientMu.Lock()
	defer sb.clientMu.Unlock()
	if client, ok := sb.clientChannels[id]; ok {
		close(client)
		delete(sb.clientChannels, id)
	}

	if len(sb.clientChannels) == 0 {
		sb.Stop()
	}
}

func (sb *broadcaster) Stop() {
	close(sb.stopChan)
	sb.wg.Wait()

	sb.clientMu.Lock()
	defer sb.clientMu.Unlock()
	for _, ch := range sb.clientChannels {
		close(ch)
	}
	select {
	case sb.disconnectChan <- sb.channel:
	case <-time.After(sb.channelSendTimeout):
	}
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

		subscriber, err := sb.pubsub.Subscribe(ctx, sb.channel)
		if err != nil {
			time.Sleep(backoff)
			backoff *= 2
			if backoff > sb.maxBackoff {
				backoff = sb.maxBackoff
			}
			continue
		}
		defer subscriber.Close()

		if err := sb.processMessages(ctx, subscriber); err != nil {
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

func (sb *broadcaster) processMessages(ctx context.Context, subscriber Subscriber) error {
	for {
		select {
		case msg, ok := <-subscriber.Channel():
			if !ok {
				return fmt.Errorf("pubsub channel closed")
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
