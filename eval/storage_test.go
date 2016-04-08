// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package eval

import "testing"
import "reflect"

func TestStorageLookup(t *testing.T) {

	data := loadSmallTestData()

	var tests = []struct {
		ref      string
		expected interface{}
	}{
		{"a[0]", float64(1)},
		{"a[3]", float64(4)},
		{"b.v1", "hello"},
		{"b.v2", "goodbye"},
		{"c[0].x[1]", false},
		{"c[0].y[0]", nil},
		{"c[0].y[1]", 3.14159},
		{"d.e[1]", "baz"},
		{"d.e", []interface{}{"bar", "baz"}},
		{"c[0].z", map[string]interface{}{"p": true, "q": false}},
		{"d[100]", notFoundError("cannot find path d[100] in storage, path references object with non-string key: 100")},
		{"dead.beef", notFoundError("cannot find path dead.beef in storage, path references object missing key: dead")},
		{"a.str", notFoundError("cannot find path a.str in storage, path references array with non-numeric key: \"str\"")},
		{"a[100]", notFoundError("cannot find path a[100] in storage, path references array with length: 4")},
		{"a[-1]", notFoundError("cannot find path a[-1] in storage, path references array using negative index: -1")},
		{"b.vdeadbeef", notFoundError("cannot find path b.vdeadbeef in storage, path references object missing key: \"vdeadbeef\"")},
		{"a[i]", &StorageError{Code: StorageNonGroundErr, Message: "cannot lookup non-ground reference: a[i]"}},
	}

	store := NewStorageFromJSONObject(data)

	for idx, tc := range tests {
		ref := parseRef(tc.ref)
		result, err := store.Lookup(ref)
		switch e := tc.expected.(type) {
		case error:
			if err == nil {
				t.Errorf("Test case %d: expected error for %v but got %v", idx+1, ref, result)
			} else if !reflect.DeepEqual(err, tc.expected) {
				t.Errorf("Test case %d: unexpected error for %v: %v, expected: %v", idx+1, ref, err, e)
			}
		default:
			if err != nil {
				t.Errorf("Test case %d: expected success for %v but got %v", idx+1, ref, err)
			}
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Test case %d: expected %f but got %f", idx+1, tc.expected, result)
			}
		}
	}

}
