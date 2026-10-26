// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import (
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/v1/server/types"
	"github.com/open-policy-agent/opa/v1/util"
)

// The requests built from a parsed body must equal the ones decoded from it.
func TestRequestFromMapMatchesDecode(t *testing.T) {
	bodies := []string{
		`{}`,
		`{"query": "data.x", "input": {"a": [1, 2.5, "b"]}, "unknowns": ["input.a"]}`,
		`{"query": "data.x", "input": null, "unknowns": [], "options": {"disableInlining": ["data.y"], "nondeterministicBuiltins": true}}`,
		`{"query": "data.x", "options": {"targetSQLTableMappings": {"sql": {"a": "b"}}, "targetDialects": ["postgresql+filters"], "maskRule": "masks"}, "snapshot_id": "s1", "n": 3}`,
	}

	for _, body := range bodies {
		t.Run(body, func(t *testing.T) {
			parsed := util.MustUnmarshalJSON([]byte(body)).(map[string]any)

			var wantC types.CompileRequestV1
			if err := util.UnmarshalJSON([]byte(body), &wantC); err != nil {
				t.Fatal(err)
			}
			gotC, err := compileRequestFromMap(parsed)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(gotC, wantC) {
				t.Errorf("compile: got %+v, want %+v", gotC, wantC)
			}

			var wantQ types.QueryRequestV1
			if err := util.UnmarshalJSON([]byte(body), &wantQ); err != nil {
				t.Fatal(err)
			}
			gotQ, err := queryRequestFromMap(parsed)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(gotQ, wantQ) {
				t.Errorf("query: got %+v, want %+v", gotQ, wantQ)
			}

			var wantF CompileFiltersRequestV1
			if err := util.UnmarshalJSON([]byte(body), &wantF); err != nil {
				t.Fatal(err)
			}
			gotF, err := compileFiltersRequestFromMap(parsed)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(gotF, wantF) {
				t.Errorf("compile filters: got %+v, want %+v", gotF, wantF)
			}
		})
	}
}

func TestRequestFromMapTypeMismatch(t *testing.T) {
	for _, body := range []string{
		`{"query": 1}`,
		`{"unknowns": "input.a"}`,
		`{"unknowns": [1]}`,
		`{"options": []}`,
		`{"options": {"nondeterministicBuiltins": "yes"}}`,
	} {
		t.Run(body, func(t *testing.T) {
			parsed := util.MustUnmarshalJSON([]byte(body)).(map[string]any)
			if _, err := compileRequestFromMap(parsed); err == nil {
				t.Error("compile: expected an error")
			}
		})
	}
}

// The authorizer's input.body has the keys the client sent, so the request must
// not honour a differently cased key that a policy checking maskRule would miss.
func TestCompileFiltersRequestFromMapExactKeys(t *testing.T) {
	parsed := util.MustUnmarshalJSON([]byte(`{"options": {"MaskRule": "data.private", "MASKRULE": "data.private"}}`)).(map[string]any)

	req, err := compileFiltersRequestFromMap(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if req.Options.MaskRule != "" {
		t.Errorf("expected no mask rule, got %q", req.Options.MaskRule)
	}
}
