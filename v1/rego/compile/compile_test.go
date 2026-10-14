package compile_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/rego/compile"
)

func TestCompileFilters(t *testing.T) {
	t.Run("single target+dialect/mask", func(t *testing.T) {
		target, dialect := "sql", "postgresql"
		unknowns := []*ast.Term{ast.MustParseTerm("input.fruit")}
		query := ast.MustParseBody("data.filters.include")
		maskRule := ast.MustParseRef("data.filters.mask")

		module := `package filters
include if input.fruit.name in input.names
mask.fruit.owner := {"replace": {"value": "***"}} if "banana" in input.names
`

		r := compile.New(
			compile.Target(target, dialect),
			compile.ParsedUnknowns(unknowns...),
			compile.ParsedQuery(query),
			compile.MaskRule(maskRule),
			compile.Rego(
				rego.Module("filters.rego", module),
				rego.Input(map[string]any{"names": []string{"apple", "banana"}}),
			),
		)

		prep, err := r.Prepare(t.Context())
		if err != nil {
			t.Fatal(err)
		}

		filters, err := prep.Compile(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if exp, act := "WHERE fruit.name IN (E'apple', E'banana')", filters.One().Query; exp != act {
			t.Errorf("query: expected %q, got %q", exp, act)
		}
		exp := map[string]any{"fruit": map[string]any{"owner": map[string]any{"replace": map[string]any{"value": "***"}}}}
		act := filters.One().Masks
		if diff := cmp.Diff(exp, act); diff != "" {
			t.Error("unexpected masks (-want, +got):", diff)
		}
	})

	t.Run("single target+dialect/mappings", func(t *testing.T) {
		target, dialect := "sql", "postgresql"
		unknowns := []*ast.Term{ast.MustParseTerm("input.fruit")}
		query := ast.MustParseBody("data.filters.include")

		module := `package filters
include if input.fruit.name in input.names
`

		r := compile.New(
			compile.Target(target, dialect),
			compile.ParsedUnknowns(unknowns...),
			compile.ParsedQuery(query),
			compile.Mappings(map[string]any{"fruit": map[string]any{
				"$self": "F",
				"name":  "N",
			}}),
			compile.Rego(
				rego.Module("filters.rego", module),
				rego.Input(map[string]any{"names": []string{"apple", "banana"}}),
			),
		)

		prep, err := r.Prepare(t.Context())
		if err != nil {
			t.Fatal(err)
		}

		filters, err := prep.Compile(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if exp, act := "WHERE F.N IN (E'apple', E'banana')", filters.One().Query; exp != act {
			t.Errorf("query: expected %q, got %q", exp, act)
		}
	})

	t.Run("multiple target+dialect", func(t *testing.T) {
		unknowns := []*ast.Term{ast.MustParseTerm("input.fruit")}
		query := ast.MustParseBody("data.filters.include")
		maskRule := ast.MustParseRef("data.filters.mask")

		module := `package filters
include if input.fruit.name in input.names
mask.fruit.owner := {"replace": {"value": "***"}}
`

		r := compile.New(
			compile.Target("sql", "sqlserver"),
			compile.Target("sql", "mysql"),
			compile.Target("ucast", "prisma"),
			compile.ParsedUnknowns(unknowns...),
			compile.ParsedQuery(query),
			compile.MaskRule(maskRule),
			compile.Rego(
				rego.Module("filters.rego", module),
				rego.Input(map[string]any{"names": []string{"apple", "banana"}}),
			),
		)

		prep, err := r.Prepare(t.Context())
		if err != nil {
			t.Fatal(err)
		}

		filters, err := prep.Compile(t.Context())
		if err != nil {
			t.Fatal(err)
		}

		t.Run("mysql", func(t *testing.T) {
			if exp, act := "WHERE fruit.name IN ('apple', 'banana')", filters.For("sql", "mysql").Query; exp != act {
				t.Errorf("query: expected %q, got %q", exp, act)
			}
			exp := map[string]any{"fruit": map[string]any{"owner": map[string]any{"replace": map[string]any{"value": "***"}}}}
			act := filters.For("sql", "mysql").Masks
			if diff := cmp.Diff(exp, act); diff != "" {
				t.Error("unexpected masks (-want, +got):", diff)
			}
		})

		t.Run("sqlserver", func(t *testing.T) {
			if exp, act := "WHERE fruit.name IN (N'apple', N'banana')", filters.For("sql", "sqlserver").Query; exp != act {
				t.Errorf("query: expected %q, got %q", exp, act)
			}
			exp := map[string]any{"fruit": map[string]any{"owner": map[string]any{"replace": map[string]any{"value": "***"}}}}
			act := filters.For("sql", "sqlserver").Masks
			if diff := cmp.Diff(exp, act); diff != "" {
				t.Error("unexpected masks (-want, +got):", diff)
			}
		})

		t.Run("ucast", func(t *testing.T) {
			{
				exp := map[string]any{
					"field":    "fruit.name",
					"operator": "in",
					"type":     "field",
					"value":    []any{"apple", "banana"},
				}
				act := filters.For("ucast", "prisma").Query
				if diff := cmp.Diff(exp, act); diff != "" {
					t.Error("unexpected query: (-want, +got)", diff)
				}
			}
			{
				exp := map[string]any{"fruit": map[string]any{"owner": map[string]any{"replace": map[string]any{"value": "***"}}}}
				act := filters.For("ucast", "prisma").Masks
				if diff := cmp.Diff(exp, act); diff != "" {
					t.Error("unexpected masks (-want, +got):", diff)
				}
			}
		})
	})
	t.Run("every over a known domain", func(t *testing.T) {
		for _, tc := range []struct {
			note   string
			banned int
			exp    string
			err    string
		}{
			{note: "unrolled", banned: 2, exp: "WHERE (fruits.name <> E'0' AND fruits.name <> E'1')"},
			{note: "domain over the limit", banned: 33, err: `"every" not permitted`},
		} {
			t.Run(tc.note, func(t *testing.T) {
				banned := make([]string, tc.banned)
				for i := range banned {
					banned[i] = strconv.Itoa(i)
				}
				prep, err := compile.New(
					compile.Target("sql", "postgresql"),
					compile.ParsedUnknowns(ast.MustParseTerm("input.fruits")),
					compile.ParsedQuery(ast.MustParseBody("data.filters.include")),
					compile.Rego(
						rego.Module("filters.rego", "package filters\ninclude if every g in input.banned { input.fruits.name != g }\n"),
						rego.Input(map[string]any{"banned": banned}),
					),
				).Prepare(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				filters, err := prep.Compile(t.Context())
				if tc.err != "" {
					if err == nil || !strings.Contains(err.Error(), tc.err) {
						t.Fatalf("expected error containing %q, got %v", tc.err, err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if act := filters.One().Query; act != tc.exp {
					t.Errorf("expected %q, got %q", tc.exp, act)
				}
			})
		}
	})
}

func TestCompileFiltersInvalidMappings(t *testing.T) {
	module := `package filters
include if input.fruit.name in input.names
`

	tests := []struct {
		note     string
		mappings map[string]any
		expErr   string
	}{
		{
			note:     "non-string $self",
			mappings: map[string]any{"fruit": map[string]any{"$self": 123}},
			expErr:   "mappings: invalid mappings: fruit.$self: expected string, got int",
		},
		{
			note:     "null $self",
			mappings: map[string]any{"fruit": map[string]any{"$self": nil}},
			expErr:   "mappings: invalid mappings: fruit.$self: expected string, got <nil>",
		},
		{
			note:     "object $self",
			mappings: map[string]any{"fruit": map[string]any{"$self": map[string]any{}}},
			expErr:   "mappings: invalid mappings: fruit.$self: expected string, got map[string]interface {}",
		},
		{
			note:     "non-string $table",
			mappings: map[string]any{"name": map[string]any{"$table": []any{"fruit"}}},
			expErr:   "mappings: invalid mappings: name.$table: expected string, got []interface {}",
		},
		{
			note:     "non-string column",
			mappings: map[string]any{"fruit": map[string]any{"$self": "F", "name": 1.5}},
			expErr:   "mappings: invalid mappings: fruit.name: expected string, got float64",
		},
		{
			note:     "per-dialect, non-string column",
			mappings: map[string]any{"postgresql": map[string]any{"fruit": map[string]any{"name": true}}},
			expErr:   "mappings: invalid mappings: fruit.name: expected string, got bool",
		},
		{
			note:     "non-map dialect entry",
			mappings: map[string]any{"postgresql": "nope"},
			expErr:   "mappings: invalid mappings for dialect postgresql",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			r := compile.New(
				compile.Target("sql", "postgresql"),
				compile.ParsedUnknowns(ast.MustParseTerm("input.fruit")),
				compile.ParsedQuery(ast.MustParseBody("data.filters.include")),
				compile.Mappings(tc.mappings),
				compile.Rego(
					rego.Module("filters.rego", module),
					rego.Input(map[string]any{"names": []string{"apple", "banana"}}),
				),
			)

			prep, err := r.Prepare(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			_, err = prep.Compile(t.Context())
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, compile.ErrInvalidMappings) {
				t.Errorf("expected ErrInvalidMappings, got %v", err)
			}
			if act := err.Error(); act != tc.expErr {
				t.Errorf("expected %q, got %q", tc.expErr, act)
			}
		})
	}
}

func TestCompileFiltersUnselectedDialectMappings(t *testing.T) {
	module := `package filters
include if input.fruit.name in input.names
`

	// Mappings for a dialect other than the one being compiled fall through to
	// the flat lookup, where the per-dialect wrapper isn't a table mapping.
	r := compile.New(
		compile.Target("sql", "mysql"),
		compile.ParsedUnknowns(ast.MustParseTerm("input.fruit")),
		compile.ParsedQuery(ast.MustParseBody("data.filters.include")),
		compile.Mappings(map[string]any{"postgresql": map[string]any{"fruit": map[string]any{
			"$self": "F",
			"name":  "N",
		}}}),
		compile.Rego(
			rego.Module("filters.rego", module),
			rego.Input(map[string]any{"names": []string{"apple", "banana"}}),
		),
	)

	prep, err := r.Prepare(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	filters, err := prep.Compile(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if exp, act := "WHERE fruit.name IN ('apple', 'banana')", filters.One().Query; exp != act {
		t.Errorf("query: expected %q, got %q", exp, act)
	}
}

func TestCompileFiltersTranslateQueriesTimer(t *testing.T) {
	module := `package filters
include if input.fruit.name in input.names
`

	m := metrics.New()
	r := compile.New(
		compile.Target("sql", "postgresql"),
		compile.ParsedUnknowns(ast.MustParseTerm("input.fruit")),
		compile.ParsedQuery(ast.MustParseBody("data.filters.include")),
		compile.Metrics(m),
		compile.Rego(
			rego.Module("filters.rego", module),
			rego.Input(map[string]any{"names": []string{"apple", "banana"}}),
		),
	)

	prep, err := r.Prepare(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prep.Compile(t.Context()); err != nil {
		t.Fatal(err)
	}

	if n := m.Timer(metrics.CompileTranslateQueries).Int64(); n == 0 {
		t.Errorf("expected %s timer to be recorded, got 0", metrics.CompileTranslateQueries)
	}
}

// A policy the target cannot render is a problem of the policy, not of the server.
func TestCompileFiltersTranslationError(t *testing.T) {
	prep, err := compile.New(
		compile.Target("sql", "postgresql"),
		compile.ParsedUnknowns(ast.MustParseTerm("input.fruits")),
		compile.ParsedQuery(ast.MustParseBody("data.filters.include")),
		compile.Rego(rego.Module("filters.rego", "package filters\ninclude if input.fruits.price > null\n")),
	).Prepare(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	_, err = prep.Compile(t.Context())
	errs, ok := errors.AsType[ast.Errors](err)
	if !ok {
		t.Fatalf("expected ast.Errors, got %T: %v", err, err)
	}
	if len(errs) != 1 || errs[0].Code != "pe_fragment_error" ||
		errs[0].Message != "null value can only be used with 'eq' or 'ne' operators" {
		t.Errorf("unexpected errors: %v", errs)
	}
}

// luceneUnknowns is the document the Lucene tests filter: input.doc.<field> is the field.
var luceneUnknowns = []*ast.Term{ast.MustParseTerm("input.doc")}

func TestCompileFiltersLucene(t *testing.T) {
	const module = `package filters
include if {
	input.doc.owner == input.user
	input.doc.level >= 3
	not input.doc.status == "draft"
}

include if input.doc.tag in ["a", "b"]

allow_all if input.user == "alice"

deny if {
	input.user == "nobody"
	input.doc.owner == "x"
}

exists if input.doc.owner != null

reversed if 3 <= input.doc.level

bare if input.doc.status
`

	tests := []struct {
		note     string
		query    string
		mappings map[string]any
		exp      any
		err      string
	}{
		{
			note:  "conjunctions and disjunctions",
			query: "data.filters.include",
			exp:   `((owner:"alice" AND level:[3 TO *] AND NOT status:"draft") OR tag:("a" OR "b"))`,
		},
		{
			note:     "mappings rename fields",
			query:    "data.filters.include",
			mappings: map[string]any{"doc": map[string]any{"owner": "owner_id"}},
			exp:      `((owner_id:"alice" AND level:[3 TO *] AND NOT status:"draft") OR tag:("a" OR "b"))`,
		},
		{
			note:     "mappings per target",
			query:    "data.filters.exists",
			mappings: map[string]any{"lucene": map[string]any{"doc": map[string]any{"owner": "o"}}},
			exp:      `o:[* TO *]`,
		},
		{
			note:  "bare reference",
			query: "data.filters.bare",
			err:   "try `input.doc.status != false`",
		},
		{
			note:  "unknown on the right",
			query: "data.filters.reversed",
			exp:   `level:[3 TO *]`,
		},
		{
			note:  "unconditional allow",
			query: "data.filters.allow_all",
			exp:   "*:*",
		},
		{
			note:  "unconditional deny",
			query: "data.filters.deny",
			exp:   nil,
		},
		{
			note:  "startswith",
			query: "startswith(input.doc.owner, \"a\")",
			exp:   `owner:a*`,
		},
		{
			note:  "endswith, value escaped",
			query: "endswith(input.doc.owner, \" OR *:*\")",
			exp:   `owner:*\ OR\ \*\:\*`,
		},
		{
			note:  "unsupported builtin",
			query: "regex.match(\"a\", input.doc.owner)",
			err:   "invalid builtin `regex.match`",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			opts := []compile.CompileOption{
				compile.Target("lucene", "solr"),
				compile.ParsedUnknowns(luceneUnknowns...),
				compile.ParsedQuery(ast.MustParseBody(tc.query)),
				compile.Rego(
					rego.Module("filters.rego", module),
					rego.Input(map[string]any{"user": "alice"}),
				),
			}
			if tc.mappings != nil {
				opts = append(opts, compile.Mappings(tc.mappings))
			}
			prep, err := compile.New(opts...).Prepare(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			filters, err := prep.Compile(t.Context())
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("expected error containing %q, got %v", tc.err, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if act := filters.For("lucene", "solr").Query; act != tc.exp {
				t.Errorf("expected %v, got %v", tc.exp, act)
			}
		})
	}
}

func TestCompileFiltersLuceneOneDocument(t *testing.T) {
	run := func(t *testing.T, module string, unknowns []string, mappings map[string]any) (*compile.Filters, error) {
		terms := make([]*ast.Term, len(unknowns))
		for i := range unknowns {
			terms[i] = ast.MustParseTerm(unknowns[i])
		}
		opts := []compile.CompileOption{
			compile.Target("lucene", "solr"),
			compile.ParsedUnknowns(terms...),
			compile.ParsedQuery(ast.MustParseBody("data.filters.include")),
			compile.Rego(rego.Module("filters.rego", "package filters\n"+module), rego.Input(map[string]any{"user": "alice"})),
		}
		if mappings != nil {
			opts = append(opts, compile.Mappings(mappings))
		}
		prep, err := compile.New(opts...).Prepare(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		return prep.Compile(t.Context())
	}

	const twoDocs = `include if {
	input.docs.owner == input.user
	input.users.role == "admin"
}
`

	t.Run("two unknowns", func(t *testing.T) {
		_, err := run(t, twoDocs, []string{"input.docs", "input.users"}, nil)
		if err == nil || !strings.Contains(err.Error(), "the lucene target filters one document, got unknowns: input.docs, input.users") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("document imported", func(t *testing.T) {
		filters, err := run(t, "import input.doc\ninclude if {\n\tdoc.owner == input.user\n\t\"a\" in doc.tags\n}\n",
			[]string{"input.doc"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if exp, act := `(owner:"alice" AND tags:"a")`, filters.One().Query; exp != act {
			t.Errorf("expected %q, got %q", exp, act)
		}
	})

	t.Run("nested fields", func(t *testing.T) {
		for _, tc := range []struct{ note, rego, exp, err string }{
			{note: "equality", rego: `input.doc.address.city == "Berlin"`, exp: `address.city:"Berlin"`},
			{note: "deeper", rego: `input.doc.a.b.c >= 3`, exp: `a.b.c:[3 TO *]`},
			{note: "not", rego: `not input.doc.address.city == "Berlin"`, exp: `(*:* AND NOT address.city:"Berlin")`},
			{note: "membership", rego: `"x" in input.doc.address.tags`, exp: `address.tags:"x"`},
			{note: "iteration", rego: `some t in input.doc.address.tags; t in {"x", "y"}`, exp: `address.tags:("x" OR "y")`},
			{note: "every", rego: `every t in input.doc.address.tags { t != "x" }`, exp: `(*:* AND NOT address.tags:"x")`},
			{note: "with a flat field", rego: "input.doc.address.city == \"Berlin\"\ninput.doc.owner == input.user", exp: `(address.city:"Berlin" AND owner:"alice")`},
			{note: "numeric index", rego: `input.doc.address.lines[0] == "x"`, err: "invalid ref operand"},
			{note: "variable index", rego: "some i\ninput.doc.address.lines[i] == \"x\"\ninput.doc.address.more[i] == \"y\"", err: "invalid ref operand"},
		} {
			t.Run(tc.note, func(t *testing.T) {
				filters, err := run(t, "include if {\n"+tc.rego+"\n}\n", []string{"input.doc"}, nil)
				if tc.err != "" {
					if err == nil || !strings.Contains(err.Error(), tc.err) {
						t.Fatalf("expected error containing %q, got %v", tc.err, err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if act := filters.One().Query; act != tc.exp {
					t.Errorf("expected %q, got %q", tc.exp, act)
				}
			})
		}
	})

	t.Run("nested field renamed by a mapping", func(t *testing.T) {
		filters, err := run(t, "include if input.doc.address.city == \"Berlin\"\n", []string{"input.doc"},
			map[string]any{"doc": map[string]any{"address.city": "town"}})
		if err != nil {
			t.Fatal(err)
		}
		if exp, act := `town:"Berlin"`, filters.One().Query; exp != act {
			t.Errorf("expected %q, got %q", exp, act)
		}
	})

	t.Run("mapping to a nested name", func(t *testing.T) {
		filters, err := run(t, "include if input.doc.city == \"Berlin\"\n", []string{"input.doc"},
			map[string]any{"doc": map[string]any{"city": "address.city"}})
		if err != nil {
			t.Fatal(err)
		}
		if exp, act := `address.city:"Berlin"`, filters.One().Query; exp != act {
			t.Errorf("expected %q, got %q", exp, act)
		}
	})

	t.Run("short unknown declared as a field of the document", func(t *testing.T) {
		filters, err := run(t, "include if {\n\tinput.doc.owner == input.user\n\t\"a\" in input.tags\n}\n",
			[]string{"input.doc", "input.tags"},
			map[string]any{"lucene": map[string]any{"tags": map[string]any{"$table": "doc"}}})
		if err != nil {
			t.Fatal(err)
		}
		if exp, act := `(owner:"alice" AND tags:"a")`, filters.One().Query; exp != act {
			t.Errorf("expected %q, got %q", exp, act)
		}
	})

	t.Run("short unknown declared as a field of another document", func(t *testing.T) {
		_, err := run(t, "include if {\n\tinput.doc.owner == input.user\n\t\"a\" in input.tags\n}\n",
			[]string{"input.doc", "input.tags"},
			map[string]any{"lucene": map[string]any{"tags": map[string]any{"$table": "other"}}})
		if err == nil || !strings.Contains(err.Error(), "filters one document, got fields of: doc, other") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("short unknown not declared", func(t *testing.T) {
		_, err := run(t, "include if \"a\" in input.tags\n", []string{"input.tags"}, nil)
		if err == nil || !strings.Contains(err.Error(), "invalid ref operand") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestCompileFiltersNestedRefOtherTargets(t *testing.T) {
	prep, err := compile.New(
		compile.Target("sql", "postgresql"),
		compile.ParsedUnknowns(ast.MustParseTerm("input.fruits")),
		compile.ParsedQuery(ast.MustParseBody("data.filters.include")),
		compile.Rego(rego.Module("filters.rego", "package filters\ninclude if input.fruits.origin.country == \"ES\"\n")),
	).Prepare(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prep.Compile(t.Context()); err == nil || !strings.Contains(err.Error(), "invalid ref operand") {
		t.Fatalf("expected the SQL target to reject a nested reference, got %v", err)
	}
}

func TestCompileFiltersLuceneUnsupportedVariant(t *testing.T) {
	_, err := compile.New(
		compile.Target("lucene", "nonesuch"),
		compile.ParsedQuery(ast.MustParseBody("data.filters.include")),
	).Prepare(t.Context())
	if err == nil || !strings.Contains(err.Error(), "unsupported variant for lucene: nonesuch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// With future.keywords.not, `not` over an unknown becomes a negated body or a
// support rule, which no target translates (yet). It must be an error, never a
// filter with the negation dropped.
func TestCompileFiltersLuceneFutureNot(t *testing.T) {
	tests := []struct {
		note string
		body string
		exp  string // expected query, if any
		err  string
	}{
		{note: "not eq", body: `not input.doc.price == 10`, err: "invalid statement"},
		{note: "not lt", body: `not input.doc.price < 12`, err: "invalid statement"},
		{note: "not in", body: `not input.doc.name in {"a", "b"}`, err: "invalid statement"},
		{note: "not with known call", body: `not input.doc.name == lower(input.user)`, err: "invalid statement"},
		{note: "explicit not body", body: "not {\n input.doc.a == 1\n input.doc.b == 2\n}", err: "invalid statement"},
		{note: "not ne", body: `not input.doc.price != 11`, err: "invalid statement"},
		{note: "ne is unaffected", body: `input.doc.price != 11`, exp: `(price:[* TO *] AND NOT price:11)`},
		{note: "not on known values is unaffected", body: "not input.user == \"bob\"\ninput.doc.price == 1", exp: `price:1`},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			for _, target := range [][2]string{{"lucene", "solr"}, {"sql", "postgresql"}} {
				if tc.err == "" && target[0] != "lucene" {
					continue // flat field refs are Lucene's
				}
				module := "package filters\nimport future.keywords.not\ninclude if {\n" + tc.body + "\n}\n"
				prep, err := compile.New(
					compile.Target(target[0], target[1]),
					compile.ParsedUnknowns(luceneUnknowns...),
					compile.ParsedQuery(ast.MustParseBody("data.filters.include")),
					compile.Rego(
						rego.Module("filters.rego", module),
						rego.Input(map[string]any{"user": "alice"}),
					),
				).Prepare(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				filters, err := prep.Compile(t.Context())
				if tc.err != "" {
					if err == nil || !strings.Contains(err.Error(), tc.err) {
						t.Fatalf("%s: expected error containing %q, got %v", target[0], tc.err, err)
					}
					continue
				}
				if err != nil {
					t.Fatalf("%s: %v", target[0], err)
				}
				if target[0] == "lucene" && filters.One().Query != tc.exp {
					t.Errorf("expected %v, got %v", tc.exp, filters.One().Query)
				}
			}
		})
	}
}

func TestCompileFiltersLuceneMultiValued(t *testing.T) {
	tests := []struct {
		note   string
		body   string
		groups []string
		exp    string
		err    string
	}{
		{note: "some, eq", body: `some p in input.doc.tags; p == "a"`, exp: `tags:"a"`},
		{note: "wildcard, eq", body: `input.doc.tags[_] == "a"`, exp: `tags:"a"`},
		{note: "wildcard, eq flipped", body: `"a" == input.doc.tags[_]`, exp: `tags:"a"`},
		{note: "some, in", body: `some p in input.doc.tags; p in {"a", "b"}`, exp: `tags:("a" OR "b")`},
		{note: "known in field", body: `"a" in input.doc.tags`, exp: `tags:"a"`},
		{note: "input value in field", body: `input.user in input.doc.tags`, exp: `tags:"alice"`},
		{note: "not known in field", body: `not "a" in input.doc.tags`, exp: `(*:* AND NOT tags:"a")`},
		{note: "independent wildcards", body: "input.doc.tags[_] == \"a\"\ninput.doc.roles[_] == \"b\"", exp: `(tags:"a" AND roles:"b")`},
		{note: "mixed with scalar field", body: "\"a\" in input.doc.tags\ninput.doc.level >= 3", exp: `(tags:"a" AND level:[3 TO *])`},
		{note: "count is zero", body: `count(input.doc.tags) == 0`, exp: `(*:* AND NOT tags:[* TO *])`},
		{note: "empty", body: `input.doc.tags == []`, exp: `(*:* AND NOT tags:[* TO *])`},
		{note: "not empty", body: `input.doc.tags != []`, exp: `tags:[* TO *]`},
		{note: "element range", body: `some l in input.doc.levels; l >= 3`, exp: `levels:[3 TO *]`},
		{note: "element range, known on the right", body: `some l in input.doc.levels; 3 > l`, exp: `levels:{* TO 3}`},
		{note: "element startswith", body: `some t in input.doc.tags; startswith(t, "a")`, exp: `tags:a*`},
		{note: "every, none starts with", body: `every t in input.doc.tags { not startswith(t, "a") }`, exp: `(*:* AND NOT tags:a*)`},
		{note: "every, none is", body: `every p in input.doc.tags { p != "a" }`, exp: `(*:* AND NOT tags:"a")`},
		{note: "every, not eq", body: `every p in input.doc.tags { not p == "a" }`, exp: `(*:* AND NOT tags:"a")`},
		{note: "every, none in", body: `every p in input.doc.tags { not p in {"a", "b"} }`, exp: `(*:* AND NOT tags:("a" OR "b"))`},
		{note: "every, none below", body: `every l in input.doc.levels { not l < 3 }`, exp: `(*:* AND NOT levels:{* TO 3})`},
		{
			note: "every over known, not in field", body: `every g in input.groups { not g in input.doc.tags }`,
			groups: []string{"g1", "g2"}, exp: `(*:* AND NOT tags:"g1" AND NOT tags:"g2")`,
		},
		{
			note: "every over known, with a positive clause", body: "input.doc.level == 1\nevery g in input.groups { not g in input.doc.tags }",
			groups: []string{"g1", "g2"}, exp: `(level:1 AND NOT tags:"g1" AND NOT tags:"g2")`,
		},
		{
			note: "every over known, comparison with field", body: `every g in input.groups { g != input.doc.owner }`,
			groups: []string{"g1", "g2"}, exp: `((owner:[* TO *] AND NOT owner:"g1") AND (owner:[* TO *] AND NOT owner:"g2"))`,
		},
		{
			note: "every over known, one element", body: `every g in input.groups { not g in input.doc.tags }`,
			groups: []string{"g1"}, exp: `(*:* AND NOT tags:"g1")`,
		},
		{note: "every over empty domain holds", body: `every g in input.groups { not g in input.doc.tags }`, groups: []string{}, exp: "*:*"},

		{note: "null element", body: `some p in input.doc.tags; p == null`, err: "invalid ref operand"},
		{note: "null in field", body: `null in input.doc.tags`, err: "must be known"},
		{note: "every, none is null", body: `every p in input.doc.tags { p != null }`, err: "can only compare the element with known values"},
		{note: "every over a known object, values only", body: `every _, g in {"x": "a"} { not g in input.doc.tags }`, exp: `(*:* AND NOT tags:"a")`},
		{note: "every known, a ground condition", body: "every g in input.groups {\n g != \"z\"\n not g in input.doc.tags\n}", groups: []string{"a", "b"}, exp: `(*:* AND NOT tags:"a" AND NOT tags:"b")`},
		{note: "every known, a ground condition that fails", body: "every g in input.groups {\n g != \"z\"\n not g in input.doc.tags\n}", groups: []string{"z"}, exp: ""},
		{note: "every over a known set", body: `every g in {"a", "b"} { not g in input.doc.tags }`, exp: `(*:* AND NOT tags:"a" AND NOT tags:"b")`},
		{note: "some, neq", body: `some p in input.doc.tags; p != "a"`, err: "cannot be expressed"},
		{note: "every, all values are", body: `every p in input.doc.tags { p == "a" }`, exp: `(*:* AND NOT tags:({* TO "a"} OR {"a" TO *}))`},
		{note: "every, all values in", body: `every p in input.doc.tags { p in {"b", "a"} }`, exp: `(*:* AND NOT tags:({* TO "a"} OR {"a" TO "b"} OR {"b" TO *}))`},
		{note: "every, all values in, numbers", body: `every l in input.doc.levels { l in [3, 1] }`, exp: `(*:* AND NOT levels:({* TO 1} OR {1 TO 3} OR {3 TO *}))`},
		{note: "same values", body: `{t | some t in input.doc.tags} == {"b", "a"}`, exp: `(tags:"a" AND tags:"b" AND NOT tags:({* TO "a"} OR {"a" TO "b"} OR {"b" TO *}))`},
		{note: "same values, none", body: `{t | some t in input.doc.tags} == set()`, exp: `(*:* AND NOT tags:[* TO *])`},
		{note: "same values, known on the left", body: `{"a"} == {t | some t in input.doc.tags}`, exp: `(tags:"a" AND NOT tags:({* TO "a"} OR {"a" TO *}))`},
		{note: "same values, mixed types", body: `{t | some t in input.doc.tags} == {"a", 1}`, err: "all strings, or all numbers"},
		{note: "every, not a single comparison", body: "every p in input.doc.tags {\np != \"a\"\np != \"b\"\n}", err: "can only compare the element with known values"},
		{note: "count is not zero", body: `count(input.doc.tags) == 2`, err: "cannot count"},
		{note: "element ranges, one element", body: `some l in input.doc.levels; l > 1; l < 3`, err: "invalid ref operand"},
		{note: "one field, both ways", body: "\"a\" in input.doc.tags\ninput.doc.tags != \"b\"", err: "input.doc.tags is used both as an array and as a single value"},
		{note: "null", body: `input.doc.owner == null`, err: "a field without a value is missing, not null"},
		{note: "array literal", body: `input.doc.tags == ["a", "b"]`, err: "input.doc.tags is an array: compare its elements"},
		{note: "set literal", body: `input.doc.tags == {"a", "b"}`, err: "input.doc.tags is an array: compare its elements"},
		{note: "correlated elements", body: "some i\ninput.doc.tags[i] == \"a\"\ninput.doc.roles[i] == \"b\"", err: "invalid ref operand"},
		{note: "index into field", body: `input.doc.tags[0] == "a"`, err: "invalid ref operand"},
		{note: "element compared to another field", body: `input.doc.tags[_] == input.doc.owner`, err: "pe_fragment_error"},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			module := "package filters\ninclude if {\n" + tc.body + "\n}\n"
			groups := tc.groups
			if groups == nil {
				groups = []string{}
			}
			prep, err := compile.New(
				compile.Target("lucene", "solr"),
				compile.ParsedUnknowns(luceneUnknowns...),
				compile.ParsedQuery(ast.MustParseBody("data.filters.include")),
				compile.Rego(
					rego.Module("filters.rego", module),
					rego.Input(map[string]any{"user": "alice", "groups": groups}),
				),
			).Prepare(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			filters, err := prep.Compile(t.Context())
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("expected error containing %q, got %v", tc.err, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var exp any = tc.exp
			if tc.exp == "" { // no body can hold
				exp = nil
			}
			if act := filters.For("lucene", "solr").Query; act != exp {
				t.Errorf("expected %v, got %v", exp, act)
			}
		})
	}
}

func TestCompileFiltersLuceneTranslationError(t *testing.T) {
	prep, err := compile.New(
		compile.Target("lucene", "solr"),
		compile.ParsedUnknowns(ast.MustParseTerm("input.doc")),
		compile.ParsedQuery(ast.MustParseBody("data.filters.include")),
		compile.Rego(rego.Module("filters.rego", "package filters\ninclude if input.doc.price > true\n")),
	).Prepare(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	_, err = prep.Compile(t.Context())
	errs, ok := errors.AsType[ast.Errors](err)
	if !ok {
		t.Fatalf("expected ast.Errors, got %T: %v", err, err)
	}
	if len(errs) != 1 || errs[0].Code != "pe_fragment_error" || errs[0].Message != "range bound must be a number or string" {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestCompileLuceneNotCombined(t *testing.T) {
	_, err := compile.New(
		compile.Target("lucene", "solr"),
		compile.Target("sql", "postgresql"),
		compile.ParsedUnknowns(luceneUnknowns...),
		compile.ParsedQuery(ast.MustParseBody("data.filters.include")),
		compile.Rego(rego.Module("filters.rego", "package filters\ninclude if input.doc.a == 1\n")),
	).Prepare(t.Context())
	if err == nil || !strings.Contains(err.Error(), "can not be combined") {
		t.Fatalf("expected error about combining targets, got %v", err)
	}
}

func TestCompileFiltersLuceneSchema(t *testing.T) {
	schema := map[string]any{"type": "object", "properties": map[string]any{
		"owner":   map[string]any{"type": "string"},
		"tags":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"address": map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}},
	}}
	for _, tc := range []struct{ note, body, exp, err string }{
		{note: "single value", body: `input.doc.owner == input.user`, exp: `owner:"alice"`},
		{note: "array", body: `"a" in input.doc.tags`, exp: `tags:"a"`},
		{note: "nested single value", body: `input.doc.address.city == "Berlin"`, exp: `address.city:"Berlin"`},
		{note: "undeclared, used as an array", body: `some l in input.doc.levels; l > 1`, exp: `levels:{1 TO *}`},
		{note: "single value as an array", body: `input.user in input.doc.owner`, err: "input.doc.owner is a single value, not an array"},
		{note: "single value iterated", body: `every o in input.doc.owner { o != "x" }`, err: "input.doc.owner is a single value, not an array"},
		{note: "array as a single value", body: `input.doc.tags == "a"`, err: "input.doc.tags is an array: compare its elements"},
		{note: "array, bare", body: `input.doc.tags`, err: "input.doc.tags is an array, which is always true"},
	} {
		t.Run(tc.note, func(t *testing.T) {
			prep, err := compile.New(
				compile.Target("lucene", "solr"),
				compile.ParsedUnknowns(luceneUnknowns...),
				compile.ParsedQuery(ast.MustParseBody("data.filters.include")),
				compile.Schema(schema),
				compile.Rego(
					rego.Module("filters.rego", "package filters\ninclude if {\n"+tc.body+"\n}\n"),
					rego.Input(map[string]any{"user": "alice"}),
				),
			).Prepare(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			filters, err := prep.Compile(t.Context())
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("expected error containing %q, got %v", tc.err, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if act := filters.One().Query; act != tc.exp {
				t.Errorf("expected %v, got %v", tc.exp, act)
			}
		})
	}
}
