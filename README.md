# Pub/Sub Broadcaster

A simple Go library for fanning out Pub/Sub messages to many clients. 

## What It Does

*   **Manager:** Creates and manages broadcasters.
*   **Broadcaster:** Grabs messages from a subscriber and sends them to all clients.

## How It Works

1.  Client connects.
2.  You ask the `Manager` for a channel for a topic (e.g., `topic-1`).
3.  `Manager` finds or makes a `broadcaster` for that topic.
4.  `Broadcaster` sends messages from the pub/sub system to all clients on that topic.

## Supported Pub/Sub Systems

*   [x] any system that can implement the subscriber interface

## Resilience and Error Handling

The system is designed to be robust and handle common failures gracefully.

*   **Pub/Sub Disconnection:** If the connection to the pub/sub system is lost, it automatically attempts to reconnect using an exponential backoff to avoid overwhelming the server. Once the pub/sub system is available again, the connection is re-established, and message flow resumes.

*   **Client Disconnection:** When a client disconnects, the manager is notified and removes the client from the appropriate broadcaster.

*   **Resource Management:** If a broadcaster loses its last client, it automatically shuts down its pub/sub subscription and signals the manager to be removed.

*   **Graceful Shutdown:** The entire system can be shut down cleanly, ensuring all connections are closed and goroutines are terminated without leaks.

## Quick Start

```go
// 1. Make a manager
redisAdapter := redis.New("localhost:6379", "")
manager := broadcaster.NewManager(
    broadcaster.WithSubscriber(redisAdapter),
)

// 2. Add a client to a topic
clientChan := manager.RegisterClient("my-topic", 1)
defer manager.UnregisterClient("my-topic", 1)

// 3. Listen for messages
go func() {
    for msg := range clientChan {
        fmt.Println("Got message:", string(msg))
    }
}()

// 4. Publish something to the topic
redisAdapter.Publish(context.Background(), "my-topic", "hello underworld")
```

Check the `examples` directory for more