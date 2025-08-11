package broadcaster

import (
	"context"
	"log/slog"
	"sync"
)

type Manager struct {
	pubsub PubSub
	logger *slog.Logger

	activeBroadcasters map[string]*broadcaster
	broadcasterMu      sync.RWMutex

	serviceCtx    context.Context
	serviceCancel context.CancelFunc
}

func NewManager(pubsub PubSub, l *slog.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		pubsub:             pubsub,
		activeBroadcasters: make(map[string]*broadcaster),
		serviceCtx:         ctx,
		serviceCancel:      cancel,
	}
}

func (s *Manager) ServiceContext() context.Context {
	return s.serviceCtx
}

func (s *Manager) AddClient(channel string, clientID int) <-chan []byte {
	s.broadcasterMu.RLock()
	sb, ok := s.activeBroadcasters[channel]
	s.broadcasterMu.RUnlock()

	if !ok {
		onEmpty := func(c string) { s.RemoveBroadcaster(c) }
		sb = NewBroadcaster(channel, s.pubsub, s.logger, onEmpty)

		s.activeBroadcasters[channel] = sb
		sb.Start(s.serviceCtx)
	}
	return sb.AddClient(clientID)
}

func (s *Manager) RemoveClient(channel string, clientID int) {
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

	s.logger.Info("Broadcaster removed due to no active clients.", slog.String("channel", channel))
}

func (s *Manager) Stop() {
	s.serviceCancel()
	s.broadcasterMu.Lock()
	for channel, sb := range s.activeBroadcasters {
		sb.Stop()
		delete(s.activeBroadcasters, channel)
	}
	s.broadcasterMu.Unlock()
	s.logger.Info("manager stopped all broadcasters.")
}
