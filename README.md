# pub/sub Broadcaster

This library implements a pub/sub message broadcasting system. A Manager creates and manages Broadcaster instances. Each Broadcaster handles a specific topic. A Broadcaster receives messages from a Subscriber interface, which connects to an external pub/sub system. Messages are then sent to all registered in-memory clients for that topic. The system includes reconnection logic for pub/sub disconnections and removes broadcasters when they have no active clients.

## Supported pub/sub Systems

*   [x] any system that can implement the subscriber interface

## Resilience and Error Handling

The system is designed to be robust and handle common failures gracefully.

*   **pub/sub Disconnection:** If the connection to the pub/sub system is lost, it attempts to reconnect using an exponential backoff. Once the pub/sub system is available again, the connection is re-established, and message flow resumes.

*   **Resource Management:** If a broadcaster loses its last client, its pub/sub subscription is shutsdown and signals the manager to be removed.

*   **Graceful Shutdown:** The entire system can be shut down cleanly, ensuring all connections are closed and goroutines are terminated without leaks.

*   **Adapter Version Decoupling:** The library's adapter interfaces allow users to provide their own pub/sub client implementations, decoupling the library from specific external package versions.

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
clientChan := manager.RegisterClient("my-topic", "client_id")
defer manager.UnregisterClient("my-topic", "client_id")

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