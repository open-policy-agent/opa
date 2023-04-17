package oracle

import (
	"errors"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
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

p { q }

q = true`,
			pos: 100,
			exp: ErrNoMatchFound,
		},
		{
			note: "no good match - literal",
			buffer: `package test

p { q > 1 }`,
			pos: 22, // this points at the number '1'
			exp: ErrNoDefinitionFound,
		},
		{
			note: "no good match - rule name",
			buffer: `package test

p { q > 1 }`,
			pos: 14, // this points at the rule 'p'
			exp: ErrNoDefinitionFound,
		},
		{
			note: "no good match - rule whitespace",
			buffer: `package test

p { q > 1 }`,
			pos: 21, // this points at the whitespace after '>'
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

import data.foo.s
import data.foo.bar as t

p {
    q
    [r]
    s[t]
}

r = true
q = true`

	const aSecondBufferModule = `package test

p {
	q
}`

	const aThirdBufferModule = `package test

f(x) {
	input.foo[x]
}

u {
	some x
	x = 1
}

v {
	x := 1
	x < 10
}

w {
	y[i]
	i > 1
}

m {
	[i, j] = [1, 2]
	j > i
}

x = "deadbeef"
y[1]
`

	const fooModule = `package foo

s = input
bar = 7`

	// NOTE(sr): Early ref rewriting adds an expression to the rule body for `x.y`
	const varInRuleRefModule = `package foo
q[x.y] = 10 {
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
			pos: 66,
			exp: &ast.Location{
				File:   "buffer.rego",
				Row:    13,
				Col:    1,
				Offset: 97,
				Text:   []byte("q = true"),
			},
		},
		{
			note: "r - another var but embedded",
			modules: map[string]string{
				"buffer.rego": aBufferModule,
			},
			pos: 73,
			exp: &ast.Location{
				File:   "buffer.rego",
				Row:    12,
				Col:    1,
				Offset: 88,
				Text:   []byte("r = true"),
			},
		},
		{
			note: "s - reference to other module",
			modules: map[string]string{
				"buffer.rego": aBufferModule,
				"foo.rego":    fooModule,
			},
			pos: 80,
			exp: &ast.Location{
				File:   "foo.rego",
				Row:    3,
				Col:    1,
				Offset: 13,
				Text:   []byte("s = input"),
			},
		},
		{
			note: "s - reference to other module but non symbol node",
			modules: map[string]string{
				"buffer.rego": aBufferModule,
				"foo.rego":    fooModule,
			},
			pos: 81, // this refers to the '[' character following 's'--this exercises the case where position does not refer to a symbol
			exp: &ast.Location{
				File:   "foo.rego",
				Row:    3,
				Col:    1,
				Offset: 13,
				Text:   []byte("s = input"),
			},
		},
		{
			note: "s - reference to other module that is not loaded",
			modules: map[string]string{
				"buffer.rego": aBufferModule,
			},
			pos: 80,
			exp: &ast.Location{
				File:   "buffer.rego",
				Row:    3,
				Col:    8,
				Offset: 21,
				Text:   []byte("data.foo.s"),
			},
		},
		{
			note: "t - embedded ref and import alias",
			modules: map[string]string{
				"buffer.rego": aBufferModule,
				"foo.rego":    fooModule,
			},
			pos: 82,
			exp: &ast.Location{
				File:   "foo.rego",
				Row:    4,
				Col:    1,
				Offset: 17,
				Text:   []byte("bar = 7"),
			},
		},
		{
			note: "t - embedded ref and import alias without other module loaded",
			modules: map[string]string{
				"buffer.rego": aBufferModule,
			},
			pos: 82,
			exp: &ast.Location{
				File:   "buffer.rego",
				Row:    4,
				Col:    8,
				Offset: 39,
				Text:   []byte("data.foo.bar"),
			},
		},
		{
			note: "intra-package ref",
			modules: map[string]string{
				"buffer.rego": aSecondBufferModule, // use a different module that references q in main buffer module used above
				"test.rego":   aBufferModule,
			},
			pos: 19,
			exp: &ast.Location{
				File:   "test.rego",
				Row:    13,
				Col:    1,
				Offset: 97,
				Text:   []byte("q = true"),
			},
		},
		{
			note: "intra-rule: function argument",
			modules: map[string]string{
				"buffer.rego": aThirdBufferModule,
			},
			pos: 32,
			exp: &ast.Location{
				File:   "buffer.rego",
				Row:    3,
				Col:    3,
				Offset: 16,
				Text:   []byte("x"),
			},
		},
		{
			note: "intra-rule: some decl",
			modules: map[string]string{
				"buffer.rego": aThirdBufferModule,
			},
			pos: 51,
			exp: &ast.Location{
				File:   "buffer.rego",
				Row:    8,
				Col:    7,
				Offset: 48,
				Text:   []byte("x"),
			},
		},
		{
			note: "intra-rule: assignment",
			modules: map[string]string{
				"buffer.rego": aThirdBufferModule,
			},
			pos: 73,
			exp: &ast.Location{
				File:   "buffer.rego",
				Row:    13,
				Col:    2,
				Offset: 65,
				Text:   []byte("x"),
			},
		},
		{
			note: "intra-rule: ref output",
			modules: map[string]string{
				"buffer.rego": aThirdBufferModule,
			},
			pos: 94,
			exp: &ast.Location{
				File:   "buffer.rego",
				Row:    18,
				Col:    4,
				Offset: 90,
				Text:   []byte("i"),
			},
		},
		{
			note: "intra-rule: unify output",
			modules: map[string]string{
				"buffer.rego": aThirdBufferModule,
			},
			pos: 129,
			exp: &ast.Location{
				File:   "buffer.rego",
				Row:    23,
				Col:    3,
				Offset: 109,
				Text:   []byte("i"),
			},
		},
		{
			note: "intra-rule: ref head",
			modules: map[string]string{
				"buffer.rego": varInRuleRefModule,
			},
			pos: 47, // "z" in "z = 1"
			exp: &ast.Location{
				File:   "buffer.rego",
				Row:    4,
				Col:    7,
				Offset: 44,
				Text:   []byte("z"),
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
			before := tc.pos - 4
			if before < 0 {
				before = 0
			}
			after := tc.pos + 5
			if after > len(buffer) {
				after = len(buffer)
			}
			t.Logf("pos is %d: \"%s<%s>%s\"", tc.pos, buffer[before:tc.pos], string(buffer[tc.pos]), buffer[tc.pos+1:after])
			o := New()
			result, err := o.FindDefinition(DefinitionQuery{
				Modules:  modules,
				Buffer:   []byte(buffer),
				Filename: "buffer.rego",
				Pos:      tc.pos,
			})
			if err != nil {
				t.Fatal(err)
			}
			if tc.exp.Compare(result.Result) != 0 {
				var expText string
				var gotText string
				if tc.exp != nil {
					expText = string(tc.exp.Text)
				}
				if result != nil {
					gotText = string(result.Result.Text)
				}
				t.Fatalf("\n\nwant:\n\n\t%#v\n\ngot:\n\n\t%#v\n\nwant (text):\n\n\t%q\n\ngot (text):\n\n\t%q", tc.exp, result, expText, gotText)
			}
		})
	}
}

func TestFindContainingNodeStack(t *testing.T) {
	const trivial = `package test

p {
    q
    r
}

r = true
q = true`

	module := ast.MustParseModule(trivial)
	module.Package.Location = nil // unset the package location to test nil tolerance

	// offset 28 is the first 'r' variable
	result := findContainingNodeStack(module, 28)

	exp := []*ast.Location{
		module.Rules[0].Loc(),
		module.Rules[0].Body.Loc(),
		module.Rules[0].Body[1].Loc(),
		module.Rules[0].Body[1].Terms.(*ast.Term).Loc(),
	}

	if len(result) != len(exp) {
		t.Fatal("expected an exact set of location pointers but got different number:", len(result), "result:", result)
	}

	for i := 0; i < len(result); i++ {
		if result[i].Loc() != exp[i] {
			t.Fatal("expected exact location pointers but found difference on i =", i, "result:", result)
		}
	}

	// Exercise special case for bodies.
	module.Rules[0].Body[1].Location = nil
	result = findContainingNodeStack(module, 28)

	exp = []*ast.Location{
		module.Rules[0].Loc(),
	}

	if len(result) != len(exp) {
		t.Fatal("expected an exact set of location pointers but got different number:", len(result), "result:", result)
	}

	for i := 0; i < len(result); i++ {
		if result[i].Loc() != exp[i] {
			t.Fatal("expected exact location pointers but found difference on i =", i, "result:", result)
		}
	}

}

func TestCompileUptoNoModules(t *testing.T) {

	compiler, module, err := compileUpto("SetRuleTree", nil, []byte("package test\np=1"), "test.rego")
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

	compiler, module, err := compileUpto("SetRuleTree", map[string]*ast.Module{
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

	_, _, err := compileUpto("DEADBEEF", map[string]*ast.Module{
		"test.rego": ast.MustParseModule("package test\np=1"),
	}, nil, "test.rego")

	if err.Error() != "unreachable: did not halt" {
		t.Fatal("expected halt error but got:", err)
	}
}
