package via

import (
	"context"
	"time"

	"github.com/alexedwards/scs/v2"
)

// Session provides access to the user's session data.
// Session data persists across page views for the same browser.
type Session struct {
	ctx     context.Context
	manager *scs.SessionManager
}

// Get retrieves a value from the session.
func (s *Session) Get(key string) any {
	if s.manager == nil || s.ctx == nil {
		return nil
	}
	return s.manager.Get(s.ctx, key)
}

// GetString retrieves a string value from the session.
func (s *Session) GetString(key string) string {
	if s.manager == nil || s.ctx == nil {
		return ""
	}
	return s.manager.GetString(s.ctx, key)
}

// GetInt retrieves an int value from the session.
func (s *Session) GetInt(key string) int {
	if s.manager == nil || s.ctx == nil {
		return 0
	}
	return s.manager.GetInt(s.ctx, key)
}

// GetBool retrieves a bool value from the session.
func (s *Session) GetBool(key string) bool {
	if s.manager == nil || s.ctx == nil {
		return false
	}
	return s.manager.GetBool(s.ctx, key)
}

// Set stores a value in the session.
func (s *Session) Set(key string, val any) {
	if s.manager == nil || s.ctx == nil {
		return
	}
	s.manager.Put(s.ctx, key, val)
}

// Delete removes a value from the session.
func (s *Session) Delete(key string) {
	if s.manager == nil || s.ctx == nil {
		return
	}
	s.manager.Remove(s.ctx, key)
}

// Clear removes all data from the session.
func (s *Session) Clear() error {
	if s.manager == nil || s.ctx == nil {
		return nil
	}
	return s.manager.Clear(s.ctx)
}

// Destroy destroys the session entirely (use for logout).
func (s *Session) Destroy() error {
	if s.manager == nil || s.ctx == nil {
		return nil
	}
	return s.manager.Destroy(s.ctx)
}

// RenewToken regenerates the session token (use after login to prevent session fixation).
func (s *Session) RenewToken() error {
	if s.manager == nil || s.ctx == nil {
		return nil
	}
	return s.manager.RenewToken(s.ctx)
}

// Exists returns true if the key exists in the session.
func (s *Session) Exists(key string) bool {
	if s.manager == nil || s.ctx == nil {
		return false
	}
	return s.manager.Exists(s.ctx, key)
}

// Keys returns all keys in the session.
func (s *Session) Keys() []string {
	if s.manager == nil || s.ctx == nil {
		return nil
	}
	return s.manager.Keys(s.ctx)
}

// ID returns the session token (cookie value).
func (s *Session) ID() string {
	if s.manager == nil || s.ctx == nil {
		return ""
	}
	return s.manager.Token(s.ctx)
}

// Pop retrieves a value and deletes it from the session (flash message pattern).
func (s *Session) Pop(key string) any {
	if s.manager == nil || s.ctx == nil {
		return nil
	}
	return s.manager.Pop(s.ctx, key)
}

// PopString retrieves a string value and deletes it from the session.
func (s *Session) PopString(key string) string {
	if s.manager == nil || s.ctx == nil {
		return ""
	}
	return s.manager.PopString(s.ctx, key)
}

// PopInt retrieves an int value and deletes it from the session.
func (s *Session) PopInt(key string) int {
	if s.manager == nil || s.ctx == nil {
		return 0
	}
	return s.manager.PopInt(s.ctx, key)
}

// PopBool retrieves a bool value and deletes it from the session.
func (s *Session) PopBool(key string) bool {
	if s.manager == nil || s.ctx == nil {
		return false
	}
	return s.manager.PopBool(s.ctx, key)
}

// GetFloat64 retrieves a float64 value from the session.
func (s *Session) GetFloat64(key string) float64 {
	if s.manager == nil || s.ctx == nil {
		return 0
	}
	return s.manager.GetFloat(s.ctx, key)
}

// PopFloat64 retrieves a float64 value and deletes it from the session.
func (s *Session) PopFloat64(key string) float64 {
	if s.manager == nil || s.ctx == nil {
		return 0
	}
	return s.manager.PopFloat(s.ctx, key)
}

// GetTime retrieves a time.Time value from the session.
func (s *Session) GetTime(key string) time.Time {
	if s.manager == nil || s.ctx == nil {
		return time.Time{}
	}
	return s.manager.GetTime(s.ctx, key)
}

// PopTime retrieves a time.Time value and deletes it from the session.
func (s *Session) PopTime(key string) time.Time {
	if s.manager == nil || s.ctx == nil {
		return time.Time{}
	}
	return s.manager.PopTime(s.ctx, key)
}

// GetBytes retrieves a []byte value from the session.
func (s *Session) GetBytes(key string) []byte {
	if s.manager == nil || s.ctx == nil {
		return nil
	}
	return s.manager.GetBytes(s.ctx, key)
}

// PopBytes retrieves a []byte value and deletes it from the session.
func (s *Session) PopBytes(key string) []byte {
	if s.manager == nil || s.ctx == nil {
		return nil
	}
	return s.manager.PopBytes(s.ctx, key)
}
