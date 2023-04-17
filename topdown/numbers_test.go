package topdown

import (
	"context"
	"math/rand"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestRandIntnZero(t *testing.T) {

	qrs, err := NewQuery(ast.MustParseBody(`rand.intn("x", 0, out)`)).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	} else if len(qrs) != 1 {
		t.Fatal("expected one result")
	}

	exp := ast.MustParseTerm(`{{out: 0}}`)

	result := queryResultSetToTerm(qrs)

	if !result.Equal(exp) {
		t.Fatalf("expected %v but got %v", exp, result)
	}
}

func TestRandIntnNegative(t *testing.T) {

	qrs, err := NewQuery(ast.MustParseBody(`rand.intn("x", -100, out)`)).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	} else if len(qrs) != 1 {
		t.Fatal("expected one result")
	}

	x, ok := qrs[0][ast.Var("out")].Value.(ast.Number).Int()
	if !ok {
		t.Fatal("expected int")
	}

	if x < 0 || x >= 100 {
		t.Fatal("expected x to be [0, 100)")
	}
}

func TestRandIntnSeedingAndCaching(t *testing.T) {

	query := `rand.intn("x", 100000, x); rand.intn("x", 1000, y); rand.intn("x", 100000, x2); rand.intn("y", 1000, z)`

	q := NewQuery(ast.MustParseBody(query)).WithSeed(rand.New(rand.NewSource(0))).WithCompiler(ast.NewCompiler())

	ctx := context.Background()

	qrs, err := q.Run(ctx)
	if err != nil {
		t.Fatal(err)
	} else if len(qrs) != 1 {
		t.Fatal("expected exactly one result but got:", qrs)
	}

	exp := ast.MustParseTerm(`
		{
			{
				x: 88007,
				x2: 88007,
				y: 796,
				z: 101
			}
		}
	`)

	result := queryResultSetToTerm(qrs)

	if !result.Equal(exp) {
		t.Fatalf("expected %v but got %v", exp, result)
	}

}

func TestRandIntnSavingDuringPartialEval(t *testing.T) {

	query := `x = "x"; y = 100; rand.intn(x, y, z)`
	c := ast.NewCompiler().
		WithCapabilities(&ast.Capabilities{Builtins: []*ast.Builtin{ast.RandIntn}})
	c.Compile(nil)

	q := NewQuery(ast.MustParseBody(query)).WithSeed(rand.New(rand.NewSource(0))).WithCompiler(c)

	queries, modules, err := q.PartialRun(context.Background())
	if err != nil {
		t.Fatal(err)
	} else if len(modules) > 0 {
		t.Fatal("expected no support")
	}

	exp := ast.MustParseBody(`rand.intn("x", 100, z); x = "x"; y = 100`)

	if len(queries) != 1 || !queries[0].Equal(exp) {
		t.Fatalf("expected %v but got: %v", exp, queries)
	}
}
