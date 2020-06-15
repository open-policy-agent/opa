package topdown

import (
	"testing"
)

func TestTopDownAggregates(t *testing.T) {

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"count", []string{`p[x] { count(a, x) }`}, "[4]"},
		{"count virtual", []string{`p[x] { count([y | q[y]], x) }`, `q[x] { x = a[_] }`}, "[4]"},
		{"count keys", []string{`p[x] { count(b, x) }`}, "[2]"},
		{"count keys virtual", []string{`p[x] { count([k | q[k] = _], x) }`, `q[k] = v { b[k] = v }`}, "[2]"},
		{"count set", []string{`p = x { count(q, x) }`, `q[x] { x = a[_] }`}, "4"},
		{"sum", []string{`p[x] { sum([1, 2, 3, 4], x) }`}, "[10]"},
		{"sum set", []string{`p = x { sum({1, 2, 3, 4}, x) }`}, "10"},
		{"sum virtual", []string{`p[x] { sum([y | q[y]], x) }`, `q[x] { a[_] = x }`}, "[10]"},
		{"sum virtual set", []string{`p = x { sum(q, x) }`, `q[x] { a[_] = x }`}, "10"},
		{"bug 2469 - precision", []string{"p = true { sum([49649733057, 1]) == 49649733058 }"}, "true"},
		{"product", []string{"p { product([1,2,3,4], 24) }"}, "true"},
		{"product set", []string{`p = x { product({1, 2, 3, 4}, x) }`}, "24"},
		{"max", []string{`p[x] { max([1, 2, 3, 4], x) }`}, "[4]"},
		{"max set", []string{`p = x { max({1, 2, 3, 4}, x) }`}, "4"},
		{"max virtual", []string{`p[x] { max([y | q[y]], x) }`, `q[x] { a[_] = x }`}, "[4]"},
		{"max virtual set", []string{`p = x { max(q, x) }`, `q[x] { a[_] = x }`}, "4"},
		{"min", []string{`p[x] { min([1, 2, 3, 4], x) }`}, "[1]"},
		{"min dups", []string{`p[x] { min([1, 2, 1, 3, 4], x) }`}, "[1]"},
		{"min out-of-order", []string{`p[x] { min([3, 2, 1, 4, 6, -7, 10], x) }`}, "[-7]"},
		{"min set", []string{`p = x { min({1, 2, 3, 4}, x) }`}, "1"},
		{"min virtual", []string{`p[x] { min([y | q[y]], x) }`, `q[x] { a[_] = x }`}, "[1]"},
		{"min virtual set", []string{`p = x { min(q, x) }`, `q[x] { a[_] = x }`}, "1"},
		{"reduce ref dest", []string{`p = true { max([1, 2, 3, 4], a[3]) }`}, "true"},
		{"reduce ref dest (2)", []string{`p = true { not max([1, 2, 3, 4, 5], a[3]) }`}, "true"},
		{"sort", []string{`p = x { sort([4, 3, 2, 1], x) }`}, "[1 ,2, 3, 4]"},
		{"sort set", []string{`p = x { sort({4,3,2,1}, x) }`}, "[1,2,3,4]"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestAll(t *testing.T) {

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"empty set", []string{`p = x { x := all(set()) }`}, "true"},
		{"empty array", []string{`p = x { x := all([]) }`}, "true"},
		{"set success", []string{`p = x { x := all({true, true, true}) }`}, "true"},
		{"array success", []string{`p = x { x := all( [true, true, true] ) }`}, "true"},
		{"set fail", []string{`p = x { x := all( {true, false, true} ) }`}, "false"},
		{"array fail", []string{`p = x { x := all( [false, true, true] ) }`}, "false"},
		{"other types", []string{`p = x { x := all( [{}, "", true, true, 123] ) }`}, "false"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestAny(t *testing.T) {

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"empty set", []string{`p = x { x := any(set()) }`}, "false"},
		{"empty array", []string{`p = x { x := any([]) }`}, "false"},
		{"set success", []string{`p = x { x := any({false, false, true}) }`}, "true"},
		{"array success", []string{`p = x { x := any( [true, true, true, false, false] ) }`}, "true"},
		{"set fail", []string{`p = x { x := any( {false, false, false} ) }`}, "false"},
		{"array fail", []string{`p = x { x := any( [false] ) }`}, "false"},
		{"other types", []string{`p = x { x := any( [true, {}, "false"] ) }`}, "true"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}
