// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build wasm_sdk_e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/test/cases"
	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/util"
)

const opaRootDir = "../../../../../"
var caseDir = flag.String("case-dir", filepath.Join(opaRootDir, "test/cases/testdata"), "set directory to load test cases from")
var exceptionsFile = flag.String("exceptions", "./exceptions.yaml", "set file to load a list of test names to exclude")

var exceptions map[string]string

func TestMain(m *testing.M) {
	exceptions = map[string]string{}

	bs, err := ioutil.ReadFile(*exceptionsFile)
	if err != nil {
		fmt.Println("Unable to load exceptions file: " + err.Error())
		os.Exit(1)
	}
	err = util.Unmarshal(bs, &exceptions)
	if err != nil {
		fmt.Println("Unable to parse exceptions file: " + err.Error())
		os.Exit(1)
	}

	addTestSleepBuiltin()

	os.Exit(m.Run())
}

func TestWasmE2E(t *testing.T) {

	ctx := context.Background()

	for _, tc := range cases.MustLoad(*caseDir).Sorted().Cases {
		name := fmt.Sprintf("%s/%s", strings.TrimPrefix(tc.Filename, opaRootDir), tc.Note)
		t.Run(name, func(t *testing.T) {

			if shouldSkip(t, tc) {
				t.SkipNow()
			}

			opts := []func(*rego.Rego){
				rego.Query(tc.Query),
			}
			for i := range tc.Modules {
				opts = append(opts, rego.Module(fmt.Sprintf("module-%d.rego", i), tc.Modules[i]))
			}
			cr, err := rego.New(opts...).Compile(ctx)
			if err != nil {
				t.Fatal(err)
			}
			o := opa.New().WithPolicyBytes(cr.Bytes)
			if tc.Data != nil {
				o = o.WithDataJSON(tc.Data)
			}
			o, err = o.Init()
			if err != nil {
				t.Fatal(err)
			}

			var input *interface{}

			if tc.InputTerm != nil {
				var x interface{} = ast.MustParseTerm(*tc.InputTerm)
				input = &x
			} else if tc.Input != nil {
				input = tc.Input
			}

			result, err := o.Eval(ctx, opa.EvalOpts{Input: input})
			assert(t, tc, result, err)
		})
	}
}

func shouldSkip(t *testing.T, tc cases.TestCase) bool {
	if reason, ok := exceptions[tc.Note]; ok {
		t.Log("Skipping test case: " + reason)
		return true
	}

	return false
}

func assert(t *testing.T, tc cases.TestCase, result *opa.Result, err error) {
	t.Helper()
	if tc.WantDefined != nil {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertDefined(t, defined(*tc.WantDefined), result)
	} else if tc.WantResult != nil {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertResultSet(t, *tc.WantResult, tc.SortBindings, result)
	} else if tc.WantErrorCode != nil || tc.WantError != nil {
		// The WASM compiler does not support strict errors so if the error
		// condition is only visible when strict errors are enabled, expect
		// an empty/undefined result from evaluation
		if tc.StrictError {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertEmptyResultSet(t, result)
			return
		}
		if err == nil {
			t.Fatal("expected error")
		}

		// TODO: implement more specific error checking, for now log results and skip the test
		if tc.WantErrorCode != nil {
			t.Logf("\nExpected Code: %s\nGot Err: %s\n", *tc.WantErrorCode, err)
		}

		if tc.WantError != nil {
			t.Logf("\nExpected Err: %s\nGot Err: %s\n", *tc.WantError, err)
		}

		t.Skip("Skipping test case: Error validation not supported")
	}
}

type defined bool

func (x defined) String() string {
	if x {
		return "defined"
	}
	return "undefined"
}

func assertDefined(t *testing.T, want defined, result *opa.Result) {
	t.Helper()
	var rs []interface{}
	if err := util.NewJSONDecoder(bytes.NewReader(result.Result)).Decode(&rs); err != nil {
		t.Fatal(err)
	}
	got := defined(len(rs) > 0)
	if got != want {
		t.Fatalf("expected %v but got %v", want, got)
	}
}

func assertEmptyResultSet(t *testing.T, result *opa.Result) {
	if result == nil {
		t.Fatal("unexpected nil result")
	}
	assertResultSet(t, []map[string]interface{}{}, false, result)
}

func assertResultSet(t *testing.T, want []map[string]interface{}, sortBindings bool, result *opa.Result) {
	t.Helper()
	a := toAST(want)

	// Round trip the wasm result through JSON to convert sets into array
	b := roundTripAstToJSON(result.Result)

	if sortBindings {
		result := ast.NewArray()
		a.Value.(*ast.Array).Sorted().Foreach(func(x *ast.Term) {
			cpy, _ := x.Value.(ast.Object).Map(func(k, v *ast.Term) (*ast.Term, *ast.Term, error) {
				return k, ast.NewTerm(v.Value.(*ast.Array).Sorted()), nil
			})
			result.Append(ast.NewTerm(cpy))
		})
		a.Value = result
		result = ast.NewArray()
		b.Value.(*ast.Array).Sorted().Foreach(func(x *ast.Term) {
			cpy, _ := x.Value.(ast.Object).Map(func(k, v *ast.Term) (*ast.Term, *ast.Term, error) {
				return k, ast.NewTerm(v.Value.(*ast.Array).Sorted()), nil
			})
			result.Append(ast.NewTerm(cpy))
		})
		b.Value = result
	}

	if !a.Equal(b) {
		t.Fatalf("expected %v but got %v", a, b)
	}

}

func toAST(a interface{}) *ast.Term {

	if bs, ok := a.([]byte); ok {
		return ast.MustParseTerm(string(bs))
	}

	buf := bytes.NewBuffer(nil)

	if err := json.NewEncoder(buf).Encode(a); err != nil {
		panic(err)
	}

	return ast.MustParseTerm(buf.String())
}

func roundTripAstToJSON(b []byte) *ast.Term {
	return toAST(ast.MustJSON(ast.MustParseTerm(string(b)).Value))
}

func addTestSleepBuiltin() {
	rego.RegisterBuiltin1(&rego.Function{
		Name: "test.sleep",
		Decl: types.NewFunction(types.Args(types.S), types.NewNull()),
	}, func(_ rego.BuiltinContext, op *ast.Term) (*ast.Term, error) {
		d, _ := time.ParseDuration(string(op.Value.(ast.String)))
		time.Sleep(d)
		return ast.NullTerm(), nil
	})
}
