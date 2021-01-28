// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package planner

import (
	"testing"
)

func TestFuncstack(t *testing.T) {
	fs := newFuncstack()

	fs.Add("data.foo.bar", "g0.data.foo.bar")

	fs.Push(map[string]string{}) // g0 -> g1
	fs.Add("data.foo.bar", "g1.data.foo.bar")
	f, ok := fs.Get("data.foo.bar")
	if exp, act := true, ok; exp != act {
		t.Fatal("expected func to be found")
	}
	if exp, act := "g1.data.foo.bar", f; exp != act {
		t.Errorf("expected func to be %v, got %v", exp, act)
	}
	if exp, act := 1, fs.gen(); exp != act {
		t.Errorf("expected fs gen to be %d, got %d", exp, act)
	}

	g1 := fs.Pop() // g1 -> g0
	if exp, act := 1, len(g1); exp != act {
		t.Errorf("expected g1 func map to have length %d, got %d", exp, act)
	}
	if exp, act := 0, fs.gen(); exp != act {
		t.Errorf("expected fs gen to be %d, got %d", exp, act)
	}

	f, ok = fs.Get("data.foo.bar")
	if exp, act := true, ok; exp != act {
		t.Fatalf("expected func to be found")
	}
	if exp, act := "g0.data.foo.bar", f; exp != act {
		t.Errorf("expected func to be %v, got %v", exp, act)
	}

	fs.Push(map[string]string{}) // g0 -> g2
	fs.Add("data.foo.bar", "g2.data.foo.bar")
	f, ok = fs.Get("data.foo.bar")
	if exp, act := true, ok; exp != act {
		t.Fatal("expected func to be found")
	}
	if exp, act := "g2.data.foo.bar", f; exp != act {
		t.Errorf("expected func to be %v, got %v", exp, act)
	}
	if exp, act := 2, fs.gen(); exp != act {
		t.Errorf("expected fs gen to be %d, got %d", exp, act)
	}

	fs.Push(map[string]string{}) // g2 -> g3
	fs.Add("data.foo.bar", "g3.data.foo.bar")
	f, ok = fs.Get("data.foo.bar")
	if exp, act := true, ok; exp != act {
		t.Fatal("expected func to be found")
	}
	if exp, act := "g3.data.foo.bar", f; exp != act {
		t.Errorf("expected func to be %v, got %v", exp, act)
	}
	_ = fs.Pop() // g3 -> g2
	_ = fs.Pop() // g2 -> g0
	if exp, act := 0, fs.gen(); exp != act {
		t.Errorf("expected fs gen to be %d, got %d", exp, act)
	}

	fs.Push(map[string]string{}) // g0 -> g4
	if exp, act := 4, fs.gen(); exp != act {
		t.Errorf("expected fs gen to be %d, got %d", exp, act)
	}
}
