// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package opalog

import (
	"testing"
)

func TestSetAdd(t *testing.T) {
	eq := func(x interface{}, y interface{}) bool { return x == y }
	s1 := NewSet(eq)
	s1.Add(1)
	s1.Add(2)
	if !s1.Contains(1) || !s1.Contains(2) {
		t.Errorf("Failure on Contains 1 || Contains 2")
	}
	if s1.Length() != 2 {
		t.Errorf("Failure on set length")
	}
}

func TestSetDifference(t *testing.T) {
	eq := func(x interface{}, y interface{}) bool { return x == y }
	s1 := NewSet(eq)
	s2 := NewSet(eq)
	s1.Add(1)
	s1.Add(2)
	s2.Add(1)
	s12 := s1.Difference(s2)
	if s12.Length() != 1 {
		t.Errorf("Failure on set difference length")
	}
	if !s12.Contains(2) {
		t.Errorf("Failure on set difference containment")
	}
	s21 := s2.Difference(s1)
	if s21.Length() != 0 {
		t.Errorf("Failure on set difference inversion length")
	}
	if s1.Length() != 2 {
		t.Errorf("Set-difference modified s1")
	}
	if !s1.Contains(1) || !s1.Contains(2) {
		t.Errorf("Set-difference changed a value in s1")
	}
	if s2.Length() != 1 {
		t.Errorf("Set-difference modified s2")
	}
	if !s2.Contains(1) {
		t.Errorf("Set-difference changed a value in s2")
	}
}

func TestSetEquality(t *testing.T) {
	eq := func(x interface{}, y interface{}) bool { return x == y }
	s1 := NewSet(eq)
	s2 := NewSet(eq)
	s1.Add(1)
	s1.Add(2)
	s2.Add(1)
	s2.Add(2)
	if !s1.Equal(s2) {
		t.Errorf("Equality on sets failed")
	}
}
