// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package edittree

import (
	"fmt"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

// These test cases require malformed tree nodes or arguments,
// so they're tested separately from normal error conditions.
func TestEditTreeNilValueCases(t *testing.T) {
	// Try to create an EditTree with a nil Term pointer.
	if tree := NewEditTree(nil); tree != nil {
		t.Fatalf("Expected nil pointer, got %v", tree)
	}

	// Try to Insert on a nil-valued EditTree node.
	badroot := EditTree{value: nil}
	if result, err := badroot.Insert(ast.StringTerm("a"), ast.IntNumberTerm(2)); err != nil {
		if err.Error() != "deleted node encountered during insert operation" {
			t.Fatalf("wrong error: %v", err.Error())
		}
	} else {
		t.Fatalf("Expected error from Insert on nil-valued tree node, got %v", result)
	}
	// Try to Delete on a nil-valued EditTree node.
	if result, err := badroot.Delete(ast.StringTerm("a")); err != nil {
		if err.Error() != "deleted node encountered during delete operation" {
			t.Fatalf("wrong error: %v", err.Error())
		}
	} else {
		t.Fatalf("Expected error from Delete on nil-valued tree node, got %v", result)
	}
	// Try to Unfold past a nil-valued EditTree node.
	if result, err := badroot.Unfold(ast.Ref{ast.StringTerm("a")}); err != nil {
		if err.Error() != "nil value encountered where composite value was expected" {
			t.Fatalf("wrong error: %v", err.Error())
		}
	} else {
		t.Fatalf("Expected error from Unfold on nil-valued tree node, got %v", result)
	}
	// Try Exists with a nil-valued EditTree node.
	if ok := badroot.Exists(ast.Ref{ast.StringTerm("a")}); ok {
		t.Fatalf("Expected false from Exists on nil-valued tree node, got true instead")
	}
	// Try Filter with a nil-valued EditTree node.
	if result := badroot.Filter([]ast.Ref{{ast.StringTerm("a")}}); result != nil {
		t.Fatalf("Expected nil from Filter with nil-valued tree node, got: %v", result)
	}

	// Try to Insert with a nil value as the key Term.
	simpleTree := NewEditTree(ast.ObjectTerm())
	if result, err := simpleTree.Insert(nil, ast.IntNumberTerm(2)); err != nil {
		if err.Error() != "nil key provided for insert operation" {
			t.Fatalf("wrong error: %v", err.Error())
		}
	} else {
		t.Fatalf("Expected error from Insert with nil-valued key Term, got %v", result)
	}
	// Try to Insert with a nil value as the value Term.
	if result, err := simpleTree.Insert(ast.StringTerm("a"), nil); err != nil {
		if err.Error() != "nil value provided for insert operation" {
			t.Fatalf("wrong error: %v", err.Error())
		}
	} else {
		t.Fatalf("Expected error from Insert with nil-valued value Term, got %v", result)
	}
	// Try to Delete with a nil value as the key Term
	if result, err := simpleTree.Delete(nil); err != nil {
		if err.Error() != "nil key provided for delete operation" {
			t.Fatalf("wrong error: %v", err.Error())
		}
	} else {
		t.Fatalf("Expected error from Delete with nil-valued key Term, got %v", result)
	}
	// Try to InsertAtPath with a nil value.
	if result, err := simpleTree.InsertAtPath(ast.Ref{ast.StringTerm("a")}, nil); err != nil {
		if err.Error() != "cannot insert nil value into EditTree" {
			t.Fatalf("wrong error: %v", err.Error())
		}
	} else {
		t.Fatalf("Expected error from InsertAtPath with nil-valued value Term, got %v", result)
	}
	// Try Filter with a nil-valued path slice.
	if result := simpleTree.Filter(nil); ast.Compare(result, ast.ObjectTerm()) != 0 {
		t.Fatalf("Expected empty Object from Filter with nil-valued path slice, got: %v", result)
	}
	if result := simpleTree.Filter([]ast.Ref{{ast.StringTerm("/a")}, nil}); ast.Compare(result, ast.ObjectTerm()) != 0 {
		t.Fatalf("Expected empty Object from Filter with nil-valued path slice, got: %v", result)
	}
}

func TestEditTreeString(t *testing.T) {
	// A nil-valued EditTree node.
	badroot := EditTree{value: nil}
	if result := badroot.String(); result != "" {
		t.Fatalf("Expected \"\", got %v", result)
	}
	// A normal EditTree node.
	normalTree := NewEditTree(ast.ArrayTerm(ast.IntNumberTerm(0), ast.IntNumberTerm(1), ast.IntNumberTerm(2)))
	if result := normalTree.String(); result != "" {
		if result != "EditTree[[0, 1, 2]]" {
			t.Fatalf("Expected EditTree[[0, 1, 2]], got %v", result)
		}
	} else {
		t.Fatal("Unexpected empty string")
	}
}

func TestEditTreeFilter(t *testing.T) {
	cases := []struct {
		note      string
		object    string
		paths     []string
		source    string
		expResult string
		expError  error
	}{
		// Simple scalar.
		{
			note:      "simple scalar",
			paths:     []string{`[]`},
			source:    `2`,
			expResult: `2`,
		},
		// Top-level keys only.
		{
			note:      "simple top-level key - object",
			paths:     []string{`"/b"`},
			source:    `{"a": 2, "b": 3}`,
			expResult: `{"b": 3}`,
		},
		{
			note:      "simple top-level key - set",
			paths:     []string{`"/b"`},
			source:    `{"a", "b", "c"}`,
			expResult: `{"b"}`,
		},
		{
			note:      "simple top-level key - array",
			paths:     []string{`"/1"`},
			source:    `["a", "b", "c"]`,
			expResult: `["b"]`,
		},
		// Nested keys.
		{
			note:      "simple nested key - object",
			paths:     []string{`"/a/b/c"`},
			source:    `{"a": {"b": {"c": 3}, "d": 4}, "e": 5}`,
			expResult: `{"a": {"b": {"c": 3}}}`,
		},
		{
			note:      "simple nested key - set",
			paths:     []string{`[{"b", {"c"}}, {"c"}]`},
			source:    `{"a", {"b", {"c"}}}`,
			expResult: `{{{"c"}}}`,
		},
		{
			note:      "simple nested key - array",
			paths:     []string{`"/1/1/0"`},
			source:    `["a", ["b", ["c", 3], 4], 5]`,
			expResult: `[[["c"]]]`,
		},
		// Overlapping paths.
		{
			note:      "overlapping nested keys - object",
			paths:     []string{`"/a/b/c"`, `"/a"`},
			source:    `{"a": {"b": {"c": 3}, "d": 4}, "e": 5}`,
			expResult: `{"a": {"b": {"c": 3}, "d": 4}}`,
		},
		// Out-of-order array indexes.
		{
			note:      "out-of-order array indexes.",
			paths:     []string{`"/4"`, `"/0"`, `"/1"`},
			source:    `["a", "b", "c", "d", "e"]`,
			expResult: `["a", "b", "e"]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			paths := make([]ast.Ref, 0, len(tc.paths))
			for i := range tc.paths {
				path, err := parsePath(ast.MustParseTerm(tc.paths[i]))
				if err != nil {
					t.Fatal("could not parse path:", tc.paths[i])
				}
				paths = append(paths, path)
			}

			et := NewEditTree(ast.MustParseTerm(tc.source))
			// If an error occurred, was it the error we expected? If not, then that's an error.

			actual := et.Filter(paths)
			// TODO: Nil check here?
			// if err != nil {
			// 	t.Fatalf("unexpected error rendering EditTree: %s", err)
			// }

			if tc.expResult != "" {
				expected := ast.MustParseTerm(tc.expResult)
				if expected.Value.Compare(actual.Value) != 0 {
					t.Errorf("Wrong result generated from filter list:\n\nExpected:\n\t%v\n\nActual:\n\t%v\n\n", expected, actual)
				}
			}
		})
	}
}

// Some of the tests in this batch of tests reference the underlying tree
// methods directly. Those tests exist specifically to boost the test
// suite's line coverage. References to scalar/composite are similar in
// purpose: ensuring as many code paths are excercised as possible.
func TestEditTreeApplyPatches(t *testing.T) {
	cases := []struct {
		note      string
		object    string
		patches   []string
		source    string
		expResult string
		expError  error
	}{
		// Ops on top-level keys only.
		{
			note:      "simple add",
			patches:   []string{`{"op": "add", "path": "/b", "value": 3}`},
			source:    `{"a": 2}`,
			expResult: `{"a": 2, "b": 3}`,
		},
		{
			note:      "simple remove",
			patches:   []string{`{"op": "remove", "path": "/a"}`},
			source:    `{"a": true}`,
			expResult: `{}`,
		},
		{
			note:      "simple replace",
			patches:   []string{`{"op": "replace", "path": "/a", "value": 3}`},
			source:    `{"a": true}`,
			expResult: `{"a": 3}`,
		},
		{
			note:      "simple move",
			patches:   []string{`{"op": "move", "path": "/b", "from": "/a"}`},
			source:    `{"a": true}`,
			expResult: `{"b": true}`,
		},
		{
			note:      "simple copy",
			patches:   []string{`{"op": "copy", "path": "/b", "from": "/a"}`},
			source:    `{"a": true}`,
			expResult: `{"a": true, "b": true}`,
		},
		{
			note:      "simple test",
			patches:   []string{`{"op": "test", "path": "/a", "value": 2}`},
			source:    `{"a": 2}`,
			expResult: `{"a": 2}`,
		},
		// Op chains:
		{
			note: "simple add-remove chain x3",
			patches: []string{
				`{"op": "add", "path": "/a", "value": 3}`,
				`{"op": "remove", "path": "/a"}`,
				`{"op": "add", "path": "/a", "value": 3}`,
				`{"op": "remove", "path": "/a"}`,
				`{"op": "add", "path": "/a", "value": 3}`,
				`{"op": "remove", "path": "/a"}`,
			},
			source:    `{"a": true}`,
			expResult: `{}`,
		},
		{
			note: "simple add-move-remove chain x2",
			patches: []string{
				`{"op": "add", "path": "/a", "value": 3}`,
				`{"op": "move", "path": "/b", "from": "/a"}`,
				`{"op": "remove", "path": "/b"}`,
				`{"op": "add", "path": "/a", "value": 3}`,
				`{"op": "move", "path": "/b", "from": "/a"}`,
				`{"op": "remove", "path": "/b"}`,
			},
			source:    `{"a": true}`,
			expResult: `{}`,
		},
		{
			note: "simple add chain x3",
			patches: []string{
				`{"op": "add", "path": "/a", "value": 3}`,
				`{"op": "add", "path": "/a", "value": 4}`,
				`{"op": "add", "path": "/a", "value": 5}`,
			},
			source:    `{"a": true}`,
			expResult: `{"a": 5}`,
		},
		{
			note: "simple replace chain x3",
			patches: []string{
				`{"op": "replace", "path": "/a", "value": 3}`,
				`{"op": "replace", "path": "/a", "value": 4}`,
				`{"op": "replace", "path": "/a", "value": 5}`,
			},
			source:    `{"a": true}`,
			expResult: `{"a": 5}`,
		},
		{
			note: "simple test chain x3",
			patches: []string{
				`{"op": "test", "path": "/a", "value": 3}`,
				`{"op": "test", "path": "/a", "value": 3}`,
				`{"op": "test", "path": "/a", "value": 3}`,
			},
			source:    `{"a": 3}`,
			expResult: `{"a": 3}`,
		},
		// Nested operation tests.
		{
			note: "nested add chain object x3",
			patches: []string{
				`{"op": "add", "path": "/a", "value": {"b": {}}}`,
				`{"op": "add", "path": "/a/b", "value": {"c": {}}}`,
				`{"op": "add", "path": "/a/b/c", "value": {"d": {}}}`,
			},
			source:    `{"a": {}}`,
			expResult: `{"a": {"b": {"c": {"d": {}}}}}`,
		},
		{
			note: "nested add chain array x3 - composites",
			patches: []string{
				`{"op": "add", "path": "/0", "value": []}`,
				`{"op": "add", "path": "/0/0", "value": []}`,
				`{"op": "add", "path": "/0/0/0", "value": []}`,
			},
			source:    `[]`,
			expResult: `[[[[]]]]`,
		},
		{
			note: "add chain array x3 - scalars",
			patches: []string{
				`{"op": "add", "path": "/0", "value": 1}`,
				`{"op": "add", "path": "/0", "value": 2}`,
				`{"op": "add", "path": "/0", "value": 3}`,
			},
			source:    `[0]`,
			expResult: `[3, 2, 1, 0]`,
		},
		{
			note: "remove chain array x3 - scalars",
			patches: []string{
				`{"op": "add", "path": "/0", "value": 1}`,
				`{"op": "add", "path": "/0", "value": 2}`,
				`{"op": "add", "path": "/0", "value": 3}`,
				`{"op": "remove", "path": "/0"}`,
				`{"op": "remove", "path": "/0"}`,
				`{"op": "remove", "path": "/0"}`,
			},
			source:    `[0]`,
			expResult: `[0]`,
		},
		{
			note: "nested add chain set complex",
			patches: []string{
				`{"op": "add", "path": [{1}, 2], "value": 2}`,           // {{1, 2}, {2}, {3}}
				`{"op": "add", "path": [{2}, 3], "value": 3}`,           // {{1, 2}, {2, 3}, {3}}
				`{"op": "add", "path": [{3}, {4}], "value": {4}}`,       // {{1, 2}, {2, 3}, {3, {4}}}
				`{"op": "add", "path": [{3, {4}}, {4}, 5], "value": 5}`, // {{1, 2}, {2, 3}, {3, {4, 5}}}
			},
			source:    `{{1}, {2}, {3}}`,
			expResult: `{{1, 2}, {2, 3}, {3, {4, 5}}}`,
		},
		{
			note: "nested remove chain object x3",
			patches: []string{
				`{"op": "remove", "path": "/a/b/c/d"}`,
				`{"op": "remove", "path": "/a/b/c"}`,
				`{"op": "remove", "path": "/a/b"}`,
			},
			source:    `{"a": {"b": {"c": {"d": {}}}}}`,
			expResult: `{"a": {}}`,
		},
		{
			note: "nested remove chain array x3",
			patches: []string{
				`{"op": "remove", "path": "/1/2/0"}`,
				`{"op": "remove", "path": "/1/1"}`,
				`{"op": "remove", "path": "/0"}`,
			},
			source:    `[1, [2, 3, [4, 5]]]`,
			expResult: `[[2, [5]]]`,
		},
		// Array operations:
		{
			note: "array add scalars",
			patches: []string{
				`{"op": "add", "path": [0], "value": 1}`, // [1, 0]
				`{"op": "add", "path": [0], "value": 2}`, // [2, 1, 0]
				`{"op": "add", "path": [0], "value": 3}`, // [3, 2, 1, 0]
			},
			source:    `[0]`,
			expResult: `[3, 2, 1, 0]`,
		},
		{
			note: "array add composites",
			patches: []string{
				`{"op": "add", "path": [0], "value": [1]}`, // [[1], 0]
				`{"op": "add", "path": [0], "value": [2]}`, // [[2], [1], 0]
				`{"op": "add", "path": [0], "value": [3]}`, // [[3], [2], [1], 0]
			},
			source:    `[0]`,
			expResult: `[[3], [2], [1], 0]`,
		},
		{
			note: "array append scalar",
			patches: []string{
				`{"op": "add", "path": "-", "value": 2}`, // [0, 1]
			},
			source:    `[0, 1]`,
			expResult: `[0, 1, 2]`,
		},
		// Object operations:
		{
			note: "object add/replace bignum scalar",
			patches: []string{
				`{"op": "add", "path": [18446744073709551616], "value": 2}`,
				`{"op": "add", "path": [18446744073709551616], "value": 3}`,
			},
			source:    `{}`,
			expResult: `{18446744073709551616: 3}`,
		},
		// Set operations:
		{
			note: "set add scalar",
			patches: []string{
				`{"op": "add", "path": [2], "value": 2}`, // {"a", 2}
			},
			source:    `{"a"}`,
			expResult: `{"a", 2}`,
		},
		{
			note: "set remove scalar",
			patches: []string{
				`{"op": "remove", "path": [2]}`, // {"a"}
			},
			source:    `{"a", 2}`,
			expResult: `{"a"}`,
		},
		{
			note: "set remove scalar child",
			patches: []string{
				`{"op": "add", "path": ["b"], "value": "b"}`, // {"a", "b"}
				`{"op": "add", "path": [2], "value": 2}`,     // {"a", "b", 2}
				`{"op": "test", "path": [2], "value": 2}`,
				`{"op": "remove", "path": ["b"]}`, // {"a", 2}
				`{"op": "remove", "path": [2]}`,   // {"a"}
			},
			source:    `{"a"}`,
			expResult: `{"a"}`,
		},
		{
			note: "nested set add composite",
			patches: []string{
				`{"op": "add", "path": [{"a"}], "value": {"a"}}`,                                           // {"a", {"a"}}
				`{"op": "add", "path": [{"a"}, {"a"}], "value": {"a"}}`,                                    // {"a", {"a", {"a"}}}
				`{"op": "add", "path": [{"a", {"a"}}, {"a"}, {"a"}], "value": {"a"}}`,                      // {"a", {"a", {"a", {"a"}}}}
				`{"op": "add", "path": [{"a", {"a", {"a"}}}, {"a", {"a"}}, {"a"}, {"a"}], "value": {"a"}}`, // {"a", {"a", {"a", {"a", {"a"}}}}}
			},
			source:    `{"a"}`,
			expResult: `{"a", {"a", {"a", {"a", {"a"}}}}}`,
		},
		{
			note: "set remove nested composite",
			patches: []string{
				`{"op": "remove", "path": [{"a", {"a", {"a", {"a"}}}}, {"a", {"a", {"a"}}}, {"a", {"a"}}, {"a"}]}`, // {"a", {"a", {"a", {"a"}}}}
				`{"op": "remove", "path": [{"a", {"a", {"a"}}}, {"a", {"a"}}, {"a"}]}`,                             // {"a", {"a", {"a"}}}
				`{"op": "remove", "path": [{"a", {"a"}}, {"a"}]}`,                                                  // {"a", {"a"}}
				`{"op": "remove", "path": [{"a"}]}`,                                                                // {"a"}
				`{"op": "remove", "path": ["a"]}`,
			},
			source:    `{"a", {"a", {"a", {"a", {"a"}}}}}`,
			expResult: `set()`,
		},
		// Operations on the root:
		{
			note: "remove root",
			patches: []string{
				`{"op": "remove", "path": []}`,
			},
			source:    `{"a"}`,
			expResult: ``,
		},
		{
			note: "add/replace root with scalar",
			patches: []string{
				`{"op": "add", "path": [], "value": 3}`,
			},
			source:    `{"a"}`,
			expResult: `3`,
		},
		{
			note: "add/replace root with composite",
			patches: []string{
				`{"op": "add", "path": [], "value": [3]}`,
			},
			source:    `{"a"}`,
			expResult: `[3]`,
		},

		// Error cases.
		// Root node cases.
		{
			note: "remove on empty root errors",
			patches: []string{
				`{"op": "remove", "path": []}`,
				`{"op": "remove", "path": []}`,
			},
			source:   `"a"`,
			expError: fmt.Errorf(`deleted node encountered during delete operation`),
		},
		// Primitive/Scalar error cases.
		{
			note: "nested add on primitive string",
			patches: []string{
				`{"op": "add", "path": "/a", "value": "example"}`,
			},
			source:   `"a"`,
			expError: fmt.Errorf(`expected composite type, found value: "a" (type: ast.String)`),
		},
		{
			note: "nested add on primitive number",
			patches: []string{
				`{"op": "add", "path": "/1", "value": "example"}`,
			},
			source:   `2`,
			expError: fmt.Errorf(`expected composite type, found value: 2 (type: ast.Number)`),
		},
		{
			note: "nested remove on primitive string",
			patches: []string{
				`{"op": "remove", "path": "/a"}`,
			},
			source:   `"a"`,
			expError: fmt.Errorf(`expected composite type, found value: "a" (type: ast.String)`),
		},
		{
			note: "nested remove on primitive number",
			patches: []string{
				`{"op": "remove", "path": "/1"}`,
			},
			source:   `2`,
			expError: fmt.Errorf(`expected composite type, found value: 2 (type: ast.Number)`),
		},
		{
			note: "nested remove on nested primitive number",
			patches: []string{
				`{"op": "test", "path": "/a/2/b", "value": 3}`,
			},
			source:   `{"a": 2}`,
			expError: fmt.Errorf(`expected composite type for path "2", found value: 2 (type: ast.Number)`),
		},
		// Object error cases.
		{
			note: "remove on non-existent Object path",
			patches: []string{
				`{"op": "remove", "path": "/b"}`,
			},
			source:   `{"a": {}}`,
			expError: fmt.Errorf(`cannot delete child key "b" that does not exist`),
		},
		{
			note: "add on non-existent nested Object path",
			patches: []string{
				`{"op": "add", "path": "/b/c", "value": "example"}`,
			},
			source:   `{"a": {}}`,
			expError: fmt.Errorf(`path "b" does not exist in object term {"a": {}}`),
		},
		{
			note: "remove on non-existent nested Object path",
			patches: []string{
				`{"op": "remove", "path": "/b/c"}`,
			},
			source:   `{"a": {}}`,
			expError: fmt.Errorf(`path "b" does not exist in object term {"a": {}}`),
		},
		{
			note: "delete fails on deleted Object path - scalar",
			patches: []string{
				`{"op": "remove", "path": "/a"}`,
				`{"op": "remove", "path": "/a"}`,
			},
			source:   `{"a": 2}`,
			expError: fmt.Errorf(`cannot delete the already deleted scalar node for key "a"`),
		},
		{
			note: "delete fails on deleted Object path - composite",
			patches: []string{
				`{"op": "add", "path": "/a", "value": {2}}`,
				`{"op": "remove", "path": "/a"}`,
				`{"op": "remove", "path": "/a"}`,
			},
			source:   `{}`,
			expError: fmt.Errorf(`cannot delete the already deleted composite node for key "a"`),
		},
		{
			note: "unfold fails on deleted Object path - scalar",
			patches: []string{
				`{"op": "add", "path": "/a", "value": 2}`,
				`{"op": "remove", "path": "/a"}`,
				`{"op": "test", "path": "/a", "value": 2}`,
			},
			source:   `{}`,
			expError: fmt.Errorf(`cannot unfold the already deleted scalar node for key "a"`),
		},
		{
			note: "unfold fails on deleted Object path - composite",
			patches: []string{
				`{"op": "add", "path": "/a", "value": {2}}`,
				`{"op": "remove", "path": "/a"}`,
				`{"op": "test", "path": "/a", "value": 2}`,
			},
			source:   `{}`,
			expError: fmt.Errorf(`cannot unfold the already deleted composite node for key "a"`),
		},
		// Array error cases.
		{
			note: "add on non-existent Array path",
			patches: []string{
				`{"op": "add", "path": "/2", "value": "example"}`,
			},
			source:   `["a"]`,
			expError: fmt.Errorf(`index for array insertion out of bounds`),
		},
		{
			note: "remove on non-existent Array path",
			patches: []string{
				`{"op": "remove", "path": "/2"}`,
			},
			source:   `["a"]`,
			expError: fmt.Errorf(`index for array delete out of bounds`),
		},
		{
			note: "add on non-existent nested Array path",
			patches: []string{
				`{"op": "add", "path": "/0/2", "value": "example"}`,
			},
			source:   `["a", [1, 2]]`,
			expError: fmt.Errorf(`expected composite type, found value: "a" (type: ast.String)`),
		},
		{
			note: "remove on non-existent nested Array path",
			patches: []string{
				`{"op": "remove", "path": "/0/1"}`,
			},
			source:   `["a", [1, 2]]`,
			expError: fmt.Errorf(`expected composite type, found value: "a" (type: ast.String)`),
		},
		{
			note: "remove on non-integer number Array path - term array",
			patches: []string{
				`{"op": "remove", "path": [1, 4.3]}`,
			},
			source:   `["a", [1, 2]]`,
			expError: fmt.Errorf(`invalid number type for indexing`),
		},
		{
			note: "remove on non-integer number Array path - string",
			patches: []string{
				`{"op": "remove", "path": "/1/4.3"}`,
			},
			source:   `["a", [1, 2]]`,
			expError: fmt.Errorf(`invalid string for indexing`),
		},
		{
			note: "remove using number with 0 prefix in Array path",
			patches: []string{
				`{"op": "remove", "path": "/1/01"}`,
			},
			source:   `["a", [1, 2]]`,
			expError: fmt.Errorf(`leading zeros are not allowed in JSON paths`),
		},
		{
			note: "add with wrong indexing type in Array path",
			patches: []string{
				`{"op": "add", "path": [1, [0]], "value": 4}`,
			},
			source:   `["a", [1, 2]]`,
			expError: fmt.Errorf(`invalid type for indexing`),
		},
		{
			note: "test on non-existent nested Array path",
			patches: []string{
				`{"op": "add", "path": "/1", "value": 1}`,
				`{"op": "add", "path": "/2", "value": 2}`,
				`{"op": "test", "path": "/1/2", "value": "example"}`,
			},
			source:   `[0]`,
			expError: fmt.Errorf(`expected composite type for path "2", found value: 1 (type: ast.Number)`),
		},
		// The "-" index is always one beyond the end of the array, thus the delete case is an error.
		{
			note: "array remove scalar using the '-' path errors",
			patches: []string{
				`{"op": "remove", "path": "-"}`,
			},
			source:   `[0, 1, 2]`,
			expError: fmt.Errorf("index for array delete out of bounds"), // Ref: https://www.rfc-editor.org/rfc/rfc6901, section 4
		},
		// Set error cases.
		{
			note: "remove on non-existent Set path",
			patches: []string{
				`{"op": "remove", "path": "/b"}`,
			},
			source:   `{"a"}`,
			expError: fmt.Errorf(`cannot delete child key "b" that does not exist`),
		},
		{
			note: "add on non-existent nested Set path",
			patches: []string{
				`{"op": "add", "path": [{"a", [2, 1]}, [2, 1], 1], "value": {"a"}}`,
			},
			source:   `{"a"}`,
			expError: fmt.Errorf(`path {"a", [2, 1]} does not exist in set term {"a"}`),
		},
		{
			note: "remove on non-existent nested Set path",
			patches: []string{
				`{"op": "remove", "path": [{"a", [2, 1]}, [2, 1], 0]}`,
			},
			source:   `{"a"}`,
			expError: fmt.Errorf(`path {"a", [2, 1]} does not exist in set term {"a"}`),
		},
		{
			note: "insert non-matching key value pair into Set",
			patches: []string{
				`{"op": "add", "path": ["b"], "value": "c"}`,
			},
			source:   `{"a"}`,
			expError: fmt.Errorf(`set key "b" does not equal value to be inserted "c"`),
		},
		{
			note: "delete fails on deleted Set path - scalar",
			patches: []string{
				`{"op": "remove", "path": "/a"}`,
				`{"op": "remove", "path": "/a"}`,
			},
			source:   `{"a"}`,
			expError: fmt.Errorf(`cannot delete the already deleted scalar node for key "a"`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			patches := ast.NewArray()
			for i := range tc.patches {
				patches = patches.Append(ast.MustParseTerm(tc.patches[i]))
			}

			et := NewEditTree(ast.MustParseTerm(tc.source))
			err := patches.Iter(func(term *ast.Term) error {
				object, ok := term.Value.(ast.Object)
				if !ok {
					return fmt.Errorf("must be an array of JSON-Patch objects, but at least one element is not an object")
				}
				patch, err := getPatch(object)
				if err != nil {
					return err
				}
				path, err := parsePath(patch.path)
				if err != nil {
					return err
				}
				switch patch.op {
				case "add":
					_, err = et.InsertAtPath(path, patch.value)
					if err != nil {
						return err
					}
				case "remove":
					_, err = et.DeleteAtPath(path)
					if err != nil {
						return err
					}
				case "replace":
					_, err = et.DeleteAtPath(path)
					if err != nil {
						return err
					}
					_, err = et.InsertAtPath(path, patch.value)
					if err != nil {
						return err
					}
				case "move":
					from, err := parsePath(patch.from)
					if err != nil {
						return err
					}
					chunk, err := et.RenderAtPath(from)
					if err != nil {
						return err
					}
					_, err = et.DeleteAtPath(from)
					if err != nil {
						return err
					}
					_, err = et.InsertAtPath(path, chunk)
					if err != nil {
						return err
					}
				case "copy":
					from, err := parsePath(patch.from)
					if err != nil {
						return err
					}
					chunk, err := et.RenderAtPath(from)
					if err != nil {
						return err
					}
					_, err = et.InsertAtPath(path, chunk)
					if err != nil {
						return err
					}
				case "test":
					chunk, err := et.RenderAtPath(path)
					if err != nil {
						return err
					}
					if !chunk.Equal(patch.value) {
						return fmt.Errorf("value from EditTree != patch value.\n\nExpected: %v\n\nFound: %v", patch.value, chunk)
					}
				}
				return nil
			})
			// If an error occurred, was it the error we expected? If not, then that's an error.
			if err != nil {
				if tc.expError != nil {
					if tc.expError.Error() != err.Error() {
						t.Fatalf("wrong error: %s", err)
					}
				} else {
					t.Fatalf("unexpected error building EditTree: %s", err)
				}
			}

			actual := et.Render()
			// TODO: Nil check here?
			// if err != nil {
			// 	t.Fatalf("unexpected error rendering EditTree: %s", err)
			// }

			if tc.expResult != "" {
				expected := ast.MustParseTerm(tc.expResult)
				if expected.Value.Compare(actual.Value) != 0 {
					t.Errorf("Wrong result generated from patch list:\n\nExpected:\n\t%v\n\nActual:\n\t%v\n\n", expected, actual)
				}
			}
		})
	}
}

func parsePath(path *ast.Term) (ast.Ref, error) {
	// paths can either be a `/` separated json path or
	// an array or set of values
	var pathSegments ast.Ref
	switch p := path.Value.(type) {
	case ast.String:
		if p == "" {
			return ast.Ref{}, nil
		}
		parts := strings.Split(strings.TrimLeft(string(p), "/"), "/")
		for _, part := range parts {
			part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
			pathSegments = append(pathSegments, ast.StringTerm(part))
		}
	case *ast.Array:
		p.Foreach(func(term *ast.Term) {
			pathSegments = append(pathSegments, term)
		})
	default:
		return nil, builtins.NewOperandErr(2, "must be one of {set, array} containing string paths or array of path segments but got %v", ast.TypeName(p))
	}

	return pathSegments, nil
}

type jsonPatch struct {
	op    string
	path  *ast.Term
	from  *ast.Term
	value *ast.Term
}

func getPatch(o ast.Object) (jsonPatch, error) {
	var out jsonPatch
	var ok bool
	getAttribute := func(attr string) (*ast.Term, error) {
		if term := o.Get(ast.StringTerm(attr)); term != nil {
			return term, nil
		}

		return nil, fmt.Errorf("missing '%s' attribute", attr)
	}

	opTerm, err := getAttribute("op")
	if err != nil {
		return out, err
	}
	op, ok := opTerm.Value.(ast.String)
	if !ok {
		return out, fmt.Errorf("attribute 'op' must be a string")
	}
	out.op = string(op)

	pathTerm, err := getAttribute("path")
	if err != nil {
		return out, err
	}
	out.path = pathTerm

	// Fetch if present:
	fromTerm, err := getAttribute("from")
	if err != nil {
		switch out.op {
		case "move", "copy":
			return out, err
		}
	} else {
		out.from = fromTerm
	}

	// Fetch if present:
	valueTerm, err := getAttribute("value")
	if err != nil {
		switch out.op {
		case "add", "replace", "test":
			return out, err
		}
	}
	out.value = valueTerm

	return out, nil
}
