package via

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ryanhamamura/via/h"
)

// computedSignal is a read-only signal whose value is derived from other signals.
// It recomputes on every read and is included in patches only when the value changes.
type computedSignal struct {
	id      string
	compute func() string
	lastVal string
	changed bool
}

func (s *computedSignal) ID() string {
	return s.id
}

func (s *computedSignal) String() string {
	return s.compute()
}

func (s *computedSignal) Int() int {
	if n, err := strconv.Atoi(s.String()); err == nil {
		return n
	}
	return 0
}

func (s *computedSignal) Bool() bool {
	val := strings.ToLower(s.String())
	return val == "true" || val == "1" || val == "yes" || val == "on"
}

func (s *computedSignal) Text() h.H {
	return h.Span(h.Data("text", "$"+s.id))
}

// recompute calls the compute function and sets changed if the value differs from lastVal.
func (s *computedSignal) recompute() {
	val := s.compute()
	if val != s.lastVal {
		s.lastVal = val
		s.changed = true
	}
}

func (s *computedSignal) patchValue() string {
	return fmt.Sprintf("%v", s.lastVal)
}
