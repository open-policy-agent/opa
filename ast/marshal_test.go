package ast

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/open-policy-agent/opa/util"
)

func TestTerm_MarshalJSON(t *testing.T) {
	testCases := map[string]struct {
		Term         *Term
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
		"location excluded": {
			Term: func() *Term {
				v, _ := InterfaceToValue("example")
				return &Term{
					Value:    v,
					Location: NewLocation([]byte{}, "example.rego", 1, 2),
					jsonFields: map[string]bool{
						"location": false,
					},
				}
			}(),
			ExpectedJSON: `{"type":"string","value":"example"}`,
		},
		"location included": {
			Term: func() *Term {
				v, _ := InterfaceToValue("example")
				return &Term{
					Value:    v,
					Location: NewLocation([]byte{}, "example.rego", 1, 2),
					jsonFields: map[string]bool{
						"location": true,
					},
				}
			}(),
			ExpectedJSON: `{"location":{"file":"example.rego","row":1,"col":2},"type":"string","value":"example"}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			bs := util.MustMarshalJSON(data.Term)
			got := string(bs)
			exp := data.ExpectedJSON

			if got != exp {
				t.Fatalf("expected:\n%s got\n%s", exp, got)
			}
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
				jsonFields: map[string]bool{
					"location": false,
				},
			},
			ExpectedJSON: `{"path":[]}`,
		},
		"location included": {
			Package: &Package{
				Path:     EmptyRef(),
				Location: NewLocation([]byte{}, "example.rego", 1, 2),
				jsonFields: map[string]bool{
					"location": true,
				},
			},
			ExpectedJSON: `{"location":{"file":"example.rego","row":1,"col":2},"path":[]}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			bs := util.MustMarshalJSON(data.Package)
			got := string(bs)
			exp := data.ExpectedJSON

			if got != exp {
				t.Fatalf("expected:\n%s got\n%s", exp, got)
			}
		})
	}
}

func TestPackage_UnmarshalJSON(t *testing.T) {
	testCases := map[string]struct {
		JSON            string
		ExpectedPackage *Package
	}{
		"base case": {
			JSON: `{"path":[]}`,
			ExpectedPackage: &Package{
				Path: EmptyRef(),
			},
		},
		"location case": {
			JSON: `{"location":{"file":"example.rego","row":1,"col":2},"path":[]}`,
			ExpectedPackage: &Package{
				Path:     EmptyRef(),
				Location: NewLocation([]byte{}, "example.rego", 1, 2),
			},
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			var pkg Package
			err := json.Unmarshal([]byte(data.JSON), &pkg)
			if err != nil {
				t.Fatal(err)
			}

			if !pkg.Equal(data.ExpectedPackage) {
				t.Fatalf("expected:\n%#v got\n%#v", data.ExpectedPackage, pkg)
			}
			if data.ExpectedPackage.Location != nil {
				if !pkg.Location.Equal(data.ExpectedPackage.Location) {
					t.Fatalf("expected location:\n%#v got\n%#v", data.ExpectedPackage, pkg)
				}
			}
		})
	}
}

// TODO: Comment has inconsistent JSON field names starting with an upper case letter. Comment Location is
// also always included for legacy reasons
func TestComment_MarshalJSON(t *testing.T) {
	testCases := map[string]struct {
		Comment      *Comment
		ExpectedJSON string
	}{
		"base case": {
			Comment: &Comment{
				Text: []byte("comment"),
			},
			ExpectedJSON: `{"Text":"Y29tbWVudA=="}`,
		},
		"location excluded, still included for legacy reasons": {
			Comment: &Comment{
				Text:     []byte("comment"),
				Location: NewLocation([]byte{}, "example.rego", 1, 2),
				jsonFields: map[string]bool{
					"location": false, // ignored
				},
			},
			ExpectedJSON: `{"Location":{"file":"example.rego","row":1,"col":2},"Text":"Y29tbWVudA=="}`,
		},
		"location included": {
			Comment: &Comment{
				Text:     []byte("comment"),
				Location: NewLocation([]byte{}, "example.rego", 1, 2),
				jsonFields: map[string]bool{
					"location": true, // ignored
				},
			},
			ExpectedJSON: `{"Location":{"file":"example.rego","row":1,"col":2},"Text":"Y29tbWVudA=="}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			bs := util.MustMarshalJSON(data.Comment)
			got := string(bs)
			exp := data.ExpectedJSON

			if got != exp {
				t.Fatalf("expected:\n%s got\n%s", exp, got)
			}
		})
	}
}

// TODO: Comment has inconsistent JSON field names starting with an upper case letter. Comment Location is
// also always included for legacy reasons
func TestComment_UnmarshalJSON(t *testing.T) {
	testCases := map[string]struct {
		JSON            string
		ExpectedComment *Comment
	}{
		"base case": {
			JSON: `{"Text":"Y29tbWVudA=="}`,
			ExpectedComment: &Comment{
				Text: []byte("comment"),
			},
		},
		"location case": {
			JSON: `{"Location":{"file":"example.rego","row":1,"col":2},"Text":"Y29tbWVudA=="}`,
			ExpectedComment: &Comment{
				Text:     []byte("comment"),
				Location: NewLocation([]byte{}, "example.rego", 1, 2),
			},
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			var comment Comment
			err := json.Unmarshal([]byte(data.JSON), &comment)
			if err != nil {
				t.Fatal(err)
			}

			equal := true
			if data.ExpectedComment.Location != nil {
				if comment.Location == nil {
					t.Fatal("expected location to be non-nil")
				}

				// comment.Equal will check the location too
				if !comment.Equal(data.ExpectedComment) {
					equal = false
				}
			} else {
				if !bytes.Equal(comment.Text, data.ExpectedComment.Text) {
					equal = false
				}
			}
			if !equal {
				t.Fatalf("expected:\n%#v got\n%#v", data.ExpectedComment, comment)
			}
		})
	}
}

func TestImport_MarshalJSON(t *testing.T) {
	testCases := map[string]struct {
		Import       *Import
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
					jsonFields: map[string]bool{
						"location": false,
					},
				}
			}(),
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
					jsonFields: map[string]bool{
						"location": true,
					},
				}
			}(),
			ExpectedJSON: `{"location":{"file":"example.rego","row":1,"col":2},"path":{"type":"string","value":"example"}}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			bs := util.MustMarshalJSON(data.Import)
			got := string(bs)
			exp := data.ExpectedJSON

			if got != exp {
				t.Fatalf("expected:\n%s got\n%s", exp, got)
			}
		})
	}
}

func TestImport_UnmarshalJSON(t *testing.T) {
	testCases := map[string]struct {
		JSON           string
		ExpectedImport *Import
	}{
		"base case": {
			JSON: `{"path":{"type":"string","value":"example"}}`,
			ExpectedImport: func() *Import {
				v, _ := InterfaceToValue("example")
				term := Term{
					Value: v,
				}
				return &Import{Path: &term}
			}(),
		},
		"location case": {
			JSON: `{"location":{"file":"example.rego","row":1,"col":2},"path":{"type":"string","value":"example"}}`,
			ExpectedImport: func() *Import {
				v, _ := InterfaceToValue("example")
				term := Term{
					Value:    v,
					Location: NewLocation([]byte{}, "example.rego", 1, 2),
				}
				return &Import{
					Path:     &term,
					Location: NewLocation([]byte{}, "example.rego", 1, 2),
					jsonFields: map[string]bool{
						"location": false,
					},
				}
			}(),
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			var imp Import
			err := json.Unmarshal([]byte(data.JSON), &imp)
			if err != nil {
				t.Fatal(err)
			}

			if !imp.Equal(data.ExpectedImport) {
				t.Fatalf("expected:\n%#v got\n%#v", data.ExpectedImport, imp)
			}
			if data.ExpectedImport.Location != nil {
				if !imp.Location.Equal(data.ExpectedImport.Location) {
					t.Fatalf("expected location:\n%#v got\n%#v", data.ExpectedImport, imp)
				}
			}
		})
	}
}

func TestRule_MarshalJSON(t *testing.T) {
	rawModule := `
	package foo
	
	# comment
	
	allow { true }
	`

	module, err := ParseModuleWithOpts("example.rego", rawModule, ParserOptions{})
	if err != nil {
		t.Fatal(err)
	}

	rule := module.Rules[0]

	testCases := map[string]struct {
		Rule         *Rule
		ExpectedJSON string
	}{
		"base case": {
			Rule:         rule.Copy(),
			ExpectedJSON: `{"body":[{"index":0,"terms":{"type":"boolean","value":true}}],"head":{"name":"allow","value":{"type":"boolean","value":true},"ref":[{"type":"var","value":"allow"}]}}`,
		},
		"location excluded": {
			Rule: func() *Rule {
				r := rule.Copy()
				r.jsonFields = map[string]bool{
					"location": false,
				}
				return r
			}(),
			ExpectedJSON: `{"body":[{"index":0,"terms":{"type":"boolean","value":true}}],"head":{"name":"allow","value":{"type":"boolean","value":true},"ref":[{"type":"var","value":"allow"}]}}`,
		},
		"location included": {
			Rule: func() *Rule {
				r := rule.Copy()
				r.jsonFields = map[string]bool{
					"location": true,
				}
				return r
			}(),
			ExpectedJSON: `{"body":[{"index":0,"terms":{"type":"boolean","value":true}}],"head":{"name":"allow","value":{"type":"boolean","value":true},"ref":[{"type":"var","value":"allow"}]},"location":{"file":"example.rego","row":6,"col":2}}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			bs := util.MustMarshalJSON(data.Rule)
			got := string(bs)
			exp := data.ExpectedJSON

			if got != exp {
				t.Fatalf("expected:\n%s got\n%s", exp, got)
			}
		})
	}
}

func TestRule_UnmarshalJSON(t *testing.T) {
	rawModule := `
	package foo
	
	# comment
	
	allow { true }
	`

	module, err := ParseModuleWithOpts("example.rego", rawModule, ParserOptions{})
	if err != nil {
		t.Fatal(err)
	}

	rule := module.Rules[0]
	// text is not marshalled to JSON so we just drop it in our examples
	rule.Location.Text = nil

	testCases := map[string]struct {
		JSON         string
		ExpectedRule *Rule
	}{
		"base case": {
			JSON: `{"body":[{"terms":{"type":"boolean","value":true},"location":{"file":"example.rego","row":6,"col":10},"index":0}],"head":{"name":"allow","value":{"type":"boolean","value":true},"location":{"file":"example.rego","row":6,"col":2},"ref":[{"type":"var","value":"allow"}]}}`,
			ExpectedRule: func() *Rule {
				r := rule.Copy()
				r.Location = nil
				return r
			}(),
		},
		"location case": {
			JSON:         `{"body":[{"terms":{"type":"boolean","value":true},"location":{"file":"example.rego","row":6,"col":10},"index":0}],"head":{"name":"allow","value":{"type":"boolean","value":true},"location":{"file":"example.rego","row":6,"col":2},"ref":[{"type":"var","value":"allow"}]},"location":{"file":"example.rego","row":6,"col":2}}`,
			ExpectedRule: rule,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			var rule Rule
			err := json.Unmarshal([]byte(data.JSON), &rule)
			if err != nil {
				t.Fatal(err)
			}

			if !rule.Equal(data.ExpectedRule) {
				t.Fatalf("expected:\n%#v got\n%#v", data.ExpectedRule, rule)
			}
			if data.ExpectedRule.Location != nil {
				if !rule.Location.Equal(data.ExpectedRule.Location) {
					t.Fatalf("expected location:\n%#v got\n%#v", data.ExpectedRule.Location, rule.Location)
				}
			}
		})
	}
}

func TestHead_MarshalJSON(t *testing.T) {
	rawModule := `
	package foo
	
	# comment
	
	allow { true }
	`

	module, err := ParseModuleWithOpts("example.rego", rawModule, ParserOptions{})
	if err != nil {
		t.Fatal(err)
	}

	head := module.Rules[0].Head

	testCases := map[string]struct {
		Head         *Head
		ExpectedJSON string
	}{
		"base case": {
			Head:         head.Copy(),
			ExpectedJSON: `{"name":"allow","value":{"type":"boolean","value":true},"ref":[{"type":"var","value":"allow"}]}`,
		},
		"location excluded": {
			Head: func() *Head {
				h := head.Copy()
				h.jsonFields = map[string]bool{
					"location": false,
				}
				return h
			}(),
			ExpectedJSON: `{"name":"allow","value":{"type":"boolean","value":true},"ref":[{"type":"var","value":"allow"}]}`,
		},
		"location included": {
			Head: func() *Head {
				h := head.Copy()
				h.jsonFields = map[string]bool{
					"location": true,
				}
				return h
			}(),
			ExpectedJSON: `{"name":"allow","value":{"type":"boolean","value":true},"ref":[{"type":"var","value":"allow"}],"location":{"file":"example.rego","row":6,"col":2}}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			bs := util.MustMarshalJSON(data.Head)
			got := string(bs)
			exp := data.ExpectedJSON

			if got != exp {
				t.Fatalf("expected:\n%s got\n%s", exp, got)
			}
		})
	}
}

func TestHead_UnmarshalJSON(t *testing.T) {
	rawModule := `
	package foo

	# comment

	allow { true }
	`

	module, err := ParseModuleWithOpts("example.rego", rawModule, ParserOptions{})
	if err != nil {
		t.Fatal(err)
	}

	head := module.Rules[0].Head
	// text is not marshalled to JSON so we just drop it in our examples
	head.Location.Text = nil

	testCases := map[string]struct {
		JSON         string
		ExpectedHead *Head
	}{
		"base case": {
			JSON: `{"name":"allow","value":{"type":"boolean","value":true},"ref":[{"type":"var","value":"allow"}]}`,
			ExpectedHead: func() *Head {
				h := head.Copy()
				h.Location = nil
				return h
			}(),
		},
		"location case": {
			JSON:         `{"name":"allow","value":{"type":"boolean","value":true},"ref":[{"type":"var","value":"allow"}],"location":{"file":"example.rego","row":6,"col":2}}`,
			ExpectedHead: head,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			var head Head
			err := json.Unmarshal([]byte(data.JSON), &head)
			if err != nil {
				t.Fatal(err)
			}

			if !head.Equal(data.ExpectedHead) {
				t.Fatalf("expected:\n%#v got\n%#v", data.ExpectedHead, head)
			}
			if data.ExpectedHead.Location != nil {
				if !head.Location.Equal(data.ExpectedHead.Location) {
					t.Fatalf("expected location:\n%#v got\n%#v", data.ExpectedHead.Location, head.Location)
				}
			}
		})
	}
}

func TestExpr_MarshalJSON(t *testing.T) {
	rawModule := `
	package foo
	
	# comment
	
	allow { true }
	`

	module, err := ParseModuleWithOpts("example.rego", rawModule, ParserOptions{})
	if err != nil {
		t.Fatal(err)
	}

	expr := module.Rules[0].Body[0]

	testCases := map[string]struct {
		Expr         *Expr
		ExpectedJSON string
	}{
		"base case": {
			Expr:         expr.Copy(),
			ExpectedJSON: `{"index":0,"terms":{"type":"boolean","value":true}}`,
		},
		"location excluded": {
			Expr: func() *Expr {
				e := expr.Copy()
				e.jsonFields = map[string]bool{
					"location": false,
				}
				return e
			}(),
			ExpectedJSON: `{"index":0,"terms":{"type":"boolean","value":true}}`,
		},
		"location included": {
			Expr: func() *Expr {
				e := expr.Copy()
				e.jsonFields = map[string]bool{
					"location": true,
				}
				return e
			}(),
			ExpectedJSON: `{"index":0,"location":{"file":"example.rego","row":6,"col":10},"terms":{"type":"boolean","value":true}}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			bs := util.MustMarshalJSON(data.Expr)
			got := string(bs)
			exp := data.ExpectedJSON

			if got != exp {
				t.Fatalf("expected:\n%s got\n%s", exp, got)
			}
		})
	}
}

func TestExpr_UnmarshalJSON(t *testing.T) {
	rawModule := `
	package foo

	# comment

	allow { true }
	`

	module, err := ParseModuleWithOpts("example.rego", rawModule, ParserOptions{})
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
			JSON:         `{"index":0,"location":{"file":"example.rego","row":6,"col":10},"terms":{"type":"boolean","value":true}}`,
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

func TestSomeDecl_MarshalJSON(t *testing.T) {
	v, _ := InterfaceToValue("example")
	term := &Term{
		Value:    v,
		Location: NewLocation([]byte{}, "example.rego", 1, 2),
	}

	testCases := map[string]struct {
		SomeDecl     *SomeDecl
		ExpectedJSON string
	}{
		"base case": {
			SomeDecl: &SomeDecl{
				Symbols:  []*Term{term},
				Location: NewLocation([]byte{}, "example.rego", 1, 2),
			},
			ExpectedJSON: `{"symbols":[{"type":"string","value":"example"}]}`,
		},
		"location excluded": {
			SomeDecl: &SomeDecl{
				Symbols:    []*Term{term},
				Location:   NewLocation([]byte{}, "example.rego", 1, 2),
				jsonFields: map[string]bool{"location": false},
			},
			ExpectedJSON: `{"symbols":[{"type":"string","value":"example"}]}`,
		},
		"location included": {
			SomeDecl: &SomeDecl{
				Symbols:    []*Term{term},
				Location:   NewLocation([]byte{}, "example.rego", 1, 2),
				jsonFields: map[string]bool{"location": true},
			},
			ExpectedJSON: `{"location":{"file":"example.rego","row":1,"col":2},"symbols":[{"type":"string","value":"example"}]}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			bs := util.MustMarshalJSON(data.SomeDecl)
			got := string(bs)
			exp := data.ExpectedJSON

			if got != exp {
				t.Fatalf("expected:\n%s got\n%s", exp, got)
			}
		})
	}
}

func TestSomeDecl_UnmarshalJSON(t *testing.T) {
	v, _ := InterfaceToValue("example")
	term := &Term{
		Value:    v,
		Location: NewLocation([]byte{}, "example.rego", 1, 2),
	}

	testCases := map[string]struct {
		JSON             string
		ExpectedSomeDecl *SomeDecl
	}{
		"base case": {
			JSON: `{"symbols":[{"type":"string","value":"example"}]}`,
			ExpectedSomeDecl: &SomeDecl{
				Symbols: []*Term{term},
			},
		},
		"location case": {
			JSON: `{"location":{"file":"example.rego","row":1,"col":2},"symbols":[{"type":"string","value":"example"}]}`,
			ExpectedSomeDecl: &SomeDecl{
				Symbols:  []*Term{term},
				Location: NewLocation([]byte{}, "example.rego", 1, 2),
			},
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			var d SomeDecl
			err := json.Unmarshal([]byte(data.JSON), &d)
			if err != nil {
				t.Fatal(err)
			}

			if len(d.Symbols) != len(data.ExpectedSomeDecl.Symbols) {
				t.Fatalf("expected:\n%#v got\n%#v", data.ExpectedSomeDecl.Symbols, d.Symbols)
			}

			if data.ExpectedSomeDecl.Location != nil {
				if !d.Location.Equal(data.ExpectedSomeDecl.Location) {
					t.Fatalf("expected location:\n%#v got\n%#v", data.ExpectedSomeDecl.Location, d.Location)
				}
			}
		})
	}
}

func TestEvery_MarshalJSON(t *testing.T) {

	rawModule := `
package foo

import future.keywords.every
	
allow { 
	every e in [1,2,3] {
		e == 1
    }
}
`

	module, err := ParseModuleWithOpts("example.rego", rawModule, ParserOptions{})
	if err != nil {
		t.Fatal(err)
	}

	every, ok := module.Rules[0].Body[0].Terms.(*Every)
	if !ok {
		t.Fatal("expected every term")
	}

	testCases := map[string]struct {
		Every        *Every
		ExpectedJSON string
	}{
		"base case": {
			Every:        every,
			ExpectedJSON: `{"body":[{"index":0,"terms":[{"type":"ref","value":[{"type":"var","value":"equal"}]},{"type":"var","value":"e"},{"type":"number","value":1}]}],"domain":{"type":"array","value":[{"type":"number","value":1},{"type":"number","value":2},{"type":"number","value":3}]},"key":null,"value":{"type":"var","value":"e"}}`,
		},
		"location excluded": {
			Every: func() *Every {
				e := every.Copy()
				e.jsonFields = map[string]bool{"location": false}
				return e
			}(),
			ExpectedJSON: `{"body":[{"index":0,"terms":[{"type":"ref","value":[{"type":"var","value":"equal"}]},{"type":"var","value":"e"},{"type":"number","value":1}]}],"domain":{"type":"array","value":[{"type":"number","value":1},{"type":"number","value":2},{"type":"number","value":3}]},"key":null,"value":{"type":"var","value":"e"}}`,
		},
		"location included": {
			Every: func() *Every {
				e := every.Copy()
				e.jsonFields = map[string]bool{"location": true}
				return e
			}(),
			ExpectedJSON: `{"body":[{"index":0,"terms":[{"type":"ref","value":[{"type":"var","value":"equal"}]},{"type":"var","value":"e"},{"type":"number","value":1}]}],"domain":{"type":"array","value":[{"type":"number","value":1},{"type":"number","value":2},{"type":"number","value":3}]},"key":null,"location":{"file":"example.rego","row":7,"col":2},"value":{"type":"var","value":"e"}}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			bs := util.MustMarshalJSON(data.Every)
			got := string(bs)
			exp := data.ExpectedJSON

			if got != exp {
				t.Fatalf("expected:\n%s got\n%s", exp, got)
			}
		})
	}
}

func TestEvery_UnmarshalJSON(t *testing.T) {
	rawModule := `
package foo

import future.keywords.every
	
allow { 
	every e in [1,2,3] {
		e == 1
    }
}
`

	module, err := ParseModuleWithOpts("example.rego", rawModule, ParserOptions{})
	if err != nil {
		t.Fatal(err)
	}

	every, ok := module.Rules[0].Body[0].Terms.(*Every)
	if !ok {
		t.Fatal("expected every term")
	}

	testCases := map[string]struct {
		JSON          string
		ExpectedEvery *Every
	}{
		"base case": {
			JSON: `{"body":[{"index":0,"terms":[{"type":"ref","value":[{"type":"var","value":"equal"}]},{"type":"var","value":"e"},{"type":"number","value":1}]}],"domain":{"type":"array","value":[{"type":"number","value":1},{"type":"number","value":2},{"type":"number","value":3}]},"key":null,"value":{"type":"var","value":"e"}}`,
			ExpectedEvery: func() *Every {
				e := every.Copy()
				e.Location = nil
				return e
			}(),
		},
		"location case": {
			JSON: `{"body":[{"index":0,"terms":[{"type":"ref","value":[{"type":"var","value":"equal"}]},{"type":"var","value":"e"},{"type":"number","value":1}]}],"domain":{"type":"array","value":[{"type":"number","value":1},{"type":"number","value":2},{"type":"number","value":3}]},"key":null,"location":{"file":"example.rego","row":7,"col":2},"value":{"type":"var","value":"e"}}`,
			ExpectedEvery: func() *Every {
				e := every.Copy()
				// text is not marshalled to JSON so we just drop it in our examples
				e.Location.Text = []byte{}
				return e
			}(),
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			var e Every
			err := json.Unmarshal([]byte(data.JSON), &e)
			if err != nil {
				t.Fatal(err)
			}

			if e.String() != data.ExpectedEvery.String() {
				t.Fatalf("expected:\n%#v got\n%#v", data.ExpectedEvery.String(), e.String())
			}

			if data.ExpectedEvery.Location != nil {
				if !e.Location.Equal(data.ExpectedEvery.Location) {
					t.Fatalf("expected location:\n%#v got\n%#v", data.ExpectedEvery.Location, e.Location)
				}
			}
		})
	}
}

func TestWith_MarshalJSON(t *testing.T) {

	rawModule := `
package foo

a {input}

b {
	a with input as 1
}
`

	module, err := ParseModuleWithOpts("example.rego", rawModule, ParserOptions{})
	if err != nil {
		t.Fatal(err)
	}

	with := module.Rules[1].Body[0].With[0]

	testCases := map[string]struct {
		With         *With
		ExpectedJSON string
	}{
		"base case": {
			With:         with,
			ExpectedJSON: `{"target":{"type":"ref","value":[{"type":"var","value":"input"}]},"value":{"type":"number","value":1}}`,
		},
		"location excluded": {
			With: func() *With {
				w := with.Copy()
				w.jsonFields = map[string]bool{"location": false}
				return w
			}(),
			ExpectedJSON: `{"target":{"type":"ref","value":[{"type":"var","value":"input"}]},"value":{"type":"number","value":1}}`,
		},
		"location included": {
			With: func() *With {
				w := with.Copy()
				w.jsonFields = map[string]bool{"location": true}
				return w
			}(),
			ExpectedJSON: `{"location":{"file":"example.rego","row":7,"col":4},"target":{"type":"ref","value":[{"type":"var","value":"input"}]},"value":{"type":"number","value":1}}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			bs := util.MustMarshalJSON(data.With)
			got := string(bs)
			exp := data.ExpectedJSON

			if got != exp {
				t.Fatalf("expected:\n%s got\n%s", exp, got)
			}
		})
	}
}

func TestWith_UnmarshalJSON(t *testing.T) {

	rawModule := `
package foo

a {input}

b {
	a with input as 1
}
`

	module, err := ParseModuleWithOpts("example.rego", rawModule, ParserOptions{})
	if err != nil {
		t.Fatal(err)
	}

	with := module.Rules[1].Body[0].With[0]

	testCases := map[string]struct {
		JSON         string
		ExpectedWith *With
	}{
		"base case": {
			JSON: `{"target":{"type":"ref","value":[{"type":"var","value":"input"}]},"value":{"type":"number","value":1}}`,
			ExpectedWith: func() *With {
				w := with.Copy()
				w.Location = nil
				return w
			}(),
		},
		"location case": {
			JSON: `{"location":{"file":"example.rego","row":7,"col":4},"target":{"type":"ref","value":[{"type":"var","value":"input"}]},"value":{"type":"number","value":1}}`,
			ExpectedWith: func() *With {
				e := with.Copy()
				// text is not marshalled to JSON so we just drop it in our examples
				e.Location.Text = []byte{}
				return e
			}(),
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			var w With
			err := json.Unmarshal([]byte(data.JSON), &w)
			if err != nil {
				t.Fatal(err)
			}

			if w.String() != data.ExpectedWith.String() {
				t.Fatalf("expected:\n%#v got\n%#v", data.ExpectedWith.String(), w.String())
			}

			if data.ExpectedWith.Location != nil {
				if !w.Location.Equal(data.ExpectedWith.Location) {
					t.Fatalf("expected location:\n%#v got\n%#v", data.ExpectedWith.Location, w.Location)
				}
			}
		})
	}
}

func TestAnnotations_MarshalJSON(t *testing.T) {

	testCases := map[string]struct {
		Annotations  *Annotations
		ExpectedJSON string
	}{
		"base case": {
			Annotations: &Annotations{
				Scope:         "rule",
				Title:         "My rule",
				Entrypoint:    true,
				Organizations: []string{"org1"},
				Description:   "My desc",
				Custom: map[string]interface{}{
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
				Custom: map[string]interface{}{
					"foo": "bar",
				},
				Location: NewLocation([]byte{}, "example.rego", 1, 4),

				jsonFields: map[string]bool{"location": false},
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
				Custom: map[string]interface{}{
					"foo": "bar",
				},
				Location: NewLocation([]byte{}, "example.rego", 1, 4),

				jsonFields: map[string]bool{"location": true},
			},
			ExpectedJSON: `{"custom":{"foo":"bar"},"description":"My desc","entrypoint":true,"location":{"file":"example.rego","row":1,"col":4},"organizations":["org1"],"scope":"rule","title":"My rule"}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			bs := util.MustMarshalJSON(data.Annotations)
			got := string(bs)
			exp := data.ExpectedJSON

			if got != exp {
				t.Fatalf("expected:\n%s got\n%s", exp, got)
			}
		})
	}
}

func TestAnnotations_UnmarshalJSON(t *testing.T) {

	testCases := map[string]struct {
		JSON                string
		ExpectedAnnotations *Annotations
	}{
		"base case": {
			JSON: `{"custom":{"foo":"bar"},"description":"My desc","entrypoint":true,"organizations":["org1"],"scope":"rule","title":"My rule"}`,
			ExpectedAnnotations: &Annotations{
				Scope:         "rule",
				Title:         "My rule",
				Entrypoint:    true,
				Organizations: []string{"org1"},
				Description:   "My desc",
				Custom: map[string]interface{}{
					"foo": "bar",
				},
			},
		},
		"location case": {
			JSON: `{"custom":{"foo":"bar"},"description":"My desc","entrypoint":true,"location":{"file":"example.rego","row":1,"col":4},"organizations":["org1"],"scope":"rule","title":"My rule"}`,
			ExpectedAnnotations: &Annotations{
				Scope:         "rule",
				Title:         "My rule",
				Entrypoint:    true,
				Organizations: []string{"org1"},
				Description:   "My desc",
				Custom: map[string]interface{}{
					"foo": "bar",
				},
				Location: NewLocation([]byte{}, "example.rego", 1, 4),
			},
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			var a Annotations
			err := json.Unmarshal([]byte(data.JSON), &a)
			if err != nil {
				t.Fatal(err)
			}

			if a.String() != data.ExpectedAnnotations.String() {
				t.Fatalf("expected:\n%#v got\n%#v", data.ExpectedAnnotations.String(), a.String())
			}

			if data.ExpectedAnnotations.Location != nil {
				if !a.Location.Equal(data.ExpectedAnnotations.Location) {
					t.Fatalf("expected location:\n%#v got\n%#v", data.ExpectedAnnotations.Location, a.Location)
				}
			}
		})
	}
}

func TestAnnotationsRef_MarshalJSON(t *testing.T) {

	testCases := map[string]struct {
		AnnotationsRef *AnnotationsRef
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

				jsonFields: map[string]bool{"location": false},
			},
			ExpectedJSON: `{"annotations":{"scope":""},"path":[]}`,
		},
		"location included": {
			AnnotationsRef: &AnnotationsRef{
				Path:        []*Term{},
				Annotations: &Annotations{},
				Location:    NewLocation([]byte{}, "example.rego", 1, 4),

				jsonFields: map[string]bool{"location": true},
			},
			ExpectedJSON: `{"annotations":{"scope":""},"location":{"file":"example.rego","row":1,"col":4},"path":[]}`,
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			bs := util.MustMarshalJSON(data.AnnotationsRef)
			got := string(bs)
			exp := data.ExpectedJSON

			if got != exp {
				t.Fatalf("expected:\n%s got\n%s", exp, got)
			}
		})
	}
}

func TestAnnotationsRef_UnmarshalJSON(t *testing.T) {

	testCases := map[string]struct {
		JSON                   string
		ExpectedAnnotationsRef *AnnotationsRef
	}{
		"base case": {
			JSON: `{"annotations":{"scope":""},"path":[]}`,
			ExpectedAnnotationsRef: &AnnotationsRef{
				Path:        []*Term{},
				Annotations: &Annotations{},
			},
		},
		"location case": {
			JSON: `{"custom":{"foo":"bar"},"description":"My desc","entrypoint":true,"location":{"file":"example.rego","row":1,"col":4},"organizations":["org1"],"scope":"rule","title":"My rule"}`,
			ExpectedAnnotationsRef: &AnnotationsRef{
				Path:        []*Term{},
				Annotations: &Annotations{},
				Location:    NewLocation([]byte{}, "example.rego", 1, 4),
			},
		},
	}

	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			var a AnnotationsRef
			err := json.Unmarshal([]byte(data.JSON), &a)
			if err != nil {
				t.Fatal(err)
			}

			if got, exp := len(a.Path), len(data.ExpectedAnnotationsRef.Path); exp != got {
				t.Fatalf("expected:\n%#v got\n%#v", exp, got)
			}

			if got, exp := a.Annotations.String(), data.ExpectedAnnotationsRef.Annotations.String(); exp != got {
				t.Fatalf("expected:\n%#v got\n%#v", exp, got)
			}

			if data.ExpectedAnnotationsRef.Location != nil {
				if !a.Location.Equal(data.ExpectedAnnotationsRef.Location) {
					t.Fatalf("expected location:\n%#v got\n%#v", data.ExpectedAnnotationsRef.Location, a.Location)
				}
			}
		})
	}
}
