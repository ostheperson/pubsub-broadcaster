package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	redis_v8 "github.com/go-redis/redis/v8"
	broadcaster "github.com/ostheperson/pubsub-broadcaster"
	redisv8adapter "github.com/ostheperson/pubsub-broadcaster/adapter/redisv8"
)

func main() {
	adapter := redisv8adapter.New(
		redis_v8.NewClient(&redis_v8.Options{
			Addr:        "",
			PoolTimeout: 5 * time.Second,
			Password:    "",
			DB:          0,
			PoolSize:    10,
		}),
	)

	manager := broadcaster.NewManager(
		broadcaster.WithSubscriber(adapter),
		broadcaster.WithInitialBackoff(1*time.Second),
		broadcaster.WithMaxBackoff(10*time.Second),
		broadcaster.WithClientBufferSize(10),
		broadcaster.WithChannelSendTimeout(200*time.Millisecond),
	)

	manager.Start()
	defer manager.Stop()

	channels := []string{"news", "sports", "weather"}
	var wg sync.WaitGroup

	// -- five clients subscribe to one of the three topics at random --
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			topic := channels[rand.Intn(len(channels))]
			logger.Info("Client subscribed", "client_id", clientID, "topic", topic)

			clientChan := manager.RegisterClient(topic, fmt.Sprint(clientID))
			defer manager.UnregisterClient(topic, fmt.Sprint(clientID))

			for msg := range clientChan {
				logger.Info("Client received message", "client_id", clientID, "topic", topic, "message", string(msg))
			}
		}(i)
	}

	// -- publish message to the three topics --

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				topic := channels[rand.Intn(len(channels))]
				message := fmt.Sprintf("Update for %s", topic)
				if err := adapter.Publish(context.Background(), topic, []byte(message)); err != nil {
					logger.Error("Failed to publish message", "error", err)
				}
			case <-manager.ServiceContext().Done():
				return
			}
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("Shutting down...")
	wg.Wait()
}
