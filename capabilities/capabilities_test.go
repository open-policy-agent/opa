// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build go1.16
// +build go1.16

package capabilities_test

import (
	"testing"

	"github.com/open-policy-agent/opa/capabilities"
	"github.com/open-policy-agent/opa/util"
)

func TestCapabilitiesEmbedded(t *testing.T) {
	ents, err := capabilities.FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) == 0 {
		t.Error("expected capabilities to be present")
	}
	for _, ent := range ents {
		cont, err := capabilities.FS.ReadFile(ent.Name())
		if err != nil {
			t.Errorf("file %v: %v", ent.Name(), err)
		}
		var x interface{}
		err = util.UnmarshalJSON(cont, &x)
		if err != nil {
			t.Errorf("file %v: %v", ent.Name(), err)
		}
	}
}
