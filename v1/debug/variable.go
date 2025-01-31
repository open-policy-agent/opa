// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package debug

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
)

type Variable interface {
	// Name returns the name of the variable.
	Name() string

	// Type returns the type of the variable.
	Type() string

	// Value returns the value of the variable.
	Value() string

	// VariablesReference returns a reference to the variables that are children of this variable.
	// E.g. this variable is a collection, such as an array, set, or object.
	VariablesReference() VarRef
}

type namedVar struct {
	name  string
	value ast.Value
}

func (nv namedVar) Name() string {
	return nv.name
}

func (nv namedVar) Type() string {
	return valueTypeName(nv.value)
}

func (nv namedVar) Value() string {
	return truncatedString(nv.value.String(), 100)
}

type variableGetter func() []namedVar

type variableManager struct {
	getters []variableGetter
}

func newVariableManager() *variableManager {
	return &variableManager{}
}

func (vs *variableManager) addVars(getter variableGetter) VarRef {
	vs.getters = append(vs.getters, getter)
	return VarRef(len(vs.getters))
}

type VarRef int

type variable struct {
	v   namedVar
	ref VarRef
}

func (v variable) Name() string {
	return v.v.Name()
}

func (v variable) Type() string {
	return v.v.Type()
}

func (v variable) Value() string {
	return v.v.Value()
}

func (v variable) VariablesReference() VarRef {
	return v.ref
}

func (vs *variableManager) vars(varRef VarRef) ([]Variable, error) {
	i := int(varRef) - 1
	if i < 0 || i >= len(vs.getters) {
		return nil, fmt.Errorf("invalid variable reference: %d", varRef)
	}

	namedVar := vs.getters[i]()
	vars := make([]Variable, len(namedVar))

	for i, nv := range namedVar {
		vars[i] = variable{
			v:   nv,
			ref: vs.subVars(nv.value),
		}
	}

	slices.SortFunc(vars, func(a, b Variable) int {
		return strings.Compare(a.Name(), b.Name())
	})

	return vars, nil
}

func truncatedString(s string, max int) string {
	if len(s) > max {
		return s[:max-2] + "..."
	}
	return s
}

func valueTypeName(v ast.Value) string {
	switch v.(type) {
	case ast.Null:
		return "null"
	case ast.Boolean:
		return "boolean"
	case ast.Number:
		return "number"
	case ast.String:
		return "string"
	case *ast.Array:
		return "array"
	case ast.Object:
		return "object"
	case ast.Set:
		return "set"
	case ast.Ref:
		return "ref"
	default:
		return "unknown"
	}
}

func (vs *variableManager) subVars(v ast.Value) VarRef {
	if obj, ok := v.(ast.Object); ok {
		vars := make([]namedVar, 0, obj.Len())
		if err := obj.Iter(func(k, v *ast.Term) error {
			vars = append(vars, namedVar{
				name:  k.String(),
				value: v.Value,
			})
			return nil
		}); err != nil {
			return 0
		}
		return vs.addVars(func() []namedVar {
			return vars
		})
	}

	if arr, ok := v.(*ast.Array); ok {
		vars := make([]namedVar, 0, arr.Len())
		for i := range arr.Len() {
			vars = append(vars, namedVar{
				name:  strconv.Itoa(i),
				value: arr.Elem(i).Value,
			})
		}
		return vs.addVars(func() []namedVar {
			return vars
		})
	}

	if set, ok := v.(ast.Set); ok {
		vars := make([]namedVar, 0, set.Len())
		for _, elem := range set.Slice() {
			vars = append(vars, namedVar{
				value: elem.Value,
			})
		}
		return vs.addVars(func() []namedVar {
			return vars
		})
	}

	return 0
}
