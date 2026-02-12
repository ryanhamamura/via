package via

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequired(t *testing.T) {
	r := Required()
	assert.NoError(t, r.validate("hello"))
	assert.Error(t, r.validate(""))
	assert.Error(t, r.validate("   "))
}

func TestRequiredCustomMessage(t *testing.T) {
	r := Required("name needed")
	err := r.validate("")
	assert.EqualError(t, err, "name needed")
}

func TestMinLen(t *testing.T) {
	r := MinLen(3)
	assert.NoError(t, r.validate("abc"))
	assert.NoError(t, r.validate("abcd"))
	assert.Error(t, r.validate("ab"))
	assert.Error(t, r.validate(""))
}

func TestMinLenCustomMessage(t *testing.T) {
	r := MinLen(5, "too short")
	err := r.validate("ab")
	assert.EqualError(t, err, "too short")
}

func TestMaxLen(t *testing.T) {
	r := MaxLen(5)
	assert.NoError(t, r.validate("abc"))
	assert.NoError(t, r.validate("abcde"))
	assert.Error(t, r.validate("abcdef"))
}

func TestMaxLenCustomMessage(t *testing.T) {
	r := MaxLen(2, "too long")
	err := r.validate("abc")
	assert.EqualError(t, err, "too long")
}

func TestMin(t *testing.T) {
	r := Min(5)
	assert.NoError(t, r.validate("5"))
	assert.NoError(t, r.validate("10"))
	assert.Error(t, r.validate("4"))
	assert.Error(t, r.validate("abc"))
}

func TestMinCustomMessage(t *testing.T) {
	r := Min(10, "need 10+")
	err := r.validate("3")
	assert.EqualError(t, err, "need 10+")
}

func TestMax(t *testing.T) {
	r := Max(10)
	assert.NoError(t, r.validate("10"))
	assert.NoError(t, r.validate("5"))
	assert.Error(t, r.validate("11"))
	assert.Error(t, r.validate("abc"))
}

func TestMaxCustomMessage(t *testing.T) {
	r := Max(5, "too big")
	err := r.validate("6")
	assert.EqualError(t, err, "too big")
}

func TestPattern(t *testing.T) {
	r := Pattern(`^\d{3}$`)
	assert.NoError(t, r.validate("123"))
	assert.Error(t, r.validate("12"))
	assert.Error(t, r.validate("abcd"))
}

func TestPatternCustomMessage(t *testing.T) {
	r := Pattern(`^\d+$`, "digits only")
	err := r.validate("abc")
	assert.EqualError(t, err, "digits only")
}

func TestEmail(t *testing.T) {
	r := Email()
	assert.NoError(t, r.validate("user@example.com"))
	assert.NoError(t, r.validate("a.b+c@foo.co"))
	assert.Error(t, r.validate("notanemail"))
	assert.Error(t, r.validate("@example.com"))
	assert.Error(t, r.validate("user@"))
	assert.Error(t, r.validate(""))
}

func TestEmailCustomMessage(t *testing.T) {
	r := Email("bad email")
	err := r.validate("nope")
	assert.EqualError(t, err, "bad email")
}

func TestCustom(t *testing.T) {
	r := Custom(func(val string) error {
		if val != "magic" {
			return fmt.Errorf("must be magic")
		}
		return nil
	})
	assert.NoError(t, r.validate("magic"))
	assert.EqualError(t, r.validate("other"), "must be magic")
}
