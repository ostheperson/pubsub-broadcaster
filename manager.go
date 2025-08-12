package broadcaster

import (
	"context"
	"sync"
	"time"
)

type Manager struct {
	// pubsub is the pub/sub client used for communication
	pubsub PubSub

	// activeBroadcasters maps channel names to their respective broadcaster instances
	activeBroadcasters map[string]*broadcaster
	// broadcasterMu protects access to the activeBroadcasters map
	broadcasterMu sync.RWMutex
	// disconnectChan receives signals from broadcasters that are ready to be removed
	disconnectChan chan string

	// serviceCtx is the context for the manager's services
	serviceCtx context.Context
	// serviceCancel is the cancel function for the serviceCtx
	serviceCancel context.CancelFunc

	initialBackoff     time.Duration
	maxBackoff         time.Duration
	clientBufferSize   int
	channelSendTimeout time.Duration
}

type Option func(*Manager)

func WithPubSub(pubsub PubSub) Option {
	return func(m *Manager) {
		m.pubsub = pubsub
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
	if m.pubsub == nil {
		panic("pubsub is required")
	}

	go m.listenForDisconnects()
	return m
}

func (s *Manager) listenForDisconnects() {
	for channel := range s.disconnectChan {
		s.RemoveBroadcaster(channel)
	}
}

func (s *Manager) ServiceContext() context.Context {
	return s.serviceCtx
}

func (s *Manager) RegisterClient(channel string, clientID string) <-chan []byte {
	s.broadcasterMu.Lock()
	defer s.broadcasterMu.Unlock()
	sb, ok := s.activeBroadcasters[channel]

	if !ok {
		sb = NewBroadcaster(
			channel,
			s.pubsub,
			s.disconnectChan,
			s.initialBackoff,
			s.maxBackoff,
			s.channelSendTimeout,
			s.clientBufferSize,
		)

		s.activeBroadcasters[channel] = sb
		sb.Start(s.serviceCtx)
	}
	return sb.AddClient(clientID)
}

func (s *Manager) UnregisterClient(channel string, clientID string) {
	s.broadcasterMu.RLock()
	sb, ok := s.activeBroadcasters[channel]
	s.broadcasterMu.RUnlock()
	if ok {
		sb.RemoveClient(clientID)
	}
}

func (s *Manager) RemoveBroadcaster(channel string) {
	s.broadcasterMu.Lock()
	delete(s.activeBroadcasters, channel)
	s.broadcasterMu.Unlock()
}

func (s *Manager) StopBroadcaster(channel string) {
	s.broadcasterMu.Lock()
	sb, ok := s.activeBroadcasters[channel]
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
