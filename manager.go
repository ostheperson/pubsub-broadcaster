package broadcaster

import (
	"context"
	"sync"
	"time"
)

type Manager struct {
	// subscriber is the pub/sub client used for communication
	subscriber Subscriber

	// activeBroadcasters maps topic names to their respective broadcaster instances
	activeBroadcasters map[string]*broadcaster
	// broadcasterMu protects access to the activeBroadcasters map
	broadcasterMu sync.RWMutex
	// disconnectChan receives signals from broadcasters that are ready to be removed
	disconnectChan chan string

	// serviceCtx is the context for the manager's services
	serviceCtx context.Context
	// serviceCancel is the cancel function for the serviceCtx
	serviceCancel context.CancelFunc

	// initialBackoff is the initial backoff duration for reconnecting to the pub/sub server
	initialBackoff time.Duration
	// maxBackoff is the maximum backoff duration for reconnecting to the pub/sub server
	maxBackoff time.Duration
	// clientBufferSize is the size of the client's message buffer
	clientBufferSize int
	// channelSendTimeout is the timeout for sending a message to a client
	channelSendTimeout time.Duration
}

type Option func(*Manager)

func WithSubscriber(subscriber Subscriber) Option {
	return func(m *Manager) {
		m.subscriber = subscriber
	}
}

func WithInitialBackoff(d time.Duration) Option {
	return func(m *Manager) {
		m.initialBackoff = d
	}
}

func WithMaxBackoff(d time.Duration) Option {
	return func(m *Manager) {
		m.maxBackoff = d
	}
}

func WithClientBufferSize(size int) Option {
	return func(m *Manager) {
		m.clientBufferSize = size
	}
}

func WithChannelSendTimeout(d time.Duration) Option {
	return func(m *Manager) {
		m.channelSendTimeout = d
	}
}

var newBroadcaster = NewBroadcaster

func NewManager(opts ...Option) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		activeBroadcasters: make(map[string]*broadcaster),
		disconnectChan:     make(chan string),
		serviceCtx:         ctx,
		serviceCancel:      cancel,
		initialBackoff:     time.Second,
		maxBackoff:         time.Minute,
		clientBufferSize:   5,
		channelSendTimeout: time.Millisecond * 100,
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.subscriber == nil {
		panic("subscriber is required")
	}

	return m
}

func (m *Manager) Start() {
	go m.listenForDisconnects()
}

func (s *Manager) listenForDisconnects() {
	for topic := range s.disconnectChan {
		s.RemoveBroadcaster(topic)
	}
}

func (s *Manager) ServiceContext() context.Context {
	return s.serviceCtx
}

func (s *Manager) RegisterClient(topic string, clientID string) <-chan []byte {
	s.broadcasterMu.Lock()
	defer s.broadcasterMu.Unlock()
	sb, ok := s.activeBroadcasters[topic]

	if !ok {
		sb = newBroadcaster(
			topic,
			s.subscriber,
			s.disconnectChan,
			s.initialBackoff,
			s.maxBackoff,
			s.channelSendTimeout,
			s.clientBufferSize,
		)

		s.activeBroadcasters[topic] = sb
		sb.Start(s.serviceCtx)
	}
	return sb.Add(clientID)
}

func (s *Manager) UnregisterClient(topic string, clientID string) {
	s.broadcasterMu.RLock()
	sb, ok := s.activeBroadcasters[topic]
	s.broadcasterMu.RUnlock()
	if ok {
		sb.Remove(clientID)
	}
}

func (s *Manager) RemoveBroadcaster(topic string) {
	s.broadcasterMu.Lock()
	delete(s.activeBroadcasters, topic)
	s.broadcasterMu.Unlock()
}

func (s *Manager) StopBroadcaster(topic string) {
	s.broadcasterMu.Lock()
	sb, ok := s.activeBroadcasters[topic]
	s.broadcasterMu.Unlock()
	if ok {
		sb.Stop()
	}
}

func (s *Manager) Stop() {
	s.serviceCancel()
	s.broadcasterMu.Lock()
	s.activeBroadcasters = make(map[string]*broadcaster)
	s.broadcasterMu.Unlock()
}
