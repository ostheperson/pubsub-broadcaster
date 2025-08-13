# pubsub Broadcaster

A simple Go library for fanning out pubsub messages to many clients. **Manager** creates and manages broadcasters. **Broadcaster** grabs messages from a subscriber and sends them to all clients.

## Supported pubsub Systems

*   [x] any system that can implement the subscriber interface

## Resilience and Error Handling

The system is designed to be robust and handle common failures gracefully.

*   **pubsub Disconnection:** If the connection to the pubsub system is lost, it attempts to reconnect using an exponential backoff. Once the pubsub system is available again, the connection is re-established, and message flow resumes.

*   **Resource Management:** If a broadcaster loses its last client, its pubsub subscription is shutsdown and signals the manager to be removed.

*   **Graceful Shutdown:** The entire system can be shut down cleanly, ensuring all connections are closed and goroutines are terminated without leaks.

## Quick Start

```go
// 1. Make a manager
redisAdapter := redis.New("localhost:6379", "")
manager := broadcaster.NewManager(
    broadcaster.WithSubscriber(redisAdapter),
)

// 2. Start the manager
manager.Start()
defer manager.Stop()

// 3. Add a client to a topic
clientChan := manager.RegisterClient("my-topic", 1)
defer manager.UnregisterClient("my-topic", 1)

// 4. Listen for messages
go func() {
    for msg := range clientChan {
        fmt.Println("Got message:", string(msg))
    }
}()

// 5. Publish something to the topic
redisAdapter.Publish(context.Background(), "my-topic", "hello underworld")
```

Check the `examples` directory for more