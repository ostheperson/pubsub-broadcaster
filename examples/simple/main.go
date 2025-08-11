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

	broadcaster "github.com/ostheperson/pubsub-broadcaster"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	redisClient := broadcaster.NewRedisClient("localhost:6379", "")
	defer redisClient.Close()

	manager := broadcaster.NewManager(redisClient)

	channels := []string{"news", "sports", "weather"}
	var wg sync.WaitGroup

	// -- 5 clients subscribe to one of the three topics at random --

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			channel := channels[rand.Intn(len(channels))]
			logger.Info("Client subscribed", "client_id", clientID, "channel", channel)

			clientChan := manager.RegisterClient(channel, clientID)
			defer manager.UnregisterClient(channel, clientID)

			for msg := range clientChan {
				logger.Info("Client received message", "client_id", clientID, "channel", channel, "message", string(msg))
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
				channel := channels[rand.Intn(len(channels))]
				message := fmt.Sprintf("Update for %s", channel)
				if err := redisClient.Publish(context.Background(), channel, message); err != nil {
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
	manager.Stop()
	wg.Wait()
}
