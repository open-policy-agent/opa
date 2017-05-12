// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"reflect"
	"testing"
)

func TestIsTargetPos(t *testing.T) {
	b := &Builtin{Name: String("dummy"), TargetPos: []int{1, 3}}
	expected := []int{1, 3}
	result := []int{}
	for i := 0; i < 4; i++ {
		if b.IsTargetPos(i) {
			result = append(result, i)
		}
	}
	if !reflect.DeepEqual(expected, result) {
		t.Errorf("Expected %v but got: %v", expected, result)
	}
}
