// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/open-policy-agent/opa/v1/types"
	"github.com/open-policy-agent/opa/v1/util"
)

const (
	typeElision = "..."

	// maxTypeErrLineWidth is the width above which a type error detail line is
	// re-rendered with nested type information elided. Details that already fit
	// are left alone: eliding them would drop information without buying any
	// readability.
	maxTypeErrLineWidth = 80

	// maxElidedTypeWidth is the width above which a type that is not itself the
	// subject of the mismatch is collapsed to its outermost constructor.
	maxElidedTypeWidth = 32

	// maxTypeDiffDepth bounds the parallel walk so that deeply nested types
	// cannot produce an unreadable message, or, for cyclic types, no message.
	maxTypeDiffDepth = 8
)

// sprintDiff returns the string representations of a and b, keeping only the
// structure that is needed to see how the two differ. Sub-types that are equal
// on both sides are collapsed to their outermost type constructor, and object
// properties and any-members that are equal on both sides are dropped
// altogether. An ellipsis marks everything that was left out.
func sprintDiff(a, b types.Type) (string, string) {
	return diffTypes(a, b, 0)
}

// sprintElided returns the string representation of t with any type information
// nested more than depth levels below t replaced by an ellipsis. A depth of zero
// keeps the outermost type constructor only; a negative depth renders t in full,
// exactly as types.Sprint does.
func sprintElided(t types.Type, depth int) string {
	if depth < 0 {
		return types.Sprint(t)
	}

	switch t := t.(type) {
	case nil:
		return types.Sprint(t)
	case *types.NamedType:
		// A name is not a level of nesting, so depth is passed through.
		return t.Name + ": " + sprintElided(t.Type, depth)
	case *types.Set:
		if depth == 0 {
			return "set[" + typeElision + "]"
		}
		return "set[" + sprintElided(t.Of(), depth-1) + "]"
	case *types.Array:
		static := make([]string, 0, t.Len())
		if depth == 0 && t.Len() > 0 {
			static = append(static, typeElision)
		} else {
			for i := range t.Len() {
				static = append(static, sprintElided(t.Select(i), depth-1))
			}
		}
		var dynamic string
		if dyn := t.Dynamic(); dyn != nil {
			if depth == 0 {
				dynamic = typeElision
			} else {
				dynamic = sprintElided(dyn, depth-1)
			}
		}
		return sprintArray(static, dynamic)
	case *types.Object:
		props := t.StaticProperties()
		static := make([]string, 0, len(props))
		if depth == 0 && len(props) > 0 {
			static = append(static, typeElision)
		} else {
			for _, p := range props {
				static = append(static, sprintProperty(p.Key, sprintElided(p.Value, depth-1)))
			}
		}
		var dynamic string
		if dyn := t.DynamicProperties(); dyn != nil {
			if depth == 0 {
				dynamic = typeElision
			} else {
				dynamic = sprintElided(dyn.Key, depth-1) + ": " + sprintElided(dyn.Value, depth-1)
			}
		}
		return sprintObject(static, dynamic)
	case types.Any:
		if len(t) == 0 {
			return t.String()
		}
		if depth == 0 {
			return sprintAny([]string{typeElision})
		}
		of := make([]string, len(t))
		for i := range t {
			of[i] = sprintElided(t[i], depth-1)
		}
		return sprintAny(of)
	case *types.Function:
		args := t.FuncArgs()
		params := make([]string, 0, len(args.Args)+1)
		if depth == 0 {
			if len(args.Args) > 0 || args.Variadic != nil {
				params = append(params, typeElision)
			}
			if t.Result() == nil {
				return sprintFunction(params, types.Sprint(nil))
			}
			return sprintFunction(params, typeElision)
		}
		for i := range args.Args {
			params = append(params, sprintElided(args.Args[i], depth-1))
		}
		if args.Variadic != nil {
			params = append(params, sprintElided(args.Variadic, depth-1)+"...")
		}
		return sprintFunction(params, sprintElided(t.Result(), depth-1))
	default:
		// Scalars, and recursive types, which are rendered by name only.
		return t.String()
	}
}

// elideType collapses t to its outermost type constructor, unless it is short
// enough that spelling it out costs nothing.
func elideType(t types.Type) string {
	if full := types.Sprint(t); utf8.RuneCountInString(full) <= maxElidedTypeWidth {
		return full
	}
	return sprintElided(t, 0)
}

func unwrapNamedType(t types.Type) types.Type {
	if n, ok := t.(*types.NamedType); ok {
		return n.Type
	}
	return t
}

// typesEqual reports whether a and b would render identically. Comparing the
// rendered form, rather than the type structure, keeps this safe for recursive
// types, which render as a name rather than as their unrolled definition.
func typesEqual(a, b types.Type) bool {
	return types.Sprint(a) == types.Sprint(b)
}

func diffTypes(a, b types.Type, depth int) (string, string) {
	// A name is not a level of nesting, so depth is passed through.
	if n, ok := a.(*types.NamedType); ok {
		left, right := diffTypes(n.Type, b, depth)
		return n.Name + ": " + left, right
	}
	if n, ok := b.(*types.NamedType); ok {
		left, right := diffTypes(a, n.Type, depth)
		return left, n.Name + ": " + right
	}

	if depth >= maxTypeDiffDepth || a == nil || b == nil || typesEqual(a, b) {
		return elideType(a), elideType(b)
	}

	switch a := a.(type) {
	case *types.Set:
		if b, ok := b.(*types.Set); ok {
			left, right := diffTypes(a.Of(), b.Of(), depth+1)
			return "set[" + left + "]", "set[" + right + "]"
		}
	case *types.Array:
		if b, ok := b.(*types.Array); ok {
			return diffArrays(a, b, depth)
		}
	case *types.Object:
		if b, ok := b.(*types.Object); ok {
			return diffObjects(a, b, depth)
		}
	case types.Any:
		if b, ok := b.(types.Any); ok {
			return diffAnys(a, b)
		}
	case *types.Function:
		if b, ok := b.(*types.Function); ok && a.Arity() == b.Arity() {
			return diffFunctions(a, b, depth)
		}
	}

	// The outermost type constructors already differ, which is all the reader
	// needs to see.
	return elideType(a), elideType(b)
}

func diffArrays(a, b *types.Array, depth int) (string, string) {
	if a.Len() != b.Len() || (a.Dynamic() == nil) != (b.Dynamic() == nil) {
		// Element types are still shown so that it is clear which of the two is
		// the longer, or which one has a dynamic tail.
		return sprintElided(a, 1), sprintElided(b, 1)
	}

	// Array elements are positional, so equal ones are collapsed rather than
	// dropped, keeping the two renderings aligned.
	left := make([]string, 0, a.Len())
	right := make([]string, 0, b.Len())
	for i := range a.Len() {
		l, r := diffTypes(a.Select(i), b.Select(i), depth+1)
		left = append(left, l)
		right = append(right, r)
	}

	var dynLeft, dynRight string
	if a.Dynamic() != nil {
		dynLeft, dynRight = diffTypes(a.Dynamic(), b.Dynamic(), depth+1)
	}

	return sprintArray(left, dynLeft), sprintArray(right, dynRight)
}

func diffObjects(a, b *types.Object, depth int) (string, string) {
	aProps, bProps := a.StaticProperties(), b.StaticProperties()

	var left, right []string
	var elided bool

	// Static properties are sorted by key on both sides, so they can be walked in
	// parallel. Those that are equal say nothing about why the two types were
	// reported together, and are dropped.
	i, j := 0, 0
	for i < len(aProps) || j < len(bProps) {
		switch {
		case j == len(bProps):
			left = append(left, sprintProperty(aProps[i].Key, elideType(aProps[i].Value)))
			i++
		case i == len(aProps):
			right = append(right, sprintProperty(bProps[j].Key, elideType(bProps[j].Value)))
			j++
		default:
			switch util.Compare(aProps[i].Key, bProps[j].Key) {
			case -1:
				left = append(left, sprintProperty(aProps[i].Key, elideType(aProps[i].Value)))
				i++
			case 1:
				right = append(right, sprintProperty(bProps[j].Key, elideType(bProps[j].Value)))
				j++
			default:
				if typesEqual(aProps[i].Value, bProps[j].Value) {
					elided = true
				} else {
					l, r := diffTypes(aProps[i].Value, bProps[j].Value, depth+1)
					left = append(left, sprintProperty(aProps[i].Key, l))
					right = append(right, sprintProperty(bProps[j].Key, r))
				}
				i++
				j++
			}
		}
	}

	aDyn, bDyn := a.DynamicProperties(), b.DynamicProperties()
	var dynLeft, dynRight string
	switch {
	case aDyn != nil && bDyn != nil:
		keyLeft, keyRight := diffTypes(aDyn.Key, bDyn.Key, depth+1)
		valLeft, valRight := diffTypes(aDyn.Value, bDyn.Value, depth+1)
		dynLeft, dynRight = keyLeft+": "+valLeft, keyRight+": "+valRight
	case aDyn != nil:
		dynLeft = elideType(aDyn.Key) + ": " + elideType(aDyn.Value)
	case bDyn != nil:
		dynRight = elideType(bDyn.Key) + ": " + elideType(bDyn.Value)
	}

	return sprintObject(withElision(left, elided), dynLeft), sprintObject(withElision(right, elided), dynRight)
}

func diffAnys(a, b types.Any) (string, string) {
	// The members of an Any are unordered as far as the reader is concerned, so
	// those that appear on both sides are dropped rather than collapsed.
	left := make([]string, 0, len(a))
	right := make([]string, 0, len(b))
	var elided bool

	for _, tpe := range a {
		if containsType(b, tpe) {
			elided = true
		} else {
			left = append(left, elideType(tpe))
		}
	}
	for _, tpe := range b {
		if !containsType(a, tpe) {
			right = append(right, elideType(tpe))
		}
	}

	return sprintAny(withElision(left, elided)), sprintAny(withElision(right, elided))
}

func diffFunctions(a, b *types.Function, depth int) (string, string) {
	aArgs, bArgs := a.FuncArgs(), b.FuncArgs()

	left := make([]string, 0, len(aArgs.Args)+1)
	right := make([]string, 0, len(bArgs.Args)+1)
	for i := range aArgs.Args {
		l, r := diffTypes(aArgs.Args[i], bArgs.Args[i], depth+1)
		left = append(left, l)
		right = append(right, r)
	}
	switch {
	case aArgs.Variadic != nil && bArgs.Variadic != nil:
		l, r := diffTypes(aArgs.Variadic, bArgs.Variadic, depth+1)
		left = append(left, l+"...")
		right = append(right, r+"...")
	case aArgs.Variadic != nil:
		left = append(left, elideType(aArgs.Variadic)+"...")
	case bArgs.Variadic != nil:
		right = append(right, elideType(bArgs.Variadic)+"...")
	}

	resLeft, resRight := diffTypes(a.Result(), b.Result(), depth+1)
	return sprintFunction(left, resLeft), sprintFunction(right, resRight)
}

func containsType(haystack types.Any, needle types.Type) bool {
	for _, tpe := range haystack {
		if typesEqual(tpe, needle) {
			return true
		}
	}
	return false
}

func withElision(parts []string, elided bool) []string {
	if !elided {
		return parts
	}
	return append(parts, typeElision)
}

func sprintProperty(key any, value string) string {
	return fmt.Sprintf("%v: %v", key, value)
}

func sprintArray(static []string, dynamic string) string {
	return sprintComposite("array", static, dynamic)
}

func sprintObject(static []string, dynamic string) string {
	return sprintComposite("object", static, dynamic)
}

func sprintComposite(prefix string, static []string, dynamic string) string {
	sb := strings.Builder{}
	sb.WriteString(prefix)
	if len(static) > 0 {
		sb.WriteString("<")
		sb.WriteString(strings.Join(static, ", "))
		sb.WriteString(">")
	}
	if dynamic != "" {
		sb.WriteString("[")
		sb.WriteString(dynamic)
		sb.WriteString("]")
	}
	return sb.String()
}

func sprintAny(of []string) string {
	if len(of) == 0 {
		return "any"
	}
	return "any<" + strings.Join(of, ", ") + ">"
}

func sprintFunction(args []string, result string) string {
	return "(" + strings.Join(args, ", ") + ") => " + result
}

func tooWideForTypeErr(lines ...string) bool {
	for _, line := range lines {
		if utf8.RuneCountInString(line) > maxTypeErrLineWidth {
			return true
		}
	}
	return false
}
