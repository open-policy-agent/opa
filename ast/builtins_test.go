// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"reflect"
	"testing"
)

func TestUnifies(t *testing.T) {
	b := &Builtin{Name: Var("dummy"), NumArgs: 4, RecTargetPos: []int{2, 3}, TargetPos: []int{1}}
	expected := []int{1, 2, 3}
	result := []int{}
	for i := 0; i < 4; i++ {
		if b.Unifies(i) {
			result = append(result, i)
		}
	}
	if !reflect.DeepEqual(expected, result) {
		t.Errorf("Expected %v but got: %v", expected, result)
	}
}

func TestUnifiesRecursively(t *testing.T) {

	b := &Builtin{Name: Var("dummy"), NumArgs: 4, RecTargetPos: []int{2, 3}, TargetPos: []int{1}}
	expected := []int{2, 3}
	result := []int{}
	for i := 0; i < 4; i++ {
		if b.UnifiesRecursively(i) {
			result = append(result, i)
		}
	}
	if !reflect.DeepEqual(expected, result) {
		t.Errorf("Expected %v but got: %v", expected, result)
	}
}
