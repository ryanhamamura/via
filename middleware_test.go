package via

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ryanhamamura/via/h"
	"github.com/stretchr/testify/assert"
)

func TestMiddlewareRunsBeforeHandler(t *testing.T) {
	var order []string

	v := New()
	v.Use(func(c *Context, next func()) {
		order = append(order, "mw")
		next()
	})
	v.Page("/", func(c *Context) {
		order = append(order, "handler")
		c.View(func() h.H { return h.Div() })
	})

	// Reset after registration (panic-check runs the raw handler)
	order = nil

	w := httptest.NewRecorder()
	v.mux.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []string{"mw", "handler"}, order)
}

func TestMiddlewareAbortSkipsHandler(t *testing.T) {
	handlerCalled := false

	v := New()
	v.Use(func(c *Context, next func()) {
		c.RedirectView("/other")
	})
	v.Page("/", func(c *Context) {
		handlerCalled = true
		c.View(func() h.H { return h.Div() })
	})

	handlerCalled = false

	w := httptest.NewRecorder()
	v.mux.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, handlerCalled)
}

func TestMiddlewareChainOrder(t *testing.T) {
	var order []string

	v := New()
	for _, label := range []string{"A", "B", "C"} {
		l := label
		v.Use(func(c *Context, next func()) {
			order = append(order, l)
			next()
		})
	}
	v.Page("/", func(c *Context) {
		order = append(order, "handler")
		c.View(func() h.H { return h.Div() })
	})

	order = nil

	w := httptest.NewRecorder()
	v.mux.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	assert.Equal(t, []string{"A", "B", "C", "handler"}, order)
}

func TestGroupPrefixRouting(t *testing.T) {
	v := New()
	g := v.Group("/admin")
	g.Page("/dashboard", func(c *Context) {
		c.View(func() h.H { return h.Div(h.Text("admin dashboard")) })
	})

	w := httptest.NewRecorder()
	v.mux.ServeHTTP(w, httptest.NewRequest("GET", "/admin/dashboard", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "admin dashboard")
}

func TestGroupMiddlewareAppliesToGroupOnly(t *testing.T) {
	var groupMwCalled bool

	v := New()
	g := v.Group("/admin", func(c *Context, next func()) {
		groupMwCalled = true
		next()
	})
	g.Page("/panel", func(c *Context) {
		c.View(func() h.H { return h.Div(h.Text("panel")) })
	})
	v.Page("/public", func(c *Context) {
		c.View(func() h.H { return h.Div(h.Text("public")) })
	})

	// Hit public page — group middleware should NOT run
	groupMwCalled = false
	w := httptest.NewRecorder()
	v.mux.ServeHTTP(w, httptest.NewRequest("GET", "/public", nil))
	assert.False(t, groupMwCalled)
	assert.Contains(t, w.Body.String(), "public")

	// Hit group page — group middleware should run
	groupMwCalled = false
	w = httptest.NewRecorder()
	v.mux.ServeHTTP(w, httptest.NewRequest("GET", "/admin/panel", nil))
	assert.True(t, groupMwCalled)
	assert.Contains(t, w.Body.String(), "panel")
}

func TestGlobalMiddlewareAppliesToGroupPages(t *testing.T) {
	var globalCalled bool

	v := New()
	v.Use(func(c *Context, next func()) {
		globalCalled = true
		next()
	})
	g := v.Group("/admin")
	g.Page("/dash", func(c *Context) {
		c.View(func() h.H { return h.Div(h.Text("dash")) })
	})

	globalCalled = false

	w := httptest.NewRecorder()
	v.mux.ServeHTTP(w, httptest.NewRequest("GET", "/admin/dash", nil))

	assert.True(t, globalCalled)
	assert.Contains(t, w.Body.String(), "dash")
}

func TestNestedGroupInheritsPrefixAndMiddleware(t *testing.T) {
	var order []string

	v := New()
	admin := v.Group("/admin", func(c *Context, next func()) {
		order = append(order, "admin")
		next()
	})
	superAdmin := admin.Group("/super", func(c *Context, next func()) {
		order = append(order, "super")
		next()
	})
	superAdmin.Page("/secret", func(c *Context) {
		order = append(order, "handler")
		c.View(func() h.H { return h.Div(h.Text("secret")) })
	})

	order = nil

	w := httptest.NewRecorder()
	v.mux.ServeHTTP(w, httptest.NewRequest("GET", "/admin/super/secret", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []string{"admin", "super", "handler"}, order)
	assert.Contains(t, w.Body.String(), "secret")
}

func TestGroupUse(t *testing.T) {
	var order []string

	v := New()
	g := v.Group("/api")
	g.Use(func(c *Context, next func()) {
		order = append(order, "added-later")
		next()
	})
	g.Page("/items", func(c *Context) {
		order = append(order, "handler")
		c.View(func() h.H { return h.Div() })
	})

	order = nil

	w := httptest.NewRecorder()
	v.mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/items", nil))

	assert.Equal(t, []string{"added-later", "handler"}, order)
}

func TestRedirectViewSetsValidView(t *testing.T) {
	v := New()
	v.Page("/test", func(c *Context) {
		c.RedirectView("/somewhere")
	})

	w := httptest.NewRecorder()
	v.mux.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "<!doctype html>")
}

func TestGlobalAndGroupMiddlewareOrder(t *testing.T) {
	var order []string

	v := New()
	v.Use(func(c *Context, next func()) {
		order = append(order, "global")
		next()
	})
	g := v.Group("/g", func(c *Context, next func()) {
		order = append(order, "group")
		next()
	})
	g.Page("/page", func(c *Context) {
		order = append(order, "handler")
		c.View(func() h.H { return h.Div() })
	})

	order = nil

	w := httptest.NewRecorder()
	v.mux.ServeHTTP(w, httptest.NewRequest("GET", "/g/page", nil))

	assert.Equal(t, []string{"global", "group", "handler"}, order)
}

// --- Action middleware tests ---

func TestActionMiddlewareRunsBeforeAction(t *testing.T) {
	var order []string

	v := New()
	c := newContext("test", "/", v)

	mw := func(_ *Context, next func()) {
		order = append(order, "mw")
		next()
	}

	trigger := c.Action(func() {
		order = append(order, "action")
	}, WithMiddleware(mw))

	entry, err := c.getAction(trigger.id)
	assert.NoError(t, err)

	chainMiddleware(entry.middleware, func(_ *Context) { entry.fn() })(c)

	assert.Equal(t, []string{"mw", "action"}, order)
}

func TestActionMiddlewareAbortSkipsAction(t *testing.T) {
	actionCalled := false

	v := New()
	c := newContext("test", "/", v)

	mw := func(_ *Context, next func()) {
		// don't call next — action should not run
	}

	trigger := c.Action(func() {
		actionCalled = true
	}, WithMiddleware(mw))

	entry, err := c.getAction(trigger.id)
	assert.NoError(t, err)

	chainMiddleware(entry.middleware, func(_ *Context) { entry.fn() })(c)

	assert.False(t, actionCalled)
}

func TestActionMiddlewareChainOrder(t *testing.T) {
	var order []string

	v := New()
	c := newContext("test", "/", v)

	var mws []Middleware
	for _, label := range []string{"A", "B", "C"} {
		l := label
		mws = append(mws, func(_ *Context, next func()) {
			order = append(order, l)
			next()
		})
	}

	trigger := c.Action(func() {
		order = append(order, "action")
	}, WithMiddleware(mws...))

	entry, err := c.getAction(trigger.id)
	assert.NoError(t, err)

	chainMiddleware(entry.middleware, func(_ *Context) { entry.fn() })(c)

	assert.Equal(t, []string{"A", "B", "C", "action"}, order)
}

func TestActionMiddlewareCombinedWithRateLimit(t *testing.T) {
	v := New()
	c := newContext("test", "/", v)

	mw := func(_ *Context, next func()) { next() }

	trigger := c.Action(func() {}, WithRateLimit(5, 10), WithMiddleware(mw))

	entry, err := c.getAction(trigger.id)
	assert.NoError(t, err)
	assert.NotNil(t, entry.limiter)
	assert.Len(t, entry.middleware, 1)
}

func TestGroupWithEmptyPrefix(t *testing.T) {
	var mwCalled bool

	v := New()
	g := v.Group("", func(c *Context, next func()) {
		mwCalled = true
		next()
	})
	g.Page("/dashboard", func(c *Context) {
		c.View(func() h.H { return h.Div(h.Text("dash")) })
	})

	mwCalled = false

	w := httptest.NewRecorder()
	v.mux.ServeHTTP(w, httptest.NewRequest("GET", "/dashboard", nil))

	assert.True(t, mwCalled)
	assert.Contains(t, w.Body.String(), "dash")
}
