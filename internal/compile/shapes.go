// Copyright 2026 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package compile

import (
	"maps"
	"slices"

	"github.com/open-policy-agent/opa/v1/ast"
)

// shape is what a field of a document holds: one value, or an array of them.
type shape int

const (
	undeclared shape = iota
	single
	array
)

// fieldShapes tells the shape of each field: as declared by the JSON Schema of the
// document doc, or else as the queries use it. A field used both ways is an error.
func fieldShapes(doc ast.Ref, schema any, queries []ast.Body) (map[string]shape, []*ast.Error) {
	declared := map[string]shape{}
	if doc != nil {
		declare(declared, schema, doc.String())
	}

	used := map[string]shape{}
	var errs []*ast.Error
	note := func(t *ast.Term, sh shape, e *ast.Expr) {
		if !fieldRef(t.Value) {
			return
		}
		field := t.Value.(ast.Ref).String()
		if declared[field] != undeclared {
			return
		}
		switch prev, ok := used[field]; {
		case !ok:
			used[field] = sh
		case prev != sh && prev != undeclared:
			errs = append(errs, err(e.Loc(), "%s is used both as an array and as a single value: "+
				"declare which it is in a schema", field))
			used[field] = undeclared
		}
	}
	var walk func(ast.Body)
	walk = func(b ast.Body) {
		for _, e := range b {
			switch {
			case e.IsEvery():
				every := e.Terms.(*ast.Every)
				note(every.Domain, array, e)
				walk(every.Body)
			case e.IsCall():
				op, ops := e.OperatorTerm().Value.(ast.Ref).String(), e.Operands()
				for i, t := range ops {
					if ref, ok := elementRef(t); ok {
						note(ast.NewTerm(ref[:len(ref)-1]), array, e)
						continue
					}
					if field, ok := elementsOf(t); ok {
						note(field, array, e)
						continue
					}
					switch {
					case (op == ast.AnyPrefixMatch.Name || op == ast.AnySuffixMatch.Name) && i == 0:
						// takes a string, or an array of them, alike
					case op == ast.Member.Name && i == 1, op == ast.Count.Name && i == 0,
						(op == ast.Equality.Name || op == ast.NotEqual.Name) && len(ops) == 2 && collection(ops[1-i].Value):
						note(t, array, e)
					default:
						note(t, single, e)
					}
				}
			default:
				if t, ok := e.Terms.(*ast.Term); ok {
					note(t, single, e)
				}
			}
		}
	}
	for _, q := range queries {
		walk(q)
	}

	maps.Copy(declared, used)
	return declared, errs
}

// declare records the shapes a JSON Schema declares below path: a property of type
// "array" is an array, one of a scalar type a single value, and an object declares
// the fields nested in it. Anything else is left undeclared.
func declare(into map[string]shape, schema any, path string) {
	obj, ok := schema.(map[string]any)
	if !ok {
		return
	}
	var types []string
	switch t := obj["type"].(type) {
	case string:
		types = []string{t}
	case []any:
		for _, x := range t {
			if s, ok := x.(string); ok && s != "null" {
				types = append(types, s)
			}
		}
	}
	if len(types) != 1 {
		return
	}
	switch types[0] {
	case "object":
		props, _ := obj["properties"].(map[string]any)
		for _, k := range slices.Sorted(maps.Keys(props)) {
			if ast.IsVarCompatibleString(k) {
				declare(into, props[k], path+"."+k)
			} else {
				declare(into, props[k], path+"["+ast.StringTerm(k).String()+"]")
			}
		}
	case "array":
		into[path] = array
	case "string", "number", "integer", "boolean":
		into[path] = single
	}
}

func (n *normalizer) shape(t *ast.Term) shape {
	if !fieldRef(t.Value) {
		return undeclared
	}
	return n.shapes[t.Value.(ast.Ref).String()]
}

func collection(v ast.Value) bool {
	switch v.(type) {
	case *ast.Array, ast.Set:
		return true
	}
	return false
}
