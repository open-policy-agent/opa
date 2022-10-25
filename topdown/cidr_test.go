package topdown

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	inmem "github.com/open-policy-agent/opa/storage/inmem/test"
)

func TestNetCIDRExpandCancellation(t *testing.T) {

	ctx := context.Background()

	compiler := compileModules([]string{
		`
		package test

		p { net.cidr_expand("1.0.0.0/1") }  # generating 2**31 hosts will take a while...
		`,
	})

	store := inmem.New()
	txn := storage.NewTransactionOrDie(ctx, store)
	cancel := NewCancel()

	query := NewQuery(ast.MustParseBody("data.test.p")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithCancel(cancel)

	go func() {
		time.Sleep(time.Millisecond * 50)
		cancel.Cancel()
	}()

	qrs, err := query.Run(ctx)

	if err == nil || err.(*Error).Code != CancelErr {
		t.Fatalf("Expected cancel error but got: %v (err: %v)", qrs, err)
	}

}

func TestNetCIDRParse(t *testing.T) {
	ctx := context.Background()
	store := inmem.New()
	txn := storage.NewTransactionOrDie(ctx, store)

	for _, tt := range []struct {
		name     string
		cidr     string
		expected bool
	}{
		{
			name:     "valid ipv4 cidr",
			cidr:     `192.168.1.0/24`,
			expected: true,
		},
		{
			name:     "empty cidr",
			cidr:     "",
			expected: false,
		},
		{
			name:     "string",
			cidr:     "there goes a string",
			expected: false,
		},
		{
			name:     "valid ipv4 address",
			cidr:     `192.168.1.2`,
			expected: false,
		},
		{
			name:     "valid ipv6 cidr",
			cidr:     `2002::1234:abcd:ffff:c0a8:101/64`,
			expected: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			module := fmt.Sprintf(
				`
				package test
				default valid = false
				valid { net.cidr_parse(%q) }
				`,
				tt.cidr,
			)

			compiler := compileModules([]string{module})

			query := NewQuery(ast.MustParseBody("data.test.valid = x")).
				WithCompiler(compiler).
				WithStore(store).
				WithTransaction(txn)

			qrs, err := query.Run(ctx)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
				return
			}

			if len(qrs) != 1 {
				t.Fatalf("unexpected query result set size: %+v", qrs)
				return
			}

			val := qrs[0][ast.Var("x")].String()
			if fmt.Sprintf("%v", tt.expected) != val {
				t.Errorf("expected %v, received %v", tt.expected, val)
			}
		})
	}
}
