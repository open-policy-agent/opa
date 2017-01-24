// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
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

	s := []string{}
	for _, err := range e {
		s = append(s, err.Error())
	}

	return fmt.Sprintf("%d errors occurred:\n%s", len(e), strings.Join(s, "\n"))
}

// ErrCode defines the types of errors returned during parsing, compiling, etc.
type ErrCode int

const (
	// ParseErr indicates an unclassified parse error occurred.
	ParseErr = iota

	// CompileErr indicates an unclassified compile error occurred.
	CompileErr = iota

	// UnsafeVarErr indicates an unsafe variable was found during compilation.
	UnsafeVarErr = iota

	// RecursionErr indicates recursion was found during compilation.
	RecursionErr = iota

	// MissingInputErr indicates the query depends on input but no input
	// document was provided.
	MissingInputErr = iota
)

// IsError returns true if err is an AST error with code.
func IsError(code ErrCode, err error) bool {
	if err, ok := err.(*Error); ok {
		return err.Code == code
	}
	return false
}

// Error represents a single error caught during parsing, compiling, etc.
type Error struct {
	Code     ErrCode   `json:"code"`
	Location *Location `json:"location"`
	Message  string    `json:"message"`
}

func (e *Error) Error() string {
	if e.Location == nil {
		return e.Message
	}

	prefix := ""

	if len(e.Location.File) > 0 {
		prefix += e.Location.File + ":" + fmt.Sprint(e.Location.Row)
	} else {
		prefix += fmt.Sprint(e.Location.Row) + ":" + fmt.Sprint(e.Location.Col)
	}

	return fmt.Sprintf("%v: %v", prefix, e.Message)
}

// NewError returns a new Error object.
func NewError(code ErrCode, loc *Location, f string, a ...interface{}) *Error {
	return &Error{
		Code:     code,
		Location: loc,
		Message:  fmt.Sprintf(f, a...),
	}
}
