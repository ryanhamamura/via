package via

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestEventTypes(t *testing.T) {
	tests := []struct {
		name     string
		attr     string
		buildEl  func(trigger *actionTrigger) h.H
	}{
		{"OnSubmit", "data-on:submit", func(tr *actionTrigger) h.H { return h.Form(tr.OnSubmit()) }},
		{"OnInput", "data-on:input", func(tr *actionTrigger) h.H { return h.Input(tr.OnInput()) }},
		{"OnFocus", "data-on:focus", func(tr *actionTrigger) h.H { return h.Input(tr.OnFocus()) }},
		{"OnBlur", "data-on:blur", func(tr *actionTrigger) h.H { return h.Input(tr.OnBlur()) }},
		{"OnMouseEnter", "data-on:mouseenter", func(tr *actionTrigger) h.H { return h.Div(tr.OnMouseEnter()) }},
		{"OnMouseLeave", "data-on:mouseleave", func(tr *actionTrigger) h.H { return h.Div(tr.OnMouseLeave()) }},
		{"OnScroll", "data-on:scroll", func(tr *actionTrigger) h.H { return h.Div(tr.OnScroll()) }},
		{"OnDblClick", "data-on:dblclick", func(tr *actionTrigger) h.H { return h.Div(tr.OnDblClick()) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var trigger *actionTrigger
			v := New()
			v.Page("/", func(c *Context) {
				trigger = c.Action(func() {})
				c.View(func() h.H { return tt.buildEl(trigger) })
			})

			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			v.mux.ServeHTTP(w, req)
			body := w.Body.String()
			assert.Contains(t, body, tt.attr)
			assert.Contains(t, body, "/_action/"+trigger.id)
		})
	}

	t.Run("WithSignal", func(t *testing.T) {
		var trigger *actionTrigger
		var sig *signal
		v := New()
		v.Page("/", func(c *Context) {
			trigger = c.Action(func() {})
			sig = c.Signal("val")
			c.View(func() h.H {
				return h.Div(trigger.OnDblClick(WithSignal(sig, "x")))
			})
		})

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		v.mux.ServeHTTP(w, req)
		body := w.Body.String()
		assert.Contains(t, body, "data-on:dblclick")
		assert.Contains(t, body, "$"+sig.ID()+"=&#39;x&#39;")
	})
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

func TestReaperCleansOrphanedContexts(t *testing.T) {
	v := New()
	c := newContext("orphan-1", "/", v)
	c.createdAt = time.Now().Add(-time.Minute) // created 1 min ago
	v.registerCtx(c)

	_, err := v.getCtx("orphan-1")
	assert.NoError(t, err)

	v.reapOrphanedContexts(10 * time.Second)

	_, err = v.getCtx("orphan-1")
	assert.Error(t, err, "orphaned context should have been reaped")
}

func TestReaperIgnoresConnectedContexts(t *testing.T) {
	v := New()
	c := newContext("connected-1", "/", v)
	c.createdAt = time.Now().Add(-time.Minute)
	c.sseConnected.Store(true)
	v.registerCtx(c)

	v.reapOrphanedContexts(10 * time.Second)

	_, err := v.getCtx("connected-1")
	assert.NoError(t, err, "connected context should survive reaping")
}

func TestReaperDisabledWithNegativeTTL(t *testing.T) {
	v := New()
	v.cfg.ContextTTL = -1
	v.startReaper()
	assert.Nil(t, v.reaperStop, "reaper should not start with negative TTL")
}

func TestCleanupCtxIdempotent(t *testing.T) {
	v := New()
	c := newContext("idempotent-1", "/", v)
	v.registerCtx(c)

	assert.NotPanics(t, func() {
		v.cleanupCtx(c)
		v.cleanupCtx(c)
	})

	_, err := v.getCtx("idempotent-1")
	assert.Error(t, err, "context should be removed after cleanup")
}

func TestDevModeRemovePersistedFix(t *testing.T) {
	v := New()
	v.cfg.DevMode = true

	dir := filepath.Join(t.TempDir(), ".via", "devmode")
	p := filepath.Join(dir, "ctx.json")
	assert.NoError(t, os.MkdirAll(dir, 0755))

	// Write a persisted context
	ctxRegMap := map[string]string{"test-ctx-1": "/"}
	f, err := os.Create(p)
	assert.NoError(t, err)
	assert.NoError(t, json.NewEncoder(f).Encode(ctxRegMap))
	f.Close()

	// Patch devModeRemovePersisted to use our temp path by calling it
	// directly — we need to override the path. Instead, test via the
	// actual function by temporarily changing the working dir.
	origDir, _ := os.Getwd()
	assert.NoError(t, os.Chdir(t.TempDir()))
	defer os.Chdir(origDir)

	// Re-create the structure in the temp dir
	assert.NoError(t, os.MkdirAll(filepath.Join(".via", "devmode"), 0755))
	p2 := filepath.Join(".via", "devmode", "ctx.json")
	f2, _ := os.Create(p2)
	json.NewEncoder(f2).Encode(map[string]string{"test-ctx-1": "/"})
	f2.Close()

	c := newContext("test-ctx-1", "/", v)
	v.devModeRemovePersisted(c)

	// Read back and verify
	f3, err := os.Open(p2)
	assert.NoError(t, err)
	defer f3.Close()
	var result map[string]string
	assert.NoError(t, json.NewDecoder(f3).Decode(&result))
	assert.Empty(t, result, "persisted context should be removed")
}
