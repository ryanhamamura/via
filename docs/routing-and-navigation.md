# Routing and Navigation

Multi-page app structure, middleware, and Via's SPA navigation system.

## Pages

Register a page with a route pattern and an init function:

```go
v.Page("/", func(c *via.Context) {
    c.View(func() h.H {
        return h.H1(h.Text("Home"))
    })
})
```

Routes use Go's standard `net/http.ServeMux` patterns. Via registers each page as a `GET` handler.

> **Gotcha:** Via runs every page init function at registration time (in a `defer/recover` block) to catch panics early. If your init function panics — e.g. by forgetting `c.View()` — the app crashes at startup, not at request time.

## Path Parameters

Use `{param}` syntax in route patterns:

```go
v.Page("/users/{id}/posts/{post_id}", func(c *via.Context) {
    userID := c.GetPathParam("id")
    postID := c.GetPathParam("post_id")

    c.View(func() h.H {
        return h.P(h.Textf("User %s, Post %s", userID, postID))
    })
})
```

`GetPathParam` returns an empty string if the parameter doesn't exist.

## Route Groups

Group pages under a shared prefix with shared middleware:

```go
admin := v.Group("/admin", authRequired)
admin.Page("/dashboard", dashboardHandler) // route: /admin/dashboard
admin.Page("/settings", settingsHandler)   // route: /admin/settings
```

### Nesting

Groups nest — the child inherits the parent's prefix and middleware:

```go
admin := v.Group("/admin", authRequired)
admin.Use(auditLog) // add middleware after creation

superAdmin := admin.Group("/super", superAdminOnly)
superAdmin.Page("/nuke", nukeHandler) // route: /admin/super/nuke
// middleware order: global → authRequired → auditLog → superAdminOnly → handler
```

### Empty prefix

Use an empty prefix when you need shared middleware without a path prefix:

```go
protected := v.Group("", authRequired)
protected.Page("/dashboard", dashboardHandler) // route: /dashboard
protected.Page("/profile", profileHandler)     // route: /profile
```

## Middleware

```go
type Middleware func(c *Context, next func())
```

Call `next()` to continue the chain. Return without calling `next()` to abort — but set a view first.

```go
func authRequired(c *via.Context, next func()) {
    if c.Session().GetString("username") == "" {
        c.Session().Set("flash", "Please log in")
        c.RedirectView("/login")
        return // don't call next — chain is aborted
    }
    next()
}
```

> **Gotcha:** Use `c.RedirectView()` in middleware, not `c.Redirect()`. The SSE connection isn't open yet during the initial page load, so `Redirect()` (which sends a patch over SSE) won't work. `RedirectView()` sets the view to one that triggers a redirect once SSE connects.

### Three levels

| Level | Registration | Scope |
|-------|-------------|-------|
| Global | `v.Use(mw...)` | Every page |
| Group | `v.Group(prefix, mw...)` or `g.Use(mw...)` | Pages in the group |
| Action | `c.Action(fn, via.WithMiddleware(mw...))` | A single action endpoint |

### Execution order

Middleware runs in registration order: global first, then group, then the handler.

```go
v.Use(logger)                        // 1st
admin := v.Group("/admin", auth)     // 2nd
admin.Use(audit)                     // 3rd
admin.Page("/x", handler)           // 4th
// execution: logger → auth → audit → handler
```

Action-level middleware runs after CSRF validation and rate limiting, when the action endpoint is invoked.

## SPA Navigation

Via intercepts same-origin link clicks and navigates without a full page reload. The SSE connection persists, and the new page's view is morphed into the DOM with a view transition.

### How it works

1. `navigate.js` (embedded in every page) intercepts clicks on `<a>` elements.
2. For same-origin links, it POSTs to `/_navigate` with the context ID, CSRF token, and target URL.
3. The server calls `c.Navigate()`, which:
   - Resets page state (stops intervals, unsubscribes PubSub, clears signals/actions/fields)
   - Runs the target page's init function (with middleware) on the **same context**
   - Pushes the new view via SSE with a view transition
   - Updates the browser URL via `history.pushState()`

### What gets cleaned up on navigate

- Intervals stop (via `pageStopChan`)
- PubSub subscriptions are unsubscribed
- Signals, actions, and fields are cleared
- The new page starts completely fresh

The SSE connection and the context itself survive. This is what makes it an SPA — the existing stream is reused.

### Layouts

Define a layout to provide persistent chrome (nav bars, sidebars) that wraps every page:

```go
v.Layout(func(content func() h.H) h.H {
    return h.Div(
        h.Nav(
            h.A(h.Href("/"), h.Text("Home")),
            h.A(h.Href("/counter"), h.Text("Counter")),
            h.A(h.Href("/clock"), h.Text("Clock")),
        ),
        h.Main(content()),
    )
})
```

The `content` parameter is the page's view function. During SPA navigation, the entire layout + content is re-rendered and morphed — Datastar's morph algorithm (idiomorph) efficiently updates only the changed parts, so the nav bar stays visually stable while the main content transitions.

> **Gotcha:** Layout state does not persist across navigations in the way page state doesn't — the layout is re-rendered from scratch each time. If you need state that survives navigation (like a selected nav item), derive it from the current route rather than storing it in a variable.

### View transitions

Animate elements across page navigations using the browser View Transitions API:

```go
// On the home page:
h.H1(h.Text("Home"), h.DataViewTransition("page-title"))

// On the counter page:
h.H1(h.Text("Counter"), h.DataViewTransition("page-title"))
```

Elements with matching `view-transition-name` values animate smoothly during SPA navigation. `DataViewTransition` sets the CSS `view-transition-name` as an inline `style` attribute. If the element also needs other inline styles, set `view-transition-name` directly in a `Style()` call instead.

Via automatically includes the `<meta name="view-transition" content="same-origin">` tag to enable the API.

### Opting out

Add `data-via-no-boost` to links that should trigger a full page reload:

```go
h.A(h.Href("/"), h.Text("Full Reload"), h.Attr("data-via-no-boost"))
```

Links are also auto-ignored when:
- They have a `target` attribute (e.g. `target="_blank"`)
- Modifier keys are held (Ctrl, Meta, Shift, Alt)
- The `href` starts with `#` or is cross-origin
- The `href` is missing

### Programmatic navigation

Trigger SPA navigation from an action handler:

```go
goCounter := c.Action(func() {
    c.Navigate("/counter", false)
})
```

The second parameter controls history behavior: `false` for `pushState` (normal navigation), `true` for `replaceState` (back/forward).

If the path doesn't match any registered route, `Navigate` falls back to `c.Redirect()` (full page navigation).

### DataIgnoreMorph

Prevent Datastar from overwriting an element during morph:

```go
h.Div(h.ID("toast-container"), h.DataIgnoreMorph())
```

The element and its subtree are skipped during DOM patches. Useful for elements with client-side state: a focused input, an animation, a third-party widget, or a toast notification container.

## Custom HTTP Handlers

Access the underlying mux for non-Via routes (APIs, webhooks, health checks):

```go
mux := v.HTTPServeMux()
mux.HandleFunc("GET /api/health", healthHandler)
mux.HandleFunc("POST /api/webhook", webhookHandler)
```

Register before `v.Start()`. These routes bypass Via's context/SSE system entirely.
