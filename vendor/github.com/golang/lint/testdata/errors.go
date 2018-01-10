// Test for naming errors.

// Package foo ...
package foo

import (
	"errors"
	"fmt"
)

var unexp = errors.New("some unexported error") // MATCH /error var.*unexp.*errFoo/

// Exp ...
var Exp = errors.New("some exported error") // MATCH /error var.*Exp.*ErrFoo/

var (
	e1 = fmt.Errorf("blah %d", 4) // MATCH /error var.*e1.*errFoo/
	// E2 ...
	E2 = fmt.Errorf("blah %d", 5) // MATCH /error var.*E2.*ErrFoo/
)

func f() {
	var whatever = errors.New("ok") // ok
	_ = whatever
}

// Check for the error strings themselves.

func g(x int) error {
	var err error
	err = fmt.Errorf("This %d is too low", x)     // MATCH /error strings.*be capitalized/
	err = fmt.Errorf("XML time")                  // ok
	err = fmt.Errorf("newlines are fun\n")        // MATCH /error strings.*end with punctuation/
	err = fmt.Errorf("Newlines are really fun\n") // MATCH /error strings.+not be capitalized/
	err = errors.New(`too much stuff.`)           // MATCH /error strings.*end with punctuation/
	err = errors.New("This %d is too low", x)     // MATCH /error strings.*be capitalized/
	return err
}
