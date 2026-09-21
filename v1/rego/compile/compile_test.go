package compile_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/open-policy-agent/opa/v1/ast"
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
