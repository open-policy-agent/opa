// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"testing"

	"github.com/open-policy-agent/opa/types"
)

func TestBuiltinDeclRoundtrip(t *testing.T) {

	bs, err := json.Marshal(Plus)
	if err != nil {
		t.Fatal(err)
	}

	var cpy Builtin

	if err := json.Unmarshal(bs, &cpy); err != nil {
		t.Fatal(err)
	}

	if types.Compare(cpy.Decl, Plus.Decl) != 0 || cpy.Name != Plus.Name || cpy.Infix != Plus.Infix || cpy.Relation != Plus.Relation {
		t.Fatal("expected:", Plus, "got:", cpy)
	}
}
