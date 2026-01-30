// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"errors"
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"

	"github.com/open-policy-agent/opa/internal/edittree"
)

func builtinJSONRemove(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	// Expect an object and a string or array/set of strings
	if _, err := builtins.ObjectOperand(operands[0].Value, 1); err != nil {
		return err
	}

	// Build a list of json pointers to remove
	paths, err := getJSONPaths(operands[1].Value)
	if err != nil {
		return err
	}

	newObj, err := jsonRemove(operands[0], ast.NewTerm(pathsToObject(paths)))
	if err != nil {
		return err
	}

	if newObj == nil {
		return nil
	}

	return iter(newObj)
}

// jsonRemove returns a new term that is the result of walking
// through a and omitting removing any values that are in b but
// have ast.Null values (ie leaf nodes for b).
func jsonRemove(a *ast.Term, b *ast.Term) (*ast.Term, error) {
	if b == nil {
		// The paths diverged, return a
		return a, nil
	}

	var bObj ast.Object
	switch bValue := b.Value.(type) {
	case ast.Object:
		bObj = bValue
	case ast.Null:
		// Means we hit a leaf node on "b", dont add the value for a
		return nil, nil
	default:
		// The paths diverged, return a
		return a, nil
	}

	switch aValue := a.Value.(type) {
	case ast.String, ast.Number, ast.Boolean, ast.Null:
		return a, nil
	case ast.Object:
		newObj := ast.NewObjectWithCapacity(aValue.Len())
		err := aValue.Iter(func(k *ast.Term, v *ast.Term) error {
			// recurse and add the diff of sub objects as needed
			diffValue, err := jsonRemove(v, bObj.Get(k))
			if err != nil || diffValue == nil {
				return err
			}
			newObj.Insert(k, diffValue)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return ast.NewTerm(newObj), nil
	case ast.Set:
		newSet := ast.NewSetWithCapacity(aValue.Len())
		err := aValue.Iter(func(v *ast.Term) error {
			// recurse and add the diff of sub objects as needed
			diffValue, err := jsonRemove(v, bObj.Get(v))
			if err != nil || diffValue == nil {
				return err
			}
			newSet.Add(diffValue)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return ast.NewTerm(newSet), nil
	case *ast.Array:
		// When indexes are removed we shift left to close empty spots in the array
		// as per the JSON patch spec.
		newArraySlice := make([]*ast.Term, 0, aValue.Len())
		for i := range aValue.Len() {
			v := aValue.Elem(i)
			// recurse and add the diff of sub objects as needed
			// Note: Keys in b will be strings for the index, eg path /a/1/b => {"a": {"1": {"b": null}}}
			diffValue, err := jsonRemove(v, bObj.Get(ast.InternedIntegerString(i)))
			if err != nil {
				return nil, err
			}
			if diffValue != nil {
				newArraySlice = append(newArraySlice, diffValue)
			}
		}
		return ast.ArrayTerm(newArraySlice...), nil
	default:
		return nil, fmt.Errorf("invalid value type %T", a)
	}
}

func builtinJSONFilter(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	// Ensure we have the right parameters, expect an object and a string or array/set of strings
	obj, err := builtins.ObjectOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	// Build a list of filter strings
	filters, err := getJSONPaths(operands[1].Value)
	if err != nil {
		return err
	}

	// Actually do the filtering
	filterObj := pathsToObject(filters)
	r, err := obj.Filter(filterObj)
	if err != nil {
		return err
	}

	return iter(ast.NewTerm(r))
}

func getJSONPaths(operand ast.Value) (paths []ast.Ref, err error) {
	switch v := operand.(type) {
	case *ast.Array:
		paths = make([]ast.Ref, 0, v.Len())
		for i := range v.Len() {
			filter, err := parsePath(v.Elem(i))
			if err != nil {
				return nil, err
			}
			paths = append(paths, filter)
		}
	case ast.Set:
		paths = make([]ast.Ref, 0, v.Len())
		for _, item := range v.Slice() {
			filter, err := parsePath(item)
			if err != nil {
				return nil, err
			}
			paths = append(paths, filter)
		}
	default:
		return nil, builtins.NewOperandTypeErr(2, v, "set", "array")
	}

	return paths, nil
}

// parsePath parses a JSON pointer path or array of path segments into an ast.Ref.
func parsePath(path *ast.Term) (ast.Ref, error) {
	// paths can either be a `/` separated json path or
	// an array or set of values
	var pathSegments ast.Ref
	switch p := path.Value.(type) {
	case ast.String:
		if p == "" {
			return ast.InternedEmptyRefValue.(ast.Ref), nil
		}

		s := strings.TrimLeft(string(p), "/")
		n := strings.Count(s, "/") + 1

		pathSegments = make(ast.Ref, 0, n)

		part, remaining, found := strings.Cut(s, "/")
		unescaped := strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		pathSegments = append(pathSegments, ast.InternedTerm(unescaped))

		for found {
			part, remaining, found = strings.Cut(remaining, "/")
			unescaped := strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
			pathSegments = append(pathSegments, ast.InternedTerm(unescaped))
		}
	case *ast.Array:
		pathSegments = make(ast.Ref, 0, p.Len())
		for i := range p.Len() {
			pathSegments = append(pathSegments, p.Elem(i))
		}
	default:
		return nil, builtins.NewOperandErr(2,
			"must be one of {set, array} containing string paths or array of path segments but got "+ast.ValueName(p),
		)
	}

	return pathSegments, nil
}

func pathsToObject(paths []ast.Ref) ast.Object {
	root := ast.NewObjectWithCapacity(len(paths))

	for _, path := range paths {
		node := root
		done := false

		// If the path is an empty JSON path, skip all further processing.
		if len(path) == 0 {
			done = true
		}

		// Otherwise, we should have 1+ path segments to work with.
		for i := 0; i < len(path)-1 && !done; i++ {
			k := path[i]
			child := node.Get(k)

			if child == nil {
				obj := ast.NewObject()
				node.Insert(k, ast.NewTerm(obj))
				node = obj
				continue
			}

			switch v := child.Value.(type) {
			case ast.Null:
				done = true
			case ast.Object:
				node = v
			default:
				panic("unreachable")
			}
		}

		if !done {
			node.Insert(path[len(path)-1], ast.InternedNullTerm)
		}
	}

	return root
}

func applyPatches(source *ast.Term, operations *ast.Array) (*ast.Term, error) {
	et := edittree.EditTreeFromPool(source)
	defer edittree.Dispose(et)

	for i := range operations.Len() {
		object, ok := operations.Elem(i).Value.(ast.Object)
		if !ok {
			return nil, errors.New("must be an array of JSON-Patch objects, but at least one element is not an object")
		}

		// Validate
		if object.Get(ast.InternedTerm("path")) == nil {
			return nil, errors.New("missing required attribute 'path'")
		}

		opTerm := object.Get(ast.InternedTerm("op"))
		if opTerm == nil {
			return nil, errors.New("missing required attribute 'op'")
		}

		opStr, ok := opTerm.Value.(ast.String)
		if !ok {
			return nil, errors.New("attribute 'op' must be a string but found: " + ast.ValueName(opTerm.Value))
		}

		path, err := parsePath(object.Get(ast.InternedTerm("path")))
		if err != nil {
			return nil, err
		}

		switch string(opStr) {
		case "add":
			value := object.Get(ast.InternedTerm("value"))
			if value == nil {
				return nil, errors.New("missing required attribute 'value'")
			}
			if _, err = et.InsertAtPath(path, value); err != nil {
				return nil, err
			}
		case "remove":
			if _, err = et.DeleteAtPath(path); err != nil {
				return nil, err
			}
		case "replace":
			if _, err = et.DeleteAtPath(path); err != nil {
				return nil, err
			}
			value := object.Get(ast.InternedTerm("value"))
			if value == nil {
				return nil, errors.New("missing required attribute 'value'")
			}
			if _, err = et.InsertAtPath(path, value); err != nil {
				return nil, err
			}
		case "move":
			fromValue := object.Get(ast.InternedTerm("from"))
			if fromValue == nil {
				return nil, errors.New("missing required attribute 'from'")
			}

			from, err := parsePath(fromValue)
			if err != nil {
				return nil, err
			}
			chunk, err := et.RenderAtPath(from)
			if err != nil {
				return nil, err
			}
			if _, err = et.DeleteAtPath(from); err != nil {
				return nil, err
			}
			if _, err = et.InsertAtPath(path, chunk); err != nil {
				return nil, err
			}
		case "copy":
			fromValue := object.Get(ast.InternedTerm("from"))
			if fromValue == nil {
				return nil, errors.New("missing required attribute 'from'")
			}
			from, err := parsePath(fromValue)
			if err != nil {
				return nil, err
			}
			chunk, err := et.RenderAtPath(from)
			if err != nil {
				return nil, err
			}
			if _, err = et.InsertAtPath(path, chunk); err != nil {
				return nil, err
			}
		case "test":
			chunk, err := et.RenderAtPath(path)
			if err != nil {
				return nil, err
			}
			value := object.Get(ast.InternedTerm("value"))
			if value == nil {
				return nil, errors.New("missing required attribute 'value'")
			}
			if !chunk.Equal(value) {
				return nil, fmt.Errorf("value from EditTree != patch value.\n\nExpected: %v\n\nFound: %v", value, chunk)
			}
		default:
			return nil, fmt.Errorf("unrecognized op: '%s'", string(opStr))
		}
	}

	return et.Render(), nil
}

func builtinJSONPatch(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	// Expect an array of operations.
	operations, err := builtins.ArrayOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	// JSON patch supports arrays, objects as well as values as the target.
	patched, err := applyPatches(operands[0], operations)
	if err != nil {
		return err
	}
	return iter(patched)
}

func init() {
	for _, key := range []string{"op", "path", "from", "value", "add", "remove", "replace", "move", "copy", "test"} {
		ast.InternStringTerm(key)
	}

	RegisterBuiltinFunc(ast.JSONFilter.Name, builtinJSONFilter)
	RegisterBuiltinFunc(ast.JSONRemove.Name, builtinJSONRemove)
	RegisterBuiltinFunc(ast.JSONPatch.Name, builtinJSONPatch)
}
