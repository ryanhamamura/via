package via

import (
	"fmt"
	"strconv"

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
	hasSignal      bool
	signalID       string
	value          string
	window         bool
	preventDefault bool
}

type withSignalOpt struct {
	signalID string
	value    string
}

func (o withSignalOpt) apply(opts *triggerOpts) {
	opts.hasSignal = true
	opts.signalID = o.signalID
	opts.value = o.value
}

type withWindowOpt struct{}

func (o withWindowOpt) apply(opts *triggerOpts) {
	opts.window = true
}

// WithWindow makes the event listener attach to the window instead of the element.
func WithWindow() ActionTriggerOption {
	return withWindowOpt{}
}

type withPreventDefaultOpt struct{}

func (o withPreventDefaultOpt) apply(opts *triggerOpts) {
	opts.preventDefault = true
}

// WithPreventDefault calls evt.preventDefault() for matched keys.
func WithPreventDefault() ActionTriggerOption {
	return withPreventDefaultOpt{}
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
	return fmt.Sprintf("$%s=%s,%s", opts.signalID, opts.value, base)
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

// KeyBinding pairs a key with an action and per-binding options.
type KeyBinding struct {
	Key     string
	Action  *actionTrigger
	Options []ActionTriggerOption
}

// KeyBind creates a KeyBinding for use with OnKeyDownMap.
func KeyBind(key string, action *actionTrigger, options ...ActionTriggerOption) KeyBinding {
	return KeyBinding{Key: key, Action: action, Options: options}
}

// OnKeyDownMap produces a single window-scoped keydown attribute that dispatches
// to different actions based on the pressed key. Each binding can reference a
// different action and carry its own signal/preventDefault options.
func OnKeyDownMap(bindings ...KeyBinding) h.H {
	if len(bindings) == 0 {
		return nil
	}

	expr := ""
	for i, b := range bindings {
		opts := applyOptions(b.Options...)

		branch := ""
		if opts.preventDefault {
			branch = "evt.preventDefault(),"
		}
		branch += buildOnExpr(actionURL(b.Action.id), &opts)

		if i > 0 {
			expr += " : "
		}
		expr += fmt.Sprintf("evt.key==='%s' ? (%s)", b.Key, branch)
	}
	expr += " : void 0"

	return h.Data("on:keydown__window", expr)
}
