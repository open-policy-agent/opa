package topdown

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

// YAML test cases at test/cases/testdata/strings for sprintf() with big integers
// will not pass due to bug in go-yaml when unmarshalling big integers.
// Issue has been raised: https://github.com/go-yaml/yaml/issues/703.
// When this issue has been resolved, the below test can be replaced
// by a YAMl test case in test/cases/testdata/strings.
func TestSprintfBigInt(t *testing.T) {

	query := `sprintf("%s",[123456789123456789123],x); sprintf("%s",[1208925819614629174706175],max_cert_serial_number)`

	q := NewQuery(ast.MustParseBody(query)).WithCompiler(ast.NewCompiler())

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
				x: "123456789123456789123",
				max_cert_serial_number: "1208925819614629174706175",
			}
		}
	`)

	result := queryResultSetToTerm(qrs)

	if !result.Equal(exp) {
		t.Fatalf("expected %v but got %v", exp, result)
	}

}
