// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Errors represents a series of errors encountered during parsing, compiling,
// etc.
type Errors []*Error

func (e Errors) Error() string {

	if len(e) == 0 {
		return "no error(s)"
	}

	if len(e) == 1 {
		return fmt.Sprintf("1 error occurred: %v", e[0].Error())
	}

	s := make([]string, len(e))
	for i, err := range e {
		s[i] = err.Error()
	}

	return fmt.Sprintf("%d errors occurred:\n%s", len(e), strings.Join(s, "\n"))
}

// Sort sorts the error slice by location. If the locations are equal then the
// error message is compared.
func (e Errors) Sort() {
	slices.SortFunc(e, func(a, b *Error) int {
		if cmp := a.Location.Compare(b.Location); cmp != 0 {
			return cmp
		}

		return strings.Compare(a.Error(), b.Error())
	})
}

const (
	// ParseErr indicates an unclassified parse error occurred.
	ParseErr = "rego_parse_error"

	// CompileErr indicates an unclassified compile error occurred.
	CompileErr = "rego_compile_error"

	// TypeErr indicates a type error was caught.
	TypeErr = "rego_type_error"

	// UnsafeVarErr indicates an unsafe variable was found during compilation.
	UnsafeVarErr = "rego_unsafe_var_error"

	// RecursionErr indicates recursion was found during compilation.
	RecursionErr = "rego_recursion_error"

	// FormatErr indicates an error occurred during formatting.
	FormatErr = "rego_format_error"
)

// IsError returns true if err is an AST error with code.
func IsError(code string, err error) bool {
	if err, ok := err.(*Error); ok {
		return err.Code == code
	}
	return false
}

// ErrorDetails defines the interface for detailed error messages.
type ErrorDetails interface {
	Lines() []string
}

// Error represents a single error caught during parsing, compiling, etc.
type Error struct {
	Code     string       `json:"code"`
	Message  string       `json:"message"`
	Location *Location    `json:"location,omitempty"`
	Details  ErrorDetails `json:"details,omitempty"`
}

func (e *Error) Error() string {
	var prefix string

	if e.Location != nil {

		if len(e.Location.File) > 0 {
			prefix += e.Location.File + ":" + strconv.Itoa(e.Location.Row)
		} else {
			prefix += strconv.Itoa(e.Location.Row) + ":" + strconv.Itoa(e.Location.Col)
		}
	}

	sb := strings.Builder{}
	if len(prefix) > 0 {
		sb.WriteString(prefix)
		sb.WriteString(": ")
	}

	sb.WriteString(e.Code)
	sb.WriteString(": ")
	sb.WriteString(e.Message)

	if e.Details != nil {
		for _, line := range e.Details.Lines() {
			sb.WriteString("\n\t")
			sb.WriteString(line)
		}
	}

	return sb.String()
}

func (e *Error) Equal(other *Error) bool {
	if e == other {
		return true
	}

	if e == nil || other == nil {
		return false
	}

	if e.Code != other.Code || e.Message != other.Message {
		return false
	}

	if !e.Location.Equal(other.Location) {
		return false
	}

	if (e.Details == nil) != (other.Details == nil) {
		return false
	}

	if e.Details != nil && !slices.Equal(e.Details.Lines(), other.Details.Lines()) {
		return false
	}

	return true
}

// NewError returns a new Error object.
func NewError(code string, loc *Location, f string, a ...any) *Error {
	return newErrorString(code, loc, fmt.Sprintf(f, a...))
}

func newErrorString(code string, loc *Location, m string) *Error {
	return &Error{
		Code:     code,
		Location: loc,
		Message:  m,
	}
}
