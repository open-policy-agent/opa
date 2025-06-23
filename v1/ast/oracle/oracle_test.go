package oracle

import (
	"errors"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/metrics"
)

func TestOracleFindDefinitionErrors(t *testing.T) {
	cases := []struct {
		note    string
		buffer  string
		modules map[string]string
		pos     int
		exp     error
	}{
		{
			note:   "buffer parse error",
			buffer: `package`,
			exp:    errors.New("buffer.rego:1: rego_parse_error: unexpected eof token"),
		},
		{
			note: "compile error",
			buffer: `package test

# NOTE(tsandall): if we relax this check then this test case becomes obsolete.
f(input)`,
			exp: errors.New("buffer.rego:4: rego_compile_error: args must not shadow input"),
		},
		{
			note: "no matching node",
			buffer: `package test
import rego.v1

p if { q }

q = true`,
			pos: 118,
			exp: ErrNoMatchFound,
		},
		{
			note: "no good match - literal",
			buffer: `package test
import rego.v1

p if { q > 1 }`,
			pos: 40, // this points at the number '1'
			exp: ErrNoDefinitionFound,
		},
		{
			note: "no good match - rule name",
			buffer: `package test
import rego.v1

p if { q > 1 }`,
			pos: 32, // this points at the rule 'p'
			exp: ErrNoDefinitionFound,
		},
		{
			note: "no good match - rule whitespace",
			buffer: `package test
import rego.v1

p if { q > 1 }`,
			pos: 39, // this points at the whitespace after '>'
			exp: ErrNoDefinitionFound,
		},
		{
			note: "no match - negative",
			buffer: `package test

p = 1`,
			pos: -100,
			exp: ErrNoMatchFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			modules := map[string]*ast.Module{}
			for k, v := range tc.modules {
				var err error
				modules[k], err = ast.ParseModule(k, v)
				if err != nil {
					t.Fatal(err)
				}
			}
			o := New()
			result, err := o.FindDefinition(DefinitionQuery{
				Modules:  modules,
				Buffer:   []byte(tc.buffer),
				Filename: "buffer.rego",
				Pos:      tc.pos,
			})
			if err == nil || result != nil {
				t.Fatal("expected error but got:", err, "result:", result)
			}
			if !strings.Contains(err.Error(), tc.exp.Error()) {
				t.Fatalf("expected %v but got %v", tc.exp, err)
			}
		})
	}
}

func TestOracleFindDefinition(t *testing.T) {
	const aBufferModule = `package test

import rego.v1
import data.foo.s
import data.foo.bar as t

p if {
    q
    [r]
    s[t]
}

r = true
q = true`

	const aSecondBufferModule = `package test
import rego.v1

p if {
	q
}`

	const aThirdBufferModule = `package test
import rego.v1

f(x) if {
	input.foo[x]
}

u if {
	some x
	x = 1
}

v if {
	x := 1
	x < 10
}

w if {
	y[i]
	i > 1
}

m if {
	[i, j] = [1, 2]
	j > i
}

x = "deadbeef"
y contains 1
`

	const fooModule = `package foo

s = input
bar = 7`

	// NOTE(sr): Early ref rewriting adds an expression to the rule body for `x.y`
	const varInRuleRefModule = `package foo
import rego.v1

q[x.y] = 10 if {
	x := input
	some z
	z = 1
}`

	cases := []struct {
		note    string
		modules map[string]string
		pos     int
		exp     *ast.Location
	}{
		{
			note: "q - a var in the body",
			modules: map[string]string{
				"buffer.rego": aBufferModule,
			},
			pos: 84,
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  14,
				Col:  1,
				Text: []byte("q = true"),
			},
		},
		{
			note: "r - another var but embedded",
			modules: map[string]string{
				"buffer.rego": aBufferModule,
			},
			pos: 91,
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  13,
				Col:  1,
				Text: []byte("r = true"),
			},
		},
		{
			note: "s - reference to other module",
			modules: map[string]string{
				"buffer.rego": aBufferModule,
				"foo.rego":    fooModule,
			},
			pos: 98,
			exp: &ast.Location{
				File: "foo.rego",
				Row:  3,
				Col:  1,
				Text: []byte("s = input"),
			},
		},
		{
			note: "s - reference to other module but non symbol node",
			modules: map[string]string{
				"buffer.rego": aBufferModule,
				"foo.rego":    fooModule,
			},
			pos: 99, // this refers to the '[' character following 's'--this exercises the case where position does not refer to a symbol
			exp: &ast.Location{
				File: "foo.rego",
				Row:  3,
				Col:  1,
				Text: []byte("s = input"),
			},
		},
		{
			note: "s - reference to other module that is not loaded",
			modules: map[string]string{
				"buffer.rego": aBufferModule,
			},
			pos: 98,
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  4,
				Col:  8,
				Text: []byte("data.foo.s"),
			},
		},
		{
			note: "some in var",
			modules: map[string]string{
				"buffer.rego": `package example

allow if {
	list := input.list
	some e in list
}`,
			},
			pos: 60,
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  4,
				Col:  2,
				Text: []byte("list"),
			},
		},
		{
			note: "some in rule",
			modules: map[string]string{
				"buffer.rego": `package example

list := [1,2,3]

allow if {
	some e in list
	e == 1
}`,
			},
			pos: 56,
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  3,
				Col:  1,
				Text: []byte("list := [1,2,3]"),
			},
		},
		{
			note: "some in rule k, v",
			modules: map[string]string{
				"buffer.rego": `package example

list := [1,2,3]

allow if {
	some k, v in list
	e == 1
}`,
			},
			pos: 59,
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  3,
				Col:  1,
				Text: []byte("list := [1,2,3]"),
			},
		},
		{
			note: "every var",
			modules: map[string]string{
				"buffer.rego": `package example

allow if {
	list := input.list
	every e in list {
		e == 1
	}
}`,
			},
			pos: 60,
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  4,
				Col:  2,
				Text: []byte("list"),
			},
		},
		{
			note: "every rule",
			modules: map[string]string{
				"buffer.rego": `package example

list := [1,2,3]

allow if {
	every e in list {
		e == 1
	}
}`,
			},
			pos: 57,
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  3,
				Col:  1,
				Text: []byte("list := [1,2,3]"),
			},
		},
		{
			note: "every in rule k, v",
			modules: map[string]string{
				"buffer.rego": `package example

list := [1,2,3]

allow if {
	every k, v in list {
		k == 1
		v == 2
	}
}`,
			},
			pos: 60,
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  3,
				Col:  1,
				Text: []byte("list := [1,2,3]"),
			},
		},
		{
			note: "t - embedded ref and import alias",
			modules: map[string]string{
				"buffer.rego": aBufferModule,
				"foo.rego":    fooModule,
			},
			pos: 100,
			exp: &ast.Location{
				File: "foo.rego",
				Row:  4,
				Col:  1,
				Text: []byte("bar = 7"),
			},
		},
		{
			note: "t - embedded ref and import alias without other module loaded",
			modules: map[string]string{
				"buffer.rego": aBufferModule,
			},
			pos: 100,
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  5,
				Col:  8,
				Text: []byte("data.foo.bar"),
			},
		},
		{
			note: "intra-package ref",
			modules: map[string]string{
				"buffer.rego": aSecondBufferModule, // use a different module that references q in main buffer module used above
				"test.rego":   aBufferModule,
			},
			pos: 37,
			exp: &ast.Location{
				File: "test.rego",
				Row:  14,
				Col:  1,
				Text: []byte("q = true"),
			},
		},
		{
			note: "intra-rule: function argument",
			modules: map[string]string{
				"buffer.rego": aThirdBufferModule,
			},
			pos: 50,
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  4,
				Col:  3,
				Text: []byte("x"),
			},
		},
		{
			note: "intra-rule: some decl",
			modules: map[string]string{
				"buffer.rego": aThirdBufferModule,
			},
			pos: 72,
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  9,
				Col:  7,
				Text: []byte("x"),
			},
		},
		{
			note: "intra-rule: assignment",
			modules: map[string]string{
				"buffer.rego": aThirdBufferModule,
			},
			pos: 97,
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  14,
				Col:  2,
				Text: []byte("x"),
			},
		},
		{
			note: "intra-rule: ref output",
			modules: map[string]string{
				"buffer.rego": aThirdBufferModule,
			},
			pos: 121,
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  19,
				Col:  4,
				Text: []byte("i"),
			},
		},
		{
			note: "intra-rule: unify output",
			modules: map[string]string{
				"buffer.rego": aThirdBufferModule,
			},
			pos: 159,
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  24,
				Col:  3,
				Text: []byte("i"),
			},
		},
		{
			note: "intra-rule: ref head",
			modules: map[string]string{
				"buffer.rego": varInRuleRefModule,
			},
			pos: 66, // "z" in "z = 1"
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  6,
				Col:  7,
				Text: []byte("z"),
			},
		},
		{
			note: "intra-rule: ref object key",
			modules: map[string]string{
				"buffer.rego": `package foo

allow if obj.key == "value"

obj := {"key": "value"}`,
			},
			pos: 22, // "o" in "obj.key"
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  5,
				Col:  1,
				Text: []byte(`obj := {"key": "value"}`),
			},
		},
		{
			note: "intra-rule: ref object key missing finds object",
			modules: map[string]string{
				"buffer.rego": `package foo

allow if obj.foobar == "value"

obj := {"key": "value"}`,
			},
			pos: 22, // "o" in "obj.foobar"
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  5,
				Col:  1,
				Text: []byte(`obj := {"key": "value"}`),
			},
		},
		{
			note: "intra-rule: ref object key finds correct head",
			modules: map[string]string{
				"buffer.rego": `package foo

allow if obj.key == "value"

obj.foobar := "value"
obj.key := "value"`,
			},
			pos: 22, // "o" in "obj.key"
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  6,
				Col:  1,
				Text: []byte(`obj.key := "value"`),
			},
		},
		{
			note: "intra-rule: ref object key non existent returns self",
			modules: map[string]string{
				"buffer.rego": `package foo

allow if bar.foo == "value"`,
			},
			pos: 22, // "b" in "bar.foo"
			exp: &ast.Location{
				File: "buffer.rego",
				Row:  3,
				Col:  10,
				Text: []byte(`bar`),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			modules := map[string]*ast.Module{}

			for k, v := range tc.modules {
				var err error
				modules[k], err = ast.ParseModule(k, v)
				if err != nil {
					t.Fatal(err)
				}
			}

			buffer := tc.modules["buffer.rego"]

			t.Logf(
				"pos is %d: \"%s<%s>%s\"",
				tc.pos,
				buffer[max(tc.pos-4, 0):tc.pos],
				string(buffer[tc.pos]),
				buffer[tc.pos+1:min(tc.pos+5, len(buffer))],
			)

			result, err := New().FindDefinition(DefinitionQuery{
				Modules:  modules,
				Buffer:   []byte(buffer),
				Filename: "buffer.rego",
				Pos:      tc.pos,
			})
			if err != nil {
				t.Fatal(err)
			}

			if !tc.exp.Equal(result.Result) {
				t.Logf("exp %q, got %q", tc.exp.Text, result.Result.Text)
				t.Errorf(`Location mismatch:
expected file=%q, row=%d, col=%d
got      file=%q, row=%d, col=%d`,
					tc.exp.File, tc.exp.Row, tc.exp.Col,
					result.Result.File, result.Result.Row, result.Result.Col)
			}

			if t.Failed() {
				showLocationContext(t, tc.modules, tc.exp, "Expected")
				showLocationContext(t, tc.modules, result.Result, "Actual")
			}
		})
	}
}

// showLocationContext is a helper that can show a row and col position within a
// buffer. e.g.
//
//	oracle_test.go:469: Col mismatch: expected 11, got 10
//	oracle_test.go:489: Expected: buffer.rego:3:11
//	      allow if bar.foo == "value"
//	                ^
//	oracle_test.go:489: Actual: buffer.rego:3:10
//	      allow if bar.foo == "value"
//	               ^
func showLocationContext(t *testing.T, modules map[string]string, loc *ast.Location, label string) {
	t.Helper()
	if content, exists := modules[loc.File]; exists {
		lines := strings.Split(content, "\n")
		if loc.Row > 0 && loc.Row <= len(lines) {
			line := lines[loc.Row-1]
			marker := strings.Repeat(" ", max(0, loc.Col-1)) + "^"
			t.Logf("%s: %s:%d:%d\n  %s\n  %s", label, loc.File, loc.Row, loc.Col, line, marker)
		}
	}
}

func TestFindContainingNodeStack(t *testing.T) {
	const trivial = `package test
import rego.v1

p if {
    q
    r
}

r = true
q = true`

	module := ast.MustParseModule(trivial)
	module.Package.Location = nil // unset the package location to test nil tolerance

	// offset 46 is the first 'r' variable
	result := findContainingNodeStack(module, 46)

	exp := []*ast.Location{
		module.Rules[0].Loc(),
		module.Rules[0].Body.Loc(),
		module.Rules[0].Body[1].Loc(),
		module.Rules[0].Body[1].Terms.(*ast.Term).Loc(),
	}

	if len(result) != len(exp) {
		t.Fatal("expected an exact set of location pointers but got different number:", len(result), "result:", result)
	}

	for i := range result {
		if result[i].Loc() != exp[i] {
			t.Fatal("expected exact location pointers but found difference on i =", i, "result:", result)
		}
	}

	// Exercise special case for bodies.
	module.Rules[0].Body[1].Location = nil
	result = findContainingNodeStack(module, 46)

	exp = []*ast.Location{
		module.Rules[0].Loc(),
	}

	if len(result) != len(exp) {
		t.Fatal("expected an exact set of location pointers but got different number:", len(result), "result:", result)
	}

	for i := range result {
		if result[i].Loc() != exp[i] {
			t.Fatal("expected exact location pointers but found difference on i =", i, "result:", result)
		}
	}
}

func TestCompileUptoNoModules(t *testing.T) {
	compiler, module, err := New().compileUpto("SetRuleTree", nil, []byte("package test\np=1"), "test.rego")
	if err != nil {
		t.Fatal(err)
	}

	rules := compiler.GetRulesExact(ast.MustParseRef("data.test.p"))
	if len(rules) != 1 {
		t.Fatal("unexpected rules:", rules)
	}

	if module == nil {
		t.Fatal("expected parsed module")
	}
}

func TestCompileUptoNoBuffer(t *testing.T) {
	compiler, module, err := New().compileUpto("SetRuleTree", map[string]*ast.Module{
		"test.rego": ast.MustParseModule("package test\np=1"),
	}, nil, "test.rego")
	if err != nil {
		t.Fatal(err)
	}

	rules := compiler.GetRulesExact(ast.MustParseRef("data.test.p"))
	if len(rules) != 1 {
		t.Fatal("unexpected rules:", rules)
	}

	if module == nil {
		t.Fatal("expected parsed module")
	}
}

func TestCompileUptoBadStageName(t *testing.T) {
	_, _, err := New().compileUpto("DEADBEEF", map[string]*ast.Module{
		"test.rego": ast.MustParseModule("package test\np=1"),
	}, nil, "test.rego")

	if err.Error() != "unreachable: did not halt" {
		t.Fatal("expected halt error but got:", err)
	}
}

func TestUsingCustomCompiler(t *testing.T) {
	m := metrics.New()
	o := New().WithCompiler(ast.NewCompiler().WithMetrics(m))
	q := DefinitionQuery{Modules: map[string]*ast.Module{"test.rego": ast.MustParseModule("package test\np=1")}}

	if _, err := o.FindDefinition(q); !errors.Is(err, ErrNoMatchFound) {
		t.Fatal("expected no definition found error but got:", err)
	}

	// Ensure metrics set on the custom compiler have been updated
	if m.Timer("compile_stage_check_imports").Int64() == 0 {
		t.Fatal("expected metrics to be updated")
	}
}
