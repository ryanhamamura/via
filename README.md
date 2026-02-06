# Via

Real-time engine for building reactive web applications in pure Go.

## Why Via?

The web became tangled in layers of JavaScript, build chains, and frameworks stacked on frameworks. Via takes a different path.

**Philosophy**
- No templates. No JavaScript. No transpilation. No hydration.
- Views are pure Go functions. HTML is composed with a type-safe DSL.
- A single SSE stream carries all reactivity — no WebSocket juggling, no polling.

**Batteries included**
- Automatic CSRF protection on every action call
- Token-bucket rate limiting (global defaults + per-action overrides)
- Cookie-based sessions backed by SQLite
- Pub/sub messaging with an embedded NATS backend
- Structured logging via zerolog
- Graceful shutdown with context draining
- Brotli compression out of the box

## Example

```go
package main

import (
	"github.com/ryanhamamura/via"
	"github.com/ryanhamamura/via/h"
)

type Counter struct{ Count int }

func main() {
	v := via.New()

	v.Page("/", func(c *via.Context) {
		data := Counter{Count: 0}
		step := c.Signal(1)

		increment := c.Action(func() {
			data.Count += step.Int()
			c.Sync()
		})

		c.View(func() h.H {
			return h.Div(
				h.P(h.Textf("Count: %d", data.Count)),
				h.Label(
					h.Text("Update Step: "),
					h.Input(h.Type("number"), step.Bind()),
				),
				h.Button(h.Text("Increment"), increment.OnClick()),
			)
		})
	})

	v.Start()
}
```

## What's built in

- **Reactive views + signals** — bind state to the DOM; changes push over SSE automatically
- **Components** — self-contained subcontexts with their own data, actions, and signals
- **Sessions** — cookie-based, backed by SQLite via `scs`
- **Pub/sub** — embedded NATS server with JetStream; generic `Publish[T]` / `Subscribe[T]` helpers
- **CSRF protection** — automatic token generation and validation on every action
- **Rate limiting** — token-bucket algorithm, configurable globally and per-action
- **Event handling** — `OnClick`, `OnChange`, `OnSubmit`, `OnInput`, `OnFocus`, `OnBlur`, `OnMouseEnter`, `OnMouseLeave`, `OnScroll`, `OnDblClick`, `OnKeyDown`, and `OnKeyDownMap` for multi-key bindings
- **Timed routines** — `OnInterval` with start/stop/update controls, tied to context lifecycle
- **Redirects** — `Redirect`, `ReplaceURL`, and format-string variants
- **Plugin system** — `func(v *V)` hooks for integrating CSS/JS libraries
- **Structured logging** — zerolog with configurable levels; console output in dev, JSON in production
- **Graceful shutdown** — listens for SIGINT/SIGTERM, drains contexts, closes pub/sub
- **Context lifecycle** — background reaper cleans up disconnected contexts; configurable TTL
- **HTML DSL** — the `h` package provides type-safe Go-native HTML composition

## Examples

The `internal/examples/` directory contains 14 runnable examples:

`chatroom` · `counter` · `countercomp` · `greeter` · `keyboard` · `livereload` · `nats-chatroom` · `pathparams` · `picocss` · `plugins` · `pubsub-crud` · `realtimechart` · `session` · `shakespeare`

## Experimental

Via is maturing — sessions, CSRF, rate limiting, pub/sub, and graceful shutdown are in place — but the API is still evolving. Expect breaking changes before `v1`.

## Contributing

- Via is intentionally minimal and opinionated — and so is contributing.
- Fork, branch, build, tinker, submit a pull request.
- Keep every line purposeful.
- Share feedback: open an issue or start a discussion.

## Credits

Via builds upon the work of these projects:

- [Datastar](https://data-star.dev) — the hypermedia framework powering browser reactivity through signals and real-time HTML patches over SSE.
- [Gomponents](https://maragu.dev/gomponents) — Go-native HTML composition that powers the `via/h` package.
