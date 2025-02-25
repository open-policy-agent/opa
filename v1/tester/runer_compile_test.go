// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package tester

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestInjectTestCaseFunc(t *testing.T) {
	testCases := []struct {
		note   string
		module string
		exp    string
	}{
		{
			note: "no head-ref, assigned last in body",
			module: `package test
				test_foo if {
					some tc in [
						{"note": "a", "x": 1},
					]
					tc.x == 1
				}`,
			// func not injected
			exp: `package test
				test_foo if {
					__local3__ = [{"note": "a", "x": 1}]
					__local2__ = __local3__[__local1__]
					__local2__.x = 1
				}`,
		},

		{
			note: "manual use of internal.test_case",
			module: `package test
				test_foo[tc.note] if {
					some tc in [
						{"note": "a", "x": 1},
					]
					tc.x == 1
					internal.test_case([tc.note, "foo", "bar"])
				}`,
			// func not injected
			exp: `package test
				test_foo[__local0__] if { 
					__local4__ = [{"note": "a", "x": 1}]
					__local3__ = __local4__[__local2__]            # func would have been injected subsequent to here
					__local3__.x = 1
					__local5__ = __local3__.note
					internal.test_case([__local5__, "foo", "bar"]) # manual use of func
					__local0__ = __local3__.note 
				}`,
		},

		{
			note: "head-ref, assigned last in body",
			module: `package test
				test_foo.foo if {
					some tc in [
						{"note": "a", "x": 1},
					]
					tc.x == 1
				}`,
			// no var assignment in body, func injected first in body
			exp: `package test
				test_foo.foo if { 
					internal.test_case(["foo"])          # func injection
					__local3__ = [{"note": "a", "x": 1}]
					__local2__ = __local3__[__local1__]
					__local2__.x = 1
				}`,
		},
		{
			note: "string in head-ref, assigned last in body",
			module: `package test
				test_foo["foo"] if {
					some tc in [
						{"note": "a", "x": 1},
					]
					tc.x == 1
				}`,
			// no var assignment in body, func injected first in body
			exp: `package test
				test_foo.foo if { 
					internal.test_case(["foo"])          # func injection
					__local3__ = [{"note": "a", "x": 1}]
					__local2__ = __local3__[__local1__]
					__local2__.x = 1
				}`,
		},

		{
			note: "const in head-ref, assigned last in body",
			module: `package test
				foo := "bar"
				test_foo[foo] if {
					some tc in [
						{"note": "a", "x": 1},
					]
					tc.x == 1
				}`,
			// var assignment can be moved up the body, func injected after moved expr
			exp: `package test
				foo := "bar" if { true }
				test_foo[__local0__] if { 
					__local0__ = data.test.foo           # generated head-ref const/var assignment, moved up
					internal.test_case([__local0__])     # func injection
					__local4__ = [{"note": "a", "x": 1}]
					__local3__ = __local4__[__local2__]
					__local3__.x = 1
				}`,
		},

		{
			note: "var in head-ref, assigned last in body",
			module: `package test
				test_foo[note] if {
					some tc in [
						{"note": "a"},
					]
					note := tc.note
				}`,
			// var assignment cannot be moved up the body, func injected last in body
			exp: `package test
				test_foo[__local3__] if { 
					__local4__ = [{"note": "a"}]
					__local2__ = __local4__[__local1__]
					__local3__ = __local2__.note        # head-ref var assignment
					internal.test_case([__local3__])    # func injection
				}`,
		},
		{
			note: "var in head-ref, assigned last in body, trailing assertions",
			module: `package test
				test_foo[note] if {
					some tc in [
						{"note": "a", "x": 1},
					]
					note := tc.note
					tc.x == 1
				}`,
			// var assignment cannot be moved up the body, func injected last in body
			exp: `package test
				test_foo[__local3__] if { 
					__local4__ = [{"note": "a", "x": 1}]
					__local2__ = __local4__[__local1__]
					__local3__ = __local2__.note        # head-ref var assignment
					internal.test_case([__local3__])    # func injection
					__local2__.x = 1
				}`,
		},
		{
			note: "var in head-ref, assigned mid-body",
			module: `package test
				test_foo[note] if {
					some tc in [
						{"note": "a", "x": 1},
					]
					note := tc.note
					tc.x == 1
				}`,
			// var assignment cannot be moved up the body, func injected after assignment
			exp: `package test
				test_foo[__local3__] if { 
					__local4__ = [{"note": "a", "x": 1}]
					__local2__ = __local4__[__local1__]
					__local3__ = __local2__.note        # head-ref var assignment
					internal.test_case([__local3__])    # func injection
					__local2__.x = 1
				}`,
		},
		{
			note: "var in head-ref, assigned after unrelated assertions",
			module: `package test
				test_foo[note] if {
					some tc in [
						{"note": "a", "x": 1},
					]
					tc.x == 1
					note := tc.note
				}`,
			// var assignment can be moved up the body, func injected after assignment
			exp: `package test
				test_foo[__local3__] if { 
					__local4__ = [{"note": "a", "x": 1}]
					__local2__ = __local4__[__local1__]
					__local2__.x = 1                    # non-generated expression, can't be moved
					__local3__ = __local2__.note        # head-ref var assignment
					internal.test_case([__local3__])    # func injection
				}`,
		},
		{
			note: "var in head-ref, non-assignment reference in body",
			module: `package test
				test_concat[note] if {
					some note, tc in {     # Compiled into roughly '__local__ = {...}; tc = __local__[note]' 
						"empty + empty": {
							"a": [],
							"b": [],
							"exp": [],
						},
					}
					
					act := array.concat(tc.a, tc.b)
					act == tc.exp
				}`,
			exp: `package test
				test_concat[__local0__] if { 
					__local3__ = {"empty + empty": {"a": [], "b": [], "exp": []}}
					__local1__ = __local3__[__local0__] # head-ref var assignment
					internal.test_case([__local0__])    # func injection
					__local5__ = __local1__.a
					__local6__ = __local1__.b
					array.concat(__local5__, __local6__, __local4__)
					__local2__ = __local4__
					__local2__ = __local1__.exp
				}`,
		},

		{
			note: "ref in head-ref",
			module: `package test
				test_foo[tc.note] if {
					some tc in [
						{"note": "a"},
					]
				}`,
			// var assignment cannot be moved up the body, func injected last in body
			exp: `package test
				test_foo[__local0__] if { 
					__local4__ = [{"note": "a"}]
					__local3__ = __local4__[__local2__]
					__local0__ = __local3__.note        # generated head-ref var assignment
					internal.test_case([__local0__])    # func injection
				}`,
		},
		{
			note: "ref in head-ref, can move above unrelated assertions",
			module: `package test
				test_foo[tc.note] if {
					some tc in [
						{"note": "a", "x": 1, "y": 2},
					]
					tc.x == 1
					tc.y == 2
				}`,
			// var assignment can be moved up the body, func injected after assignment
			exp: `package test
				test_foo[__local0__] if { 
					__local4__ = [{"note": "a", "x": 1, "y": 2}]
					__local3__ = __local4__[__local2__]
					__local0__ = __local3__.note                 # generated head-ref var assignment
					internal.test_case([__local0__])			 # func injection
					__local3__.x = 1
					__local3__.y = 2
					}`,
		},

		// Multiple head-ref test-case terms

		{
			note: "multi-term head-ref, assigned last in body",
			module: `package test
				test_foo.foo.bar if {
					some tc in [
						{"note": "a", "x": 1},
					]
					tc.x == 1
				}`,
			// no var assignment in body, func injected first in body
			exp: `package test
				test_foo.foo.bar if { 
					internal.test_case(["foo", "bar"])   # func injection
					__local3__ = [{"note": "a", "x": 1}]
					__local2__ = __local3__[__local1__]
					__local2__.x = 1
				}`,
		},
		{
			note: "multiple vars in head-ref",
			module: `package test
				test_foo[note1][note2] if {
					some flag in [
						{"note": "on", "a": 1},
					]
					note1 := flag.note
					some tc in [
						{"note": "a", "x": 1},
					]
					note2 := tc.note
					flag.a == 1
					tc.x == 1
				}`,
			// var assignment cannot be moved up the body, func injected after last head-ref assignment
			exp: `package test
				test_foo[__local3__][__local7__] = true if { 
					__local8__ = [{"a": 1, "note": "on"}]; 
					__local2__ = __local8__[__local1__]; 
					__local3__ = __local2__.note;                 # manual head-ref var assignment
					__local9__ = [{"note": "a", "x": 1}]; 
					__local6__ = __local9__[__local5__]; 
					__local7__ = __local6__.note;                 # manual head-ref var assignment
					internal.test_case([__local3__, __local7__]); # func injection, after last head-ref assignment
					__local2__.a = 1; 
					__local6__.x = 1 
				}`,
		},
		{
			note: "multiple vars in head-ref, manual assignment below unrelated assertion(s)",
			module: `package test
				test_foo[note1][note2] if {
					some flag in [
						{"note": "on", "a": 1},
					]
					some tc in [
						{"note": "a", "x": 1},
					]
					note2 := tc.note
					flag.a == 1
					note1 := flag.note
					tc.x == 1
				}`,
			// var assignment cannot be moved up the body, func injected after last head-ref assignment
			exp: `package test
				test_foo[__local7__][__local6__] if { 
					__local8__ = [{"a": 1, "note": "on"}]
					__local2__ = __local8__[__local1__]
					__local9__ = [{"note": "a", "x": 1}]
					__local5__ = __local9__[__local4__]
					__local6__ = __local5__.note                 # manual head-ref var assignment 
					__local2__.a = 1
					__local7__ = __local2__.note                 # manual head-ref var assignment, cannot be moved
					internal.test_case([__local7__, __local6__]) # func injection, after last head-ref assignment
					__local5__.x = 1 
				}`,
		},
		{
			note: "multiple refs in head-ref",
			module: `package test
				test_foo[tc.note][flag.note] if {
					some flag in [
						{"note": "on", "a": 1},
					]
					some tc in [
						{"note": "a", "x": 1},
					]
					flag.a == 1
					tc.x == 1
				}`,
			// var assignment can be moved up the body, func injected after last head-ref assignment
			exp: `package test
				test_foo[__local0__][__local1__] if { 
					__local8__ = [{"a": 1, "note": "on"}]
					__local4__ = __local8__[__local3__]
					__local1__ = __local4__.note                 # generated head-ref var assignment, moved up
					__local9__ = [{"note": "a", "x": 1}]
					__local7__ = __local9__[__local6__]
					__local0__ = __local7__.note                 # generated head-ref var assignment, moved up
					internal.test_case([__local0__, __local1__]) # func injection, after last head-ref assignment
					__local4__.a = 1; 
					__local7__.x = 1 
				}`,
		},
		{
			note: "multiple refs in head-ref, mixed with ground terms",
			module: `package test
				test_foo[tc.note].bar[flag.note].baz if {
					some flag in [
						{"note": "on", "a": 1},
					]
					some tc in [
						{"note": "a", "x": 1},
					]
					flag.a == 1
					tc.x == 1
				}`,
			// var assignment cannot be moved up the body, func injected last in body
			exp: `package test
				test_foo[__local0__].bar[__local1__].baz if { 
					__local8__ = [{"a": 1, "note": "on"}]
					__local4__ = __local8__[__local3__]
					__local1__ = __local4__.note                               # generated head-ref var assignment, moved up
					__local9__ = [{"note": "a", "x": 1}]
					__local7__ = __local9__[__local6__]
					__local0__ = __local7__.note                               # generated head-ref var assignment, moved up
					internal.test_case([__local0__, "bar", __local1__, "baz"]) # func injection, after last head-ref assignment
					__local4__.a = 1
					__local7__.x = 1
				}`,
		},
		{
			note: "multiple vars in head-ref, non-assignment reference in body",
			module: `package example_test
				test_sign_token[note][alg] if {
					some note, tc in {
						"claims": {
							"claims": {"foo": "bar"},
						},
						"no claims": {
							"claims": {},
						},
					}
				
					some alg in [
						"HS256",
						"HS512",
					]
				
					secret := "foobar"
					key := base64.encode(secret)
				
					token := io.jwt.encode_sign({
						"typ": "JWT",
						"alg": alg
					}, tc.claims, {
						"kty": "oct",
						"k": key
					})
				
					[valid, _, payload] := io.jwt.decode_verify(token, {"secret": secret})
					valid
					payload = tc.claims
				}`,
			exp: `package example_test
				test_sign_token[__local0__][__local4__] if { 
					__local11__ = {"claims": {"claims": {"foo": "bar"}}, "no claims": {"claims": {}}}
					__local1__ = __local11__[__local0__]
					__local12__ = ["HS256", "HS512"]
					__local4__ = __local12__[__local3__]
					internal.test_case([__local0__, __local4__])                  # func injection
					__local5__ = "foobar"; base64.encode(__local5__, __local13__)
					__local6__ = __local13__
					__local16__ = __local1__.claims
					io.jwt.encode_sign({"alg": __local4__, "typ": "JWT"}, __local16__, {"k": __local6__, "kty": "oct"}, __local14__)
					__local7__ = __local14__
					io.jwt.decode_verify(__local7__, {"secret": __local5__}, __local15__)
					[__local8__, __local9__, __local10__] = __local15__
					__local8__
					__local10__ = __local1__.claims
				}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.note, func(t *testing.T) {
			modules := map[string]*ast.Module{
				"test.rego": ast.MustParseModule(tc.module),
			}

			exp := ast.MustParseModule(tc.exp)

			c := ast.NewCompiler()
			c.WithStageAfter("RewriteLocalVars", ast.CompilerStageDefinition{
				Name:       "InjectTestCaseFunc",
				MetricName: "inject_test_case_func",
				Stage:      injectTestCaseFunc,
			})

			c.Compile(modules)
			if c.Failed() {
				t.Fatalf("Unexpected error(s): %v", c.Errors)
			}

			result := c.Modules["test.rego"]
			if !result.Equal(exp) {
				t.Fatalf("Expected:\n\n%v\n\nbut got:\n\n%v", exp, result)
			}
		})
	}
}
