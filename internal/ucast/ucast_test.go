// Copyright 2025 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package ucast

import (
	"testing"
)

// Note: Currently only implements tests for the Postgres dialect.
func TestUCASTNodeAsSQL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Note    string
		Source  UCASTNode
		Dialect string
		Result  string
		Error   string
	}{
		{
			Note:    "Nil argument",
			Source:  UCASTNode{Type: "field", Op: "eq", Field: "name", Value: nil},
			Dialect: "postgres",
			Result:  "",
			Error:   "field expression requires a value",
		},
		{
			Note:    "special handling for NULL",
			Source:  UCASTNode{Type: "field", Op: "eq", Field: "name", Value: Null{}},
			Dialect: "postgres",
			Result:  "WHERE name IS NULL",
		},
		{
			Note:    "special handling for NOT NULL",
			Source:  UCASTNode{Type: "field", Op: "ne", Field: "name", Value: Null{}},
			Dialect: "postgres",
			Result:  "WHERE name IS NOT NULL",
		},
		{
			Note: "Basic compound expression",
			Source: UCASTNode{Type: "compound", Op: "and", Value: []UCASTNode{
				{Type: "field", Op: "eq", Field: "name", Value: "bob"},
				{Type: "field", Op: "gt", Field: "salary", Value: 50000},
			}},
			Dialect: "postgres",
			Result:  "WHERE (name = E'bob' AND salary > 50000)",
		},
		{
			Note:    "startswith + pattern",
			Source:  UCASTNode{Type: "field", Field: "name", Op: "startswith", Value: `f\oo_b%ar`},
			Dialect: "postgres",
			Result:  `WHERE name LIKE E'f\\\\oo\\_b\\%ar%'`,
		},
		{
			Note:    "endswith + pattern",
			Source:  UCASTNode{Type: "field", Field: "name", Op: "endswith", Value: `f\oo_b%ar`},
			Dialect: "postgres",
			Result:  `WHERE name LIKE E'%f\\\\oo\\_b\\%ar'`,
		},
		{
			Note:    "contains + pattern",
			Source:  UCASTNode{Type: "field", Field: "name", Op: "contains", Value: `f\oo_b%ar`},
			Dialect: "postgres",
			Result:  `WHERE name LIKE E'%f\\\\oo\\_b\\%ar%'`,
		},
		{
			Note: "Basic nested compound expression",
			Source: UCASTNode{Type: "compound", Op: "and", Value: []UCASTNode{
				{Type: "field", Op: "eq", Field: "name", Value: "bob"},
				{Type: "field", Op: "gt", Field: "salary", Value: 50000},
				{Type: "compound", Op: "or", Value: []UCASTNode{
					{Type: "field", Op: "eq", Field: "role", Value: "admin"},
					{Type: "field", Op: "ge", Field: "salary", Value: 100000},
				}},
			}},
			Dialect: "postgres",
			Result:  "WHERE (name = E'bob' AND salary > 50000 AND (role = E'admin' OR salary >= 100000))",
		},
		{
			Note:    "'in' expression",
			Source:  UCASTNode{Type: "field", Field: "f", Op: "in", Value: []any{"foo", "bar"}},
			Dialect: "postgres",
			Result:  "WHERE f IN (E'foo', E'bar')",
		},
		{
			Note: "'not' compound expression",
			Source: UCASTNode{Type: "compound", Op: "not", Value: []UCASTNode{
				{Type: "field", Op: "eq", Field: "name", Value: "bob"},
			}},
			Dialect: "postgres",
			Result:  "WHERE NOT name = E'bob'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.Note, func(t *testing.T) {
			t.Parallel()

			actual, err := tc.Source.AsSQL(tc.Dialect)
			if err != nil && tc.Error != err.Error() {
				t.Fatal(err)
			}

			if actual != tc.Result {
				t.Fatalf("expected SQL string: '%s', got string: '%s'", tc.Result, actual)
			}
		})
	}
}
