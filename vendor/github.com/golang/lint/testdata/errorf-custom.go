// Test for allowed errors.New(fmt.Sprintf()) when a custom errors package is imported.

// Package foo ...
package foo

import (
	"fmt"

	"github.com/pkg/errors"
)

func f(x int) error {
	if x > 10 {
		return errors.New(fmt.Sprintf("something %d", x)) // OK
	}
	if x > 5 {
		return errors.New(g("blah")) // OK
	}
	if x > 4 {
		return errors.New("something else") // OK
	}
	return nil
}

func g(s string) string { return "prefix: " + s }
