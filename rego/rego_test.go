// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rego

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func TestRegoCaptureTermsRewrite(t *testing.T) {

	ctx := context.Background()

	r := New(
		Query(`x; deadbeef = 1; y; z`),
		Package(`test`),
		Module("", `
			package test
			x = 1
			y = 2
			z = 3
		`),
	)

	rs, err := r.Eval(ctx)

	if len(rs) != 1 || len(rs[0].Expressions) != 4 || err != nil {
		t.Fatalf("Unexpected result set: %v (err: %v)", rs, err)
	}

	expected := map[string]interface{}{
		"x":            json.Number("1"),
		"y":            json.Number("2"),
		"z":            json.Number("3"),
		"deadbeef = 1": true,
	}

	for _, ev := range rs[0].Expressions {
		if !reflect.DeepEqual(expected[ev.Text], ev.Value) {
			t.Fatalf("Expected %v == %v but got: %v", ev.Text, expected[ev.Text], ev.Value)
		}
	}
}
