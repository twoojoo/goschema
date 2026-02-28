package schema

import (
	"fmt"
	"strings"
)

// ValidationError represents a single field-level validation failure.
type ValidationError struct {
	Field   string // JSON field path (e.g. "address.street")
	Message string // Human-readable reason
	Value   any    // The value that failed validation
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("field %q: %s (got %v)", e.Field, e.Message, e.Value)
}

// ValidationErrors is a collection of ValidationError returned when one or
// more fields fail validation. It implements the error interface.
type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	msgs := make([]string, len(ve))
	for i, e := range ve {
		msgs[i] = e.Error()
	}
	return strings.Join(msgs, "; ")
}

// Has returns true if there is at least one validation error for the given
// JSON field path.
func (ve ValidationErrors) Has(field string) bool {
	for _, e := range ve {
		if e.Field == field {
			return true
		}
	}
	return false
}
