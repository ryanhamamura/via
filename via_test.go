package via

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ryanhamamura/via/h"
	"github.com/stretchr/testify/assert"
)

func TestPageRoute(t *testing.T) {
	v := New()
	v.Page("/", func(c *Context) {
		c.View(func() h.H {
			return h.Div(h.Text("Hello Via!"))
		})
	})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	v.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Hello Via!")
	assert.Contains(t, w.Body.String(), "<!doctype html>")
}

func TestDatastarJS(t *testing.T) {
	v := New()
	v.Page("/", func(c *Context) {
		c.View(func() h.H { return h.Div() })
	})

	req := httptest.NewRequest("GET", "/_datastar.js", nil)
	w := httptest.NewRecorder()
	v.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/javascript", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "🖕JS_DS🚀")
}

func TestCustomDatastarContent(t *testing.T) {
	customScript := []byte("// Custom Datastar Script")
	v := New()
	v.Config(Options{
		DatastarContent: customScript,
	})
	v.Page("/", func(c *Context) {
		c.View(func() h.H { return h.Div() })
	})

	req := httptest.NewRequest("GET", "/_datastar.js", nil)
	w := httptest.NewRecorder()
	v.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/javascript", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "Custom Datastar Script")
}

func TestCustomDatastarPath(t *testing.T) {
	v := New()
	v.Config(Options{
		DatastarPath: "/assets/datastar.js",
	})
	v.Page("/test", func(c *Context) {
		c.View(func() h.H { return h.Div() })
	})

	// Custom path should serve the script
	req := httptest.NewRequest("GET", "/assets/datastar.js", nil)
	w := httptest.NewRecorder()
	v.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/javascript", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "🖕JS_DS🚀")

	// Page should reference the custom path in script tag
	req2 := httptest.NewRequest("GET", "/test", nil)
	w2 := httptest.NewRecorder()
	v.mux.ServeHTTP(w2, req2)
	assert.Contains(t, w2.Body.String(), `src="/assets/datastar.js"`)
}

func TestSignal(t *testing.T) {
	var sig *signal
	v := New()
	v.Page("/", func(c *Context) {
		sig = c.Signal("test")
		c.View(func() h.H { return h.Div() })
	})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	v.mux.ServeHTTP(w, req)

	assert.Equal(t, "test", sig.String())
}

func TestAction(t *testing.T) {
	var trigger *actionTrigger
	var sig *signal
	v := New()
	v.Page("/", func(c *Context) {
		trigger = c.Action(func() {})
		sig = c.Signal("value")
		c.View(func() h.H {
			return h.Div(
				h.Button(trigger.OnClick()),
				h.Input(trigger.OnChange()),
				h.Input(trigger.OnKeyDown("Enter")),
				h.Button(trigger.OnClick(WithSignal(sig, "test"))),
				h.Button(trigger.OnClick(WithSignalInt(sig, 42))),
			)
		})
	})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	v.mux.ServeHTTP(w, req)
	body := w.Body.String()
	assert.Contains(t, body, "data-on:click")
	assert.Contains(t, body, "data-on:change__debounce.200ms")
	assert.Contains(t, body, "data-on:keydown")
	assert.Contains(t, body, "/_action/")
}

func TestOnKeyDownWithWindow(t *testing.T) {
	var trigger *actionTrigger
	v := New()
	v.Page("/", func(c *Context) {
		trigger = c.Action(func() {})
		c.View(func() h.H {
			return h.Div(trigger.OnKeyDown("Enter", WithWindow()))
		})
	})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	v.mux.ServeHTTP(w, req)
	body := w.Body.String()
	assert.Contains(t, body, "data-on:keydown__window")
	assert.Contains(t, body, "evt.key===&#39;Enter&#39;")
}

func TestOnKeyDownMap(t *testing.T) {
	t.Run("multiple bindings with different actions", func(t *testing.T) {
		var move, shoot *actionTrigger
		var dir *signal
		v := New()
		v.Page("/", func(c *Context) {
			dir = c.Signal("none")
			move = c.Action(func() {})
			shoot = c.Action(func() {})
			c.View(func() h.H {
				return h.Div(
					OnKeyDownMap(
						KeyBind("w", move, WithSignal(dir, "up")),
						KeyBind("ArrowUp", move, WithSignal(dir, "up"), WithPreventDefault()),
						KeyBind(" ", shoot, WithPreventDefault()),
					),
				)
			})
		})

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		v.mux.ServeHTTP(w, req)
		body := w.Body.String()

		// Single attribute, window-scoped
		assert.Contains(t, body, "data-on:keydown__window")

		// Key dispatching
		assert.Contains(t, body, "evt.key===&#39;w&#39;")
		assert.Contains(t, body, "evt.key===&#39;ArrowUp&#39;")
		assert.Contains(t, body, "evt.key===&#39; &#39;")

		// Different actions referenced
		assert.Contains(t, body, "/_action/"+move.id)
		assert.Contains(t, body, "/_action/"+shoot.id)

		// preventDefault only on ArrowUp and space branches
		assert.Contains(t, body, "evt.key===&#39;ArrowUp&#39; ? (evt.preventDefault()")
		assert.Contains(t, body, "evt.key===&#39; &#39; ? (evt.preventDefault()")

		// 'w' branch should NOT have preventDefault
		assert.NotContains(t, body, "evt.key===&#39;w&#39; ? (evt.preventDefault()")
	})

	t.Run("WithSignal per binding", func(t *testing.T) {
		var move *actionTrigger
		var dir *signal
		v := New()
		v.Page("/", func(c *Context) {
			dir = c.Signal("none")
			move = c.Action(func() {})
			c.View(func() h.H {
				return h.Div(
					OnKeyDownMap(
						KeyBind("w", move, WithSignal(dir, "up")),
						KeyBind("s", move, WithSignal(dir, "down")),
					),
				)
			})
		})

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		v.mux.ServeHTTP(w, req)
		body := w.Body.String()

		assert.Contains(t, body, "$"+dir.ID()+"=&#39;up&#39;")
		assert.Contains(t, body, "$"+dir.ID()+"=&#39;down&#39;")
	})

	t.Run("empty bindings returns nil", func(t *testing.T) {
		result := OnKeyDownMap()
		assert.Nil(t, result)
	})
}

func TestConfig(t *testing.T) {
	v := New()
	v.Config(Options{DocumentTitle: "Test"})
	assert.Equal(t, "Test", v.cfg.DocumentTitle)
}

func TestPage_PanicsOnNoView(t *testing.T) {
	assert.Panics(t, func() {
		v := New()
		v.Page("/", func(c *Context) {})
	})
}
