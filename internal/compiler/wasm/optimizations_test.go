package wasm

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/planner"
)

func TestRemoveUnusedCode(t *testing.T) {
	policy, err := planner.New().
		WithQueries([]planner.QuerySet{
			{
				Name:    "test",
				Queries: []ast.Body{ast.MustParseBody(`input.foo = 1`)},
			},
		}).Plan()

	if err != nil {
		t.Fatal(err)
	}

	c := New().WithPolicy(policy)
	mod, err := c.Compile()
	if err != nil {
		t.Fatal(err)
	}

	// NOTE(sr): our unused code elimination has the following invariant:
	// if a function is not used, both its code and its name are removed
	// from the code section, and name sections respectively.
	idxToName := map[uint32]string{}
	for _, nm := range mod.Names.Functions {
		idxToName[nm.Index] = nm.Name
	}
	for i, seg := range mod.Code.Segments {
		idx := i + c.functionImportCount()
		noop := len(seg.Code) == 3
		name, ok := idxToName[uint32(idx)]
		if noop && ok {
			t.Errorf("func[%d] has name (%s) and no code", idx, name)
		}
		if !noop && !ok {
			t.Errorf("func[%d] has code but no name", idx)
		}
	}

	// Having established that, we can check that this simple policy
	// has no re2-related code by consulting the name map:
	for _, nm := range mod.Names.Functions {
		if strings.Contains(nm.Name, "re2") {
			t.Errorf("expected no re2-related functions, found %s", nm.Name)
		}
	}
}
