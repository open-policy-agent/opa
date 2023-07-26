// Copyright 2023 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package schemas_test

import (
	"testing"

	"github.com/open-policy-agent/opa/schemas"
	"github.com/open-policy-agent/opa/util"
)

func TestSchemasEmbedded(t *testing.T) {
	ents, err := schemas.FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) == 0 {
		t.Error("expected schemas to be present")
	}
	for _, ent := range ents {
		cont, err := schemas.FS.ReadFile(ent.Name())
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
