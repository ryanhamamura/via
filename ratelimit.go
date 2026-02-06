package via

import "golang.org/x/time/rate"

const (
	defaultActionRate  float64 = 10.0
	defaultActionBurst int     = 20
)

// RateLimitConfig configures token-bucket rate limiting for actions.
// Zero values fall back to defaults. Rate of -1 disables limiting entirely.
type RateLimitConfig struct {
	Rate  float64
	Burst int
}

// ActionOption configures per-action behaviour when passed to Context.Action.
type ActionOption func(*actionEntry)

type actionEntry struct {
	fn      func()
	limiter *rate.Limiter // nil = use context default
}

// WithRateLimit returns an ActionOption that gives this action its own
// token-bucket limiter, overriding the context-level default.
func WithRateLimit(r float64, burst int) ActionOption {
	return func(e *actionEntry) {
		e.limiter = newLimiter(RateLimitConfig{Rate: r, Burst: burst}, defaultActionRate, defaultActionBurst)
	}
}

// newLimiter creates a *rate.Limiter from cfg, substituting defaults for zero
// values. A Rate of -1 disables limiting (returns nil).
func newLimiter(cfg RateLimitConfig, defaultRate float64, defaultBurst int) *rate.Limiter {
	r := cfg.Rate
	b := cfg.Burst
	if r == -1 {
		return nil
	}
	if r == 0 {
		r = defaultRate
	}
	if b == 0 {
		b = defaultBurst
	}
	return rate.NewLimiter(rate.Limit(r), b)
}
