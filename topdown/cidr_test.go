package topdown

import (
	"context"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
)

func TestNetCIDROverlap(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"cidr match", []string{`p[x] { net.cidr_overlap("192.168.1.0/24", "192.168.1.67", x) }`}, "[true]"},
		{"cidr mismatch", []string{`p[x] { net.cidr_overlap("192.168.1.0/28", "192.168.1.67", x) }`}, "[false]"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestNetCIDRIntersects(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"cidr subnet overlaps", []string{`p[x] { net.cidr_intersects("192.168.1.0/25", "192.168.1.64/25", x) }`}, "[true]"},
		{"cidr subnet does not overlap", []string{`p[x] { net.cidr_intersects("192.168.1.0/24", "192.168.2.0/24", x) }`}, "[false]"},
		{"cidr ipv6 subnet overlaps", []string{`p[x] { net.cidr_intersects("fd1e:5bfe:8af3:9ddc::/64", "fd1e:5bfe:8af3:9ddc:1111::/72", x) }`}, "[true]"},
		{"cidr ipv6 subnet does not overlap", []string{`p[x] { net.cidr_intersects("fd1e:5bfe:8af3:9ddc::/64", "2001:4860:4860::8888/32", x) }`}, "[false]"},
		{"cidr subnet overlap malformed cidr a", []string{`p[x] { net.cidr_intersects("not-a-cidr", "192.168.1.0/24", x) }`}, &Error{Code: BuiltinErr}},
		{"cidr subnet overlap malformed cidr b", []string{`p[x] { net.cidr_intersects("192.168.1.0/28", "not-a-cidr", x) }`}, &Error{Code: BuiltinErr}},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestNetCIDRContains(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"cidr contains subnet", []string{`p[x] { net.cidr_contains("10.0.0.0/8", "10.1.0.0/24", x) }`}, "[true]"},
		{"cidr does not contain subnet partial", []string{`p[x] { net.cidr_contains("172.17.0.0/24", "172.17.0.0/16", x) }`}, "[false]"},
		{"cidr does not contain subnet", []string{`p[x] { net.cidr_contains("10.0.0.0/8", "192.168.1.0/24", x) }`}, "[false]"},
		{"cidr contains single ip subnet", []string{`p[x] { net.cidr_contains("10.0.0.0/8", "10.1.1.1/32", x) }`}, "[true]"},
		{"cidr contains subnet ipv6", []string{`p[x] { net.cidr_contains("2001:4860:4860::8888/32", "2001:4860:4860:1234::8888/40", x) }`}, "[true]"},
		{"cidr contains single ip subnet ipv6", []string{`p[x] { net.cidr_contains("2001:4860:4860::8888/32", "2001:4860:4860:1234:5678:1234:5678:8888/128", x) }`}, "[true]"},
		{"cidr does not contain subnet partial ipv6", []string{`p[x] { net.cidr_contains("2001:4860::/96", "2001:4860::/32", x) }`}, "[false]"},
		{"cidr does not contain subnet ipv6", []string{`p[x] { net.cidr_contains("2001:4860::/32", "fd1e:5bfe:8af3:9ddc::/64", x) }`}, "[false]"},
		{"cidr subnet overlap malformed cidr a", []string{`p[x] { net.cidr_contains("not-a-cidr", "192.168.1.67", x) }`}, &Error{Code: BuiltinErr}},
		{"cidr subnet overlap malformed cider b", []string{`p[x] { net.cidr_contains("192.168.1.0/28", "not-a-cidr", x) }`}, &Error{Code: BuiltinErr}},
		{"cidr contains ip", []string{`p[x] { net.cidr_contains("10.0.0.0/8", "10.1.2.3", x) }`}, "[true]"},
		{"cidr does not contain ip", []string{`p[x] { net.cidr_contains("10.0.0.0/8", "192.168.1.1", x) }`}, "[false]"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestNetCIDRContainsMatches(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{
			note:     "strings",
			rules:    []string{`p = x { x := net.cidr_contains_matches("1.1.1.0/24", "1.1.1.1") }`},
			expected: `[["1.1.1.0/24", "1.1.1.1"]]`,
		},
		{
			note:     "arrays",
			rules:    []string{`p = x { x := net.cidr_contains_matches(["1.1.2.0/24", "1.1.1.0/24"], ["1.1.1.1", "1.1.2.1"]) }`},
			expected: `[[0,1], [1,0]]`,
		},
		{
			note:     "arrays of tuples",
			rules:    []string{`p = x { x := net.cidr_contains_matches([["1.1.2.0/24", 1], "1.1.1.0/24"], ["1.1.1.1", "1.1.2.1"]) }`},
			expected: `[[0,1], [1,0]]`,
		},
		{
			note:     "bad array",
			rules:    []string{`p = x { x := net.cidr_contains_matches(["1.1.2.0/24", "1.1.1.0/24"], ["1.1.1.1", data.a[0]]) }`},
			expected: &Error{Code: BuiltinErr, Message: "net.cidr_contains_matches: operand 2: element must be string or non-empty array"},
		},
		{
			note:     "sets of strings",
			rules:    []string{`p = x { x := net.cidr_contains_matches({"1.1.2.0/24", "1.1.1.0/24"}, {"1.1.1.1", "1.1.2.1"}) }`},
			expected: `[["1.1.1.0/24", "1.1.1.1"], ["1.1.2.0/24", "1.1.2.1"]]`,
		},
		{
			note:     "sets of tuples",
			rules:    []string{`p = x { x := net.cidr_contains_matches({["1.1.2.0/24", "foo"], ["1.1.1.0/24", "bar"]}, {["1.1.1.1", "baz"], ["1.1.2.1", "qux"]}) }`},
			expected: `[[["1.1.1.0/24", "bar"], ["1.1.1.1", "baz"]], [["1.1.2.0/24", "foo"], ["1.1.2.1", "qux"]]]`,
		},
		{
			note:     "bad set",
			rules:    []string{`p = x { x := net.cidr_contains_matches({["1.1.2.0/24", "foo"], ["1.1.1.0/24", "bar"]}, {data.a[0], ["1.1.2.1", "qux"]}) }`},
			expected: &Error{Code: BuiltinErr, Message: `net.cidr_contains_matches: operand 2: element must be string or non-empty array`},
		},
		{
			note:     "bad set tuple element",
			rules:    []string{`p = x { x := net.cidr_contains_matches({["1.1.2.0/24", "foo"], ["1.1.1.0/24", "bar"]}, {[], ["1.1.2.1", "qux"]}) }`},
			expected: &Error{Code: BuiltinErr, Message: `net.cidr_contains_matches: operand 2: element must be string or non-empty array`},
		},
		{
			note:     "objects",
			rules:    []string{`p = x { x := net.cidr_contains_matches({"k1": "1.1.1.1/24", "k2": ["1.1.1.2/24", 1]}, "1.1.1.128") }`},
			expected: `[["k1", "1.1.1.128"], ["k2", "1.1.1.128"]]`,
		},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestNetCIDRExpand(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{
			note: "cidr includes host and broadcast",
			rules: []string{
				`p = x { net.cidr_expand("192.168.1.1/30", x) }`,
			},
			expected: `[
				"192.168.1.0",
				"192.168.1.1",
				"192.168.1.2",
				"192.168.1.3"
			]`,
		},
		{
			note: "cidr last octet all 1s",
			rules: []string{
				`p = x { net.cidr_expand("172.16.100.255/30", x) }`,
			},
			expected: `[
				"172.16.100.252",
				"172.16.100.253",
				"172.16.100.254",
				"172.16.100.255"
			]`,
		},
		{
			note: "cidr all bits",
			rules: []string{
				`p = x { net.cidr_expand("192.168.1.1/32", x) }`,
			},
			expected: `[
				"192.168.1.1"
			]`,
		},
		{
			note: "cidr invalid mask",
			rules: []string{
				`p = x { net.cidr_expand("192.168.1.1/33", x) }`,
			},
			expected: &Error{Code: BuiltinErr, Message: "net.cidr_expand: invalid CIDR address: 192.168.1.1/33"},
		},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestNetCIDRExpandCancellation(t *testing.T) {

	ctx := context.Background()

	compiler := compileModules([]string{
		`
		package test

		p { net.cidr_expand("1.0.0.0/1") }  # generating 2**31 hosts will take a while...
		`,
	})

	store := inmem.NewFromObject(map[string]interface{}{})
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
