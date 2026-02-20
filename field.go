package via

// Field is a signal with built-in validation rules and error state.
// It embeds *Signal, so all signal methods (Bind, String, Int, Bool, SetValue, Text, ID)
// work transparently.
type Field struct {
	*Signal
	rules      []Rule
	errors     []string
	initialVal any
}

// Validate runs all rules against the current value.
// Clears previous errors, populates new ones, returns true if all rules pass.
func (f *Field) Validate() bool {
	f.errors = nil
	val := f.String()
	for _, r := range f.rules {
		if err := r.validate(val); err != nil {
			f.errors = append(f.errors, err.Error())
		}
	}
	return len(f.errors) == 0
}

// HasError returns true if this field has any validation errors.
func (f *Field) HasError() bool {
	return len(f.errors) > 0
}

// FirstError returns the first validation error message, or "" if valid.
func (f *Field) FirstError() string {
	if len(f.errors) > 0 {
		return f.errors[0]
	}
	return ""
}

// Errors returns all current validation error messages.
func (f *Field) Errors() []string {
	return f.errors
}

// AddError manually adds an error message (useful for server-side or cross-field validation).
func (f *Field) AddError(msg string) {
	f.errors = append(f.errors, msg)
}

// ClearErrors removes all validation errors from this field.
func (f *Field) ClearErrors() {
	f.errors = nil
}

// Reset restores the field value to its initial value and clears all errors.
func (f *Field) Reset() {
	f.SetValue(f.initialVal)
	f.errors = nil
}
