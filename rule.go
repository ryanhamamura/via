package via

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Rule defines a single validation check for a Field.
type Rule struct {
	validate func(val string) error
}

// Required rejects empty or whitespace-only values.
func Required(msg ...string) Rule {
	m := "This field is required"
	if len(msg) > 0 {
		m = msg[0]
	}
	return Rule{func(val string) error {
		if strings.TrimSpace(val) == "" {
			return errors.New(m)
		}
		return nil
	}}
}

// MinLen rejects values shorter than n characters.
func MinLen(n int, msg ...string) Rule {
	m := fmt.Sprintf("Must be at least %d characters", n)
	if len(msg) > 0 {
		m = msg[0]
	}
	return Rule{func(val string) error {
		if utf8.RuneCountInString(val) < n {
			return errors.New(m)
		}
		return nil
	}}
}

// MaxLen rejects values longer than n characters.
func MaxLen(n int, msg ...string) Rule {
	m := fmt.Sprintf("Must be at most %d characters", n)
	if len(msg) > 0 {
		m = msg[0]
	}
	return Rule{func(val string) error {
		if utf8.RuneCountInString(val) > n {
			return errors.New(m)
		}
		return nil
	}}
}

// Min parses the value as an integer and rejects values less than n.
func Min(n int, msg ...string) Rule {
	m := fmt.Sprintf("Must be at least %d", n)
	if len(msg) > 0 {
		m = msg[0]
	}
	return Rule{func(val string) error {
		v, err := strconv.Atoi(val)
		if err != nil {
			return errors.New("Must be a valid number")
		}
		if v < n {
			return errors.New(m)
		}
		return nil
	}}
}

// Max parses the value as an integer and rejects values greater than n.
func Max(n int, msg ...string) Rule {
	m := fmt.Sprintf("Must be at most %d", n)
	if len(msg) > 0 {
		m = msg[0]
	}
	return Rule{func(val string) error {
		v, err := strconv.Atoi(val)
		if err != nil {
			return errors.New("Must be a valid number")
		}
		if v > n {
			return errors.New(m)
		}
		return nil
	}}
}

// Pattern rejects values that don't match the regular expression re.
func Pattern(re string, msg ...string) Rule {
	m := "Invalid format"
	if len(msg) > 0 {
		m = msg[0]
	}
	compiled := regexp.MustCompile(re)
	return Rule{func(val string) error {
		if !compiled.MatchString(val) {
			return errors.New(m)
		}
		return nil
	}}
}

var emailRegexp = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// Email rejects values that don't look like an email address.
func Email(msg ...string) Rule {
	m := "Invalid email address"
	if len(msg) > 0 {
		m = msg[0]
	}
	return Rule{func(val string) error {
		if !emailRegexp.MatchString(val) {
			return errors.New(m)
		}
		return nil
	}}
}

// Custom creates a rule from a user-provided validation function.
// The function should return nil for valid input and an error for invalid input.
func Custom(fn func(string) error) Rule {
	return Rule{validate: fn}
}
