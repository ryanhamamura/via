package via

import (
	"fmt"
	"testing"

	"github.com/ryanhamamura/via/h"
	"github.com/stretchr/testify/assert"
)

func newTestField(initial any, rules ...Rule) *Field {
	v := New()
	var f *Field
	v.Page("/", func(c *Context) {
		f = c.Field(initial, rules...)
		c.View(func() h.H { return h.Div() })
	})
	return f
}

func TestFieldCreation(t *testing.T) {
	f := newTestField("hello", Required())
	assert.Equal(t, "hello", f.String())
	assert.NotEmpty(t, f.ID())
}

func TestFieldSignalDelegation(t *testing.T) {
	f := newTestField(42)
	assert.Equal(t, "42", f.String())
	assert.Equal(t, 42, f.Int())

	f.SetValue("new")
	assert.Equal(t, "new", f.String())

	// Bind returns an h.H element
	assert.NotNil(t, f.Bind())
}

func TestFieldValidateSingleRule(t *testing.T) {
	f := newTestField("", Required())
	assert.False(t, f.Validate())
	assert.True(t, f.HasError())
	assert.Equal(t, "This field is required", f.FirstError())

	f.SetValue("ok")
	assert.True(t, f.Validate())
	assert.False(t, f.HasError())
	assert.Equal(t, "", f.FirstError())
}

func TestFieldValidateMultipleRules(t *testing.T) {
	f := newTestField("ab", Required(), MinLen(3))
	assert.False(t, f.Validate())
	errs := f.Errors()
	assert.Len(t, errs, 1)
	assert.Equal(t, "Must be at least 3 characters", errs[0])

	f.SetValue("")
	assert.False(t, f.Validate())
	errs = f.Errors()
	assert.Len(t, errs, 2)
}

func TestFieldErrors(t *testing.T) {
	f := newTestField("")
	assert.Nil(t, f.Errors())
	assert.False(t, f.HasError())
	assert.Equal(t, "", f.FirstError())
}

func TestFieldAddError(t *testing.T) {
	f := newTestField("ok")
	f.AddError("username taken")
	assert.True(t, f.HasError())
	assert.Equal(t, "username taken", f.FirstError())
	assert.Len(t, f.Errors(), 1)
}

func TestFieldClearErrors(t *testing.T) {
	f := newTestField("", Required())
	f.Validate()
	assert.True(t, f.HasError())
	f.ClearErrors()
	assert.False(t, f.HasError())
}

func TestFieldReset(t *testing.T) {
	f := newTestField("initial", Required(), MinLen(3))
	f.SetValue("changed")
	f.AddError("some error")

	f.Reset()
	assert.Equal(t, "initial", f.String())
	assert.False(t, f.HasError())
}

func TestValidateAll(t *testing.T) {
	v := New()
	var username, email *Field
	v.Page("/", func(c *Context) {
		username = c.Field("", Required(), MinLen(3))
		email = c.Field("", Required(), Email())
		c.View(func() h.H { return h.Div() })
	})

	// both empty → both fail
	assert.False(t, false) // sanity
	ok := username.Validate() && email.Validate()
	assert.False(t, ok)

	// simulate ValidateAll via context
	v2 := New()
	var u2, e2 *Field
	v2.Page("/", func(c *Context) {
		u2 = c.Field("joe", Required(), MinLen(3))
		e2 = c.Field("joe@x.com", Required(), Email())
		c.View(func() h.H { return h.Div() })

		assert.True(t, c.ValidateAll(u2, e2))
	})
}

func TestValidateAllPartialFailure(t *testing.T) {
	v := New()
	v.Page("/", func(c *Context) {
		good := c.Field("valid", Required())
		bad := c.Field("", Required())
		c.View(func() h.H { return h.Div() })

		// ValidateAll must run ALL fields even if first passes
		ok := c.ValidateAll(good, bad)
		assert.False(t, ok)
		assert.False(t, good.HasError())
		assert.True(t, bad.HasError())
	})
}

func TestResetFields(t *testing.T) {
	v := New()
	v.Page("/", func(c *Context) {
		a := c.Field("a", Required())
		b := c.Field("b", Required())
		c.View(func() h.H { return h.Div() })

		a.SetValue("changed-a")
		b.SetValue("changed-b")
		a.AddError("err")

		c.ResetFields(a, b)
		assert.Equal(t, "a", a.String())
		assert.Equal(t, "b", b.String())
		assert.False(t, a.HasError())
	})
}

func TestFieldValidateClearsPreviousErrors(t *testing.T) {
	f := newTestField("", Required())
	f.Validate()
	assert.True(t, f.HasError())

	f.SetValue("ok")
	f.Validate()
	assert.False(t, f.HasError())
}

func TestFieldCustomValidator(t *testing.T) {
	f := newTestField("bad", Custom(func(val string) error {
		if val == "bad" {
			return fmt.Errorf("no bad words")
		}
		return nil
	}))
	assert.False(t, f.Validate())
	assert.Equal(t, "no bad words", f.FirstError())

	f.SetValue("good")
	assert.True(t, f.Validate())
}
