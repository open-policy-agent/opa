// Copyright 2025 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-policy-agent/opa/internal/compile"
	"github.com/open-policy-agent/opa/v1/ast"
)

const (
	ucastAcceptHeader = "application/vnd.opa.ucast.prisma+json"
	sqlAcceptHeader   = "application/vnd.opa.sql.postgresql+json"
)

// Error is needed here because the ast.Error type cannot be
// unmarshalled from JSON: it contains an interface value.
type Error struct {
	Location *ast.Location    `json:"location,omitempty"`
	Details  *compile.Details `json:"details,omitempty"`
	Code     string           `json:"code"`
	Message  string           `json:"message"`
}

// NOTE(sr): The important thing about these tests is that we don't mock
// the partially-evaluated Rego. Instead, we store the data filter policy,
// run the PE-post-analysing handler, and have assertions on its response.
// The happy-path assertions all request a UCAST response, with the "all"
// variant, which includes all the builtins. For error cases, we check
// both constrained UCAST, and SQL.
func TestPostPartialChecks(t *testing.T) {
	t.Parallel()
	const defaultPath = "filters/include"
	defaultInput := map[string]any{
		"a":      true,
		"b":      false,
		"colour": "orange",
	}
	defaultUnknowns := []string{"input.fruits", "input.baskets"}

	for _, tc := range []struct {
		input        any
		result       any
		mappings     map[string]any
		note         string
		target       string
		rego         string
		query        string
		skip         string
		errors       []Error
		regoVerbatim bool
		omitUnknowns bool
	}{
		{
			note:   "happy path",
			rego:   `include if input.fruits.colour == input.colour`,
			result: map[string]any{"type": "field", "field": "fruits.colour", "operator": "eq", "value": "orange"},
		},
		{
			note:   "unconditional NO",
			rego:   `include if false`,
			result: nil,
		},
		{
			note:   "unconditional YES",
			rego:   `include if true`,
			result: map[string]any{},
		},
		{
			note:         "happy path, reading unknowns from metadata (document scope)",
			omitUnknowns: true,
			rego: `
# METADATA
# scope: document
# compile:
#  unknowns:
#   - input.fruits
include if input.fruits.colour == input.colour

_use_metadata := rego.metadata.chain()`,
			result: map[string]any{"type": "field", "field": "fruits.colour", "operator": "eq", "value": "orange"},
		},
		{
			note:         "happy path, reading unknowns from metadata (package scope)",
			omitUnknowns: true,
			regoVerbatim: true,
			rego: `
# METADATA
# scope: package
# compile:
#  unknowns:
#   - input.fruits
package filters

include if input.fruits.colour == input.colour

_use_metadata := rego.metadata.chain()`,
			result: map[string]any{"type": "field", "field": "fruits.colour", "operator": "eq", "value": "orange"},
		},
		{
			note:         "happy path, reading unknowns from metadata (package scope, no kludge)",
			omitUnknowns: true,
			regoVerbatim: true,
			rego: `
# METADATA
# scope: package
# compile:
#  unknowns:
#   - input.fruits
package filters

include if input.fruits.colour == input.colour

_use_metadata := rego.metadata.chain()`,
			result: map[string]any{"type": "field", "field": "fruits.colour", "operator": "eq", "value": "orange"},
		},
		{
			note:         "happy path, reading unknowns from metadata chain, correct rule only (document scope)",
			omitUnknowns: true,
			regoVerbatim: true,
			rego: `
# METADATA
# scope: package
# compile:
#  unknowns:
#   - input.fruits
package filters

# METADATA
# scope: document
# compile:
#  unknowns:
#   - input.baskets
include if {
	input.fruits.colour == input.colour
	input.baskets.colour == input.colour
}

# METADATA
# scope: document
# description: if this metadata were picked up, the checks would fail
# compile:
#  unknowns:
#   - input.colour
red_herring if true

_use_metadata := rego.metadata.chain()`,
			result: map[string]any{
				"type":     "compound",
				"operator": "and",
				"value": []any{
					map[string]any{"type": "field", "field": "fruits.colour", "operator": "eq", "value": "orange"},
					map[string]any{"type": "field", "field": "baskets.colour", "operator": "eq", "value": "orange"},
				},
			},
		},
		{
			note:         "happy path, reading unknowns from metadata chain, correct rule only (document scope, without kludge)",
			omitUnknowns: true,
			regoVerbatim: true,
			rego: `
# METADATA
# scope: package
# compile:
#  unknowns:
#   - input.fruits
package filters

# METADATA
# scope: document
# compile:
#  unknowns:
#   - input.baskets
include if {
			input.fruits.colour == input.colour
			input.baskets.colour == input.colour
}

# METADATA
# scope: document
# description: if this metadata were picked up, the checks would fail
# compile:
#  unknowns:
#   - input.colour
red_herring if true
`,
			result: map[string]any{
				"type":     "compound",
				"operator": "and",
				"value": []any{
					map[string]any{"type": "field", "field": "fruits.colour", "operator": "eq", "value": "orange"},
					map[string]any{"type": "field", "field": "baskets.colour", "operator": "eq", "value": "orange"},
				},
			},
		},
		{
			note:   "undefined field value",
			rego:   `include if input.fruits.colour == null`,
			result: map[string]any{"type": "field", "field": "fruits.colour", "operator": "eq", "value": nil},
		},
		{
			note: "happy path, compound 'and'",
			rego: `include if { input.fruits.colour == input.colour; input.fruits.name == "clementine" }`,
			result: map[string]any{
				"type":     "compound",
				"operator": "and",
				"value": []any{
					map[string]any{"type": "field", "field": "fruits.colour", "operator": "eq", "value": "orange"},
					map[string]any{"type": "field", "field": "fruits.name", "operator": "eq", "value": "clementine"},
				},
			},
		},
		{
			note: "happy path, compound 'or'",
			rego: `include if input.fruits.colour == input.colour
				include if input.fruits.name == "clementine"`,
			result: map[string]any{
				"type":     "compound",
				"operator": "or",
				"value": []any{
					map[string]any{"type": "field", "field": "fruits.colour", "operator": "eq", "value": "orange"},
					map[string]any{"type": "field", "field": "fruits.name", "operator": "eq", "value": "clementine"},
				},
			},
		},
		{
			note: "happy path, compound mixed",
			rego: `include if input.fruits.colour == input.colour
				include if { input.fruits.name == "clementine"; input.fruits.price > 10 }`,
			result: map[string]any{
				"type":     "compound",
				"operator": "or",
				"value": []any{
					map[string]any{"type": "field", "field": "fruits.colour", "operator": "eq", "value": "orange"},
					map[string]any{
						"type":     "compound",
						"operator": "and",
						"value": []any{
							map[string]any{"type": "field", "field": "fruits.name", "operator": "eq", "value": "clementine"},
							map[string]any{"type": "field", "field": "fruits.price", "operator": "gt", "value": float64(10)},
						},
					},
				},
			},
		},
		{
			note:   "happy path, reversed",
			rego:   `include if input.colour == input.fruits.colour`,
			result: map[string]any{"type": "field", "field": "fruits.colour", "operator": "eq", "value": "orange"},
		},
		{
			note:   "happy path, comparison",
			rego:   `include if input.fruits.price > 10`,
			result: map[string]any{"type": "field", "field": "fruits.price", "operator": "gt", "value": float64(10)},
		},
		{
			note:   "happy path, comparison with two unknowns",
			target: "application/vnd.opa.ucast.all+json",
			rego:   `include if input.fruits.price < input.fruits.max_price`,
			result: map[string]any{"type": "field", "field": "fruits.price", "operator": "lt", "value": map[string]any{"field": "fruits.max_price"}},
		},
		{
			note:   "happy path, comparison, reversed",
			rego:   `include if 10 <= input.fruits.price`,
			result: map[string]any{"type": "field", "field": "fruits.price", "operator": "gt", "value": float64(10)},
		},
		{
			note:   "happy path, not-equal",
			rego:   `include if input.fruits.name != "apple"`,
			result: map[string]any{"type": "field", "field": "fruits.name", "operator": "ne", "value": "apple"},
		},
		{
			note: "happy path, mapped short unknown",
			rego: `package filters
# METADATA
# compile:
#  unknowns:
#   - input.name
include if input.name != "apple"`,
			regoVerbatim: true,
			omitUnknowns: true,
			mappings: map[string]any{
				"name": map[string]any{"$table": "fruits"},
			},
			result: map[string]any{"type": "field", "field": "fruits.name", "operator": "ne", "value": "apple"},
		},
		{
			note: "happy path, double-mapped short unknown",
			rego: `package filters
# METADATA
# compile:
#  unknowns:
#   - input.name
include if input.name != "apple"`,
			regoVerbatim: true,
			omitUnknowns: true,
			mappings: map[string]any{
				"name":   map[string]any{"$table": "fruits"},
				"fruits": map[string]any{"$self": "FRUIT", "name": "NAME"},
			},
			result: map[string]any{"type": "field", "field": "FRUIT.NAME", "operator": "ne", "value": "apple"},
		},
		{
			note: "happy path, not+equal",
			rego: `include if not input.fruits.name == "apple"`,
			result: map[string]any{
				"type":     "compound",
				"operator": "not",
				"value": []any{
					map[string]any{"type": "field", "field": "fruits.name", "operator": "eq", "value": "apple"}},
			},
		},
		{
			note:   "happy path, not-equal, reversed",
			rego:   `include if "apple" != input.fruits.name`,
			result: map[string]any{"type": "field", "field": "fruits.name", "operator": "ne", "value": "apple"},
		},
		{
			note:   "happy path, startswith",
			rego:   `include if startswith(input.fruits.name, "app")`,
			result: map[string]any{"type": "field", "field": "fruits.name", "operator": "startswith", "value": "app"},
		},
		{
			note:   "happy path, endswith",
			rego:   `include if endswith(input.fruits.name, "le")`,
			result: map[string]any{"type": "field", "field": "fruits.name", "operator": "endswith", "value": "le"},
		},
		{
			note:   "happy path, contains",
			rego:   `include if contains(input.fruits.name, "ppl")`,
			result: map[string]any{"type": "field", "field": "fruits.name", "operator": "contains", "value": "ppl"},
		},
		{
			note:   "happy path, internal.member_2 (ucast/linq)",
			rego:   `include if input.fruits.name in {"apple", "pear"}`,
			target: "application/vnd.opa.ucast.linq+json",
			result: map[string]any{"type": "field", "field": "fruits.name", "operator": "in", "value": []any{"apple", "pear"}},
		},
		{
			note:   "invalid expression: not+equal (ucast/linq)",
			target: "application/vnd.opa.ucast.linq+json",
			rego:   `include if not input.fruits.name == "apple"`,
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 12),
					Message:  "\"not\" not permitted: unsupported feature \"not\" for UCAST (LINQ)",
				},
			},
		},
		{
			note:   "invalid builtin: internal.member_2 (ucast/minimal)",
			rego:   `include if input.fruits.name in {"apple", "pear"}`,
			target: "application/vnd.opa.ucast.minimal+json",
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 30),
					Message:  "invalid builtin `in`: unsupported for UCAST",
				},
			},
		},
		{
			note:   "invalid builtin: startswith (ucast/linq)",
			rego:   `include if startswith(input.fruits.name, "app")`,
			target: "application/vnd.opa.ucast.linq+json",
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 12),
					Message:  "invalid builtin `startswith`: unsupported for UCAST (LINQ)",
				},
			},
		},
		{
			note:   "invalid builtin: startswith (ucast)",
			rego:   `include if startswith(input.fruits.name, "app")`,
			target: "application/vnd.opa.ucast.minimal+json",
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 12),
					Message:  "invalid builtin `startswith`: unsupported for UCAST",
				},
			},
		},
		{
			note:   "invalid builtin: startswith (sqlite)",
			rego:   `include if startswith(input.fruits.name, "app")`,
			target: "application/vnd.opa.sql.sqlite+json",
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 12),
					Message:  "invalid builtin `startswith`: unsupported for SQL (sqlite)",
				},
			},
		},
		{
			note:   "invalid builtin: endswith (sqlite)",
			rego:   `include if endswith(input.fruits.name, "ple")`,
			target: "application/vnd.opa.sql.sqlite+json",
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 12),
					Message:  "invalid builtin `endswith`: unsupported for SQL (sqlite)",
				},
			},
		},
		{
			note:   "invalid builtin: contains (sqlite)",
			rego:   `include if contains(input.fruits.name, "pp")`,
			target: "application/vnd.opa.sql.sqlite+json",
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 12),
					Message:  "invalid builtin `contains`: unsupported for SQL (sqlite)",
				},
			},
		},
		{
			note: "invalid builtin",
			rego: `include if object.get(input, ["fruits", "colour"], "grey") == input.colour`,
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 12),
					Message:  "invalid builtin `object.get`",
				},
			},
		},
		{
			note: "invalid use of 'k, v in...'",
			rego: `include if "k", input.fruits.colour in {"k": "grey", "k2": input.colour}`,
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 37),
					Message:  "invalid use of \"... in ...\"",
				},
			},
		},
		{
			note:   "invalid use of '...in...' (ucast)",
			rego:   `include if "k", input.fruits.colour in {"k": "grey", "k2": "orange"}`,
			target: "application/vnd.opa.ucast.linq+json",
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 37),
					Message:  "invalid use of \"... in ...\"",
				},
			},
		},
		{
			note: "nested comp",
			rego: `include if (input.fruits.colour == "orange")>0`,
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 45),
					Message:  `gt: nested call operand: equal(input.fruits.colour, "orange")`, // TODO(sr): make this a user-friendlier message
				},
			},
		},
		{
			note: "nested call, object.get",
			rego: `user := object.get(input, ["user"], "unknown")
include if user == input.fruits.user`,
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 9),
					Message:  `eq: nested call operand: object.get(input, ["user"], "unknown")`, // TODO(sr): make this a user-friendlier message
				},
			},
		},
		{
			note:   "rhs+lhs both unknown",
			rego:   `include if input.fruits.colour == input.baskets.colour`,
			target: "application/vnd.opa.ucast.all+json",
			result: map[string]any{
				"type":     "field",
				"field":    "fruits.colour",
				"operator": "eq",
				"value":    map[string]any{"field": "baskets.colour"},
			},
		},
		{ // TODO(sr): ucast-prisma doesn't support this yet
			note:   "rhs+lhs both unknown, unsupported",
			rego:   `include if input.fruits.colour == input.baskets.colour`,
			target: "application/vnd.opa.ucast.prisma+json",
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 12),
					Message:  `reference to field: unsupported feature "field-ref" for UCAST (prisma)`,
				},
			},
		},
		{
			note: "contains: rhs unknown",
			rego: `include if contains("foobar", input.fruits.colour)`,
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 12),
					Message:  "rhs of contains must be known",
				},
			},
		},
		{
			note: "startswith: rhs unknown",
			rego: `include if startswith("foobar", input.fruits.colour)`,
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 12),
					Message:  "rhs of startswith must be known",
				},
			},
		},
		{
			note: "endswith: rhs unknown",
			rego: `include if endswith("foobar", input.fruits.colour)`,
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 12),
					Message:  "rhs of endswith must be known",
				},
			},
		},
		{
			note: "non-scalar comparison",
			rego: `include if input.fruits.colour <= {"green", "blue"}`, // nonsense, but still
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 32), // NOTE(sr): 32 is the `<=`
					Message:  "both rhs and lhs non-scalar/non-ground",
				},
			},
		},
		{ // NOTE(sr): only supported for SQL-ish targets and prisma (see below)
			note:   "equality with var",
			rego:   `include if input.fruits.colour = _`,
			target: "application/vnd.opa.ucast.linq+json",
			errors: []Error{
				{
					Code:    "pe_fragment_error",
					Message: `existence of field: unsupported feature "existence-ref" for UCAST (LINQ)`,
				},
			},
		},
		{
			note:   "equality with var (sql)",
			rego:   `include if input.fruits.colour = _`,
			target: "application/vnd.opa.sql.mysql+json",
			result: "WHERE fruits.colour IS NOT NULL",
		},
		{
			note:   "equality with var (sql), reversed",
			rego:   `include if _ = input.fruits.colour`,
			target: "application/vnd.opa.sql.mysql+json",
			result: "WHERE fruits.colour IS NOT NULL",
		},
		{
			note:   "equality with var (prisma)",
			rego:   `include if input.fruits.colour = _`,
			target: "application/vnd.opa.ucast.prisma+json",
			result: map[string]any{
				"type":     "field",
				"field":    "fruits.colour",
				"operator": "ne",
				"value":    nil,
			},
		},
		{
			note: "not a call/term",
			rego: `include if input.fruits.colour`,
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 12),
					Message:  "invalid statement \"input.fruits.colour\"",
					Details:  &compile.Details{Extra: "try `input.fruits.colour != false`"},
				},
			},
		},
		{
			note: "not a call/term, using with",
			rego: `include if {
	foo with input.fruits.colour as "red"
}
foo if input.fruits.colour`,
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 4, 2),
					Message:  "\"with\" not permitted",
				},
			},
		},
		{
			note: "not a call/every",
			rego: `include if every x in input.fruits.xs { x != "y" }`,
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 12),
					Message:  "\"every\" not permitted",
				},
			},
		},
		{
			note: "reference other row",
			rego: `include if {
   some other in input.fruits
   input.fruits.price > other.price
}`,
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 5, 23),
					Message:  "gt: invalid ref operand: input.fruits[__local1__1].price",
				},
			},
		},
		{
			note: "support module: default rule that is not false",
			rego: `include if other
default other := 100
other if input.fruits.price > 100
`,
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 12),
					Message:  "use of default rule in data.filters.other",
				},
			},
		},
		{
			note: "support module: multi-value rule",
			rego: `include if mv
mv contains 1 if input.fruits.price <= 1
mv contains 2 if input.fruits.price <= 2
`,
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 12),
					Message:  "use of multi-value rule in data.filters.mv",
				},
			},
		},
		{
			// NOTE(sr): This could lead to data policies getting accepted _now_ that at
			// some later time -- when the bug is fixed -- would no longer be valid.
			note: "support module: default function",
			rego: `include if cheap(input.fruits)
default cheap(_) := true
cheap(f) if f.price < 100
`,
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 12),
					Message:  "use of default rule in data.filters.other",
				},
			},
			skip: `https://github.com/open-policy-agent/opa/issues/7220`,
		},
		{
			note: "ref into module: complete rule with else",
			rego: `include if other
other if input.fruits.price > 100
else := input.fruits.extra
`,
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 12),
					Message:  "invalid data reference \"data.filters.other\"",
					Details:  &compile.Details{Extra: "has rule \"data.filters.other\" an `else`?"},
				},
			},
		},
		{
			note: "ref into module: function with else",
			rego: `include if func(input.fruits)
func(f) if f.price > 100
else := true
`,
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 12),
					Message:  "invalid data reference \"data.filters.func(input.fruits)\"",
					Details:  &compile.Details{Extra: "has function \"data.filters.func(...)\" an `else`?"},
				},
			},
		},
		{
			// NOTE(sr): this seems like one of the lower-hanging fruit for translating:
			// queries: [not data.partial.__not1_0_2__]
			// support:
			//   package partial
			//   __not1_0_2__ if input.fruits.price > 100
			note: "invalid expression: complete rule with not",
			rego: `include if not other
other if input.fruits.price > 100
`,
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Location: ast.NewLocation(nil, "filters.rego", 3, 12),
					Message:  "\"not\" not permitted",
				},
			},
		},
		{
			note:         "bad metadata unknowns",
			omitUnknowns: true,
			rego: `
# METADATA
# compile:
#  unknowns:
#   - inpu.fruits
#   - data.whatever
#   - future.keywrds
include if input.fruits.colour == input.colour

_use_metadata := rego.metadata.chain()`,
			errors: []Error{
				{
					Code:     "invalid_unknown",
					Location: ast.NewLocation(nil, "filters.rego", 4, 1),
					Message:  "unknowns must be prefixed with `input` or `data`: inpu.fruits",
				},
				{
					Code:     "invalid_unknown",
					Location: ast.NewLocation(nil, "filters.rego", 4, 1),
					Message:  "unknowns must be prefixed with `input` or `data`: future.keywrds",
				},
			},
		},
		{
			note:         "bad metadata unknowns (no kludge)",
			omitUnknowns: true,
			rego: `
# METADATA
# compile:
#  unknowns:
#   - inpu.fruits
#   - data.whatever
#   - future.keywrds
include if input.fruits.colour == input.colour
`,
			errors: []Error{
				{
					Code:     "invalid_unknown",
					Location: ast.NewLocation(nil, "filters.rego", 4, 1),
					Message:  "unknowns must be prefixed with `input` or `data`: inpu.fruits",
				},
				{
					Code:     "invalid_unknown",
					Location: ast.NewLocation(nil, "filters.rego", 4, 1),
					Message:  "unknowns must be prefixed with `input` or `data`: future.keywrds",
				},
			},
		},
		{
			note:   "non-det builtin with known args: http.send",
			rego:   fmt.Sprintf(`include if input.fruits.yummy == http.send({"method": "POST", "url": "%s"}).body.p`, testserver.URL),
			result: map[string]any{"type": "field", "field": "fruits.yummy", "operator": "eq", "value": true},
		},
		{
			note: "non-det builtin with unknown args: http.send",
			rego: fmt.Sprintf(`include if input.fruits.yummy == http.send({"method": "POST", "url": "%s", "body": input.fruits.taste}).body.p`, testserver.URL),
			errors: []Error{
				{
					Code:     "pe_fragment_error",
					Message:  "invalid builtin `http.send`",
					Location: ast.NewLocation(nil, "filters.rego", 3, 34),
				},
				{
					Code:     "pe_fragment_error",
					Message:  "eq: invalid ref operand: __local0__1.body.p",
					Location: ast.NewLocation(nil, "filters.rego", 3, 34),
				},
			},
		},
		{
			note: "non-determistic builtin (arity 0): opa.runtime()",
			rego: `include if {
		 		some k, v in opa.runtime()
				input.fruits[k] == v
			}`,
			result: map[string]any{
				"type":     "compound",
				"operator": "or",
				"value": []any{
					map[string]any{"type": "field", "field": "fruits.foo", "operator": "eq", "value": "bar"},
					map[string]any{"type": "field", "field": "fruits.fox", "operator": "eq", "value": float64(100)},
				},
			},
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			if tc.skip != "" {
				t.Skip(tc.skip)
			}
			var unknowns []string
			if !tc.omitUnknowns {
				unknowns = defaultUnknowns
			}
			rego := "package filters\nimport rego.v1\n" + tc.rego
			if tc.regoVerbatim {
				rego = tc.rego
			}
			path := cmp.Or(tc.query, defaultPath)
			input := cmp.Or(tc.input, any(defaultInput))
			target := cmp.Or(tc.target, ucastAcceptHeader)

			// second, query the compile API
			payload := map[string]any{
				"input":    input,
				"unknowns": unknowns,
				"options": map[string]any{
					"targetSQLTableMappings": map[string]any{
						"ucast": tc.mappings,
					},
				},
			}

			jsonData, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("Failed to marshal JSON: %v", err)
			}

			f := setup(t, rego, nil)

			req, err := http.NewRequest("POST", "/v1/compile/"+path, bytes.NewBuffer(jsonData))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", target)

			expCode := http.StatusOK
			if len(tc.errors) > 0 {
				expCode = http.StatusBadRequest
			}

			resp := map[string]any{}
			if len(tc.errors) > 0 {
				resp["errors"] = tc.errors
				resp["code"] = "evaluation_error"
				resp["message"] = "error(s) occurred while evaluating query"
			}
			if tc.result != nil {
				resp["result"] = map[string]any{"query": tc.result}
			}
			expResp, _ := json.Marshal(resp)
			if err := f.executeRequest(req, expCode, string(expResp)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

var testserver = srv(func(w http.ResponseWriter, _ *http.Request) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]any{
		"p": true,
	})
})

func srv(f func(http.ResponseWriter, *http.Request) error) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			w.WriteHeader(500)
			fmt.Fprintln(w, err.Error())
		}
	}))
}
