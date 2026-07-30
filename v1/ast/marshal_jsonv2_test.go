// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build go1.27

package ast

import (
	"encoding/json/v2"
	"strings"
	"testing"

	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
	"github.com/open-policy-agent/opa/v1/util"
)

func resetJSONOptions() {
	astJSON.SetOptions(astJSON.Defaults())
}

func TestGeneric_MarshalWithLocationJSONOptions(t *testing.T) {
	testCases := map[string]struct {
		Term         *Term
		Options      astJSON.Options
		ExpectedJSON string
	}{
		"base case, no location options set": {
			Term: func() *Term {
				v, _ := InterfaceToValue("example")
				return &Term{
					Value:    v,
					Location: NewLocation([]byte{}, "example.rego", 1, 2),
				}
			}(),
			ExpectedJSON: `{"type":"string","value":"example"}`,
		},
		"location included, location text excluded": {
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{
						Term: true,
					},
					IncludeLocationText: false,
				},
			},
			Term: func() *Term {
				v, _ := InterfaceToValue("example")
				return &Term{
					Value:    v,
					Location: NewLocation([]byte{}, "example.rego", 1, 2),
				}
			}(),
			ExpectedJSON: `{"location":{"file":"example.rego","row":1,"col":2},"type":"string","value":"example"}`,
		},
		"location included, location text also included": {
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{
						Term: true,
					},
					IncludeLocationText: true,
				},
			},
			Term: func() *Term {
				v, _ := InterfaceToValue("example")
				t := &Term{
					Value:    v,
					Location: NewLocation([]byte("things"), "example.rego", 1, 2),
				}
				return t
			}(),
			ExpectedJSON: `{"location":{"file":"example.rego","row":1,"col":2,"text":"dGhpbmdz"},"type":"string","value":"example"}`,
		},
		"location included, location text included, file excluded": {
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{
						Term: true,
					},
					IncludeLocationText: true,
					ExcludeLocationFile: true,
				},
			},
			Term: func() *Term {
				v, _ := InterfaceToValue("example")
				t := &Term{
					Value:    v,
					Location: NewLocation([]byte("things"), "example.rego", 1, 2),
				}
				return t
			}(),
			ExpectedJSON: `{"location":{"row":1,"col":2,"text":"dGhpbmdz"},"type":"string","value":"example"}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			astJSON.SetOptions(data.Options)
			t.Cleanup(resetJSONOptions)

			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(data.Term))
		})
	}
}

func TestTerm_MarshalJSON(t *testing.T) {
	testCases := map[string]struct {
		Term         *Term
		Options      astJSON.Options
		ExpectedJSON string
	}{
		"base case": {
			Term: func() *Term {
				v, _ := InterfaceToValue("example")
				return &Term{
					Value:    v,
					Location: NewLocation([]byte{}, "example.rego", 1, 2),
				}
			}(),
			ExpectedJSON: `{"type":"string","value":"example"}`,
		},
		"ref with no parts": {
			Term:         RefTerm(),
			ExpectedJSON: `{"type":"ref","value":null}`,
		},
		"location excluded": {
			Term: func() *Term {
				v, _ := InterfaceToValue("example")
				return &Term{
					Value:    v,
					Location: NewLocation([]byte{}, "example.rego", 1, 2),
				}
			}(),
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{
						Term: false,
					},
				},
			},
			ExpectedJSON: `{"type":"string","value":"example"}`,
		},
		"location included": {
			Term: func() *Term {
				v, _ := InterfaceToValue("example")
				return &Term{
					Value:    v,
					Location: NewLocation([]byte{}, "example.rego", 1, 2),
				}
			}(),
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{
						Term: true,
					},
				},
			},
			ExpectedJSON: `{"location":{"file":"example.rego","row":1,"col":2},"type":"string","value":"example"}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			astJSON.SetOptions(data.Options)
			t.Cleanup(resetJSONOptions)

			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(data.Term))
		})
	}
}

func TestTerm_UnmarshalJSON(t *testing.T) {
	testCases := map[string]struct {
		JSON         string
		ExpectedTerm *Term
	}{
		"base case": {
			JSON: `{"type":"string","value":"example"}`,
			ExpectedTerm: func() *Term {
				v, _ := InterfaceToValue("example")
				return &Term{
					Value: v,
				}
			}(),
		},
		"location case": {
			JSON: `{"location":{"file":"example.rego","row":1,"col":2},"type":"string","value":"example"}`,
			ExpectedTerm: func() *Term {
				v, _ := InterfaceToValue("example")
				return &Term{
					Value:    v,
					Location: NewLocation([]byte{}, "example.rego", 1, 2),
				}
			}(),
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			var term Term
			err := json.Unmarshal([]byte(data.JSON), &term)
			if err != nil {
				t.Fatal(err)
			}

			if !term.Equal(data.ExpectedTerm) {
				t.Fatalf("expected:\n%#v got\n%#v", data.ExpectedTerm, term)
			}
			if data.ExpectedTerm.Location != nil {
				if !term.Location.Equal(data.ExpectedTerm.Location) {
					t.Fatalf("expected location:\n%#v got\n%#v", data.ExpectedTerm, term)
				}
			}
		})
	}
}

func TestPackage_MarshalJSON(t *testing.T) {
	testCases := map[string]struct {
		Package      *Package
		Options      astJSON.Options
		ExpectedJSON string
	}{
		"base case": {
			Package: &Package{
				Path: EmptyRef(),
			},
			ExpectedJSON: `{"path":[]}`,
		},
		"location excluded": {
			Package: &Package{
				Path:     EmptyRef(),
				Location: NewLocation([]byte{}, "example.rego", 1, 2),
			},
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{
						Package: false,
					},
				},
			},
			ExpectedJSON: `{"path":[]}`,
		},
		"location included": {
			Package: &Package{
				Path:     EmptyRef(),
				Location: NewLocation([]byte{}, "example.rego", 1, 2),
			},
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{
						Package: true,
					},
				},
			},
			ExpectedJSON: `{"location":{"file":"example.rego","row":1,"col":2},"path":[]}`,
		},
		"location included, but nil": {
			Package: &Package{
				Path: EmptyRef(),
			},
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{
						Package: true,
					},
				},
			},
			ExpectedJSON: `{"path":[]}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			astJSON.SetOptions(data.Options)
			t.Cleanup(resetJSONOptions)

			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(data.Package))
		})
	}
}

// TestModule_MarshalJSON_PackageScopedAnnotations asserts that package-scoped
// annotations are only emitted in the module's annotations list, and never
// nested under the package object.
func TestModule_MarshalJSON_PackageScopedAnnotations(t *testing.T) {
	module := &Module{
		Package:     MustParsePackage("package foo"),
		Annotations: []*Annotations{{Scope: "package", Title: "pkg"}},
	}

	exp := `{"package":{"path":[{"type":"var","value":"data"},{"type":"string","value":"foo"}]},` +
		`"annotations":[{"scope":"package","title":"pkg"}]}`

	assertJsonEqual(t, exp, util.MustMarshalJSON(module))
}

// TODO: Comment has inconsistent JSON field names starting with an upper case letter. Comment Location is
// also always included for legacy reasons
func TestComment_MarshalJSON(t *testing.T) {
	testCases := map[string]struct {
		Comment      *Comment
		Options      astJSON.Options
		ExpectedJSON string
	}{
		"base case": {
			Comment: &Comment{
				Text: []byte("comment"),
			},
			ExpectedJSON: `{"Text":"Y29tbWVudA==","Location":null}`,
		},
		"location excluded, still included for legacy reasons": {
			Comment: &Comment{
				Text:     []byte("comment"),
				Location: NewLocation([]byte{}, "example.rego", 1, 2),
			},
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{
						Comment: false, // ignored
					},
				},
			},
			ExpectedJSON: `{"Text":"Y29tbWVudA==","Location":{"file":"example.rego","row":1,"col":2}}`,
		},
		"location included": {
			Comment: &Comment{
				Text:     []byte("comment"),
				Location: NewLocation([]byte{}, "example.rego", 1, 2),
			},
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{
						Comment: true, // ignored
					},
				},
			},
			ExpectedJSON: `{"Text":"Y29tbWVudA==","Location":{"file":"example.rego","row":1,"col":2}}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			astJSON.SetOptions(data.Options)
			t.Cleanup(resetJSONOptions)

			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(data.Comment))
		})
	}
}

func TestImport_MarshalJSON(t *testing.T) {
	testCases := map[string]struct {
		Import       *Import
		Options      astJSON.Options
		ExpectedJSON string
	}{
		"base case": {
			Import: func() *Import {
				v, _ := InterfaceToValue("example")
				term := Term{
					Value:    v,
					Location: NewLocation([]byte{}, "example.rego", 1, 2),
				}
				return &Import{Path: &term}
			}(),
			ExpectedJSON: `{"path":{"type":"string","value":"example"}}`,
		},
		"location excluded": {
			Import: func() *Import {
				v, _ := InterfaceToValue("example")
				term := Term{
					Value:    v,
					Location: NewLocation([]byte{}, "example.rego", 1, 2),
				}
				return &Import{
					Path:     &term,
					Location: NewLocation([]byte{}, "example.rego", 1, 2),
				}
			}(),
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{
						Import: false,
					},
				},
			},
			ExpectedJSON: `{"path":{"type":"string","value":"example"}}`,
		},
		"location included": {
			Import: func() *Import {
				v, _ := InterfaceToValue("example")
				term := Term{
					Value:    v,
					Location: NewLocation([]byte{}, "example.rego", 1, 2),
				}
				return &Import{
					Path:     &term,
					Location: NewLocation([]byte{}, "example.rego", 1, 2),
				}
			}(),
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{
						Import: true,
					},
				},
			},
			ExpectedJSON: `{"location":{"file":"example.rego","row":1,"col":2},"path":{"type":"string","value":"example"}}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			astJSON.SetOptions(data.Options)
			t.Cleanup(resetJSONOptions)

			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(data.Import))
		})
	}
}

func TestRule_MarshalJSON(t *testing.T) {
	rawModule := `
	package foo

	# comment

	allow if { true }
	`

	module, err := ParseModuleWithOpts("example.rego", rawModule, ParserOptions{AllFutureKeywords: true})
	if err != nil {
		t.Fatal(err)
	}

	rule := module.Rules[0]

	testCases := map[string]struct {
		Rule         *Rule
		Options      astJSON.Options
		ExpectedJSON string
	}{
		"base case": {
			Rule:         rule,
			ExpectedJSON: `{"body":[{"index":0,"terms":{"type":"boolean","value":true}}],"head":{"name":"allow","value":{"type":"boolean","value":true},"ref":[{"type":"var","value":"allow"}]}}`,
		},
		"location excluded": {
			Rule: rule,
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{
						Rule: false,
					},
				},
			},
			ExpectedJSON: `{"body":[{"index":0,"terms":{"type":"boolean","value":true}}],"head":{"name":"allow","value":{"type":"boolean","value":true},"ref":[{"type":"var","value":"allow"}]}}`,
		},
		"location included": {
			Rule: rule,
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{
						Rule: true,
					},
				},
			},
			ExpectedJSON: `{"body":[{"index":0,"terms":{"type":"boolean","value":true}}],"head":{"name":"allow","value":{"type":"boolean","value":true},"ref":[{"type":"var","value":"allow"}]},"location":{"file":"example.rego","row":6,"col":2}}`,
		},
		"annotations included": {
			Rule: func() *Rule {
				r := rule.Copy()
				r.Annotations = []*Annotations{{
					Scope:         "rule",
					Title:         "My rule",
					Entrypoint:    true,
					Organizations: []string{"org1"},
					Description:   "My desc",
					Custom: map[string]any{
						"foo": "bar",
					}}}
				return r
			}(),
			ExpectedJSON: `{"annotations":[{"custom":{"foo":"bar"},"description":"My desc","entrypoint":true,"organizations":["org1"],"scope":"rule","title":"My rule"}],"body":[{"index":0,"terms":{"type":"boolean","value":true}}],"head":{"name":"allow","value":{"type":"boolean","value":true},"ref":[{"type":"var","value":"allow"}]}}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			astJSON.SetOptions(data.Options)
			t.Cleanup(resetJSONOptions)

			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(data.Rule))
		})
	}
}

func TestHead_MarshalJSON(t *testing.T) {
	rawModule := `
	package foo

	# comment

	allow if { true }
	`

	module, err := ParseModuleWithOpts("example.rego", rawModule, ParserOptions{AllFutureKeywords: true})
	if err != nil {
		t.Fatal(err)
	}

	head := module.Rules[0].Head

	testCases := map[string]struct {
		Head         *Head
		Options      astJSON.Options
		ExpectedJSON string
	}{
		"base case": {
			Head:         head.Copy(),
			ExpectedJSON: `{"name":"allow","value":{"type":"boolean","value":true},"ref":[{"type":"var","value":"allow"}]}`,
		},
		"location excluded": {
			Head: head,
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{
						Head: false,
					},
				},
			},
			ExpectedJSON: `{"name":"allow","value":{"type":"boolean","value":true},"ref":[{"type":"var","value":"allow"}]}`,
		},
		"location included": {
			Head: head,
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{
						Head: true,
					},
				},
			},
			ExpectedJSON: `{"name":"allow","value":{"type":"boolean","value":true},"ref":[{"type":"var","value":"allow"}],"location":{"file":"example.rego","row":6,"col":2}}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			astJSON.SetOptions(data.Options)
			t.Cleanup(resetJSONOptions)

			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(data.Head))
		})
	}
}

func TestRuleHeadRefWithTermLocations_MarshalJSON(t *testing.T) {
	policy := `package test

import rego.v1

ref.head[rule].test contains "value" if {
	rule := "rule"
}`

	astJSON.SetOptions(astJSON.Options{
		MarshalOptions: astJSON.MarshalOptions{
			IncludeLocation: astJSON.NodeToggle{
				Head: true,
				Term: true,
			},
		},
	})
	t.Cleanup(resetJSONOptions)

	module, err := ParseModuleWithOpts("test.rego", policy, ParserOptions{})
	if err != nil {
		t.Fatal(err)
	}

	bs, err := json.Marshal(module.Rules[0].Head)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure marshalled JSON includes location for any term
	expectedJSON := `{"key":{"location":{"file":"test.rego","row":5,"col":30},"type":"string","value":"value"},"ref":[{"location":{"file":"test.rego","row":5,"col":1},"type":"var","value":"ref"},{"location":{"file":"test.rego","row":5,"col":5},"type":"string","value":"head"},{"location":{"file":"test.rego","row":5,"col":10},"type":"var","value":"rule"},{"location":{"file":"test.rego","row":5,"col":16},"type":"string","value":"test"}],"location":{"file":"test.rego","row":5,"col":1}}`

	assertJsonEqual(t, expectedJSON, bs)
}

func TestExpr_MarshalJSON(t *testing.T) {
	rawModule := `
	package foo

	# comment

	allow if { true }
	`

	module, err := ParseModuleWithOpts("example.rego", rawModule, ParserOptions{AllFutureKeywords: true})
	if err != nil {
		t.Fatal(err)
	}

	expr := module.Rules[0].Body[0]

	testCases := map[string]struct {
		Expr         *Expr
		Options      astJSON.Options
		ExpectedJSON string
	}{
		"base case": {
			Expr:         expr,
			ExpectedJSON: `{"index":0,"terms":{"type":"boolean","value":true}}`,
		},
		"nil terms slice": {
			Expr:         &Expr{Terms: []*Term(nil)},
			ExpectedJSON: `{"index":0,"terms":null}`,
		},
		"location excluded": {
			Expr: expr,
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{
						Expr: false,
					},
				},
			},
			ExpectedJSON: `{"index":0,"terms":{"type":"boolean","value":true}}`,
		},
		"location included": {
			Expr: expr,
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{
						Expr: true,
					},
				},
			},
			ExpectedJSON: `{"index":0,"location":{"file":"example.rego","row":6,"col":13},"terms":{"type":"boolean","value":true}}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			astJSON.SetOptions(data.Options)
			t.Cleanup(resetJSONOptions)

			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(data.Expr))
		})
	}
}

func TestExpr_UnmarshalJSON(t *testing.T) {
	rawModule := `
	package foo

	# comment

	allow if { true }
	`

	module, err := ParseModuleWithOpts("example.rego", rawModule, ParserOptions{AllFutureKeywords: true})
	if err != nil {
		t.Fatal(err)
	}

	expr := module.Rules[0].Body[0]
	// text is not marshalled to JSON so we just drop it in our examples
	expr.Location.Text = nil

	testCases := map[string]struct {
		JSON         string
		ExpectedExpr *Expr
	}{
		"base case": {
			JSON: `{"index":0,"terms":{"type":"boolean","value":true}}`,
			ExpectedExpr: func() *Expr {
				e := expr.Copy()
				e.Location = nil
				return e
			}(),
		},
		"location case": {
			JSON:         `{"index":0,"location":{"file":"example.rego","row":6,"col":13},"terms":{"type":"boolean","value":true}}`,
			ExpectedExpr: expr,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			var expr Expr
			err := json.Unmarshal([]byte(data.JSON), &expr)
			if err != nil {
				t.Fatal(err)
			}

			if !expr.Equal(data.ExpectedExpr) {
				t.Fatalf("expected:\n%#v got\n%#v", data.ExpectedExpr, expr)
			}
			if data.ExpectedExpr.Location != nil {
				if !expr.Location.Equal(data.ExpectedExpr.Location) {
					t.Fatalf("expected location:\n%#v got\n%#v", data.ExpectedExpr.Location, expr.Location)
				}
			}
		})
	}
}

func TestCall_MarshalJSON(t *testing.T) {
	testCases := map[string]struct {
		Call         Call
		ExpectedJSON string
	}{
		"base case": {
			Call:         Call{VarTerm("eq"), NumberTerm("1")},
			ExpectedJSON: `[{"type":"var","value":"eq"},{"type":"number","value":1}]`,
		},
		"nil call": {
			Call:         Call(nil),
			ExpectedJSON: `null`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(data.Call))
		})
	}
}

func TestSomeDecl_MarshalJSON(t *testing.T) {
	v, _ := InterfaceToValue("example")
	term := &Term{
		Value:    v,
		Location: NewLocation([]byte{}, "example.rego", 1, 2),
	}

	testCases := map[string]struct {
		SomeDecl     *SomeDecl
		Options      astJSON.Options
		ExpectedJSON string
	}{
		"base case": {
			SomeDecl: &SomeDecl{
				Symbols:  []*Term{term},
				Location: NewLocation([]byte{}, "example.rego", 1, 2),
			},
			ExpectedJSON: `{"symbols":[{"type":"string","value":"example"}]}`,
		},
		"nil symbols": {
			SomeDecl:     &SomeDecl{},
			ExpectedJSON: `{"symbols":null}`,
		},
		"location excluded": {
			SomeDecl: &SomeDecl{
				Symbols:  []*Term{term},
				Location: NewLocation([]byte{}, "example.rego", 1, 2),
			},
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{IncludeLocation: astJSON.NodeToggle{SomeDecl: false}},
			},
			ExpectedJSON: `{"symbols":[{"type":"string","value":"example"}]}`,
		},
		"location included": {
			SomeDecl: &SomeDecl{
				Symbols:  []*Term{term},
				Location: NewLocation([]byte{}, "example.rego", 1, 2),
			},
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{IncludeLocation: astJSON.NodeToggle{SomeDecl: true}},
			},
			ExpectedJSON: `{"location":{"file":"example.rego","row":1,"col":2},"symbols":[{"type":"string","value":"example"}]}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			astJSON.SetOptions(data.Options)
			t.Cleanup(resetJSONOptions)

			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(data.SomeDecl))
		})
	}
}

func TestEvery_MarshalJSON(t *testing.T) {

	rawModule := `
package foo

allow if {
	every e in [1,2,3] {
		e == 1
    }
}
`

	module, err := ParseModuleWithOpts("example.rego", rawModule, ParserOptions{AllFutureKeywords: true})
	if err != nil {
		t.Fatal(err)
	}

	every, ok := module.Rules[0].Body[0].Terms.(*Every)
	if !ok {
		t.Fatal("expected every term")
	}

	testCases := map[string]struct {
		Every        *Every
		Options      astJSON.Options
		ExpectedJSON string
	}{
		"base case": {
			Every:        every,
			ExpectedJSON: `{"body":[{"index":0,"terms":[{"type":"ref","value":[{"type":"var","value":"equal"}]},{"type":"var","value":"e"},{"type":"number","value":1}]}],"domain":{"type":"array","value":[{"type":"number","value":1},{"type":"number","value":2},{"type":"number","value":3}]},"key":null,"value":{"type":"var","value":"e"}}`,
		},
		"location excluded": {
			Every: every,
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{IncludeLocation: astJSON.NodeToggle{Every: false}},
			},
			ExpectedJSON: `{"body":[{"index":0,"terms":[{"type":"ref","value":[{"type":"var","value":"equal"}]},{"type":"var","value":"e"},{"type":"number","value":1}]}],"domain":{"type":"array","value":[{"type":"number","value":1},{"type":"number","value":2},{"type":"number","value":3}]},"key":null,"value":{"type":"var","value":"e"}}`,
		},
		"location included": {
			Every:        every,
			Options:      astJSON.Options{MarshalOptions: astJSON.MarshalOptions{IncludeLocation: astJSON.NodeToggle{Every: true}}},
			ExpectedJSON: `{"body":[{"index":0,"terms":[{"type":"ref","value":[{"type":"var","value":"equal"}]},{"type":"var","value":"e"},{"type":"number","value":1}]}],"domain":{"type":"array","value":[{"type":"number","value":1},{"type":"number","value":2},{"type":"number","value":3}]},"key":null,"location":{"file":"example.rego","row":5,"col":2},"value":{"type":"var","value":"e"}}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			astJSON.SetOptions(data.Options)
			t.Cleanup(resetJSONOptions)

			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(data.Every))
		})
	}
}

func TestWith_MarshalJSON(t *testing.T) {

	rawModule := `
package foo

a if {input}

b if {
	a with input as 1
}
`

	module, err := ParseModuleWithOpts("example.rego", rawModule, ParserOptions{AllFutureKeywords: true})
	if err != nil {
		t.Fatal(err)
	}

	with := module.Rules[1].Body[0].With[0]

	testCases := map[string]struct {
		Options      astJSON.Options
		ExpectedJSON string
	}{
		"base case": {
			ExpectedJSON: `{"target":{"type":"ref","value":[{"type":"var","value":"input"}]},"value":{"type":"number","value":1}}`,
		},
		"location excluded": {
			Options:      astJSON.Options{MarshalOptions: astJSON.MarshalOptions{IncludeLocation: astJSON.NodeToggle{With: false}}},
			ExpectedJSON: `{"target":{"type":"ref","value":[{"type":"var","value":"input"}]},"value":{"type":"number","value":1}}`,
		},
		"location included": {
			Options:      astJSON.Options{MarshalOptions: astJSON.MarshalOptions{IncludeLocation: astJSON.NodeToggle{With: true}}},
			ExpectedJSON: `{"location":{"file":"example.rego","row":7,"col":4},"target":{"type":"ref","value":[{"type":"var","value":"input"}]},"value":{"type":"number","value":1}}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			astJSON.SetOptions(data.Options)
			t.Cleanup(resetJSONOptions)

			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(with))
		})
	}
}

func TestAnnotations_MarshalJSON(t *testing.T) {
	testCases := map[string]struct {
		Annotations  *Annotations
		Options      astJSON.Options
		ExpectedJSON string
	}{
		"base case": {
			Annotations: &Annotations{
				Scope:         "rule",
				Title:         "My rule",
				Entrypoint:    true,
				Organizations: []string{"org1"},
				Description:   "My desc",
				Custom: map[string]any{
					"foo": "bar",
				},
				Location: NewLocation([]byte{}, "example.rego", 1, 4),
			},
			ExpectedJSON: `{"custom":{"foo":"bar"},"description":"My desc","entrypoint":true,"organizations":["org1"],"scope":"rule","title":"My rule"}`,
		},
		"location excluded": {
			Annotations: &Annotations{
				Scope:         "rule",
				Title:         "My rule",
				Entrypoint:    true,
				Organizations: []string{"org1"},
				Description:   "My desc",
				Custom: map[string]any{
					"foo": "bar",
				},
				Location: NewLocation([]byte{}, "example.rego", 1, 4),
			},
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{Annotations: false},
				},
			},
			ExpectedJSON: `{"custom":{"foo":"bar"},"description":"My desc","entrypoint":true,"organizations":["org1"],"scope":"rule","title":"My rule"}`,
		},
		"location included": {
			Annotations: &Annotations{
				Scope:         "rule",
				Title:         "My rule",
				Entrypoint:    true,
				Organizations: []string{"org1"},
				Description:   "My desc",
				Custom: map[string]any{
					"foo": "bar",
				},
				Location: NewLocation([]byte{}, "example.rego", 1, 4),
			},
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{Annotations: true},
				},
			},
			ExpectedJSON: `{"custom":{"foo":"bar"},"description":"My desc","entrypoint":true,"location":{"file":"example.rego","row":1,"col":4},"organizations":["org1"],"scope":"rule","title":"My rule"}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			astJSON.SetOptions(data.Options)
			t.Cleanup(resetJSONOptions)

			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(data.Annotations))
		})
	}
}

func TestAnnotations_MarshalJSON_Compile(t *testing.T) {
	// Regression: Annotations.MarshalJSON used to silently drop the
	// `Compile` field even though the struct tag is `compile,omitempty`.
	// Default-reflection unmarshal still reads `compile`, so the round-trip
	// was asymmetric until this was fixed.
	a := &Annotations{
		Scope: "rule",
		Compile: &CompileAnnotation{
			Unknowns: []Ref{MustParseRef("input.x"), MustParseRef("input.y")},
			MaskRule: MustParseRef("data.policy.mask"),
		},
	}

	bs, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(bs, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	compile, ok := got["compile"].(map[string]any)
	if !ok {
		t.Fatalf("expected `compile` key in marshaled output, got: %s", bs)
	}
	if _, ok := compile["unknowns"].([]any); !ok {
		t.Errorf("expected compile.unknowns to be a JSON array, got: %v", compile["unknowns"])
	}
	if _, ok := compile["mask_rule"].([]any); !ok {
		t.Errorf("expected compile.mask_rule to be a JSON array (Ref), got: %v", compile["mask_rule"])
	}

	// nil Compile should not emit the key (`omitempty` semantics).
	a.Compile = nil
	bs, err = json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal nil compile: %v", err)
	}
	if strings.Contains(string(bs), "compile") {
		t.Errorf("expected nil Compile to be omitted, got: %s", bs)
	}
}

func TestAnnotationsRef_MarshalJSON(t *testing.T) {
	testCases := map[string]struct {
		AnnotationsRef *AnnotationsRef
		Options        astJSON.Options
		ExpectedJSON   string
	}{
		"base case": {
			AnnotationsRef: &AnnotationsRef{
				Path: []*Term{},
				// using an empty annotations object here since Annotations marshalling is tested separately
				Annotations: &Annotations{},
				Location:    NewLocation([]byte{}, "example.rego", 1, 4),
			},
			ExpectedJSON: `{"annotations":{"scope":""},"path":[]}`,
		},
		"location excluded": {
			AnnotationsRef: &AnnotationsRef{
				Path:        []*Term{},
				Annotations: &Annotations{},
				Location:    NewLocation([]byte{}, "example.rego", 1, 4),
			},
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{AnnotationsRef: false},
				},
			},
			ExpectedJSON: `{"annotations":{"scope":""},"path":[]}`,
		},
		"location included": {
			AnnotationsRef: &AnnotationsRef{
				Path:        []*Term{},
				Annotations: &Annotations{},
				Location:    NewLocation([]byte{}, "example.rego", 1, 4),
			},
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{AnnotationsRef: true},
				},
			},
			ExpectedJSON: `{"annotations":{"scope":""},"location":{"file":"example.rego","row":1,"col":4},"path":[]}`,
		},
		"no annotations, location included": {
			AnnotationsRef: &AnnotationsRef{
				Path:     []*Term{},
				Location: NewLocation([]byte{}, "example.rego", 1, 4),
			},
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{AnnotationsRef: true},
				},
			},
			ExpectedJSON: `{"location":{"file":"example.rego","row":1,"col":4},"path":[]}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			astJSON.SetOptions(data.Options)
			t.Cleanup(resetJSONOptions)

			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(data.AnnotationsRef))
		})
	}
}

func TestNewAnnotationsRef_JSONOptions(t *testing.T) {
	tests := []struct {
		note        string
		module      string
		expected    []string
		options     ParserOptions
		jsonOptions astJSON.Options
	}{
		{
			note: "all JSON marshaller options set to true",
			module: `# METADATA
# title: pkg
# description: pkg
# organizations:
# - pkg
# related_resources:
# - https://pkg
# authors:
# - pkg
# schemas:
# - input.foo: {"type": "boolean"}
# custom:
#  pkg: pkg
package test

# METADATA
# scope: document
# title: doc
# description: doc
# organizations:
# - doc
# related_resources:
# - https://doc
# authors:
# - doc
# schemas:
# - input.bar: {"type": "integer"}
# custom:
#  doc: doc

# METADATA
# title: rule
# description: rule
# organizations:
# - rule
# related_resources:
# - https://rule
# authors:
# - rule
# schemas:
# - input.baz: {"type": "string"}
# custom:
#  rule: rule
p = 1`,
			options: ParserOptions{
				ProcessAnnotation: true,
			},
			jsonOptions: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{
					IncludeLocation: astJSON.NodeToggle{
						Term:           true,
						Package:        true,
						Comment:        true,
						Import:         true,
						Rule:           true,
						Head:           true,
						Expr:           true,
						SomeDecl:       true,
						Every:          true,
						With:           true,
						Annotations:    true,
						AnnotationsRef: true,
					},
				},
			},
			expected: []string{
				`{"annotations":{"authors":[{"name":"pkg"}],"custom":{"pkg":"pkg"},"description":"pkg","location":{"file":"","row":1,"col":1},"organizations":["pkg"],"related_resources":[{"ref":"https://pkg"}],"schemas":[{"path":[{"type":"var","value":"input"},{"type":"string","value":"foo"}],"definition":{"type":"boolean"}}],"scope":"package","title":"pkg"},"location":{"file":"","row":14,"col":1},"path":[{"location":{"file":"","row":14,"col":9},"type":"var","value":"data"},{"location":{"file":"","row":14,"col":9},"type":"string","value":"test"}]}`,
				`{"annotations":{"authors":[{"name":"doc"}],"custom":{"doc":"doc"},"description":"doc","location":{"file":"","row":16,"col":1},"organizations":["doc"],"related_resources":[{"ref":"https://doc"}],"schemas":[{"path":[{"type":"var","value":"input"},{"type":"string","value":"bar"}],"definition":{"type":"integer"}}],"scope":"document","title":"doc"},"location":{"file":"","row":44,"col":1},"path":[{"location":{"file":"","row":14,"col":9},"type":"var","value":"data"},{"location":{"file":"","row":14,"col":9},"type":"string","value":"test"},{"location":{"file":"","row":44,"col":1},"type":"string","value":"p"}]}`,
				`{"annotations":{"authors":[{"name":"rule"}],"custom":{"rule":"rule"},"description":"rule","location":{"file":"","row":31,"col":1},"organizations":["rule"],"related_resources":[{"ref":"https://rule"}],"schemas":[{"path":[{"type":"var","value":"input"},{"type":"string","value":"baz"}],"definition":{"type":"string"}}],"scope":"rule","title":"rule"},"location":{"file":"","row":44,"col":1},"path":[{"location":{"file":"","row":14,"col":9},"type":"var","value":"data"},{"location":{"file":"","row":14,"col":9},"type":"string","value":"test"},{"location":{"file":"","row":44,"col":1},"type":"string","value":"p"}]}`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			astJSON.SetOptions(tc.jsonOptions)
			t.Cleanup(resetJSONOptions)

			module := MustParseModuleWithOpts(tc.module, tc.options)

			if len(tc.expected) != len(module.Annotations) {
				t.Fatalf("expected %d annotations got %d", len(tc.expected), len(module.Annotations))
			}

			for i, a := range module.Annotations {
				assertJsonEqual(t,
					tc.expected[i],
					util.MustMarshalJSON(NewAnnotationsRef(a)),
				)
			}

		})
	}
}

func TestNot_MarshalJSON(t *testing.T) {
	rawModule := `
		package test
		
		import future.keywords.not
		
		implicit_body if {
			not input.x + 2 == 42
		}

		explicit_body if {
			not {
				x := input.x
				y := 2
				z := x + y
				z == 42
			}
		}
	`

	module, err := ParseModule("example.rego", rawModule)
	if err != nil {
		t.Fatal(err)
	}

	testCases := map[string]struct {
		Not          *Not
		Options      astJSON.Options
		ExpectedJSON string
	}{
		"implicit body: base case": {
			Not:          module.Rules[0].Body[0].Terms.(*Not),
			ExpectedJSON: `{"body":[{"index":0,"terms":[{"type":"ref","value":[{"type":"var","value":"equal"}]},{"type":"call","value":[{"type":"ref","value":[{"type":"var","value":"plus"}]},{"type":"ref","value":[{"type":"var","value":"input"},{"type":"string","value":"x"}]},{"type":"number","value":2}]},{"type":"number","value":42}]}],"explicit_body":false,"type":"not"}`,
		},
		"explicit body: base case": {
			Not:          module.Rules[1].Body[0].Terms.(*Not),
			ExpectedJSON: `{"body":[{"index":0,"terms":[{"type":"ref","value":[{"type":"var","value":"assign"}]},{"type":"var","value":"x"},{"type":"ref","value":[{"type":"var","value":"input"},{"type":"string","value":"x"}]}]},{"index":1,"terms":[{"type":"ref","value":[{"type":"var","value":"assign"}]},{"type":"var","value":"y"},{"type":"number","value":2}]},{"index":2,"terms":[{"type":"ref","value":[{"type":"var","value":"assign"}]},{"type":"var","value":"z"},{"type":"call","value":[{"type":"ref","value":[{"type":"var","value":"plus"}]},{"type":"var","value":"x"},{"type":"var","value":"y"}]}]},{"index":3,"terms":[{"type":"ref","value":[{"type":"var","value":"equal"}]},{"type":"var","value":"z"},{"type":"number","value":42}]}],"explicit_body":true,"type":"not"}`,
		},
		"implicit body: location excluded": {
			Not: module.Rules[0].Body[0].Terms.(*Not),
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{IncludeLocation: astJSON.NodeToggle{Not: false}},
			},
			ExpectedJSON: `{"body":[{"index":0,"terms":[{"type":"ref","value":[{"type":"var","value":"equal"}]},{"type":"call","value":[{"type":"ref","value":[{"type":"var","value":"plus"}]},{"type":"ref","value":[{"type":"var","value":"input"},{"type":"string","value":"x"}]},{"type":"number","value":2}]},{"type":"number","value":42}]}],"explicit_body":false,"type":"not"}`,
		},
		"explicit body: location excluded": {
			Not: module.Rules[1].Body[0].Terms.(*Not),
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{IncludeLocation: astJSON.NodeToggle{Not: false}},
			},
			ExpectedJSON: `{"body":[{"index":0,"terms":[{"type":"ref","value":[{"type":"var","value":"assign"}]},{"type":"var","value":"x"},{"type":"ref","value":[{"type":"var","value":"input"},{"type":"string","value":"x"}]}]},{"index":1,"terms":[{"type":"ref","value":[{"type":"var","value":"assign"}]},{"type":"var","value":"y"},{"type":"number","value":2}]},{"index":2,"terms":[{"type":"ref","value":[{"type":"var","value":"assign"}]},{"type":"var","value":"z"},{"type":"call","value":[{"type":"ref","value":[{"type":"var","value":"plus"}]},{"type":"var","value":"x"},{"type":"var","value":"y"}]}]},{"index":3,"terms":[{"type":"ref","value":[{"type":"var","value":"equal"}]},{"type":"var","value":"z"},{"type":"number","value":42}]}],"explicit_body":true,"type":"not"}`,
		},
		"implicit body: location included": {
			Not:          module.Rules[0].Body[0].Terms.(*Not),
			Options:      astJSON.Options{MarshalOptions: astJSON.MarshalOptions{IncludeLocation: astJSON.NodeToggle{Not: true}}},
			ExpectedJSON: `{"body":[{"index":0,"terms":[{"type":"ref","value":[{"type":"var","value":"equal"}]},{"type":"call","value":[{"type":"ref","value":[{"type":"var","value":"plus"}]},{"type":"ref","value":[{"type":"var","value":"input"},{"type":"string","value":"x"}]},{"type":"number","value":2}]},{"type":"number","value":42}]}],"explicit_body":false,"location":{"file":"example.rego","row":7,"col":4},"type":"not"}`,
		},
		"explicit body: location included": {
			Not:          module.Rules[1].Body[0].Terms.(*Not),
			Options:      astJSON.Options{MarshalOptions: astJSON.MarshalOptions{IncludeLocation: astJSON.NodeToggle{Not: true}}},
			ExpectedJSON: `{"body":[{"index":0,"terms":[{"type":"ref","value":[{"type":"var","value":"assign"}]},{"type":"var","value":"x"},{"type":"ref","value":[{"type":"var","value":"input"},{"type":"string","value":"x"}]}]},{"index":1,"terms":[{"type":"ref","value":[{"type":"var","value":"assign"}]},{"type":"var","value":"y"},{"type":"number","value":2}]},{"index":2,"terms":[{"type":"ref","value":[{"type":"var","value":"assign"}]},{"type":"var","value":"z"},{"type":"call","value":[{"type":"ref","value":[{"type":"var","value":"plus"}]},{"type":"var","value":"x"},{"type":"var","value":"y"}]}]},{"index":3,"terms":[{"type":"ref","value":[{"type":"var","value":"equal"}]},{"type":"var","value":"z"},{"type":"number","value":42}]}],"explicit_body":true,"location":{"file":"example.rego","row":11,"col":4},"type":"not"}`,
		},
		"explicit body: location included, also for nested expressions": {
			Not:          module.Rules[1].Body[0].Terms.(*Not),
			Options:      astJSON.Options{MarshalOptions: astJSON.MarshalOptions{IncludeLocation: astJSON.NodeToggle{Not: true, Expr: true}}},
			ExpectedJSON: `{"body":[{"index":0,"location":{"file":"example.rego","row":12,"col":5},"terms":[{"type":"ref","value":[{"type":"var","value":"assign"}]},{"type":"var","value":"x"},{"type":"ref","value":[{"type":"var","value":"input"},{"type":"string","value":"x"}]}]},{"index":1,"location":{"file":"example.rego","row":13,"col":5},"terms":[{"type":"ref","value":[{"type":"var","value":"assign"}]},{"type":"var","value":"y"},{"type":"number","value":2}]},{"index":2,"location":{"file":"example.rego","row":14,"col":5},"terms":[{"type":"ref","value":[{"type":"var","value":"assign"}]},{"type":"var","value":"z"},{"type":"call","value":[{"type":"ref","value":[{"type":"var","value":"plus"}]},{"type":"var","value":"x"},{"type":"var","value":"y"}]}]},{"index":3,"location":{"file":"example.rego","row":15,"col":5},"terms":[{"type":"ref","value":[{"type":"var","value":"equal"}]},{"type":"var","value":"z"},{"type":"number","value":42}]}],"explicit_body":true,"location":{"file":"example.rego","row":11,"col":4},"type":"not"}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			astJSON.SetOptions(data.Options)
			t.Cleanup(resetJSONOptions)

			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(data.Not))
		})
	}
}

func TestNot_UnmarshalJSON(t *testing.T) {
	rawModule := `
		package test
		
		import future.keywords.not
		
		implicit_body if {
			not input.x + 2 == 42
		}

		explicit_body if {
			not {
				x := input.x
				y := 2
				z := x + y
				z == 42
			}
		}
	`

	module, err := ParseModule("example.rego", rawModule)
	if err != nil {
		t.Fatal(err)
	}

	implicitBodyExpr := module.Rules[0].Body[0]
	// text is not marshalled to JSON so we just drop it in our examples
	implicitBodyExpr.Location.Text = nil

	explicitBodyExpr := module.Rules[1].Body[0]
	explicitBodyExpr.Location.Text = nil

	testCases := map[string]struct {
		JSON         string
		ExpectedExpr *Expr
	}{
		"implicit body": {
			JSON: `{"index":0,"terms":{"type":"not","body":[{"index":0,"terms":[{"type":"ref","value":[{"type":"var","value":"equal"}]},{"type":"call","value":[{"type":"ref","value":[{"type":"var","value":"plus"}]},{"type":"ref","value":[{"type":"var","value":"input"},{"type":"string","value":"x"}]},{"type":"number","value":2}]},{"type":"number","value":42}]}],"explicit_body":false}}`,
			ExpectedExpr: func() *Expr {
				e := implicitBodyExpr.Copy()
				e.Location = nil
				return e
			}(),
		},
		"explicit body": {
			JSON: `{"index":0,"terms":{"body":[{"index":0,"terms":[{"type":"ref","value":[{"type":"var","value":"assign"}]},{"type":"var","value":"x"},{"type":"ref","value":[{"type":"var","value":"input"},{"type":"string","value":"x"}]}]},{"index":1,"terms":[{"type":"ref","value":[{"type":"var","value":"assign"}]},{"type":"var","value":"y"},{"type":"number","value":2}]},{"index":2,"terms":[{"type":"ref","value":[{"type":"var","value":"assign"}]},{"type":"var","value":"z"},{"type":"call","value":[{"type":"ref","value":[{"type":"var","value":"plus"}]},{"type":"var","value":"x"},{"type":"var","value":"y"}]}]},{"index":3,"terms":[{"type":"ref","value":[{"type":"var","value":"equal"}]},{"type":"var","value":"z"},{"type":"number","value":42}]}],"explicit_body":true,"type":"not"}}`,
			ExpectedExpr: func() *Expr {
				e := explicitBodyExpr.Copy()
				e.Location = nil
				return e
			}(),
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			var expr Expr
			err := json.Unmarshal([]byte(data.JSON), &expr)
			if err != nil {
				t.Fatal(err)
			}

			if !expr.Equal(data.ExpectedExpr) {
				t.Fatalf("expected:\n%#v got\n%#v", data.ExpectedExpr, expr)
			}
			if data.ExpectedExpr.Location != nil {
				if !expr.Location.Equal(data.ExpectedExpr.Location) {
					t.Fatalf("expected location:\n%#v got\n%#v", data.ExpectedExpr.Location, expr.Location)
				}
			}
		})
	}
}

func TestNot_MarshalUnmarshalRoundTrip(t *testing.T) {
	rawModule := `
		package test

		import future.keywords.not

		implicit_body if {
			not input.x + 2 == 42
		}

		explicit_body if {
			not {
				x := input.x
				y := 2
				z := x + y
				z == 42
			}
		}
	`

	module, err := ParseModule("example.rego", rawModule)
	if err != nil {
		t.Fatal(err)
	}

	testCases := map[string]struct {
		Expr *Expr
	}{
		"implicit body": {
			Expr: module.Rules[0].Body[0],
		},
		"explicit body": {
			Expr: module.Rules[1].Body[0],
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			bs := util.MustMarshalJSON(data.Expr)

			var expr Expr
			err := json.Unmarshal(bs, &expr)
			if err != nil {
				t.Fatalf("unmarshal failed: %v\njson: %s", err, string(bs))
			}

			if !expr.Equal(data.Expr) {
				t.Fatalf("round-trip mismatch\noriginal: %#v\ngot:      %#v", data.Expr, &expr)
			}
		})
	}
}

func TestNot_UnmarshalJSON_Errors(t *testing.T) {
	testCases := map[string]struct {
		JSON   string
		expErr string
	}{
		"body is not an array": {
			JSON:   `{"index":0,"terms":{"type":"not","body":"invalid","explicit_body":false}}`,
			expErr: "invalid body field type",
		},
		"body is missing": {
			JSON:   `{"index":0,"terms":{"type":"not","explicit_body":false}}`,
			expErr: "invalid body field type",
		},
		"explicit_body is not a bool": {
			JSON:   `{"index":0,"terms":{"type":"not","body":[],"explicit_body":"yes"}}`,
			expErr: "unable to unmarshal explicit_body field",
		},
		"body contains invalid expression": {
			JSON:   `{"index":0,"terms":{"type":"not","body":[{"index":0,"terms":"bad"}],"explicit_body":false}}`,
			expErr: "unable to unmarshal not body",
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			var expr Expr
			err := json.Unmarshal([]byte(data.JSON), &expr)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), data.expErr) {
				t.Fatalf("expected error containing %q, got: %v", data.expErr, err)
			}
		})
	}
}

func TestArgs_MarshalJSON(t *testing.T) {
	x := VarTerm("x").SetLocation(NewLocation([]byte("x"), "example.rego", 1, 2))

	testCases := map[string]struct {
		Args         Args
		Options      astJSON.Options
		ExpectedJSON string
	}{
		"nil": {
			Args:         nil,
			ExpectedJSON: `null`,
		},
		"empty": {
			Args:         Args{},
			ExpectedJSON: `[]`,
		},
		"base case": {
			Args:         Args{x, VarTerm("y")},
			ExpectedJSON: `[{"type":"var","value":"x"},{"type":"var","value":"y"}]`,
		},
		"term location included": {
			Args: Args{x},
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{IncludeLocation: astJSON.NodeToggle{Term: true}},
			},
			ExpectedJSON: `[{"location":{"file":"example.rego","row":1,"col":2},"type":"var","value":"x"}]`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			astJSON.SetOptions(data.Options)
			t.Cleanup(resetJSONOptions)

			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(data.Args))
		})
	}
}

func TestVar_MarshalJSON(t *testing.T) {
	testCases := map[string]struct {
		Var          Var
		ExpectedJSON string
	}{
		"base case": {
			Var:          Var("x"),
			ExpectedJSON: `"x"`,
		},
		"empty": {
			Var:          Var(""),
			ExpectedJSON: `""`,
		},
		"wildcard": {
			Var:          Var("$01"),
			ExpectedJSON: `"$01"`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(data.Var))
		})
	}
}

func TestAuthorAnnotation_MarshalJSON(t *testing.T) {
	testCases := map[string]struct {
		Author       *AuthorAnnotation
		ExpectedJSON string
	}{
		"base case": {
			Author:       &AuthorAnnotation{Name: "John Doe", Email: "john@example.com"},
			ExpectedJSON: `{"name":"John Doe","email":"john@example.com"}`,
		},
		"no email": {
			Author:       &AuthorAnnotation{Name: "John Doe"},
			ExpectedJSON: `{"name":"John Doe"}`,
		},
		"empty": {
			Author:       &AuthorAnnotation{},
			ExpectedJSON: `{"name":""}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(data.Author))
		})
	}
}

func TestSchemaAnnotation_MarshalJSON(t *testing.T) {
	loc := NewLocation([]byte("input"), "example.rego", 1, 2)
	path := Ref{VarTerm("input").SetLocation(loc), StringTerm("foo").SetLocation(loc)}
	definition := any(map[string]any{"type": "boolean"})

	testCases := map[string]struct {
		Schema       *SchemaAnnotation
		Options      astJSON.Options
		ExpectedJSON string
	}{
		"empty": {
			Schema:       &SchemaAnnotation{},
			ExpectedJSON: `{"path":null}`,
		},
		"path and schema": {
			Schema:       &SchemaAnnotation{Path: path, Schema: MustParseRef("schema.foo")},
			ExpectedJSON: `{"path":[{"type":"var","value":"input"},{"type":"string","value":"foo"}],"schema":[{"type":"var","value":"schema"},{"type":"string","value":"foo"}]}`,
		},
		"path and definition": {
			Schema:       &SchemaAnnotation{Path: path, Definition: &definition},
			ExpectedJSON: `{"path":[{"type":"var","value":"input"},{"type":"string","value":"foo"}],"definition":{"type":"boolean"}}`,
		},
		"term location included": {
			Schema: &SchemaAnnotation{Path: path},
			Options: astJSON.Options{
				MarshalOptions: astJSON.MarshalOptions{IncludeLocation: astJSON.NodeToggle{Term: true}},
			},
			ExpectedJSON: `{"path":[{"type":"var","value":"input"},{"type":"string","value":"foo"}]}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			astJSON.SetOptions(data.Options)
			t.Cleanup(resetJSONOptions)

			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(data.Schema))
		})
	}

	t.Run("path terms are not mutated", func(t *testing.T) {
		astJSON.SetOptions(astJSON.Options{
			MarshalOptions: astJSON.MarshalOptions{IncludeLocation: astJSON.NodeToggle{Term: true, AnnotationsRef: true}},
		})
		t.Cleanup(resetJSONOptions)

		s := &SchemaAnnotation{Path: Ref{VarTerm("input").SetLocation(loc)}}
		util.MustMarshalJSON(s)

		if s.Path[0].Location != loc {
			t.Fatalf("expected path term location to be left alone, got %v", s.Path[0].Location)
		}
	})
}

func TestModule_UnmarshalJSON(t *testing.T) {
	mod := MustParseModule(`package test

p if { q }
q := 1
r := 2 if { input.x } else := 3 if { input.y }
`)

	bs := util.MustMarshalJSON(mod)

	var roundtrip Module
	if err := util.UnmarshalJSON(bs, &roundtrip); err != nil {
		t.Fatal(err)
	}

	if exp, got := len(mod.Rules), len(roundtrip.Rules); exp != got {
		t.Fatalf("expected %d rules, got %d", exp, got)
	}

	WalkRules(&roundtrip, func(rule *Rule) bool {
		if rule.Module != &roundtrip {
			t.Errorf("rule %v: expected module pointer to be set, got %v", rule.Head, rule.Module)
		}
		return false
	})
}

func TestTemplateString_MarshalJSON(t *testing.T) {
	testCases := map[string]struct {
		TemplateString *TemplateString
		ExpectedJSON   string
	}{
		"nil parts": {
			TemplateString: &TemplateString{},
			ExpectedJSON:   `{"parts":null,"multi_line":false}`,
		},
		"empty parts": {
			TemplateString: &TemplateString{Parts: []Node{}},
			ExpectedJSON:   `{"parts":[],"multi_line":false}`,
		},
		"base case": {
			TemplateString: &TemplateString{Parts: []Node{StringTerm("foo"), VarTerm("x")}, MultiLine: true},
			ExpectedJSON:   `{"parts":[{"type":"string","value":"foo"},{"type":"var","value":"x"}],"multi_line":true}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			assertJsonEqual(t, data.ExpectedJSON, util.MustMarshalJSON(data.TemplateString))
		})
	}
}

func TestSchemaAnnotation_MarshalJSON_InvalidDefinition(t *testing.T) {
	definition := any(func() {})

	_, err := json.Marshal(&SchemaAnnotation{Path: MustParseRef("input.x"), Definition: &definition})
	if err == nil {
		t.Fatal("expected error")
	}
	// The encoder's wording differs between encoding/json v1 and v2.
	if exp := "func()"; !strings.Contains(err.Error(), exp) {
		t.Fatalf("expected error containing %q, got: %v", exp, err)
	}
}
