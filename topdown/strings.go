// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

func builtinFormatInt(a, b ast.Value) (ast.Value, error) {

	input, err := builtins.NumberOperand(a, 1)
	if err != nil {
		return nil, err
	}

	base, err := builtins.NumberOperand(b, 2)
	if err != nil {
		return nil, err
	}

	var format string
	switch base {
	case ast.Number("2"):
		format = "%b"
	case ast.Number("8"):
		format = "%o"
	case ast.Number("10"):
		format = "%d"
	case ast.Number("16"):
		format = "%x"
	default:
		return nil, builtins.NewOperandEnumErr(2, "2", "8", "10", "16")
	}

	f := builtins.NumberToFloat(input)
	i, _ := f.Int(nil)

	return ast.String(fmt.Sprintf(format, i)), nil
}

func builtinConcat(a, b ast.Value) (ast.Value, error) {

	join, err := builtins.StringOperand(a, 1)
	if err != nil {
		return nil, err
	}

	strs := []string{}

	switch b := b.(type) {
	case ast.Array:
		for i := range b {
			s, ok := b[i].Value.(ast.String)
			if !ok {
				return nil, builtins.NewOperandElementErr(2, b, b[i].Value, ast.StringTypeName)
			}
			strs = append(strs, string(s))
		}
	case *ast.Set:
		var err error
		stopped := b.Iter(func(x *ast.Term) bool {
			s, ok := x.Value.(ast.String)
			if !ok {
				err = builtins.NewOperandElementErr(2, b, x.Value, ast.StringTypeName)
				return true
			}
			strs = append(strs, string(s))
			return false
		})
		if stopped {
			return nil, err
		}
	default:
		return nil, builtins.NewOperandTypeErr(2, b, ast.SetTypeName, ast.ArrayTypeName)
	}

	return ast.String(strings.Join(strs, string(join))), nil
}

func builtinIndexOf(a, b ast.Value) (ast.Value, error) {
	base, err := builtins.StringOperand(a, 1)
	if err != nil {
		return nil, err
	}

	search, err := builtins.StringOperand(b, 2)
	if err != nil {
		return nil, err
	}

	index := strings.Index(string(base), string(search))
	return ast.IntNumberTerm(index).Value, nil
}

func builtinSubstring(a, b, c ast.Value) (ast.Value, error) {

	base, err := builtins.StringOperand(a, 1)
	if err != nil {
		return nil, err
	}

	startIndex, err := builtins.IntOperand(b, 2)
	if err != nil {
		return nil, err
	}

	length, err := builtins.IntOperand(c, 3)
	if err != nil {
		return nil, err
	}

	var s ast.String
	if length < 0 {
		s = ast.String(base[startIndex:])
	} else {
		s = ast.String(base[startIndex : startIndex+length])
	}

	return s, nil
}

func builtinContains(a, b ast.Value) error {
	s, err := builtins.StringOperand(a, 1)
	if err != nil {
		return err
	}

	substr, err := builtins.StringOperand(b, 2)
	if err != nil {
		return err
	}

	if !strings.Contains(string(s), string(substr)) {
		return BuiltinEmpty{}
	}

	return nil
}

func builtinStartsWith(a, b ast.Value) error {
	s, err := builtins.StringOperand(a, 1)
	if err != nil {
		return err
	}

	prefix, err := builtins.StringOperand(b, 2)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(string(s), string(prefix)) {
		return BuiltinEmpty{}
	}

	return nil
}

func builtinEndsWith(a, b ast.Value) error {
	s, err := builtins.StringOperand(a, 1)
	if err != nil {
		return err
	}

	suffix, err := builtins.StringOperand(b, 2)
	if err != nil {
		return err
	}

	if !strings.HasSuffix(string(s), string(suffix)) {
		return BuiltinEmpty{}
	}

	return nil
}

func builtinLower(a ast.Value) (ast.Value, error) {
	s, err := builtins.StringOperand(a, 1)
	if err != nil {
		return nil, err
	}

	return ast.String(strings.ToLower(string(s))), nil
}

func builtinUpper(a ast.Value) (ast.Value, error) {
	s, err := builtins.StringOperand(a, 1)
	if err != nil {
		return nil, err
	}

	return ast.String(strings.ToUpper(string(s))), nil
}

func builtinSplit(a, b ast.Value) (ast.Value, error) {
	s, err := builtins.StringOperand(a, 1)
	if err != nil {
		return nil, err
	}
	d, err := builtins.StringOperand(b, 2)
	elems := strings.Split(string(s), string(d))
	arr := make(ast.Array, len(elems))
	for i := range arr {
		arr[i] = ast.StringTerm(elems[i])
	}
	return arr, nil
}

func init() {
	RegisterFunctionalBuiltin2(ast.FormatInt.Name, builtinFormatInt)
	RegisterFunctionalBuiltin2(ast.Concat.Name, builtinConcat)
	RegisterFunctionalBuiltin2(ast.IndexOf.Name, builtinIndexOf)
	RegisterFunctionalBuiltin3(ast.Substring.Name, builtinSubstring)
	RegisterFunctionalBuiltinVoid2(ast.Contains.Name, builtinContains)
	RegisterFunctionalBuiltinVoid2(ast.StartsWith.Name, builtinStartsWith)
	RegisterFunctionalBuiltinVoid2(ast.EndsWith.Name, builtinEndsWith)
	RegisterFunctionalBuiltin1(ast.Upper.Name, builtinUpper)
	RegisterFunctionalBuiltin1(ast.Lower.Name, builtinLower)
	RegisterFunctionalBuiltin2(ast.Split.Name, builtinSplit)
}
