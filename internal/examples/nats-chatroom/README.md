# NATS Chatroom Example (Embedded)

A chatroom built with Via and an **embedded NATS server**, demonstrating pub/sub messaging as an alternative to the custom `Rooms` implementation in `../chatroom`.

Via includes an embedded NATS server that starts automatically — no external server required.

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

Messages persist to disk via JetStream. Streams are declared in `Options.Streams` and created automatically when `v.Start()` initializes the embedded NATS server:

```go
v.Config(via.Options{
    Streams: []via.StreamConfig{{
        Name:     "CHAT",
        Subjects: []string{"chat.>"},
        MaxMsgs:  1000,
        MaxAge:   24 * time.Hour,
    }},
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
- `via.Subscribe(c, subject, handler)` for receiving
- `via.Publish(c, subject, data)` for sending
- Streams declared in `Options` — NATS handles delivery, no polling
