// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import "testing"
import "encoding/json"
import "reflect"
import "fmt"

func TestMergeDocs(t *testing.T) {

	tests := []struct {
		a string
		b string
		c interface{}
	}{
		{`{"x": 1, "y": 2}`, `{"z": 3}`, `{"x": 1, "y": 2, "z": 3}`},
		{`{"x": {"y": 2}}`, `{"z": 3, "x": {"q": 4}}`, `{"x": {"y": 2, "q": 4}, "z": 3}`},
		{`{"x": 1}`, `{"x": 1}`, fmt.Errorf("x: merge error: float64 cannot merge into float64")},
		{`{"x": {"y": [{"z": 2}]}}`, `{"x": {"y": [{"z": 3}]}}`, fmt.Errorf("x: y: merge error: []interface {} cannot merge into []interface {}")},
	}

	for _, tc := range tests {
		a := map[string]interface{}{}
		if err := json.Unmarshal([]byte(tc.a), &a); err != nil {
			panic(err)
		}

		b := map[string]interface{}{}
		if err := json.Unmarshal([]byte(tc.b), &b); err != nil {
			panic(err)
		}

		switch c := tc.c.(type) {
		case error:
			_, err := mergeDocs(a, b)
			if !reflect.DeepEqual(err.Error(), c.Error()) {
				t.Errorf("Expected error to be exactly %v but got: %v", c, err)
			}

		case string:
			expected := map[string]interface{}{}
			if err := json.Unmarshal([]byte(c), &expected); err != nil {
				panic(err)
			}

			result, err := mergeDocs(a, b)
			if err != nil {
				t.Errorf("Unexpected error on merge(%v, %v): %v", a, b, err)
				continue
			}

			if !reflect.DeepEqual(result, expected) {
				t.Errorf("Expected merge(%v, %v) to be %v but got: %v", a, b, expected, result)
			}
		}
	}
}
