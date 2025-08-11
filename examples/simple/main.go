package broadcaster

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	broadcaster "github.com/ostheperson/pubsub-broadcaster"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	redisClient := broadcaster.NewRedisClient("localhost:6379", "")
	defer redisClient.Close()

	manager := broadcaster.NewManager(redisClient, logger)
	defer manager.Stop()

	channel := "my-channel"

	clientChan := manager.AddClient(channel, 1)
	defer manager.RemoveClient(channel, 1)

	go func() {
		for msg := range clientChan {
			logger.Info("Received message", "message", string(msg))
		}
	}()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				err := redisClient.Publish(context.Background(), channel, "hello world")
				if err != nil {
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
}
