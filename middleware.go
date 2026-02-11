package via

// Middleware wraps a page init function. Call next to continue the chain;
// return without calling next to abort (set a view first, e.g. RedirectView).
type Middleware func(c *Context, next func())

// Group is a route group with a shared prefix and middleware stack.
type Group struct {
	v          *V
	prefix     string
	middleware []Middleware
}

// Use appends middleware to the global stack.
// Global middleware runs before every page handler.
func (v *V) Use(mw ...Middleware) {
	v.middleware = append(v.middleware, mw...)
}

// Group creates a route group with the given path prefix and middleware.
// Routes registered on the group are prefixed and run the group's middleware
// after any global middleware.
func (v *V) Group(prefix string, mw ...Middleware) *Group {
	return &Group{
		v:          v,
		prefix:     prefix,
		middleware: mw,
	}
}

// Page registers a route on this group. The full route is the group prefix
// concatenated with route.
func (g *Group) Page(route string, initContextFn func(c *Context)) {
	fullRoute := g.prefix + route
	allMw := make([]Middleware, 0, len(g.v.middleware)+len(g.middleware))
	allMw = append(allMw, g.v.middleware...)
	allMw = append(allMw, g.middleware...)
	wrapped := chainMiddleware(allMw, initContextFn)
	g.v.page(fullRoute, initContextFn, wrapped)
}

// Group creates a nested sub-group that inherits this group's prefix and
// middleware, then adds its own.
func (g *Group) Group(prefix string, mw ...Middleware) *Group {
	combined := make([]Middleware, len(g.middleware), len(g.middleware)+len(mw))
	copy(combined, g.middleware)
	combined = append(combined, mw...)
	return &Group{
		v:          g.v,
		prefix:     g.prefix + prefix,
		middleware: combined,
	}
}

// Use appends middleware to this group's stack.
func (g *Group) Use(mw ...Middleware) {
	g.middleware = append(g.middleware, mw...)
}

// WithMiddleware returns an ActionOption that attaches middleware to an action.
// Action middleware runs after CSRF/rate-limit checks and signal injection.
func WithMiddleware(mw ...Middleware) ActionOption {
	return func(e *actionEntry) {
		e.middleware = append(e.middleware, mw...)
	}
}

// chainMiddleware wraps handler with the given middleware, outer-first.
func chainMiddleware(mws []Middleware, handler func(*Context)) func(*Context) {
	if len(mws) == 0 {
		return handler
	}
	chained := handler
	for i := len(mws) - 1; i >= 0; i-- {
		mw := mws[i]
		next := chained
		chained = func(c *Context) {
			mw(c, func() { next(c) })
		}
	}
	return chained
}
