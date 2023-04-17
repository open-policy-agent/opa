// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package deepcopy

import (
	"reflect"
	"testing"
)

func TestDeepCopyMapRoot(t *testing.T) {
	target := map[string]interface{}{
		"a": map[string]interface{}{
			"b": []interface{}{
				"c",
				"d",
			},
			"e": "f",
		},
		"x": "y",
	}
	result := DeepCopy(target).(map[string]interface{})
	if !reflect.DeepEqual(target, result) {
		t.Fatal("Expected result of DeepCopy to be DeepEqual with original.")
	}
	result["a"] = "mutated"
	if target["a"] == "mutated" {
		t.Fatal("Expected target to remain unmutated when the DeepCopy result was mutated")
	}
}
