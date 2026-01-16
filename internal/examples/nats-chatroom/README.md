# NATS Chatroom Example (Embedded)

A chatroom built with Via and an **embedded NATS server**, demonstrating pub/sub messaging as an alternative to the custom `Rooms` implementation in `../chatroom`.

Uses `delaneyj/toolbelt/embeddednats` to run NATS inside the same binary - no external server required.

## Key Differences from Original Chatroom

| Aspect | Original (`../chatroom`) | This Example |
|--------|-------------------------|--------------|
| Pub/sub | Custom `Rooms` struct (~160 lines) | NATS subjects |
| Member tracking | Manual `map[TU]Syncable` | NATS handles subscribers |
| Publish timing | Ticker every 100ms + dirty flag | Instant delivery |
| Durability | None (in-memory) | JetStream persists to disk |
| Multi-instance | Not supported | Works across server instances |
| External deps | None | **None** (NATS embedded in binary) |

## Run the Example

```bash
go run ./internal/examples/nats-chatroom
```

That's it. No separate NATS server needed.

Open multiple browser tabs at http://localhost:7331 to see messages broadcast across all clients.

## How Embedded NATS Works

```go
// Start embedded NATS server (JetStream enabled by default)
ns, err := embeddednats.New(ctx,
    embeddednats.WithDirectory("./data/nats"),
)
ns.WaitForServer()

// Get client connection to embedded server
nc, err := ns.Client()
```

Data is persisted to `./data/nats/` for JetStream durability.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Single Binary                         │
│                                                          │
│  Browser A          Embedded NATS         Browser B      │
│      │                   │                    │          │
│      │-- Via Action ---> │                    │          │
│      │   (Send msg)      │                    │          │
│      │                   │                    │          │
│      │              nc.Publish()              │          │
│      │              "chat.room.Go"            │          │
│      │                   │                    │          │
│      │<-- Subscribe -----|---- Subscribe --->│          │
│      │    callback       │    callback        │          │
│      │                   │                    │          │
│      │-- c.Sync() ------>│<--- c.Sync() -----|          │
│      │   (SSE)           │     (SSE)          │          │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

## JetStream Durability

Messages persist to disk via JetStream:

```go
js.AddStream(&nats.StreamConfig{
    Name:      "CHAT",
    Subjects:  []string{"chat.>"},
    MaxMsgs:   1000,  // Keep last 1000 messages
    MaxAge:    24 * time.Hour,
})
```

Stop and restart the app - chat history survives.

## Code Comparison

**Original chatroom - 160+ lines of custom pub/sub:**
- `Rooms` struct with named rooms
- `Room` with member tracking, mutex, dirty flag
- Ticker-based publish loop
- Manual join/leave channels

**This example - ~60 lines of NATS integration:**
- `embeddednats.New()` starts the server
- `nc.Subscribe(subject, handler)` for receiving
- `nc.Publish(subject, data)` for sending
- NATS handles delivery, no polling

## Next Steps

If this pattern proves useful, it could be promoted to a Via plugin:

```go
// Hypothetical future API
v.Config(via.WithEmbeddedNATS("./data/nats"))

// In page init
c.Subscribe("events.user.*", func(data []byte) {
    c.Sync()
})

c.Publish("events.user.login", userData)
```
