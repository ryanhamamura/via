package via

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLimiter_Defaults(t *testing.T) {
	l := newLimiter(RateLimitConfig{}, defaultActionRate, defaultActionBurst)
	require.NotNil(t, l)
	assert.InDelta(t, defaultActionRate, float64(l.Limit()), 0.001)
	assert.Equal(t, defaultActionBurst, l.Burst())
}

func TestNewLimiter_CustomValues(t *testing.T) {
	l := newLimiter(RateLimitConfig{Rate: 5, Burst: 10}, defaultActionRate, defaultActionBurst)
	require.NotNil(t, l)
	assert.InDelta(t, 5.0, float64(l.Limit()), 0.001)
	assert.Equal(t, 10, l.Burst())
}

func TestNewLimiter_DisabledWithNegativeRate(t *testing.T) {
	l := newLimiter(RateLimitConfig{Rate: -1}, defaultActionRate, defaultActionBurst)
	assert.Nil(t, l)
}

func TestTokenBucket_AllowsBurstThenRejects(t *testing.T) {
	l := newLimiter(RateLimitConfig{Rate: 1, Burst: 3}, 1, 3)
	require.NotNil(t, l)

	for i := 0; i < 3; i++ {
		assert.True(t, l.Allow(), "request %d should be allowed within burst", i)
	}
	assert.False(t, l.Allow(), "request beyond burst should be rejected")
}

func TestWithRateLimit_CreatesLimiter(t *testing.T) {
	entry := actionEntry{fn: func() {}}
	opt := WithRateLimit(2, 4)
	opt(&entry)

	require.NotNil(t, entry.limiter)
	assert.InDelta(t, 2.0, float64(entry.limiter.Limit()), 0.001)
	assert.Equal(t, 4, entry.limiter.Burst())
}

func TestContextAction_WithRateLimit(t *testing.T) {
	v := New()
	c := newContext("test-rl", "/", v)

	called := false
	c.Action(func() { called = true }, WithRateLimit(1, 2))

	// Verify the entry has its own limiter
	for _, entry := range c.actionRegistry {
		require.NotNil(t, entry.limiter)
		assert.InDelta(t, 1.0, float64(entry.limiter.Limit()), 0.001)
		assert.Equal(t, 2, entry.limiter.Burst())
	}
	assert.False(t, called)
}

func TestContextAction_DefaultNoPerActionLimiter(t *testing.T) {
	v := New()
	c := newContext("test-no-rl", "/", v)

	c.Action(func() {})

	for _, entry := range c.actionRegistry {
		assert.Nil(t, entry.limiter, "entry without WithRateLimit should have nil limiter")
	}
}

func TestContextLimiter_DefaultsApplied(t *testing.T) {
	v := New()
	c := newContext("test-ctx-limiter", "/", v)

	require.NotNil(t, c.actionLimiter)
	assert.InDelta(t, defaultActionRate, float64(c.actionLimiter.Limit()), 0.001)
	assert.Equal(t, defaultActionBurst, c.actionLimiter.Burst())
}

func TestContextLimiter_DisabledViaConfig(t *testing.T) {
	v := New()
	v.actionRateLimit = RateLimitConfig{Rate: -1}
	c := newContext("test-disabled", "/", v)

	assert.Nil(t, c.actionLimiter)
}

func TestContextLimiter_CustomConfig(t *testing.T) {
	v := New()
	v.Config(Options{ActionRateLimit: RateLimitConfig{Rate: 50, Burst: 100}})
	c := newContext("test-custom", "/", v)

	require.NotNil(t, c.actionLimiter)
	assert.InDelta(t, 50.0, float64(c.actionLimiter.Limit()), 0.001)
	assert.Equal(t, 100, c.actionLimiter.Burst())
}
