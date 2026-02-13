# PubSub and Sessions

Infrastructure for multi-user real-time communication and persistent state.

## PubSub

Via includes an embedded NATS server that starts automatically with `via.New()`. No external services required — pub/sub works out of the box.

### Interface

```go
type PubSub interface {
    Publish(subject string, data []byte) error
    Subscribe(subject string, handler func(data []byte)) (Subscription, error)
    Close() error
}

type Subscription interface {
    Unsubscribe() error
}
```

You can replace the default NATS with any backend implementing this interface via `Options.PubSub`.

### Basic pub/sub

```go
// Subscribe to messages
via.Subscribe(c, "chat.room.general", func(msg ChatMessage) {
    messages = append(messages, msg)
    c.Sync()
})

// Publish a message
via.Publish(c, "chat.room.general", ChatMessage{
    User:    username,
    Message: text,
    Time:    time.Now().UnixMilli(),
})
```

The generic helpers `via.Publish[T]` and `via.Subscribe[T]` handle JSON marshaling/unmarshaling automatically. They are package-level functions (not methods) because Go doesn't support generic methods.

Raw byte-level access is also available on the context:

```go
c.Publish("subject", []byte("raw data"))
c.Subscribe("subject", func(data []byte) { /* ... */ })
```

### Auto-cleanup

Subscriptions created via `c.Subscribe()` or `via.Subscribe()` are tracked on the context and automatically unsubscribed when:

- The context is disposed (browser disconnects, tab closes)
- SPA navigation moves to a different page

You don't need to manually unsubscribe in normal usage.

### Custom backend

Replace the embedded NATS with your own PubSub implementation:

```go
v.Config(via.Options{
    PubSub: myRedisBackend,
})
```

This disables the embedded NATS server. The `NATSConn()` and `JetStream()` accessors will return nil.

## JetStream

NATS JetStream provides persistent, replayable message streams. Useful for chat history, event logs, or any scenario where new subscribers need to catch up on past messages.

### Ensure a stream exists

```go
err := via.EnsureStream(v, via.StreamConfig{
    Name:     "CHAT",
    Subjects: []string{"chat.>"},
    MaxMsgs:  1000,
    MaxAge:   24 * time.Hour,
})
```

| Field | Description |
|-------|-------------|
| `Name` | Stream name |
| `Subjects` | NATS subjects to capture (supports wildcards: `>` matches all sub-levels) |
| `MaxMsgs` | Maximum number of messages to retain |
| `MaxAge` | Maximum age before messages are discarded |

Call `EnsureStream` during app initialization, before `v.Start()`.

### Replay history

Retrieve recent messages from a stream:

```go
messages, err := via.ReplayHistory[ChatMessage](v, "chat.room.general", 50)
```

Returns up to the last `limit` messages on the subject, deserialized as `T`. Use this when a new user joins and needs to see recent history.

### Direct NATS access

For advanced use cases, access the NATS connection and JetStream context directly:

```go
nc := v.NATSConn()       // *nats.Conn, nil if custom PubSub
js := v.JetStream()      // nats.JetStreamContext, nil if custom PubSub
```

### PubSub accessor

Access the configured PubSub backend from the `V` instance:

```go
ps := v.PubSub() // via.PubSub interface, nil if none configured
```

## Sessions

Via uses [SCS](https://github.com/alexedwards/scs) for cookie-based session management.

### Setup with SQLite

```go
db, _ := sql.Open("sqlite3", "app.db")

sm, _ := via.NewSQLiteSessionManager(db)
sm.Lifetime = 24 * time.Hour
sm.Cookie.SameSite = http.SameSiteLaxMode

v.Config(via.Options{SessionManager: sm})
```

`NewSQLiteSessionManager` creates the `sessions` table and index if they don't exist. The returned `*scs.SessionManager` can be configured further (lifetime, cookie settings) before passing to `Config`.

A default in-memory session manager is always available, even without explicit configuration. Use `NewSQLiteSessionManager` when you need sessions to survive server restarts.

### Session API

Access the session from any context:

```go
s := c.Session()
```

**Getters:**

| Method | Return type |
|--------|-------------|
| `s.Get(key)` | `any` |
| `s.GetString(key)` | `string` |
| `s.GetInt(key)` | `int` |
| `s.GetBool(key)` | `bool` |
| `s.GetFloat64(key)` | `float64` |
| `s.GetTime(key)` | `time.Time` |
| `s.GetBytes(key)` | `[]byte` |

**Pop** (get and delete — useful for flash messages):

| Method | Return type |
|--------|-------------|
| `s.Pop(key)` | `any` |
| `s.PopString(key)` | `string` |
| `s.PopInt(key)` | `int` |
| `s.PopBool(key)` | `bool` |
| `s.PopFloat64(key)` | `float64` |
| `s.PopTime(key)` | `time.Time` |
| `s.PopBytes(key)` | `[]byte` |

**Mutators:**

| Method | Description |
|--------|-------------|
| `s.Set(key, val)` | Store a value |
| `s.Delete(key)` | Remove a single key |
| `s.Clear()` | Remove all session data |
| `s.Destroy()` | Destroy the entire session (for logout) |
| `s.RenewToken()` | Regenerate session ID (prevents session fixation — call after login) |

**Introspection:**

| Method | Description |
|--------|-------------|
| `s.Exists(key)` | True if key exists |
| `s.Keys()` | All keys in the session |
| `s.ID()` | Session token (cookie value) |

All getters return zero values if the key doesn't exist or the session manager is nil.

### Auth pattern

A common login/logout flow using sessions and middleware:

```go
// Middleware
func authRequired(c *via.Context, next func()) {
    if c.Session().GetString("username") == "" {
        c.Session().Set("flash", "Please log in first")
        c.RedirectView("/login")
        return
    }
    next()
}

// Login page
v.Page("/login", func(c *via.Context) {
    user := c.Signal("")
    pass := c.Signal("")
    flash := c.Session().PopString("flash")

    login := c.Action(func() {
        if authenticate(user.String(), pass.String()) {
            c.Session().RenewToken()
            c.Session().Set("username", user.String())
            c.Redirect("/dashboard")
        } else {
            flash = "Invalid credentials"
            c.Sync()
        }
    })

    c.View(func() h.H {
        return h.Form(login.OnSubmit(),
            h.If(flash != "", h.P(h.Text(flash))),
            h.Input(h.Type("text"), user.Bind(), h.Placeholder("Username")),
            h.Input(h.Type("password"), pass.Bind(), h.Placeholder("Password")),
            h.Button(h.Type("submit"), h.Text("Log In")),
        )
    })
})

// Protected pages
protected := v.Group("", authRequired)
protected.Page("/dashboard", dashboardHandler)

// Logout action (inside a protected page)
logout := c.Action(func() {
    c.Session().Destroy()
    c.Redirect("/login")
})
```

Key points:
- Call `RenewToken()` after login to prevent session fixation.
- Use `PopString` for flash messages — they're read once then removed.
- Use `RedirectView` in middleware, `Redirect` in actions. See the [gotcha in routing](routing-and-navigation.md#middleware).
