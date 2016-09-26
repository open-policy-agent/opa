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
type Errors []error

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
