package via

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ryanhamamura/via/h"
)

// actionTrigger represents a trigger to an event handler fn
type actionTrigger struct {
	id string
}

// ActionTriggerOption configures behavior of action triggers
type ActionTriggerOption interface {
	apply(*triggerOpts)
}

type triggerOpts struct {
	hasSignal bool
	signalID  string
	value     string
	window    bool
}

type withWindowOpt struct{}

func (o withWindowOpt) apply(opts *triggerOpts) { opts.window = true }

// WithWindow scopes the event listener to the window instead of the element.
func WithWindow() ActionTriggerOption { return withWindowOpt{} }

type withSignalOpt struct {
	signalID string
	value    string
}

func (o withSignalOpt) apply(opts *triggerOpts) {
	opts.hasSignal = true
	opts.signalID = o.signalID
	opts.value = o.value
}

// WithSignal sets a signal value before triggering the action.
func WithSignal(sig *signal, value string) ActionTriggerOption {
	return withSignalOpt{
		signalID: sig.ID(),
		value:    fmt.Sprintf("'%s'", value),
	}
}

// WithSignalInt sets a signal to an int value before triggering the action.
func WithSignalInt(sig *signal, value int) ActionTriggerOption {
	return withSignalOpt{
		signalID: sig.ID(),
		value:    strconv.Itoa(value),
	}
}

func buildOnExpr(base string, opts *triggerOpts) string {
	if !opts.hasSignal {
		return base
	}
	return fmt.Sprintf("$%s=%s;%s", opts.signalID, opts.value, base)
}

func applyOptions(options ...ActionTriggerOption) triggerOpts {
	var opts triggerOpts
	for _, opt := range options {
		opt.apply(&opts)
	}
	return opts
}

func actionURL(id string) string {
	return fmt.Sprintf("@get('/_action/%s')", id)
}

// OnClick returns a via.h DOM attribute that triggers on click. It can be added
// to element nodes in a view.
func (a *actionTrigger) OnClick(options ...ActionTriggerOption) h.H {
	opts := applyOptions(options...)
	return h.Data("on:click", buildOnExpr(actionURL(a.id), &opts))
}

// OnChange returns a via.h DOM attribute that triggers on input change. It can be added
// to element nodes in a view.
func (a *actionTrigger) OnChange(options ...ActionTriggerOption) h.H {
	opts := applyOptions(options...)
	return h.Data("on:change__debounce.200ms", buildOnExpr(actionURL(a.id), &opts))
}

// OnKeyDown returns a via.h DOM attribute that triggers when a key is pressed.
// key: optional, see https://developer.mozilla.org/en-US/docs/Web/API/KeyboardEvent/key
// Example: OnKeyDown("Enter")
func (a *actionTrigger) OnKeyDown(key string, options ...ActionTriggerOption) h.H {
	opts := applyOptions(options...)
	var condition string
	if key != "" {
		condition = fmt.Sprintf("evt.key==='%s' &&", key)
	}
	attrName := "on:keydown"
	if opts.window {
		attrName = "on:keydown__window"
	}
	return h.Data(attrName, fmt.Sprintf("%s%s", condition, buildOnExpr(actionURL(a.id), &opts)))
}

// KeyBinding pairs a key name with action trigger options for use with OnKeyDownMap.
type KeyBinding struct {
	Key     string
	Options []ActionTriggerOption
}

// KeyBind creates a KeyBinding for use with OnKeyDownMap.
func KeyBind(key string, options ...ActionTriggerOption) KeyBinding {
	return KeyBinding{Key: key, Options: options}
}

// OnKeyDownMap combines multiple key bindings into a single data-on:keydown__window
// attribute using a JS ternary chain. This avoids HTML attribute deduplication issues
// that occur when multiple OnKeyDown calls target the same element.
func (a *actionTrigger) OnKeyDownMap(bindings ...KeyBinding) h.H {
	var parts []string
	for _, b := range bindings {
		opts := applyOptions(b.Options...)
		expr := buildOnExpr(actionURL(a.id), &opts)
		parts = append(parts, fmt.Sprintf("evt.key==='%s' ? (%s)", b.Key, expr))
	}
	combined := strings.Join(parts, " : ") + " : void 0"
	return h.Data("on:keydown__window", combined)
}
