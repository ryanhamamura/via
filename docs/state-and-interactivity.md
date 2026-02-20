# State and Interactivity

This is the core reactive model — signals, actions, views, components, and validation.

## Context Lifecycle

A `*Context` is created per browser visit. It holds all page state: signals, actions, fields, subscriptions, and the view function.

```
Browser hits page → new Context created → init function runs → HTML rendered
                                                                    ↓
                                        SSE connection opens ← browser loads page
                                                ↓
                        action fires → signals injected from browser → handler runs → Sync() → DOM patched
```

The context lives until the browser tab closes (detected via a `beforeunload` beacon) or the server shuts down. There is no background reaper — contexts persist across temporary SSE disconnections so backgrounded tabs resume seamlessly.

During [SPA navigation](routing-and-navigation.md#spa-navigation), the context itself survives — only page-level state (signals, actions, fields, intervals, subscriptions) is reset. The SSE connection persists.

## Signals

Signals are reactive values synchronized between server and browser. Create one with an initial value:

```go
name := c.Signal("world")
count := c.Signal(0)
items := c.Signal([]string{"a", "b"})
```

### Reading values

```go
name.String()  // "world"
count.Int()    // 0
count.Bool()   // false (parses "true", "1", "yes", "on")
```

Signal values come from the browser. Before every action call, the browser sends all current signal values to the server. You always read the latest browser state inside action handlers.

### Writing values

```go
name.SetValue("Via")
c.SyncSignals()  // push only changed signals to browser
// or
c.Sync()  // re-render view AND push changed signals
```

`SetValue` marks the signal as changed. The change is not sent to the browser until you call `Sync()` or `SyncSignals()`.

### Rendering in the view

```go
// Two-way binding on an input — browser edits update the signal
h.Input(h.Type("text"), name.Bind())

// Reactive text display — updates when the signal changes
h.Span(name.Text())

// Read value at render time — static until next Sync()
h.P(h.Textf("Count: %d", count.Int()))
```

`Bind()` outputs a `data-bind` attribute for two-way binding. `Text()` outputs a `<span data-text="$signalID">` for reactive display.

## Actions

Actions are server-side event handlers. They run on the server when triggered by a browser event.

```go
submit := c.Action(func() {
    // signals are already injected — read them here
    fmt.Println(name.String())
    count.SetValue(count.Int() + 1)
    c.Sync()
})
```

### Trigger methods

Attach an action to a DOM event by calling a trigger method in the view:

```go
h.Button(h.Text("Submit"), submit.OnClick())
h.Input(name.Bind(), submit.OnKeyDown("Enter"))
h.Select(category.Bind(), filter.OnChange())
h.Form(submit.OnSubmit())
```

Available triggers:

| Method | Event | Notes |
|--------|-------|-------|
| `OnClick()` | `click` | |
| `OnDblClick()` | `dblclick` | |
| `OnChange()` | `change` | |
| `OnInput()` | `input` | No debounce |
| `OnSubmit()` | `submit` | |
| `OnKeyDown(key)` | `keydown` | Filtered by key name (e.g. `"Enter"`, `"Escape"`) |
| `OnFocus()` | `focus` | |
| `OnBlur()` | `blur` | |
| `OnMouseEnter()` | `mouseenter` | |
| `OnMouseLeave()` | `mouseleave` | |
| `OnScroll()` | `scroll` | |

### Trigger options

Every trigger method accepts `ActionTriggerOption` values:

```go
// Set a signal value before the action fires
submit.OnClick(via.WithSignal(mode, "delete"))
submit.OnClick(via.WithSignalInt(page, 3))

// Listen on window instead of the element
submit.OnKeyDown("Escape", via.WithWindow())

// Prevent browser default behavior
submit.OnKeyDown("ArrowDown", via.WithPreventDefault())
```

### Multi-key dispatch

`OnKeyDownMap` binds multiple keys to different actions in a single attribute:

```go
via.OnKeyDownMap(
    via.KeyBind("w", move, via.WithSignal(dir, "up")),
    via.KeyBind("s", move, via.WithSignal(dir, "down")),
    via.KeyBind("ArrowUp", move, via.WithSignal(dir, "up"), via.WithPreventDefault()),
    via.KeyBind("ArrowDown", move, via.WithSignal(dir, "down"), via.WithPreventDefault()),
)
```

This produces a single `data-on:keydown__window` attribute. Place it on any element in the view.

### Action options

```go
// Per-action rate limiting (overrides the context-level default)
c.Action(handler, via.WithRateLimit(5, 10))

// Per-action middleware (runs after CSRF and rate-limit checks)
c.Action(handler, via.WithMiddleware(requireAdmin))
```

## Views and Sync

Every page handler must call `c.View()` to define the UI:

```go
c.View(func() h.H {
    return h.Div(
        h.P(h.Textf("Hello, %s!", name.String())),
    )
})
```

> **Gotcha:** If you forget `c.View()`, the app panics at startup during route registration — not at request time.

The view function is re-evaluated on every `c.Sync()`. The resulting HTML is pushed to the browser via SSE, where Datastar morphs the DOM.

### Sync variants

| Method | What it sends |
|--------|---------------|
| `c.Sync()` | Re-renders the view HTML **and** pushes changed signals |
| `c.SyncSignals()` | Pushes only changed signals, no view re-render |
| `c.SyncElements(elem...)` | Pushes specific HTML elements to merge into the DOM. Each element **must have an ID** matching an existing DOM element |
| `c.ExecScript(js)` | Sends JavaScript for the browser to execute (auto-removed after execution) |

Use `SyncSignals()` when only signal values changed and the view structure is the same. Use `SyncElements()` for targeted updates without re-rendering the entire view. Use `ExecScript()` to interact with client-side libraries (e.g. pushing data to a chart).

## Components

Extract reusable UI with `c.Component()`:

```go
func counterFn(c *via.Context) {
    count := 0
    step := c.Signal(1)

    increment := c.Action(func() {
        count += step.Int()
        c.Sync()
    })

    c.View(func() h.H {
        return h.Div(
            h.P(h.Textf("Count: %d", count)),
            h.Input(h.Type("number"), step.Bind()),
            h.Button(h.Text("+"), increment.OnClick()),
        )
    })
}

// In a page:
v.Page("/", func(c *via.Context) {
    counter1 := c.Component(counterFn)
    counter2 := c.Component(counterFn)

    c.View(func() h.H {
        return h.Div(
            h.H2(h.Text("Counter 1")), counter1(),
            h.H2(h.Text("Counter 2")), counter2(),
        )
    })
})
```

Each component instance gets its own closure state, but signals, actions, and fields are registered on the parent page context. Components share the parent's SSE stream — `c.Sync()` from a component re-renders the entire page view.

## Fields and Validation

Fields are signals with validation rules. Use them for form inputs:

```go
username := c.Field("", via.Required(), via.MinLen(3), via.MaxLen(20))
email := c.Field("", via.Required(), via.Email())
age := c.Field("", via.Required(), via.Min(13), via.Max(120))
website := c.Field("", via.Pattern(`^https?://`, "Must start with http:// or https://"))
```

### Built-in rules

| Rule | Description |
|------|-------------|
| `Required(msg...)` | Rejects empty/whitespace-only values |
| `MinLen(n, msg...)` | Minimum character count (Unicode-aware) |
| `MaxLen(n, msg...)` | Maximum character count (Unicode-aware) |
| `Min(n, msg...)` | Minimum numeric value (parsed as int) |
| `Max(n, msg...)` | Maximum numeric value (parsed as int) |
| `Email(msg...)` | Email format regex |
| `Pattern(re, msg...)` | Custom regex |
| `Custom(fn)` | `func(string) error` — return non-nil to fail |

All rules accept an optional custom error message as the last argument.

### Using fields in views and actions

```go
submit := c.Action(func() {
    if !c.ValidateAll() {
        c.Sync()
        return
    }
    // Server-side validation
    if userExists(username.String()) {
        username.AddError("Username taken")
        c.Sync()
        return
    }
    createUser(username.String(), email.String())
    c.ResetFields()
    c.Sync()
})

c.View(func() h.H {
    return h.Form(submit.OnSubmit(),
        h.Input(h.Type("text"), username.Bind(), h.Placeholder("Username")),
        h.If(username.HasError(), h.Small(h.Text(username.FirstError()))),

        h.Input(h.Type("email"), email.Bind(), h.Placeholder("Email")),
        h.If(email.HasError(), h.Small(h.Text(email.FirstError()))),

        h.Button(h.Type("submit"), h.Text("Sign Up")),
    )
})
```

| Method | Description |
|--------|-------------|
| `field.Validate()` | Run rules, return true if all pass |
| `field.HasError()` | True if any validation errors exist |
| `field.FirstError()` | First error message, or `""` |
| `field.Errors()` | All error messages |
| `field.AddError(msg)` | Add a custom server-side error |
| `field.ClearErrors()` | Remove all errors |
| `field.Reset()` | Restore initial value and clear errors |
| `c.ValidateAll(fields...)` | Validate given fields (or all if none specified). Does not short-circuit — all fields get validated so all errors are populated |
| `c.ResetFields(fields...)` | Reset given fields (or all if none specified) |

Fields embed `*signal`, so `Bind()`, `Text()`, `String()`, `Int()`, `Bool()`, `SetValue()`, and `ID()` all work.

## OnInterval

Run a function at regular intervals, tied to the page lifecycle:

```go
stop := c.OnInterval(time.Second, func() {
    now = time.Now()
    c.Sync()
})
```

- Starts immediately — no separate start call needed.
- Returns a `func()` that stops the interval (idempotent).
- Automatically stops on context disposal (tab close) or SPA navigation away.
- Call `c.Sync()` inside the handler to push updates to the browser.

## Navigation Helpers

| Method | Effect |
|--------|--------|
| `c.Redirect(url)` | Full page navigation. Disposes the context, browser loads a new page |
| `c.Redirectf(fmt, args...)` | `Redirect` with `fmt.Sprintf` |
| `c.RedirectView(url)` | Sets the view to trigger a redirect on SSE connect. Use in [middleware](routing-and-navigation.md#middleware) to abort the chain and redirect |
| `c.ReplaceURL(url)` | Updates the browser URL bar without navigation. Useful for reflecting state in query params |
| `c.ReplaceURLf(fmt, args...)` | `ReplaceURL` with `fmt.Sprintf` |
| `c.Navigate(path, popstate)` | [SPA navigation](routing-and-navigation.md#spa-navigation). Resets page state, runs the target page handler on the same context, pushes the new view with a view transition |

> **Gotcha:** In middleware, use `c.RedirectView()`, not `c.Redirect()`. `Redirect` sends a patch over SSE, but the SSE connection isn't established yet during the initial page load.
