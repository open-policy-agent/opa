// Package contextkeytypes verifies that correct types are used as keys in
// calls to context.WithValue.
package contextkeytypes

import (
	"context"
	"fmt"
)

type ctxKey struct{}

func contextKeyTypeTests() {
	fmt.Println()                               // not in package context
	context.TODO()                              // wrong function
	c := context.Background()                   // wrong function
	context.WithValue(c, "foo", "bar")          // MATCH /should not use basic type( untyped|)? string as key in context.WithValue/
	context.WithValue(c, true, "bar")           // MATCH /should not use basic type( untyped|)? bool as key in context.WithValue/
	context.WithValue(c, 1, "bar")              // MATCH /should not use basic type( untyped|)? int as key in context.WithValue/
	context.WithValue(c, int8(1), "bar")        // MATCH /should not use basic type int8 as key in context.WithValue/
	context.WithValue(c, int16(1), "bar")       // MATCH /should not use basic type int16 as key in context.WithValue/
	context.WithValue(c, int32(1), "bar")       // MATCH /should not use basic type int32 as key in context.WithValue/
	context.WithValue(c, rune(1), "bar")        // MATCH /should not use basic type rune as key in context.WithValue/
	context.WithValue(c, int64(1), "bar")       // MATCH /should not use basic type int64 as key in context.WithValue/
	context.WithValue(c, uint(1), "bar")        // MATCH /should not use basic type uint as key in context.WithValue/
	context.WithValue(c, uint8(1), "bar")       // MATCH /should not use basic type uint8 as key in context.WithValue/
	context.WithValue(c, byte(1), "bar")        // MATCH /should not use basic type byte as key in context.WithValue/
	context.WithValue(c, uint16(1), "bar")      // MATCH /should not use basic type uint16 as key in context.WithValue/
	context.WithValue(c, uint32(1), "bar")      // MATCH /should not use basic type uint32 as key in context.WithValue/
	context.WithValue(c, uint64(1), "bar")      // MATCH /should not use basic type uint64 as key in context.WithValue/
	context.WithValue(c, uintptr(1), "bar")     // MATCH /should not use basic type uintptr as key in context.WithValue/
	context.WithValue(c, float32(1.0), "bar")   // MATCH /should not use basic type float32 as key in context.WithValue/
	context.WithValue(c, float64(1.0), "bar")   // MATCH /should not use basic type float64 as key in context.WithValue/
	context.WithValue(c, complex64(1i), "bar")  // MATCH /should not use basic type complex64 as key in context.WithValue/
	context.WithValue(c, complex128(1i), "bar") // MATCH /should not use basic type complex128 as key in context.WithValue/
	context.WithValue(c, ctxKey{}, "bar")       // ok
	context.WithValue(c, &ctxKey{}, "bar")      // ok
	context.WithValue(c, invalid{}, "bar")      // ok
}
