package compile_test

import (
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
