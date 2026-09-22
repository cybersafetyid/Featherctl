// Package validator holds the input validation helpers shared by every
// feature.
package validator

import (
	"fmt"
	"strings"
)

// FieldError describes one field that failed validation.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Errors is everything that failed validation in one request.
type Errors []FieldError

// Error implements the error interface.
func (e Errors) Error() string {
	parts := make([]string, 0, len(e))
	for _, field := range e {
		parts = append(parts, field.Field+" "+field.Message)
	}
	return strings.Join(parts, ", ")
}

// Checker accumulates validation failures for a single request.
type Checker struct {
	errs Errors
}

// New returns an empty Checker.
func New() *Checker {
	return &Checker{}
}

// Required records a failure when value is blank.
func (c *Checker) Required(field, value string) {
	if strings.TrimSpace(value) == "" {
		c.add(field, "must not be empty")
	}
}

// MinInt records a failure when value is smaller than min.
func (c *Checker) MinInt(field string, value, min int) {
	if value < min {
		c.add(field, fmt.Sprintf("must be at least %d", min))
	}
}

// Err returns every recorded failure, or nil when the input was valid.
func (c *Checker) Err() error {
	if len(c.errs) == 0 {
		return nil
	}
	return c.errs
}

// add records one failure.
func (c *Checker) add(field, message string) {
	c.errs = append(c.errs, FieldError{Field: field, Message: message})
}
