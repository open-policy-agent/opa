// Copyright 2026 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package compile

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/internal/ucast"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
)

// luceneValues are the values Solr indexes for a field: a scalar is one value, an
// array its elements, and a missing field or an empty array none.
func luceneValues(x any) []any {
	switch v := x.(type) {
	case nil:
		return nil
	case []any:
		return v
	default:
		return []any{v}
	}
}

// luceneCmp orders two values as Solr does a numeric or a string field.
func luceneCmp(x, y any) int {
	a, aerr := strconv.ParseFloat(fmt.Sprint(x), 64)
	b, berr := strconv.ParseFloat(fmt.Sprint(y), 64)
	if aerr == nil && berr == nil {
		return cmp.Compare(a, b)
	}
	return strings.Compare(fmt.Sprint(x), fmt.Sprint(y))
}

// luceneEval interprets the tree the way Solr evaluates the query AsLucene renders
// for it: a comparison on a field holds if SOME of its values satisfy it.
func luceneEval(t *testing.T, n *ucast.UCASTNode, doc map[string]any) bool {
	t.Helper()
	if n.Type == "compound" {
		nodes := n.Value.([]ucast.UCASTNode)
		switch n.Op {
		case "not":
			return !luceneEval(t, &nodes[0], doc)
		case "and", "or":
			for i := range nodes {
				if luceneEval(t, &nodes[i], doc) == (n.Op == "or") {
					return n.Op == "or"
				}
			}
			return n.Op == "and"
		}
		t.Fatalf("unknown compound %s", n.Op)
	}

	vs := luceneValues(doc[strings.TrimPrefix(n.Field, "doc.")])
	some := func(f func(any) bool) bool { return slices.ContainsFunc(vs, f) }
	same := func(a, b any) bool { return fmt.Sprint(a) == fmt.Sprint(b) }

	if _, isNull := n.Value.(ucast.Null); isNull {
		return (n.Op == "eq") == (len(vs) == 0)
	}
	switch n.Op {
	case "eq":
		return some(func(v any) bool { return same(v, n.Value) })
	case "ne": // the field has a value, and none is v
		return len(vs) > 0 && !some(func(v any) bool { return same(v, n.Value) })
	case "in":
		return some(func(v any) bool {
			for _, w := range n.Value.([]any) {
				if same(v, w) {
					return true
				}
			}
			return false
		})
	case "startswith", "endswith", "contains":
		match := map[string]func(string, string) bool{"startswith": strings.HasPrefix, "endswith": strings.HasSuffix, "contains": strings.Contains}[n.Op]
		patterns, ok := n.Value.([]any)
		if !ok {
			patterns = []any{n.Value}
		}
		return some(func(v any) bool {
			s, ok := v.(string)
			return ok && slices.ContainsFunc(patterns, func(p any) bool { return match(s, p.(string)) })
		})
	case "outside":
		return some(func(v any) bool {
			for _, w := range n.Value.([]any) {
				if same(v, w) {
					return false
				}
			}
			return true
		})
	case "gt":
		return some(func(v any) bool { return luceneCmp(v, n.Value) > 0 })
	case "gte":
		return some(func(v any) bool { return luceneCmp(v, n.Value) >= 0 })
	case "lt":
		return some(func(v any) bool { return luceneCmp(v, n.Value) < 0 })
	case "lte":
		return some(func(v any) bool { return luceneCmp(v, n.Value) <= 0 })
	}
	t.Fatalf("unknown operator %s", n.Op)
	return false
}

type luceneCase struct {
	note   string
	rego   string // body of the rule `include`, or whole rules if it starts with "include"
	groups []string
	reject string // the translation fails with this error

	// inferred leaves the shapes of the fields to the policy, instead of declaring
	// them in luceneSchema.
	inferred bool
}

// luceneSchema declares the shapes of the fields of the documents of luceneDocs.
var luceneSchema = map[string]any{"type": "object", "properties": map[string]any{
	"tags":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
	"colors": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
	"name":   map[string]any{"type": "string"},
	"level":  map[string]any{"type": "integer"},
}}

// translate runs the pipeline the Compile API runs for the Lucene target, up to
// the tree: partial evaluation, the multi-valued rewrite, the checks.
func translate(t *testing.T, c luceneCase) (node *ucast.UCASTNode, all, none bool, err error) {
	t.Helper()
	pq, perr := rego.New(
		rego.Module("f.rego", luceneModule(c)),
		rego.Query("data.filters.include"),
		rego.Input(map[string]any{"user": "u", "groups": toAny(c.groups)}),
		rego.Unknowns([]string{"input.doc"}),
	).Partial(t.Context())
	if perr != nil {
		t.Fatal(perr)
	}
	if pq.Queries == nil {
		return nil, false, true, nil
	}
	var schema any = luceneSchema
	if c.inferred {
		schema = nil
	}
	qs, errs := NormalizeMultiValued(pq.Queries, ast.MustParseRef("input.doc"), schema)
	if errs != nil {
		return nil, false, false, errs[0]
	}
	if qs == nil { // no body can hold
		return nil, false, true, nil
	}
	pq.Queries = qs
	cons, _ := NewConstraints("lucene", "solr")
	if errs := Check(pq, NewConstraintSet(cons), nil).ASTErrors(); errs != nil {
		return nil, false, false, errs[0]
	}
	node = BodiesToUCAST(qs, &Opts{})
	return node, node == nil, false, nil
}

func luceneModule(c luceneCase) string {
	if strings.HasPrefix(c.rego, "include") {
		return "package filters\n" + c.rego + "\n"
	}
	return "package filters\ninclude if {\n" + c.rego + "\n}\n"
}

func toAny(xs []string) []any {
	out := make([]any, len(xs))
	for i := range xs {
		out[i] = xs[i]
	}
	return out
}

// regoHolds evaluates the policy on the document with the improved `not`, under
// which a negation holds for an undefined value, as Lucene's does for a missing field.
func regoHolds(t *testing.T, c luceneCase, doc map[string]any) bool {
	t.Helper()
	module := strings.Replace(luceneModule(c), "package filters\n", "package filters\nimport future.keywords.not\n", 1)
	rs, err := rego.New(
		rego.Module("f.rego", module),
		rego.Query("data.filters.include"),
		rego.Input(map[string]any{"user": "u", "groups": toAny(c.groups), "doc": doc}),
	).Eval(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return len(rs) > 0 && rs[0].Expressions[0].Value == true
}

// luceneDocs enumerates documents over small domains. The array fields are always
// present, possibly empty; the single-valued fields may be missing.
func luceneDocs() []map[string]any {
	var docs []map[string]any
	for _, tags := range [][]any{{}, {"a"}, {"b"}, {"a", "b"}, {"c"}, {"a", "a"}} {
		for _, colors := range [][]any{{}, {"a"}, {"b", "c"}} {
			for _, name := range []any{nil, "a", "b", "c"} {
				for _, level := range []any{nil, 1, 2, 3} {
					d := map[string]any{"tags": tags, "colors": colors}
					if name != nil {
						d["name"] = name
					}
					if level != nil {
						d["level"] = level
					}
					docs = append(docs, d)
				}
			}
		}
	}
	return docs
}

func TestLuceneMatchesRego(t *testing.T) {
	cases := []luceneCase{
		// Arrays.
		{note: "some, eq", rego: `some p in input.doc.tags; p == "a"`},
		{note: "wildcard, eq", rego: `input.doc.tags[_] == "a"`},
		{note: "some, in", rego: `some p in input.doc.tags; p in {"a", "b"}`},
		{note: "known in field", rego: `"a" in input.doc.tags`},
		{note: "not known in field", rego: `not "a" in input.doc.tags`},
		{note: "independent wildcards", rego: "input.doc.tags[_] == \"a\"\ninput.doc.colors[_] == \"b\""},
		{note: "disjunction", rego: "include if \"a\" in input.doc.tags\ninclude if \"b\" in input.doc.colors"},
		{note: "count is zero", rego: `count(input.doc.tags) == 0`},
		{note: "not count is zero", rego: `not count(input.doc.tags) == 0`, reject: "nested call operand"},
		{note: "empty", rego: `input.doc.tags == []`},
		{note: "not empty", rego: `input.doc.tags != []`},
		{note: "not empty, negated", rego: `not input.doc.tags == []`},
		{note: "every, none is", rego: `every p in input.doc.tags { p != "a" }`},
		{note: "every, not eq", rego: `every p in input.doc.tags { not p == "a" }`},
		{note: "every, none in", rego: `every p in input.doc.tags { not p in {"a", "b"} }`},
		{note: "every known, not in field", rego: `every g in input.groups { not g in input.doc.tags }`, groups: []string{"a", "b"}},
		{note: "every known, one", rego: `every g in input.groups { not g in input.doc.tags }`, groups: []string{"c"}},
		{note: "every known, empty", rego: `every g in input.groups { not g in input.doc.tags }`, groups: []string{}},
		{note: "every known, two exprs", rego: "every g in input.groups {\n not g in input.doc.colors\n not g in input.doc.tags\n}", groups: []string{"a", "c"}},
		{note: "every known, ground condition holds", rego: "every g in input.groups {\n g != \"z\"\n not g in input.doc.tags\n}", groups: []string{"a", "b"}},
		{note: "every known, ground condition fails", rego: "every g in input.groups {\n g != \"z\"\n not g in input.doc.tags\n}", groups: []string{"a", "z"}},
		{note: "every known, ground fails in one rule", rego: "include if every g in input.groups {\n g != \"z\"\n not g in input.doc.tags\n}\ninclude if \"c\" in input.doc.colors", groups: []string{"z"}},
		{note: "every known, only ground", rego: "every g in input.groups {\n g != \"z\"\n}\n\"a\" in input.doc.tags", groups: []string{"a"}},
		{note: "every over a set", rego: `every g in {"a", "b"} { not g in input.doc.tags }`},
		{note: "every with key", rego: "every i, g in input.groups {\n i != 5\n not g in input.doc.tags\n}", groups: []string{"a"}},
		{note: "every known, in field", rego: `every g in input.groups { g in input.doc.tags }`, groups: []string{"a"}},
		{note: "two memberships", rego: "\"a\" in input.doc.tags\n\"a\" in input.doc.colors"},
		{note: "in set and not in other", rego: "some p in input.doc.tags; p in {\"a\", \"c\"}\nnot \"b\" in input.doc.colors"},
		{note: "element startswith", rego: `some p in input.doc.tags; startswith(p, "a")`},
		{note: "element endswith", rego: `some p in input.doc.colors; endswith(p, "c")`},
		{note: "element contains", rego: `some p in input.doc.colors; contains(p, "b")`},
		{note: "element any suffix", rego: `some p in input.doc.colors; strings.any_suffix_match(p, ["c", "x"])`},
		{note: "array any prefix", rego: `strings.any_prefix_match(input.doc.tags, {"a", "c"})`},
		{note: "array any prefix, and a member", rego: "strings.any_prefix_match(input.doc.tags, [\"b\"])\n\"a\" in input.doc.tags"},
		{note: "array any prefix, inferred", rego: "strings.any_prefix_match(input.doc.tags, [\"b\"])\n\"a\" in input.doc.tags", inferred: true},
		{note: "not array any prefix", rego: `not strings.any_prefix_match(input.doc.tags, ["a"])`},
		{note: "every, none has a prefix", rego: `every p in input.doc.tags { not strings.any_prefix_match(p, ["a", "b"]) }`},
		{note: "every, none ends with", rego: `every p in input.doc.colors { not endswith(p, "c") }`},
		{note: "every, none starts with", rego: `every p in input.doc.tags { not startswith(p, "a") }`},
		{note: "every, all start with", rego: `every p in input.doc.tags { startswith(p, "a") }`, reject: "compare the element with known values"},
		{note: "array startswith", rego: `startswith(input.doc.tags, "a")`, reject: "input.doc.tags is an array"},
		{note: "element range", rego: `some p in input.doc.tags; p > "a"`},
		{note: "element range, known on the right", rego: `some p in input.doc.tags; "b" >= p`},
		{note: "element ranges, two elements", rego: "some p in input.doc.tags; p > \"a\"\nsome q in input.doc.tags; q < \"c\""},
		{note: "element ranges, one element", rego: `some p in input.doc.tags; p > "a"; p < "c"`, reject: "invalid ref operand"},
		{note: "every, none below", rego: `every p in input.doc.tags { not p < "b" }`},
		{note: "every, none above, known on the left", rego: `every p in input.doc.tags { not "b" < p }`},
		{note: "every, all above", rego: `every p in input.doc.tags { p > "b" }`, reject: "compare the element with known values"},
		{note: "null element eq", rego: `some p in input.doc.tags; p == null`, reject: "invalid ref operand"},
		{note: "null known in field", rego: `null in input.doc.tags`, reject: "must be known"},
		{note: "every, none null", rego: `every p in input.doc.tags { p != null }`, reject: "compare the element with known values"},
		{note: "some neq", rego: `some p in input.doc.tags; p != "a"`, reject: "cannot be expressed"},
		{note: "every, all are", rego: `every p in input.doc.tags { p == "a" }`},
		{note: "every, all are, flipped", rego: `every p in input.doc.tags { "a" == p }`},
		{note: "every, all in", rego: `every p in input.doc.tags { p in {"a", "b"} }`},
		{note: "every, all in an array", rego: `every p in input.doc.tags { p in ["b", "a", "b"] }`},
		{note: "every, all in, none", rego: `every p in input.doc.tags { p in set() }`},
		{note: "every, not ne", rego: `every p in input.doc.tags { not p != "a" }`},
		{note: "every, all in, mixed types", rego: `every p in input.doc.tags { p in {"a", 1} }`, reject: "all strings, or all numbers"},
		{note: "same values", rego: `{p | some p in input.doc.tags} == {"a", "b"}`},
		{note: "same values, flipped", rego: `{"a"} == {p | some p in input.doc.tags}`},
		{note: "same values, wildcard", rego: `{p | p := input.doc.tags[_]} == {"b", "c"}`},
		{note: "same values, none", rego: `{p | some p in input.doc.tags} == set()`},
		{note: "same values, and a member", rego: "{p | some p in input.doc.colors} == {\"b\", \"c\"}\n\"a\" in input.doc.tags"},
		{note: "same values, not", rego: `not {p | some p in input.doc.tags} == {"a"}`, reject: `"not" not permitted`},
		{note: "same values, ne", rego: `{p | some p in input.doc.tags} != {"a"}`, reject: "only be equal to a known set"},
		{note: "same values, single value", rego: `{p | some p in input.doc.name} == {"a"}`, reject: "input.doc.name is a single value"},
		{note: "array eq", rego: `input.doc.tags == "a"`, reject: "input.doc.tags is an array"},
		{note: "array ne", rego: `input.doc.tags != "a"`, reject: "input.doc.tags is an array"},
		{note: "array in", rego: `input.doc.tags in {"a"}`, reject: "input.doc.tags is an array"},
		{note: "array range", rego: `input.doc.tags >= "a"`, reject: "input.doc.tags is an array"},
		{note: "array bare", rego: `input.doc.tags`, reject: "always true"},

		// Single values.
		{note: "eq", rego: `input.doc.name == "a"`},
		{note: "ne", rego: `input.doc.name != "a"`},
		{note: "not eq", rego: `not input.doc.name == "a"`},
		{note: "range", rego: "input.doc.level > 1\ninput.doc.level < 3"},
		{note: "not lt", rego: `not input.doc.level < 2`},
		{note: "in", rego: `input.doc.name in {"a", "b"}`},
		{note: "startswith", rego: `startswith(input.doc.name, "a")`},
		{note: "endswith", rego: `endswith(input.doc.name, "c")`},
		{note: "contains", rego: `contains(input.doc.name, "b")`},
		{note: "startswith empty", rego: `startswith(input.doc.name, "")`},
		{note: "not startswith", rego: `not startswith(input.doc.name, "a")`},
		{note: "any prefix", rego: `strings.any_prefix_match(input.doc.name, ["a", "b"])`},
		{note: "any suffix, set", rego: `strings.any_suffix_match(input.doc.name, {"c"})`},
		{note: "any prefix, none", rego: `strings.any_prefix_match(input.doc.name, set())`},
		{note: "any suffix, empty", rego: `strings.any_suffix_match(input.doc.name, "")`},
		{note: "not any prefix", rego: `not strings.any_prefix_match(input.doc.name, ["a", "c"])`},
		{note: "startswith, known first", rego: `startswith("abc", input.doc.name)`, reject: "must be known"},
		{note: "not in", rego: `not input.doc.name in {"a", "b"}`},
		{note: "null", rego: `input.doc.name == null`, reject: "missing, not null"},
		{note: "not null", rego: `input.doc.name != null`},
		{note: "not not null", rego: `not input.doc.name != null`},
		{note: "every known, ne", rego: `every g in input.groups { g != input.doc.name }`, groups: []string{"a", "b"}},
		{note: "single in field", rego: `"a" in input.doc.name`, reject: "input.doc.name is a single value"},
		{note: "single some", rego: `some n in input.doc.name; n == "a"`, reject: "input.doc.name is a single value"},
		{note: "single count", rego: `count(input.doc.name) == 0`, reject: "input.doc.name is a single value"},
		{note: "single empty", rego: `input.doc.name == []`, reject: "input.doc.name is a single value"},
		{note: "single every", rego: `every n in input.doc.name { n != "a" }`, reject: "input.doc.name is a single value"},

		// Both.
		{note: "member and range", rego: "\"a\" in input.doc.tags\ninput.doc.level >= 2"},
		{note: "member, not eq", rego: "\"a\" in input.doc.tags\nnot input.doc.name == \"b\""},
		{note: "not member, or eq", rego: "include if not \"a\" in input.doc.tags\ninclude if input.doc.level == 3"},
		{note: "every, member, range", rego: "every p in input.doc.tags { p != \"a\" }\n\"b\" in input.doc.colors\ninput.doc.level >= 2"},
		{note: "every known, or member", rego: "include if every g in input.groups { not g in input.doc.tags }\ninclude if \"c\" in input.doc.colors", groups: []string{"a", "b"}},

		// Without a schema, the policy's use decides.
		{note: "inferred, both", rego: "\"a\" in input.doc.tags\nnot input.doc.name == \"b\"\ninput.doc.level >= 2", inferred: true},
		{note: "inferred, every and range", rego: "every p in input.doc.tags { p != \"a\" }\ninput.doc.level > 1", inferred: true},
		{note: "inferred, one field both ways", rego: "\"a\" in input.doc.tags\ninput.doc.tags != \"b\"", inferred: true, reject: "used both as an array and as a single value"},
		{note: "inferred, both ways in two rules", rego: "include if \"a\" in input.doc.tags\ninclude if input.doc.tags == \"b\"", inferred: true, reject: "used both"},
	}
	for _, c := range cases {
		t.Run(c.note, func(t *testing.T) {
			node, all, none, err := translate(t, c)
			if c.reject != "" {
				if err == nil || !strings.Contains(err.Error(), c.reject) {
					t.Fatalf("expected error containing %q, got %v", c.reject, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			holds := func(doc map[string]any) bool { return all || (!none && luceneEval(t, node, doc)) }

			for _, doc := range luceneDocs() {
				if want, got := regoHolds(t, c, doc), holds(doc); want != got {
					t.Fatalf("rego=%v lucene=%v for %v", want, got, doc)
				}
			}
		})
	}
}
