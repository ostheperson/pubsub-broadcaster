package broadcaster

import (
	"context"
	"sync"
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
}

func NewManager(pubsub PubSub) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		pubsub:             pubsub,
		activeBroadcasters: make(map[string]*broadcaster),
		disconnectChan:     make(chan string),
		serviceCtx:         ctx,
		serviceCancel:      cancel,
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

func (s *Manager) RegisterClient(channel string, clientID int) <-chan []byte {
	s.broadcasterMu.Lock()
	defer s.broadcasterMu.Unlock()
	sb, ok := s.activeBroadcasters[channel]

	if !ok {
		sb = NewBroadcaster(channel, s.pubsub, s.disconnectChan)

		s.activeBroadcasters[channel] = sb
		sb.Start(s.serviceCtx)
	}
	return sb.AddClient(clientID)
}

func (s *Manager) UnregisterClient(channel string, clientID int) {
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

func (s *Manager) Stop() {
	s.serviceCancel()
	s.broadcasterMu.Lock()
	for channel, sb := range s.activeBroadcasters {
		sb.Stop()
		delete(s.activeBroadcasters, channel)
	}
	s.broadcasterMu.Unlock()
}
