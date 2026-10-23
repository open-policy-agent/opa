// Copyright 2026 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package ucast

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestUCASTNodeAsLucene(t *testing.T) {
	t.Parallel()

	f := func(op, field string, v any) UCASTNode {
		return UCASTNode{Type: "field", Op: op, Field: "doc." + field, Value: v}
	}
	c := func(op string, ns ...UCASTNode) UCASTNode {
		return UCASTNode{Type: "compound", Op: op, Value: ns}
	}

	tests := []struct {
		note   string
		source UCASTNode
		exp    string
		err    string
	}{
		{note: "eq string", source: f("eq", "name", "bob"), exp: `name:"bob"`},
		{note: "eq number", source: f("eq", "age", json.Number("42")), exp: `age:42`},
		{note: "eq bool", source: f("eq", "active", true), exp: `active:true`},
		{note: "ne requires the field", source: f("ne", "name", "bob"), exp: `(name:[* TO *] AND NOT name:"bob")`},
		{note: "not eq anchored", source: c("not", f("eq", "name", "bob")), exp: `(*:* AND NOT name:"bob")`},
		{note: "eq null", source: f("eq", "name", Null{}), exp: `(*:* AND NOT name:[* TO *])`},
		{note: "ne null", source: f("ne", "name", Null{}), exp: `name:[* TO *]`},
		{note: "gt", source: f("gt", "n", json.Number("5")), exp: `n:{5 TO *}`},
		{note: "startswith", source: f("startswith", "n", "app"), exp: `n:app*`},
		{note: "endswith", source: f("endswith", "n", "le"), exp: `n:*le`},
		{note: "contains", source: f("contains", "n", "pl"), exp: `n:*pl*`},
		{note: "startswith, escaped", source: f("startswith", "n", `a b*?:"\`), exp: `n:a\ b\*\?\:\"\\*`},
		{note: "startswith, empty", source: f("startswith", "n", ""), exp: `n:[* TO *]`},
		{note: "startswith, any of", source: f("startswith", "n", []any{"a", "b c"}), exp: `n:(a* OR b\ c*)`},
		{note: "endswith, any of one", source: f("endswith", "n", []any{"a"}), exp: `n:*a`},
		{note: "startswith, any of none", source: f("startswith", "n", []any{}), exp: `(*:* AND NOT *:*)`},
		{note: "startswith, any of, with empty", source: f("startswith", "n", []any{"a", ""}), exp: `n:[* TO *]`},
		{note: "not startswith", source: c("not", f("startswith", "n", "a")), exp: `(*:* AND NOT n:a*)`},
		{note: "outside", source: f("outside", "t", []any{"a", "b"}), exp: `t:({* TO "a"} OR {"a" TO "b"} OR {"b" TO *})`},
		{note: "outside one", source: f("outside", "n", []any{json.Number("3")}), exp: `n:({* TO 3} OR {3 TO *})`},
		{note: "outside none", source: f("outside", "t", []any{}), exp: `t:[* TO *]`},
		{note: "not outside", source: c("not", f("outside", "t", []any{"a"})), exp: `(*:* AND NOT t:({* TO "a"} OR {"a" TO *}))`},
		{note: "gte", source: f("gte", "n", json.Number("5")), exp: `n:[5 TO *]`},
		{note: "lt", source: f("lt", "n", json.Number("5")), exp: `n:{* TO 5}`},
		{note: "lte", source: f("lte", "n", json.Number("5")), exp: `n:[* TO 5]`},
		{note: "lte string bound", source: f("lte", "d", "2020-01-01T00:00:00Z"), exp: `d:[* TO "2020-01-01T00:00:00Z"]`},
		{note: "in", source: f("in", "x", []any{"a", json.Number("1"), true}), exp: `x:("a" OR 1 OR true)`},
		{note: "in empty", source: f("in", "x", []any{}), exp: `(*:* AND NOT *:*)`},
		{note: "nested field", source: f("eq", "resource.a-b", "x"), exp: `resource.a-b:"x"`},
		{note: "escaping", source: f("eq", "x", `a"b\c`), exp: `x:"a\"b\\c"`},
		{note: "injection stays in the phrase", source: f("eq", "x", `" OR *:* OR x:"`), exp: `x:"\" OR *:* OR x:\""`},
		{
			note:   "and, positives first",
			source: c("and", c("not", f("eq", "a", "x")), f("eq", "b", "y"), f("gt", "c", json.Number("1"))),
			exp:    `(b:"y" AND c:{1 TO *} AND NOT a:"x")`,
		},
		{
			note:   "and, only negatives anchored once",
			source: c("and", c("not", f("eq", "a", "x")), c("not", f("eq", "b", "y"))),
			exp:    `(*:* AND NOT a:"x" AND NOT b:"y")`,
		},
		{
			note:   "and, single child",
			source: c("and", f("eq", "a", "x")),
			exp:    `a:"x"`,
		},
		{
			note:   "or anchors negatives per branch",
			source: c("or", f("eq", "a", "x"), c("not", f("eq", "b", "y"))),
			exp:    `(a:"x" OR (*:* AND NOT b:"y"))`,
		},
		{
			note:   "not",
			source: c("not", f("eq", "a", "x")),
			exp:    `(*:* AND NOT a:"x")`,
		},
		{
			note:   "not of not",
			source: c("not", c("not", f("eq", "a", "x"))),
			exp:    `a:"x"`,
		},
		{
			note:   "not ne",
			source: c("not", f("ne", "a", "x")),
			exp:    `(*:* AND NOT (a:[* TO *] AND NOT a:"x"))`,
		},
		{
			note:   "not over and",
			source: c("not", c("and", f("eq", "a", "x"), f("eq", "b", "y"))),
			exp:    `(*:* AND NOT (a:"x" AND b:"y"))`,
		},
		{
			note:   "negated or inside and",
			source: c("and", f("eq", "a", "x"), c("not", c("or", f("eq", "b", "y"), f("eq", "c", "z")))),
			exp:    `(a:"x" AND NOT (b:"y" OR c:"z"))`,
		},
		{
			note:   "nested",
			source: c("or", c("and", f("eq", "a", "x"), f("lt", "b", json.Number("3"))), f("in", "c", []any{"p", "q"})),
			exp:    `((a:"x" AND b:{* TO 3}) OR c:("p" OR "q"))`,
		},
		{
			note: "fields of two documents",
			source: c("and",
				UCASTNode{Type: "field", Op: "eq", Field: "docs.tags", Value: "a"},
				UCASTNode{Type: "field", Op: "eq", Field: "users.tags", Value: "b"}),
			err: "filters one document, got fields of: docs, users",
		},
		{
			note: "fields of two documents, nested",
			source: c("or",
				UCASTNode{Type: "field", Op: "eq", Field: "docs.a", Value: "x"},
				c("not", UCASTNode{Type: "field", Op: "eq", Field: "users.a", Value: "y"})),
			err: "filters one document",
		},
		{note: "field ref", source: f("eq", "a", FieldRef{Field: "b"}), err: "references to other fields"},
		{note: "null with gt", source: f("gt", "a", Null{}), err: "null value can only be used"},
		{note: "bool range bound", source: f("gt", "a", true), err: "range bound must be a number or string"},
		{note: "startswith, not a string", source: f("startswith", "a", json.Number("1")), err: "'startswith' pattern requires string argument"},
		{note: "startswith, any of, not a string", source: f("startswith", "a", []any{"x", json.Number("1")}), err: "'startswith' pattern requires string argument"},
		{note: "outside, bool", source: f("outside", "a", []any{true}), err: "range bound must be a number or string"},
		{note: "outside without collection", source: f("outside", "a", "x"), err: "requires collection argument"},
		{note: "in without collection", source: f("in", "a", "x"), err: "requires collection argument"},
		{note: "in with bad element", source: f("in", "a", []any{map[string]any{}}), err: "unsupported value"},
		{note: "unsupported operator", source: f("nin", "a", []any{"x"}), err: "unsupported operator for lucene: nin"},
		{note: "unsupported value", source: f("eq", "a", []any{"x"}), err: "unsupported value"},
		{note: "unsupported node type", source: UCASTNode{Type: "document", Op: "exists"}, err: "unsupported node type"},
		{note: "compound without array", source: UCASTNode{Type: "compound", Op: "and", Value: "x"}, err: "requires an array of nodes"},
		{note: "empty and", source: c("and"), err: "requires a value"},
		{note: "not with two values", source: c("not", f("eq", "a", "x"), f("eq", "b", "y")), err: "exactly one value"},
		{note: "unknown compound operator", source: c("xor", f("eq", "a", "x")), err: "unrecognized operator: xor"},
		{note: "nested error", source: c("or", f("eq", "a", "x"), f("gt", "b", Null{})), err: "null value"},
		{note: "nested error under not", source: c("not", f("nin", "b", []any{"x"})), err: "unsupported operator"},
		{note: "nested error under and", source: c("and", f("nin", "b", []any{"x"})), err: "unsupported operator"},
		{note: "empty field", source: f("eq", "", "x"), err: "empty field name"},
		{note: "trailing dot", source: f("eq", "a.", "x"), err: "invalid lucene field name"},
		{note: "empty segment", source: f("eq", "a..b", "x"), err: "invalid lucene field name"},
		{note: "leading dot", source: f("eq", ".a", "x"), err: "invalid lucene field name"},
		{note: "field with space", source: f("eq", `doc.a b`, "x"), err: "invalid lucene field name"},
		{note: "field injection", source: f("eq", `doc.a:* OR x`, "x"), err: "invalid lucene field name"},
		{note: "field starting with digit", source: f("eq", `1a`, "x"), err: "invalid lucene field name"},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()
			act, err := tc.source.AsLucene()
			if tc.err != "" {
				if err == nil {
					t.Fatalf("expected error %q, got result %q", tc.err, act)
				}
				if !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("expected error %q, got %q", tc.err, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if act != tc.exp {
				t.Errorf("expected %q, got %q", tc.exp, act)
			}
		})
	}
}

// A value from the input can not leave its phrase: the query is the field and one
// quoted phrase, which ends where the query does and unescapes to the value.
func FuzzLuceneValueStaysQuoted(f *testing.F) {
	for _, s := range []string{`" OR *:* OR x:"`, `a\`, `\"`, `\\"`, `{!lucene}*:*`, "a\nb"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, v string) {
		q, err := (&UCASTNode{Type: "field", Op: "eq", Field: "x", Value: v}).AsLucene()
		if err != nil {
			t.Fatal(err)
		}
		phrase, ok := strings.CutPrefix(q, `x:"`)
		if !ok {
			t.Fatalf("not a phrase on x: %s", q)
		}
		var unescaped strings.Builder
		for i := 0; i < len(phrase); i++ {
			switch phrase[i] {
			case '\\':
				i++
				if i == len(phrase) {
					t.Fatalf("dangling escape: %s", q)
				}
			case '"':
				if i != len(phrase)-1 {
					t.Fatalf("phrase ends at %d, before the query does: %s", i, q)
				}
				if unescaped.String() != v {
					t.Fatalf("phrase is %q, not the value %q", unescaped.String(), v)
				}
				return
			}
			unescaped.WriteByte(phrase[i])
		}
		t.Fatalf("phrase not closed: %s", q)
	})
}

// A pattern from the input is literal: the term is its characters, each of them
// escaped unless a letter or digit, and only the wildcards added around it are not.
func FuzzLuceneWildcardEscaped(f *testing.F) {
	for _, s := range []string{"app", `a b`, `*`, `\`, `x OR y:*`, "a\u3000b", "\U0001F600", "é1"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, v string) {
		if v == "" || !utf8.ValidString(v) {
			return
		}
		q, err := (&UCASTNode{Type: "field", Op: "contains", Field: "x", Value: v}).AsLucene()
		if err != nil {
			t.Fatal(err)
		}
		term, ok := strings.CutPrefix(q, "x:*")
		if !ok || !strings.HasSuffix(term, "*") {
			t.Fatalf("not a contains pattern on x: %s", q)
		}
		term = strings.TrimSuffix(term, "*")
		var unescaped strings.Builder
		rs := []rune(term)
		for i := 0; i < len(rs); i++ {
			if rs[i] == '\\' {
				i++
				if i == len(rs) {
					t.Fatalf("dangling escape: %s", q)
				}
			} else if !unicode.IsLetter(rs[i]) && !unicode.IsDigit(rs[i]) {
				t.Fatalf("unescaped %q: %s", rs[i], q)
			}
			unescaped.WriteRune(rs[i])
		}
		if unescaped.String() != v {
			t.Fatalf("pattern is %q, not the value %q", unescaped.String(), v)
		}
	})
}
