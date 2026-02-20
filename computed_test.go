package via

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/ryanhamamura/via/h"
	"github.com/stretchr/testify/assert"
)

func TestComputedBasic(t *testing.T) {
	v := New()
	var cs *computedSignal
	v.Page("/", func(c *Context) {
		sig1 := c.Signal("hello")
		sig2 := c.Signal("world")
		cs = c.Computed(func() string {
			return sig1.String() + " " + sig2.String()
		})
		c.View(func() h.H { return h.Div() })
	})
	assert.Equal(t, "hello world", cs.String())
}

func TestComputedReactivity(t *testing.T) {
	v := New()
	var cs *computedSignal
	var sig1 *Signal
	v.Page("/", func(c *Context) {
		sig1 = c.Signal("a")
		sig2 := c.Signal("b")
		cs = c.Computed(func() string {
			return sig1.String() + sig2.String()
		})
		c.View(func() h.H { return h.Div() })
	})
	assert.Equal(t, "ab", cs.String())
	sig1.SetValue("x")
	assert.Equal(t, "xb", cs.String())
}

func TestComputedInt(t *testing.T) {
	v := New()
	var cs *computedSignal
	v.Page("/", func(c *Context) {
		sig := c.Signal(21)
		cs = c.Computed(func() string {
			return fmt.Sprintf("%d", sig.Int()*2)
		})
		c.View(func() h.H { return h.Div() })
	})
	assert.Equal(t, 42, cs.Int())
}

func TestComputedBool(t *testing.T) {
	v := New()
	var cs *computedSignal
	v.Page("/", func(c *Context) {
		sig := c.Signal("true")
		cs = c.Computed(func() string {
			return sig.String()
		})
		c.View(func() h.H { return h.Div() })
	})
	assert.True(t, cs.Bool())
}

func TestComputedText(t *testing.T) {
	v := New()
	var cs *computedSignal
	v.Page("/", func(c *Context) {
		cs = c.Computed(func() string { return "hi" })
		c.View(func() h.H { return h.Div() })
	})

	var buf bytes.Buffer
	err := cs.Text().Render(&buf)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), `data-text="$`+cs.ID()+`"`)
}

func TestComputedChangeDetection(t *testing.T) {
	v := New()
	var ctx *Context
	var sig *Signal
	v.Page("/", func(c *Context) {
		ctx = c
		sig = c.Signal("a")
		c.Computed(func() string {
			return sig.String() + "!"
		})
		c.View(func() h.H { return h.Div() })
	})

	// First patch includes computed (changed=true from init)
	patch1 := ctx.prepareSignalsForPatch()
	assert.NotEmpty(t, patch1)

	// Second patch: nothing changed, computed should not be included
	patch2 := ctx.prepareSignalsForPatch()
	// Regular signal still has changed=true (not reset in prepareSignalsForPatch),
	// but computed should not appear since its value didn't change.
	hasComputed := false
	ctx.signals.Range(func(_, value any) bool {
		if cs, ok := value.(*computedSignal); ok {
			_, inPatch := patch2[cs.ID()]
			hasComputed = inPatch
		}
		return true
	})
	assert.False(t, hasComputed)

	// After changing dependency, computed should reappear
	sig.SetValue("b")
	patch3 := ctx.prepareSignalsForPatch()
	found := false
	ctx.signals.Range(func(_, value any) bool {
		if cs, ok := value.(*computedSignal); ok {
			if v, ok := patch3[cs.ID()]; ok {
				assert.Equal(t, "b!", v)
				found = true
			}
		}
		return true
	})
	assert.True(t, found)
}

func TestComputedInComponent(t *testing.T) {
	v := New()
	var cs *computedSignal
	var parentCtx *Context
	v.Page("/", func(c *Context) {
		parentCtx = c
		c.Component(func(comp *Context) {
			sig := comp.Signal("via")
			cs = comp.Computed(func() string {
				return "hello " + sig.String()
			})
			comp.View(func() h.H { return h.Div() })
		})
		c.View(func() h.H { return h.Div() })
	})

	assert.Equal(t, "hello via", cs.String())
	// Verify it's stored on the parent page context
	found := false
	parentCtx.signals.Range(func(_, value any) bool {
		if stored, ok := value.(*computedSignal); ok && stored.ID() == cs.ID() {
			found = true
		}
		return true
	})
	assert.True(t, found)
}

func TestComputedIsReadOnly(t *testing.T) {
	// Compile-time guarantee: *computedSignal has no Bind() or SetValue() methods.
	// This test exists as documentation — if someone adds those methods, the
	// interface assertion below will need updating and serve as a reminder.
	var cs interface{} = &computedSignal{}
	type writable interface {
		SetValue(any)
	}
	type bindable interface {
		Bind() h.H
	}
	_, isWritable := cs.(writable)
	_, isBindable := cs.(bindable)
	assert.False(t, isWritable, "computedSignal must not have SetValue")
	assert.False(t, isBindable, "computedSignal must not have Bind")
}

func TestComputedInjectSignalsSkips(t *testing.T) {
	v := New()
	var ctx *Context
	var cs *computedSignal
	v.Page("/", func(c *Context) {
		ctx = c
		cs = c.Computed(func() string { return "fixed" })
		c.View(func() h.H { return h.Div() })
	})

	// Simulate browser sending back a value for the computed signal — should be ignored
	ctx.injectSignals(map[string]any{
		cs.ID(): "injected",
	})
	assert.Equal(t, "fixed", cs.String())
}
