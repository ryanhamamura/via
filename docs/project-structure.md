# Project Structure

Via's closure-based page model pulls signals, actions, and views into a single scope — similar to Svelte's single-file components. This works well at every scale, but the way you organize files should evolve as your app grows.

## Stage 1: Everything in main.go

For small apps and prototypes, keep everything in `main.go`. This is the right choice when your app is under ~150 lines or has a single page.

Within the file, follow this ordering convention inside each page:

```go
v.Page("/", func(c *via.Context) {
    // State — plain Go variables and signals
    count := 0
    step := c.Signal(1)

    // Actions — event handlers that mutate state
    increment := c.Action(func() {
        count += step.Int()
        c.Sync()
    })

    // View — returns the HTML tree
    c.View(func() h.H {
        return h.Div(
            h.P(h.Textf("Count: %d", count)),
            h.Button(h.Text("+"), increment.OnClick()),
        )
    })
})
```

State → signals → actions → view. This reads top-to-bottom and matches the data flow: state is declared, actions mutate it, the view renders it.

The [counter](../internal/examples/counter/main.go) and [greeter](../internal/examples/greeter/main.go) examples use this layout.

## Stage 2: Page per file

When `main.go` has multiple pages or exceeds ~150 lines, extract each page into its own file as a package-level function.

`main.go` becomes the app skeleton — setup, configuration, routes, and start:

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

    v.AppendToHead(
        h.Link(h.Rel("stylesheet"), h.Href("/css/pico.css")),
    )

    v.Page("/", HomePage)
    v.Page("/chat", ChatPage)

    v.Start()
}
```

Each page lives in its own file with a descriptive name:

```go
// home.go
package main

import (
    "github.com/ryanhamamura/via"
    "github.com/ryanhamamura/via/h"
)

func HomePage(c *via.Context) {
    greeting := c.Signal("Hello")

    c.View(func() h.H {
        return h.Div(h.P(h.Text(greeting.String())))
    })
}
```

Components follow the same pattern — keep them in the page file if single-use, or extract to their own file if reused across pages. Middleware goes in the same file as the route group it protects, or in `middleware.go` if shared.

```
myapp/
├── main.go         # skeleton + routes
├── home.go         # func HomePage(c *via.Context)
├── chat.go         # func ChatPage(c *via.Context)
└── middleware.go    # shared middleware
```

## Stage 3: Co-located CSS and shared types

As pages accumulate custom styling, CSS strings in Go become hard to maintain — no syntax highlighting, no linting. Extract them to `.css` files alongside the pages they belong to and use `//go:embed` to load them.

```go
// main.go
package main

import "embed"

//go:embed chat.css
var chatCSS string

func main() {
    v := via.New()

    v.AppendToHead(
        h.Link(h.Rel("stylesheet"), h.Href("https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css")),
        h.StyleEl(h.Raw(chatCSS)),
    )

    // ...
}
```

When multiple pages share the same structs, extract them to `types.go`. Framework-agnostic domain logic (helpers, dummy data, business rules) gets its own file too.

```
myapp/
├── main.go         # skeleton + routes + global styles
├── home.go
├── chat.go
├── chat.css        # //go:embed in main.go
├── types.go        # shared types
└── userdata.go     # helpers, dummy data
```

The [nats-chatroom](../internal/examples/nats-chatroom/) example demonstrates this layout.

## CSS Approaches

Via doesn't prescribe a CSS strategy. Two approaches work well:

**CSS framework classes in Go code** — Use Pico, Tailwind, or similar. Classes go directly in the view via `h.Class()`. Good for rapid prototyping since there's nothing to extract.

```go
h.Div(h.Class("container"),
    h.Button(h.Class("primary"), h.Text("Save")),
)
```

**Co-located `.css` files with `//go:embed`** — Write plain CSS in a separate file, embed it, and inject via `AppendToHead`. You get syntax highlighting, linting, and clean separation.

```go
//go:embed chat.css
var chatCSS string

// in main():
v.AppendToHead(h.StyleEl(h.Raw(chatCSS)))
```

Use a framework for quick prototypes and dashboards. Switch to co-located CSS files when you have significant custom styling or want tooling support.

## Next Steps

- [Getting Started](getting-started.md) — The core loop and configuration
- [State and Interactivity](state-and-interactivity.md) — Signals, actions, components, validation
