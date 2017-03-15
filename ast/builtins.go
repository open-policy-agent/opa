// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import "github.com/open-policy-agent/opa/types"

// Builtins is the registry of built-in functions supported by OPA.
// Call RegisterBuiltin to add a new built-in.
var Builtins []*Builtin

// RegisterBuiltin adds a new built-in function to the registry.
func RegisterBuiltin(b *Builtin) {
	Builtins = append(Builtins, b)
	BuiltinMap[b.Name] = b
	if len(b.Infix) > 0 {
		BuiltinMap[b.Infix] = b
	}
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

	// Binary
	And, Or,

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

	// JSON
	JSONMarshal, JSONUnmarshal,
}

// BuiltinMap provides a convenient mapping of built-in names to
// built-in definitions.
var BuiltinMap map[Var]*Builtin

/**
 * Unification
 */

// Equality represents the "=" operator.
var Equality = &Builtin{
	Name:  Var("eq"),
	Infix: Var("="),
	Args: []types.Type{
		types.A,
		types.A,
	},
	TargetPos: []int{0, 1},
}

/**
 * Comparisons
 */

// GreaterThan represents the ">" comparison operator.
var GreaterThan = &Builtin{
	Name:  Var("gt"),
	Infix: Var(">"),
	Args: []types.Type{
		types.A,
		types.A,
	},
}

// GreaterThanEq represents the ">=" comparison operator.
var GreaterThanEq = &Builtin{
	Name:  Var("gte"),
	Infix: Var(">="),
	Args: []types.Type{
		types.A,
		types.A,
	},
}

// LessThan represents the "<" comparison operator.
var LessThan = &Builtin{
	Name:  Var("lt"),
	Infix: Var("<"),
	Args: []types.Type{
		types.A,
		types.A,
	},
}

// LessThanEq represents the "<=" comparison operator.
var LessThanEq = &Builtin{
	Name:  Var("lte"),
	Infix: Var("<="),
	Args: []types.Type{
		types.A,
		types.A,
	},
}

// NotEqual represents the "!=" comparison operator.
var NotEqual = &Builtin{
	Name:  Var("neq"),
	Infix: Var("!="),
	Args: []types.Type{
		types.A,
		types.A,
	},
}

/**
 * Arithmetic
 */

// Plus adds two numbers together.
var Plus = &Builtin{
	Name:  Var("plus"),
	Infix: Var("+"),
	Args: []types.Type{
		types.N,
		types.N,
		types.N,
	},
	TargetPos: []int{2},
}

// Minus subtracts the second number from the first number or computes the diff
// between two sets.
var Minus = &Builtin{
	Name:  Var("minus"),
	Infix: Var("-"),
	Args: []types.Type{
		types.NewAny(types.N, types.NewSet(types.A)),
		types.NewAny(types.N, types.NewSet(types.A)),
		types.NewAny(types.N, types.NewSet(types.A)),
	},
	TargetPos: []int{2},
}

// Multiply multiplies two numbers together.
var Multiply = &Builtin{
	Name:  Var("mul"),
	Infix: Var("*"),
	Args: []types.Type{
		types.N,
		types.N,
		types.N,
	},
	TargetPos: []int{2},
}

// Divide divides the first number by the second number.
var Divide = &Builtin{
	Name:  Var("div"),
	Infix: Var("/"),
	Args: []types.Type{
		types.N,
		types.N,
		types.N,
	},
	TargetPos: []int{2},
}

// Round rounds the number up to the nearest integer.
var Round = &Builtin{
	Name: Var("round"),
	Args: []types.Type{
		types.N,
		types.N,
	},
	TargetPos: []int{1},
}

// Abs returns the number without its sign.
var Abs = &Builtin{
	Name: Var("abs"),
	Args: []types.Type{
		types.N,
		types.N,
	},
	TargetPos: []int{1},
}

/**
 * Binary
 */

// TODO(tsandall): update binary operators to support integers.

// And performs an intersection operation on sets.
var And = &Builtin{
	Name:  Var("and"),
	Infix: Var("&"),
	Args: []types.Type{
		types.NewSet(types.A),
		types.NewSet(types.A),
		types.NewSet(types.A),
	},
	TargetPos: []int{2},
}

// Or performs a union operation on sets.
var Or = &Builtin{
	Name:  Var("or"),
	Infix: Var("|"),
	Args: []types.Type{
		types.NewSet(types.A),
		types.NewSet(types.A),
		types.NewSet(types.A),
	},
	TargetPos: []int{2},
}

/**
 * Aggregates
 */

// Count takes a collection or string and counts the number of elements in it.
var Count = &Builtin{
	Name: Var("count"),
	Args: []types.Type{
		types.NewAny(
			types.NewSet(types.A),
			types.NewArray(nil, types.A),
			types.NewObject(nil, types.A),
			types.S,
		),
		types.N,
	},
	TargetPos: []int{1},
}

// Sum takes an array or set of numbers and sums them.
var Sum = &Builtin{
	Name: Var("sum"),
	Args: []types.Type{
		types.NewAny(
			types.NewSet(types.N),
			types.NewArray(nil, types.N),
		),
		types.N,
	},
	TargetPos: []int{1},
}

// Max returns the maximum value in a collection.
var Max = &Builtin{
	Name: Var("max"),
	Args: []types.Type{
		types.NewAny(
			types.NewSet(types.A),
			types.NewArray(nil, types.A),
		),
		types.A,
	},
	TargetPos: []int{1},
}

/**
 * Casting
 */

// ToNumber takes a string, bool, or number value and converts it to a number.
// Strings are converted to numbers using strconv.Atoi.
// Boolean false is converted to 0 and boolean true is converted to 1.
var ToNumber = &Builtin{
	Name: Var("to_number"),
	Args: []types.Type{
		types.NewAny(
			types.N,
			types.S,
			types.B,
			types.NewNull(),
		),
		types.N,
	},
	TargetPos: []int{1},
}

/**
 * Regular Expressions
 */

// RegexMatch takes two strings and evaluates to true if the string in the second
// position matches the pattern in the first position.
var RegexMatch = &Builtin{
	Name: Var("re_match"),
	Args: []types.Type{
		types.S,
		types.S,
	},
}

/**
 * Strings
 */

// Concat joins an array of strings with an input string.
var Concat = &Builtin{
	Name: Var("concat"),
	Args: []types.Type{
		types.S,
		types.NewAny(
			types.NewSet(types.S),
			types.NewArray(nil, types.S),
		),
		types.S,
	},
	TargetPos: []int{2},
}

// FormatInt returns the string representation of the number in the given base after converting it to an integer value.
var FormatInt = &Builtin{
	Name: Var("format_int"),
	Args: []types.Type{
		types.N,
		types.N,
		types.S,
	},
	TargetPos: []int{2},
}

// IndexOf returns the index of a substring contained inside a string
var IndexOf = &Builtin{
	Name: Var("indexof"),
	Args: []types.Type{
		types.S,
		types.S,
		types.N,
	},
	TargetPos: []int{2},
}

// Substring returns the portion of a string for a given start index and a length.
//   If the length is less than zero, then substring returns the remainder of the string.
var Substring = &Builtin{
	Name: Var("substring"),
	Args: []types.Type{
		types.S,
		types.N,
		types.N,
		types.S,
	},
	TargetPos: []int{3},
}

// Contains returns true if the search string is included in the base string
var Contains = &Builtin{
	Name: Var("contains"),
	Args: []types.Type{
		types.S,
		types.S,
	},
}

// StartsWith returns true if the search string begins with the base string
var StartsWith = &Builtin{
	Name: Var("startswith"),
	Args: []types.Type{
		types.S,
		types.S,
	},
}

// EndsWith returns true if the search string begins with the base string
var EndsWith = &Builtin{
	Name: Var("endswith"),
	Args: []types.Type{
		types.S,
		types.S,
	},
}

// Lower returns the input string but with all characters in lower-case
var Lower = &Builtin{
	Name: Var("lower"),
	Args: []types.Type{
		types.S,
		types.S,
	},
	TargetPos: []int{1},
}

// Upper returns the input string but with all characters in upper-case
var Upper = &Builtin{
	Name: Var("upper"),
	Args: []types.Type{
		types.S,
		types.S,
	},
	TargetPos: []int{1},
}

/**
 * JSON
 */

// JSONMarshal serializes the input term.
var JSONMarshal = &Builtin{
	Name: Var("json_marshal"),
	Args: []types.Type{
		types.A,
		types.S,
	},
	TargetPos: []int{1},
}

// JSONUnmarshal deserializes the input string.
var JSONUnmarshal = &Builtin{
	Name: Var("json_unmarshal"),
	Args: []types.Type{
		types.S,
		types.A,
	},
	TargetPos: []int{1},
}

/**
 * Deprecated built-ins.
 */

// SetDiff has been replaced by the minus built-in.
var SetDiff = &Builtin{
	Name: Var("set_diff"),
	Args: []types.Type{
		types.NewSet(types.A),
		types.NewSet(types.A),
		types.NewSet(types.A),
	},
	TargetPos: []int{2},
}

// Builtin represents a built-in function supported by OPA. Every
// built-in function is uniquely identified by a name.
type Builtin struct {
	Name      Var          // Unique name of built-in function, e.g., <Name>(term,term,...,term)
	Infix     Var          // Unique name of infix operator. Default should be unset.
	Args      []types.Type // Built-in argument type declaration.
	TargetPos []int        // Argument positions that bind outputs. Indexing is zero-based.
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
