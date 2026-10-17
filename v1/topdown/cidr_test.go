package topdown

import (
	"strconv"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
)

func TestNetCIDRExpandCancellation(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	compiler := compileModules([]string{
		`
		package test

		p if { net.cidr_expand("1.0.0.0/1") }  # generating 2**31 hosts will take a while...
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

// The last address of an IPv6 network like ::/80 is an IPv4-mapped address (::ffff:ffff:ffff).
// (*net.IPNet).Contains turns such addresses into 4 bytes that never match an IPv6 network.
func TestNetCIDRContainsIPv6NetworkReachingIPv4MappedBlock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note     string
		query    string
		expected bool
	}{
		// a network contains itself, also when its range reaches into ::ffff:0:0/96
		{"network contains itself /80", `net.cidr_contains("::/80", "::/80")`, true},
		{"network contains itself /81", `net.cidr_contains("::8000:0:0/81", "::8000:0:0/81")`, true},
		{"network contains itself /95", `net.cidr_contains("::/95", "::/95")`, true},
		{"network contains itself /96", `net.cidr_contains("::/96", "::/96")`, true},
		{"network contains itself /128", `net.cidr_contains("2001:db8::1/128", "2001:db8::1/128")`, true},
		// a network contains its subnets
		{"all of IPv6 contains /80", `net.cidr_contains("::/0", "::/80")`, true},
		{"/79 contains /80", `net.cidr_contains("::/79", "::/80")`, true},
		{"/80 contains /96", `net.cidr_contains("::/80", "::/96")`, true},
		{"/32 contains /48", `net.cidr_contains("2001:db8::/32", "2001:db8:1::/48")`, true},
		// ... and not other networks
		{"/80 does not contain /79", `net.cidr_contains("::/80", "::/79")`, false},
		{"/80 does not contain another /80", `net.cidr_contains("0:0:0:1::/80", "::/80")`, false},
		{"/96 does not contain /80", `net.cidr_contains("::/96", "::/80")`, false},
		{"/32 does not contain a network in another /32", `net.cidr_contains("2001:db8::/32", "2001:db9::/48")`, false},
		{"/48 does not contain its /32", `net.cidr_contains("2001:db8:1::/48", "2001:db8::/32")`, false},
		// IPv4 and IPv6 are still different families
		{"all of IPv6 does not contain an IPv4 network", `net.cidr_contains("::/0", "1.2.3.0/24")`, false},
		{"all of IPv6 does not contain an IPv4 address", `net.cidr_contains("::/0", "1.2.3.4")`, false},
		{"all of IPv4 does not contain an IPv6 network", `net.cidr_contains("0.0.0.0/0", "::/80")`, false},
		// IPv4 networks and IPv4-mapped IPv6 networks keep behaving as before
		{"IPv4 contains IPv4", `net.cidr_contains("1.2.3.0/24", "1.2.3.128/25")`, true},
		{"IPv4 contains mapped IPv4 address", `net.cidr_contains("1.2.3.0/24", "::ffff:1.2.3.4")`, true},
		{"mapped IPv6 network contains IPv4 address", `net.cidr_contains("::ffff:1.2.3.0/120", "1.2.3.4")`, true},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()
			runTopDownTestCase(t, map[string]any{}, tc.note, []string{`p := x if { x := ` + tc.query + ` }`}, strconv.FormatBool(tc.expected))
		})
	}
}
