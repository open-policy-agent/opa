// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"regexp"
	"testing"
)

var generatedLocalRE = regexp.MustCompile(`^__local\d+__$`)

// numberLocals returns a copy of x with the generated locals of each rule, or
// of x if it holds no rules, numbered from 0 in order of appearance. Tests
// compare compiled code through it so that they don't depend on how a compile
// numbers its locals, only on which are the same.
func numberLocals[T any](x T) T {
	var cpy any
	switch v := any(x).(type) {
	case *Module:
		m := v.Copy()
		for i, r := range m.Rules {
			m.Rules[i] = renumberLocals(r).(*Rule)
		}
		cpy = m
	case *Rule:
		cpy = renumberLocals(v.Copy())
	case Body:
		cpy = renumberLocals(v.Copy())
	case Ref:
		cpy = renumberLocals(v.Copy())
	default:
		panic(fmt.Sprintf("numberLocals: unsupported type %T", x))
	}
	return cpy.(T)
}

func renumberLocals(x any) any {
	names, _ := localNames(x)
	if len(names) == 0 {
		return x
	}
	out, err := TransformVars(x, func(v Var) (Value, error) {
		if n, ok := names[v]; ok {
			return n, nil
		}
		return v, nil
	})
	if err != nil {
		panic(err)
	}
	return out
}

// localNames maps the generated locals of x to their numberLocals names, and
// lists them in order of appearance.
func localNames(x any) (map[Var]Var, []Var) {
	names := map[Var]Var{}
	var order []Var
	WalkVars(x, func(v Var) bool {
		if _, ok := names[v]; !ok && generatedLocalRE.MatchString(string(v)) {
			names[v] = Var(fmt.Sprintf("%s%d__", LocalVarPrefix, len(names)))
			order = append(order, v)
		}
		return false
	})
	return names, order
}

// numberRewrittenVars lists what rw maps the generated locals of each rule of
// m to, in the names numberLocals gives them.
func numberRewrittenVars(m *Module, rw map[Var]Var) []string {
	var out []string
	for i, r := range m.Rules {
		names, order := localNames(r)
		for _, v := range order {
			orig, ok := rw[v]
			if !ok {
				continue
			}
			if !generatedLocalRE.MatchString(string(orig)) {
				out = append(out, fmt.Sprintf("rule %d: %v=%v", i, names[v], orig))
				continue
			}
			// orig may be a local that a later rewrite took out of the rule.
			if _, ok := names[orig]; !ok {
				names[orig] = Var(fmt.Sprintf("%s%d__", LocalVarPrefix, len(names)))
			}
			out = append(out, fmt.Sprintf("rule %d: %v=%v", i, names[v], names[orig]))
		}
	}
	return out
}

func TestNumberLocals(t *testing.T) {
	tests := []struct {
		note  string
		a, b  string
		equal bool
	}{
		{
			note:  "renumbered",
			a:     `p if { __local3__ = input.x; __local7__ = __local3__ }`,
			b:     `p if { __local0__ = input.x; __local1__ = __local0__ }`,
			equal: true,
		},
		{
			note: "two locals merged",
			a:    `p if { __local3__ = input.x; __local7__ = __local3__ }`,
			b:    `p if { __local0__ = input.x; __local0__ = __local0__ }`,
		},
		{
			note: "user var kept",
			a:    `p if { x = input.x; __local1__ = x }`,
			b:    `p if { y = input.x; __local0__ = y }`,
		},
		{
			note:  "else shares the rule's numbering",
			a:     `p := __local4__ if { __local4__ = input.x } else := __local4__ if { __local4__ = input.y }`,
			b:     `p := __local0__ if { __local0__ = input.x } else := __local0__ if { __local0__ = input.y }`,
			equal: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			a, b := MustParseRuleWithOpts(tc.a, ParserOptions{RegoVersion: RegoV1}), MustParseRuleWithOpts(tc.b, ParserOptions{RegoVersion: RegoV1})
			if got := numberLocals(a).Equal(numberLocals(b)); got != tc.equal {
				t.Errorf("equal: %v, want %v\n%v\n%v", got, tc.equal, numberLocals(a), numberLocals(b))
			}
		})
	}
}
