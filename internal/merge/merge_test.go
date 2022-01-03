// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package merge

import (
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/util"
)

func TestMergeDocs(t *testing.T) {

	tests := []struct {
		a  string
		b  string
		c  string
		ok bool
	}{
		{`{"x": 1, "y": 2}`, `{"z": 3}`, `{"x": 1, "y": 2, "z": 3}`, true},
		{`{"x": {"y": 2}}`, `{"z": 3, "x": {"q": 4}}`, `{"x": {"y": 2, "q": 4}, "z": 3}`, true},
		{`{"x": 1}`, `{"x": 1}`, "", false},
		{`{"x": {"y": [{"z": 2}]}}`, `{"x": {"y": [{"z": 3}]}}`, "", false},
	}

	for _, tc := range tests {
		a := map[string]interface{}{}
		if err := util.UnmarshalJSON([]byte(tc.a), &a); err != nil {
			panic(err)
		}
		aInitial := map[string]interface{}{}
		if err := util.UnmarshalJSON([]byte(tc.a), &aInitial); err != nil {
			panic(err)
		}

		b := map[string]interface{}{}
		if err := util.UnmarshalJSON([]byte(tc.b), &b); err != nil {
			panic(err)
		}

		if len(tc.c) == 0 {

			c, ok := InterfaceMaps(a, b)
			if ok {
				t.Errorf("Expected merge(%v,%v) == false but got: %v", a, b, c)
			}

			if !reflect.DeepEqual(a, aInitial) {
				t.Errorf("Expected conflicting merge to not mutate a (%v) but got a: %v", aInitial, a)
			}

		} else {

			expected := map[string]interface{}{}
			if err := util.UnmarshalJSON([]byte(tc.c), &expected); err != nil {
				panic(err)
			}

			c, ok := InterfaceMaps(a, b)
			if !ok || !reflect.DeepEqual(c, expected) {
				t.Errorf("Expected merge(%v, %v) == %v but got: %v (ok: %v)", a, b, expected, c, ok)
			}

			if reflect.DeepEqual(a, aInitial) || !reflect.DeepEqual(a, c) {
				t.Errorf("Expected merge to mutate a (%v) but got %v", aInitial, a)
			}

		}
	}
}
