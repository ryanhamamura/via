package via

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ryanhamamura/via/h"
)

// Signal represents a value that is reactive in the browser. Signals
// are synced with the server right before an action triggers.
//
// Use Bind() to connect a signal to an input and Text() to display it
// reactively on an html element.
type Signal struct {
	id      string
	val     any
	changed bool
	err     error
}

// ID returns the signal ID.
func (s *Signal) ID() string {
	return s.id
}

// Err returns a signal error or nil if it contains no error.
//
// It is useful to check for errors after updating signals with
// dynamic values.
func (s *Signal) Err() error {
	return s.err
}

// Bind binds this signal to an input element. When the input changes
// its value the signal updates in real-time in the browser.
//
// Example:
//
//	h.Input(h.Type("number"), mysignal.Bind())
func (s *Signal) Bind() h.H {
	return h.Data("bind", s.id)
}

// Text binds the signal value to an html span element as text.
//
// Example:
//
//	h.Div(mysignal.Text())
func (s *Signal) Text() h.H {
	return h.Span(h.Data("text", "$"+s.id))
}

// SetValue updates the signal’s value and marks it for synchronization with the browser.
// The change will be propagated to the browser using *Context.Sync() or *Context.SyncSignals().
func (s *Signal) SetValue(v any) {
	s.val = v
	s.changed = true
	s.err = nil
}

// String returns the signal value as a string.
func (s *Signal) String() string {
	return fmt.Sprintf("%v", s.val)
}

// Bool tries to read the signal value as a bool.
// Returns the value or false on failure.
func (s *Signal) Bool() bool {
	val := strings.ToLower(s.String())
	return val == "true" || val == "1" || val == "yes" || val == "on"
}

// Int tries to read the signal value as an int.
// Returns the value or 0 on failure.
func (s *Signal) Int() int {
	if n, err := strconv.Atoi(s.String()); err == nil {
		return n
	}
	return 0
}

