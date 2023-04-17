// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
)

func TestTimeSeeding(t *testing.T) {

	query := `time.now_ns(x)`
	clock := time.Now()
	q := NewQuery(ast.MustParseBody(query)).WithTime(clock).WithCompiler(ast.NewCompiler())

	ctx := context.Background()

	qrs, err := q.Run(ctx)
	if err != nil {
		t.Fatal(err)
	} else if len(qrs) != 1 {
		t.Fatal("expected exactly one result but got:", qrs)
	}

	exp := ast.MustParseTerm(fmt.Sprintf(`
		{
			{
				x: %v
			}
		}
	`, clock.UnixNano()))

	result := queryResultSetToTerm(qrs)

	if !result.Equal(exp) {
		t.Fatalf("expected %v but got %v", exp, result)
	}

}
