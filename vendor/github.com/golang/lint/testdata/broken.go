// Test of code that is malformed, but accepted by go/parser.
// See https://golang.org/issue/11271 for discussion.
// OK

// Package pkg ...
package pkg

// Foo is a method with a missing receiver.
func () Foo() {}
