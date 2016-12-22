// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

// Builtins is the registry of built-in functions supported by OPA.
// Call RegisterBuiltin to add a new built-in.
var Builtins []*Builtin

// RegisterBuiltin adds a new built-in function to the registry.
func RegisterBuiltin(b *Builtin) {
	Builtins = append(Builtins, b)
	BuiltinMap[b.Name] = b
}

// DefaultBuiltins is the registry of built-in functions supported in OPA
// by default. When adding a new built-in function to OPA, update this
// list.
var DefaultBuiltins = [...]*Builtin{
	// =
	Equality,

	// Comparisons
	GreaterThan, GreaterThanEq, LessThan, LessThanEq, NotEqual,

	// Arithmetic
	Plus, Minus, Multiply, Divide, Round, Abs,

	// Aggregates
	Count, Sum, Max,

	// Casting
	ToNumber,

	// Regular Expressions
	RegexMatch,

	// Sets
	SetDiff,

	// Strings
	Concat, FormatInt, IndexOf, Substring, Lower, Upper, Contains, StartsWith, EndsWith,
}

// BuiltinMap provides a convenient mapping of built-in names to
// built-in definitions.
var BuiltinMap map[Var]*Builtin

/**
 * Unification
 */

// Equality represents the "=" operator.
var Equality = &Builtin{
	Name:      Var("eq"),
	Infix:     Var("="),
	NumArgs:   2,
	TargetPos: []int{0, 1},
}

/**
 * Comparisons
 */

// GreaterThan represents the ">" comparison operator.
var GreaterThan = &Builtin{
	Name:    Var("gt"),
	Infix:   Var(">"),
	NumArgs: 2,
}

// GreaterThanEq represents the ">=" comparison operator.
var GreaterThanEq = &Builtin{
	Name:    Var("gte"),
	Infix:   Var(">="),
	NumArgs: 2,
}

// LessThan represents the "<" comparison operator.
var LessThan = &Builtin{
	Name:    Var("lt"),
	Infix:   Var("<"),
	NumArgs: 2,
}

// LessThanEq represents the "<=" comparison operator.
var LessThanEq = &Builtin{
	Name:    Var("lte"),
	Infix:   Var("<="),
	NumArgs: 2,
}

// NotEqual represents the "!=" comparison operator.
var NotEqual = &Builtin{
	Name:    Var("neq"),
	Infix:   Var("!="),
	NumArgs: 2,
}

/**
 * Arithmetic
 */

// Plus adds two numbers together.
var Plus = &Builtin{
	Name:      Var("plus"),
	NumArgs:   3,
	TargetPos: []int{2},
}

// Minus subtracts the second number from the first number.
var Minus = &Builtin{
	Name:      Var("minus"),
	NumArgs:   3,
	TargetPos: []int{2},
}

// Multiply multiplies two numbers together.
var Multiply = &Builtin{
	Name:      Var("mul"),
	NumArgs:   3,
	TargetPos: []int{2},
}

// Divide divides the first number by the second number.
var Divide = &Builtin{
	Name:      Var("div"),
	NumArgs:   3,
	TargetPos: []int{2},
}

// Round rounds the number up to the nearest integer.
var Round = &Builtin{
	Name:      Var("round"),
	NumArgs:   2,
	TargetPos: []int{1},
}

// Abs returns the number without its sign.
var Abs = &Builtin{
	Name:      Var("abs"),
	NumArgs:   2,
	TargetPos: []int{1},
}

/**
 * Aggregates
 */

// Count takes a collection and counts the number of elements in it.
var Count = &Builtin{
	Name:      Var("count"),
	NumArgs:   2,
	TargetPos: []int{1},
}

// Sum takes an array of numbers and sums them.
var Sum = &Builtin{
	Name:      Var("sum"),
	NumArgs:   2,
	TargetPos: []int{1},
}

// Max returns the maximum value in a collection.
var Max = &Builtin{
	Name:      Var("max"),
	NumArgs:   2,
	TargetPos: []int{1},
}

/**
 * Casting
 */

// ToNumber takes a string, bool, or number value and converts it to a number.
// Strings are converted to numbers using strconv.Atoi.
// Boolean false is converted to 0 and boolean true is converted to 1.
var ToNumber = &Builtin{
	Name:      Var("to_number"),
	NumArgs:   2,
	TargetPos: []int{1},
}

/**
 * Regular Expressions
 */

// RegexMatch takes two strings and evaluates to true if the string in the second
// position matches the pattern in the first position.
var RegexMatch = &Builtin{
	Name:    Var("re_match"),
	NumArgs: 2,
}

/**
 * Sets
 */

// SetDiff returns the difference between two sets. The difference is all of the
// elements in the first set that are not in the second set.
var SetDiff = &Builtin{
	Name:      Var("set_diff"),
	NumArgs:   3,
	TargetPos: []int{2},
}

/**
 * Strings
 */

// Concat joins an array of strings to with an input string.
var Concat = &Builtin{
	Name:      Var("concat"),
	NumArgs:   3,
	TargetPos: []int{2},
}

// FormatInt returns the string representation of the number in the given base after converting it to an integer value.
var FormatInt = &Builtin{
	Name:      Var("format_int"),
	NumArgs:   3,
	TargetPos: []int{2},
}

// IndexOf returns the index of a substring contained inside a string
var IndexOf = &Builtin{
	Name:      Var("indexof"),
	NumArgs:   3,
	TargetPos: []int{2},
}

// Substring returns the portion of a string for a given start index and a length.
//   If the length is less than zero, then substring returns the remainder of the string.
var Substring = &Builtin{
	Name:      Var("substring"),
	NumArgs:   4,
	TargetPos: []int{3},
}

// Contains returns true if the search string is included in the base string
var Contains = &Builtin{
	Name:    Var("contains"),
	NumArgs: 2,
}

// StartsWith returns true if the search string begins with the base string
var StartsWith = &Builtin{
	Name:    Var("startswith"),
	NumArgs: 2,
}

// EndsWith returns true if the search string begins with the base string
var EndsWith = &Builtin{
	Name:    Var("endswith"),
	NumArgs: 2,
}

// Lower returns the input string but with all characters in lower-case
var Lower = &Builtin{
	Name:      Var("lower"),
	NumArgs:   2,
	TargetPos: []int{1},
}

// Upper returns the input string but with all characters in upper-case
var Upper = &Builtin{
	Name:      Var("upper"),
	NumArgs:   2,
	TargetPos: []int{1},
}

// Builtin represents a built-in function supported by OPA. Every
// built-in function is uniquely identified by a name.
type Builtin struct {
	Name      Var   // Unique name of built-in function, e.g., <Name>(term,term,...,term)
	Infix     Var   // Unique name of infix operator. Default should be unset.
	NumArgs   int   // Total number of args required by built-in.
	TargetPos []int // Argument positions that bind outputs. Indexing is zero-based.
}

// Expr creates a new expression for the built-in with the given terms.
func (b *Builtin) Expr(terms ...*Term) *Expr {
	ts := []*Term{VarTerm(string(b.Name))}
	for _, t := range terms {
		ts = append(ts, t)
	}
	return &Expr{
		Terms: ts,
	}
}

// IsTargetPos returns true if a variable in the i-th position will be
// bound when the expression is evaluated.
func (b *Builtin) IsTargetPos(i int) bool {
	for _, x := range b.TargetPos {
		if x == i {
			return true
		}
	}
	return false
}

func init() {
	BuiltinMap = map[Var]*Builtin{}
	for _, b := range DefaultBuiltins {
		RegisterBuiltin(b)
	}
}
