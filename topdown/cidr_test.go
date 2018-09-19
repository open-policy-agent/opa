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
