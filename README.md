# Pub/Sub Broadcaster

A simple Go library for fanning out Redis Pub/Sub messages to many clients.

## What It Does

*   **Manager:** Makes and manages broadcasters.
*   **Broadcaster:** Grabs messages from a Redis topic and sends them to all clients.

## How It Works

1.  Client connects.
2.  You ask the `Manager` for a channel for a topic (e.g., `game-1`).
3.  `Manager` finds or makes a `broadcaster` for that topic.
4.  `Broadcaster` sends messages from Redis to all clients on that topic.

## Resilience and Error Handling

The system is designed to be robust and handle common failures gracefully.

*   **Redis Disconnection:** If the connection to Redis is lost, it automatically attempts to reconnect using an exponential backoff to avoid overwhelming the server. Once Redis is available again, the connection is re-established, and message flow resumes.

*   **Client Disconnection:** When a client disconnects, the manager is notified and removes the client from the appropriate broadcaster.

*   **Resource Management:** If a broadcaster loses its last client, it automatically shuts down its Redis subscription and signals the manager to be removed.

*   **Graceful Shutdown:** The entire system can be shut down cleanly, ensuring all connections are closed and goroutines are terminated without leaks.

## Use It For

*   Live Chat
*   Sports Score Updates
*   IoT Dashboards
*   Anything real-time.

## Quick Start

```go
// 1. Make a manager
redisClient := broadcaster.NewRedisClient("localhost:6379", "")
manager := broadcaster.NewManager(redisClient)

// 2. Add a client to a topic
clientChan := manager.AddClient("my-topic", "client-id")
defer manager.RemoveClient("my-topic", "client-id")

// 3. Listen for messages
go func() {
    for msg := range clientChan {
        fmt.Println("Got message:", string(msg))
    }
}()

// 4. Publish something to the topic in Redis
redisClient.Publish(context.Background(), "my-topic", "hello underworld")
```

Check the `examples` directory for more
