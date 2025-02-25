// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/v1/types"
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

func TestAllBuiltinsHaveDescribedArguments(t *testing.T) {
	for _, b := range Builtins {
		if b.deprecated || b.Infix != "" || b.Name == "print" || b.Name == "internal.print" || b.Name == "internal.test_case" {
			continue
		}

		t.Run(b.Name, func(t *testing.T) {
			namedAndDescribed(t, "arg", b.Decl.NamedFuncArgs().Args...)
			namedAndDescribed(t, "res", b.Decl.NamedResult())
		})
	}
}

func namedAndDescribed(t *testing.T, typ string, args ...types.Type) {
	t.Helper()

	for i, arg := range args {
		t.Run(fmt.Sprintf("%s=%d", typ, i), func(t *testing.T) {
			typ, ok := arg.(*types.NamedType)
			if !ok {
				t.Fatalf("expected arg to be %T, got %T", typ, arg)
			}
			if typ.Name == "" {
				t.Error("empty name")
			}
			if typ.Descr == "" {
				t.Error("empty description")
			}
		})
	}
}
