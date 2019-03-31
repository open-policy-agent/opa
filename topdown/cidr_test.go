package topdown

import "testing"

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
		{"cidr subnet overlap malformed cidr a", []string{`p[x] { net.cidr_intersects("not-a-cidr", "192.168.1.0/24", x) }`}, new(Error)},
		{"cidr subnet overlap malformed cidr b", []string{`p[x] { net.cidr_intersects("192.168.1.0/28", "not-a-cidr", x) }`}, new(Error)},
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
		{"cidr subnet overlap malformed cidr a", []string{`p[x] { net.cidr_contains("not-a-cidr", "192.168.1.67", x) }`}, new(Error)},
		{"cidr subnet overlap malformed cider b", []string{`p[x] { net.cidr_contains("192.168.1.0/28", "not-a-cidr", x) }`}, new(Error)},
		{"cidr contains ip", []string{`p[x] { net.cidr_contains("10.0.0.0/8", "10.1.2.3", x) }`}, "[true]"},
		{"cidr does not contain ip", []string{`p[x] { net.cidr_contains("10.0.0.0/8", "192.168.1.1", x) }`}, "[false]"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}
