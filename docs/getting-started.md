# Getting Started

Via is a server-side reactive web framework for Go. The browser connects over SSE (Server-Sent Events), and all state lives on the server — signals, actions, and view rendering happen in Go. The browser is a thin display layer that Datastar keeps in sync via DOM morphing.

## Core Loop

Every Via app follows the same pattern:

```go
package main

import (
    "github.com/ryanhamamura/via"
    "github.com/ryanhamamura/via/h"
)

func main() {
    v := via.New()

    v.Config(via.Options{
        DocumentTitle: "My App",
    })

    v.Page("/", func(c *via.Context) {
        count := 0
        step := c.Signal(1)

        increment := c.Action(func() {
            count += step.Int()
            c.Sync()
        })

        c.View(func() h.H {
            return h.Div(
                h.P(h.Textf("Count: %d", count)),
                h.Label(
                    h.Text("Step: "),
                    h.Input(h.Type("number"), step.Bind()),
                ),
                h.Button(h.Text("+"), increment.OnClick()),
            )
        })
    })

    v.Start()
}
```

What happens:

1. `via.New()` creates the app, starts an embedded NATS server, and registers internal routes (`/_sse`, `/_action/{id}`, `/_navigate`, `/_session/close`).
2. `v.Config()` applies settings.
3. `v.Page()` registers a route. The init function receives a `*Context` where you define signals, actions, and the view.
4. `v.Start()` starts the HTTP server and blocks until SIGINT/SIGTERM.

When a browser hits the page, Via creates a new `Context`, runs the init function, renders the full HTML document, and opens an SSE connection. From that point, every `c.Sync()` re-renders the view and pushes a DOM patch to the browser.

## Configuration

```go
v.Config(via.Options{
    DevMode:         true,
    ServerAddress:   ":8080",
    LogLevel:        via.LogLevelDebug,
    DocumentTitle:   "My App",
    Plugins:         []via.Plugin{MyPlugin},
    SessionManager:  sm,
    PubSub:          customBackend,
    ContextTTL:      60 * time.Second,
    ActionRateLimit: via.RateLimitConfig{Rate: 20, Burst: 40},
})
```

| Field | Default | Description |
|-------|---------|-------------|
| `DevMode` | `false` | Enables context persistence across restarts, console logger, and Datastar inspector widget |
| `ServerAddress` | `":3000"` | HTTP listen address |
| `LogLevel` | `InfoLevel` | Minimum log level. Use `via.LogLevelDebug`, `LogLevelInfo`, `LogLevelWarn`, `LogLevelError` |
| `Logger` | (auto) | Replace the default logger entirely. When set, `LogLevel` and `DevMode` have no effect on logging |
| `DocumentTitle` | `"⚡ Via"` | The `<title>` of the HTML document |
| `Plugins` | `nil` | Slice of plugin functions executed during `Config()` |
| `SessionManager` | in-memory | Cookie-based session manager. See [PubSub and Sessions](pubsub-and-sessions.md) |
| `DatastarContent` | (embedded) | Custom Datastar JS bytes |
| `DatastarPath` | `"/_datastar.js"` | URL path for the Datastar script |
| `PubSub` | embedded NATS | Custom PubSub backend. Replaces the default NATS. See [PubSub and Sessions](pubsub-and-sessions.md) |
| `ContextTTL` | `30s` | Max time a context survives without an SSE connection before cleanup. Negative value disables the reaper |
| `ActionRateLimit` | `10 req/s, burst 20` | Default token-bucket rate limiter for action endpoints. Rate of `-1` disables limiting |

## Static Files

Serve files from a directory:

```go
v.Static("/assets/", "./static")
```

Or from an embedded filesystem:

```go
//go:embed static
var staticFS embed.FS

v.StaticFS("/assets/", staticFS)
```

Both disable directory listing and return 404 for directory paths.

## Head and Foot Injection

Add elements to every page's `<head>` or end of `<body>`:

```go
v.AppendToHead(
    h.Link(h.Rel("stylesheet"), h.Href("/assets/style.css")),
    h.Meta(h.Attr("name", "viewport"), h.Attr("content", "width=device-width, initial-scale=1")),
)

v.AppendToFoot(
    h.Script(h.Src("/assets/app.js")),
)
```

These are additive and affect all pages globally.

## Plugins

A plugin is a `func(v *via.V)` that mutates the app during configuration — registering routes, injecting assets, or applying middleware.

```go
func PicoCSSPlugin(v *via.V) {
    v.HTTPServeMux().HandleFunc("GET /css/pico.css", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/css")
        w.Write(picoCSSBytes)
    })
    v.AppendToHead(h.Link(h.Rel("stylesheet"), h.Href("/css/pico.css")))
}

// Usage:
v.Config(via.Options{
    Plugins: []via.Plugin{PicoCSSPlugin},
})
```

Plugins have full access to the `*V` public API: `HTTPServeMux()`, `AppendToHead()`, `AppendToFoot()`, `Config()`, etc.

## DevMode

Enable during development for a better feedback loop:

```go
v.Config(via.Options{DevMode: true})
```

What it does:

- **Console logger** — Human-readable log output with timestamps.
- **Context persistence** — Saves context-to-route mappings to `.via/devmode/ctx.json`. On server restart, reconnecting browsers restore their state instead of getting a blank page. Pair with [Air](https://github.com/air-verse/air) for hot-reloading.
- **Datastar inspector** — Injects a widget showing live signal values and SSE activity.

## Custom HTTP Handlers

Access the underlying `*http.ServeMux` for custom routes:

```go
mux := v.HTTPServeMux()
mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("ok"))
})
```

Register custom handlers before calling `v.Start()`.

## Next Steps

- [State and Interactivity](state-and-interactivity.md) — Signals, actions, components, validation
- [Routing and Navigation](routing-and-navigation.md) — Multi-page apps, middleware, SPA navigation
- [PubSub and Sessions](pubsub-and-sessions.md) — Real-time messaging, persistent sessions
- [HTML DSL](html-dsl.md) — The `h` package reference
- [Project Structure](project-structure.md) — Organizing files as your app grows
