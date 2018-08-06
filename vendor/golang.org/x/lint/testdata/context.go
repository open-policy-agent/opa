// Test that context.Context is the first arg to a function.

// Package foo ...
package foo

import (
	"context"
)

// A proper context.Context location
func x(ctx context.Context) { // ok
}

// A proper context.Context location
func x(ctx context.Context, s string) { // ok
}

// An invalid context.Context location
func y(s string, ctx context.Context) { // MATCH /context.Context should be the first parameter.*/
}

// An invalid context.Context location with more than 2 args
func y(s string, r int, ctx context.Context, x int) { // MATCH /context.Context should be the first parameter.*/
}
