// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"testing"
)

// TestIntersection tests intersection of the given input sets
func TestIntersection(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"intersection_0_sets", []string{`p = x { intersection(set(), x) }`}, "[]"},
		{"intersection_2_sets", []string{`p = x { intersection({set(), {1, 2}}, x) }`}, "[]"},
		{"intersection_2_sets", []string{`p = x { s1 = {1, 2, 3}; s2 = {2}; intersection({s1, s2}, x) }`}, "[2]"},
		{"intersection_3_sets", []string{`p = x { s1 = {1, 2, 3}; s2 = {2, 3, 4}; s3 = {4, 5, 6}; intersection({s1, s2, s3}, x) }`}, "[]"},
		{"intersection_4_sets", []string{`p = x { s1 = {"a", "b", "c", "d"}; s2 = {"b", "c", "d"}; s3 = {"c", "d"}; s4 = {"d"}; intersection({s1, s2, s3, s4}, x) }`}, "[\"d\"]"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

// TestUnion tests union of the given input sets
func TestUnion(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"union_0_sets", []string{`p = x { union(set(), x) }`}, "[]"},
		{"union_2_sets", []string{`p = x { union({set(), {1, 2}}, x) }`}, "[1, 2]"},
		{"union_2_sets", []string{`p = x { s1 = {1, 2, 3}; s2 = {2}; union({s1, s2}, x) }`}, "[1, 2, 3]"},
		{"union_3_sets", []string{`p = x { s1 = {1, 2, 3}; s2 = {2, 3, 4}; s3 = {4, 5, 6}; union({s1, s2, s3}, x) }`}, "[1, 2, 3, 4, 5, 6]"},
		{"union_4_sets", []string{`p = x { s1 = {"a", "b", "c", "d"}; s2 = {"b", "c", "d"}; s3 = {"c", "d"}; s4 = {"d"}; union({s1, s2, s3, s4}, x) }`}, "[\"a\", \"b\", \"c\", \"d\"]"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}
