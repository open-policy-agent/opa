// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package proto

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode"

	"github.com/bufbuild/protocompile"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/open-policy-agent/opa/e2e/proto/protoroundtrip"
	"github.com/open-policy-agent/opa/e2e/proto/protoschemacheck"
	"github.com/open-policy-agent/opa/internal/planner"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/ir"
	"github.com/open-policy-agent/opa/v1/test/cases"
)

func TestPlanProtoConsistency(t *testing.T) {
	locationT := reflect.TypeOf(ir.Location{})

	stmtMsg := func(name string, goType reflect.Type, opts ...func(*protoschemacheck.MessageSpec)) protoschemacheck.MessageSpec {
		m := protoschemacheck.MessageSpec{
			Name:              name,
			GoType:            goType,
			SkipEmbeddedTypes: []reflect.Type{locationT},
		}
		for _, opt := range opts {
			opt(&m)
		}
		return m
	}

	withFieldOverride := func(overrides map[string]string) func(*protoschemacheck.MessageSpec) {
		return func(m *protoschemacheck.MessageSpec) {
			m.FieldNameOverride = overrides
		}
	}

	stmtBodies := []protoschemacheck.MessageSpec{
		stmtMsg("ArrayAppendStmt", reflect.TypeOf(ir.ArrayAppendStmt{})),
		stmtMsg("AssignIntStmt", reflect.TypeOf(ir.AssignIntStmt{})),
		stmtMsg("AssignVarOnceStmt", reflect.TypeOf(ir.AssignVarOnceStmt{})),
		stmtMsg("AssignVarStmt", reflect.TypeOf(ir.AssignVarStmt{})),
		stmtMsg("BlockStmt", reflect.TypeOf(ir.BlockStmt{})),
		stmtMsg("BreakStmt", reflect.TypeOf(ir.BreakStmt{})),
		stmtMsg("CallDynamicStmt", reflect.TypeOf(ir.CallDynamicStmt{})),
		// CallStmt.Func → proto "function" (avoids reserved-keyword
		// collision; mirrors Func.return → result).
		stmtMsg("CallStmt", reflect.TypeOf(ir.CallStmt{}),
			withFieldOverride(map[string]string{"func": "function"})),
		stmtMsg("DotStmt", reflect.TypeOf(ir.DotStmt{})),
		stmtMsg("EqualStmt", reflect.TypeOf(ir.EqualStmt{})),
		stmtMsg("IsArrayStmt", reflect.TypeOf(ir.IsArrayStmt{})),
		stmtMsg("IsDefinedStmt", reflect.TypeOf(ir.IsDefinedStmt{})),
		stmtMsg("IsObjectStmt", reflect.TypeOf(ir.IsObjectStmt{})),
		stmtMsg("IsSetStmt", reflect.TypeOf(ir.IsSetStmt{})),
		stmtMsg("IsUndefinedStmt", reflect.TypeOf(ir.IsUndefinedStmt{})),
		stmtMsg("LenStmt", reflect.TypeOf(ir.LenStmt{})),
		stmtMsg("MakeArrayStmt", reflect.TypeOf(ir.MakeArrayStmt{})),
		stmtMsg("MakeNullStmt", reflect.TypeOf(ir.MakeNullStmt{})),
		stmtMsg("MakeNumberIntStmt", reflect.TypeOf(ir.MakeNumberIntStmt{})),
		// MakeNumberRefStmt's Go field has no json tag, so its derived
		// JSON name is "Index"; the proto's canonical field is "index".
		// The runtime's MarshalJSON also emits a deprecated "Index" alias
		// — JSON-only contract, not modeled in the proto.
		stmtMsg("MakeNumberRefStmt", reflect.TypeOf(ir.MakeNumberRefStmt{}),
			withFieldOverride(map[string]string{"Index": "index"})),
		stmtMsg("MakeObjectStmt", reflect.TypeOf(ir.MakeObjectStmt{})),
		stmtMsg("MakeSetStmt", reflect.TypeOf(ir.MakeSetStmt{})),
		stmtMsg("NopStmt", reflect.TypeOf(ir.NopStmt{})),
		stmtMsg("NotEqualStmt", reflect.TypeOf(ir.NotEqualStmt{})),
		stmtMsg("NotStmt", reflect.TypeOf(ir.NotStmt{})),
		stmtMsg("ObjectInsertOnceStmt", reflect.TypeOf(ir.ObjectInsertOnceStmt{})),
		stmtMsg("ObjectInsertStmt", reflect.TypeOf(ir.ObjectInsertStmt{})),
		stmtMsg("ObjectMergeStmt", reflect.TypeOf(ir.ObjectMergeStmt{})),
		stmtMsg("ResetLocalStmt", reflect.TypeOf(ir.ResetLocalStmt{})),
		stmtMsg("ResultSetAddStmt", reflect.TypeOf(ir.ResultSetAddStmt{})),
		stmtMsg("ReturnLocalStmt", reflect.TypeOf(ir.ReturnLocalStmt{})),
		stmtMsg("ScanStmt", reflect.TypeOf(ir.ScanStmt{})),
		stmtMsg("SetAddStmt", reflect.TypeOf(ir.SetAddStmt{})),
		stmtMsg("WithStmt", reflect.TypeOf(ir.WithStmt{})),
	}

	structural := []protoschemacheck.MessageSpec{
		{Name: "Policy", GoType: reflect.TypeOf(ir.Policy{})},
		{Name: "Static", GoType: reflect.TypeOf(ir.Static{})},
		{Name: "Plans", GoType: reflect.TypeOf(ir.Plans{})},
		{Name: "Funcs", GoType: reflect.TypeOf(ir.Funcs{})},
		// BuiltinFunc.Decl has no proto counterpart by design — see
		// the comment on `BuiltinFunc` in plan.proto.
		{
			Name:         "BuiltinFunc",
			GoType:       reflect.TypeOf(ir.BuiltinFunc{}),
			SkipGoFields: []string{"Decl"},
		},
		{Name: "Plan", GoType: reflect.TypeOf(ir.Plan{})},
		// Func.Return → proto "result" (avoids reserved-keyword collision).
		{
			Name:              "Func",
			GoType:            reflect.TypeOf(ir.Func{}),
			FieldNameOverride: map[string]string{"return": "result"},
		},
		{Name: "Block", GoType: reflect.TypeOf(ir.Block{})},
		{Name: "StringConst", GoType: reflect.TypeOf(ir.StringConst{})},
		{Name: "Operand", GoType: reflect.TypeOf(ir.Operand{})},
		{Name: "UnplannedRule", GoType: reflect.TypeOf(ir.UnplannedRule{})},
		{Name: "Location", GoType: locationT},
		// Stmt envelope: file/col/row come from ir.Location; oneof
		// validated via the OneofSpec below.
		{Name: "Stmt", GoType: locationT},
	}

	messages := make([]protoschemacheck.MessageSpec, 0, len(structural)+len(stmtBodies))
	messages = append(messages, structural...)
	messages = append(messages, stmtBodies...)

	stmtDiscToCase := map[string]string{}
	stmtDiscToGoType := map[string]reflect.Type{}
	for k, v := range ir.StmtKinds() {
		stmtDiscToCase[k] = camelToSnake(k)
		stmtDiscToGoType[k] = reflect.TypeOf(v)
	}

	valDiscToCase := map[string]string{}
	valDiscToGoType := map[string]reflect.Type{}
	for k, v := range ir.ValKinds() {
		valDiscToCase[k] = camelToSnake(k)
		valDiscToGoType[k] = reflect.TypeOf(v)
	}

	protoschemacheck.Run(t, protoschemacheck.Spec{
		ProtoPath:      "plan.proto",
		ImportPaths:    []string{"../../v1/ir"},
		Messages:       messages,
		OpaqueMessages: []string{"Val"},
		Oneofs: []protoschemacheck.OneofSpec{
			{
				MessageName:           "Stmt",
				OneofName:             "kind",
				DiscriminatorToCase:   stmtDiscToCase,
				DiscriminatorToGoType: stmtDiscToGoType,
			},
			{
				MessageName:           "Val",
				OneofName:             "kind",
				DiscriminatorToCase:   valDiscToCase,
				DiscriminatorToGoType: valDiscToGoType,
			},
		},
	})
}

func TestPlanProtoRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	codec := loadPlanCodec(t)

	tests := []struct {
		note   string
		module string
		query  string
	}{
		{
			note: "scalar comparison",
			module: `package test
				p if { input.foo == 7 }`,
			query: "data.test.p = true",
		},
		{
			note: "every / scan",
			module: `package test
				p if { every i in input.foo { i > 0 } }`,
			query: "data.test.p = true",
		},
		{
			note: "composite construction and negation",
			module: `package test
				p contains x if {
					some x in input.xs
					not x == "skip"
				}
				q := {k: v | some k, v in input.m}`,
			query: "data.test.p = x",
		},
		{
			note: "with override",
			module: `package test
				import data.lib
				p if { lib.allow with input as {"u": "alice"} }
				`,
			query: "data.test.p = true",
		},
	}

	opts := roundTripCmpOpts()
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			plan := compileToPolicy(t, tc.module, tc.query)
			bs, err := codec.Encode(plan)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			var got ir.Policy
			if err := codec.Decode(bs, &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if diff := cmp.Diff(plan, &got, opts...); diff != "" {
				t.Fatalf("round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestPlanProtoRoundTripYAMLSuite runs the round-trip codec against every
// case the planner accepts under v1/test/cases/testdata/v1.
func TestPlanProtoRoundTripYAMLSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	codec := loadPlanCodec(t)
	corpus, err := cases.Load("../../v1/test/cases/testdata/v1")
	if err != nil {
		t.Fatalf("load YAML cases: %v", err)
	}
	if len(corpus.Cases) == 0 {
		t.Fatalf("no YAML cases loaded; did the test data move?")
	}

	opts := roundTripCmpOpts()
	for _, tc := range corpus.Cases {
		t.Run(tc.Note, func(t *testing.T) {
			plan, err := planYAMLCase(tc)
			if err != nil {
				t.Skipf("planner did not accept case (not drift): %v", err)
				return
			}
			bs, err := codec.Encode(plan)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			var got ir.Policy
			if err := codec.Decode(bs, &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if diff := cmp.Diff(plan, &got, opts...); diff != "" {
				t.Fatalf("round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// roundTripCmpOpts returns the cmp.Options that ignore wire-format-irrelevant
// fields (debug-only Location internals, BuiltinFunc.Decl absent from proto)
// and equate nil with empty for slices and maps.
func roundTripCmpOpts() []cmp.Option {
	return []cmp.Option{
		cmpopts.IgnoreUnexported(ir.Location{}),
		cmpopts.IgnoreFields(ir.Location{}, "Text"),
		cmpopts.IgnoreFields(ir.BuiltinFunc{}, "Decl"),
		cmpopts.EquateEmpty(),
	}
}

func planYAMLCase(tc cases.TestCase) (*ir.Policy, error) {
	if tc.Query == "" {
		return nil, errors.New("case has no query")
	}
	if len(tc.Modules) == 0 {
		return nil, errors.New("case has no modules")
	}
	moduleMap := map[string]string{}
	for i, m := range tc.Modules {
		moduleMap[fmt.Sprintf("module%d.rego", i)] = m
	}
	c, err := ast.CompileModules(moduleMap)
	if err != nil {
		return nil, fmt.Errorf("compile modules: %w", err)
	}
	mods := make([]*ast.Module, 0, len(c.Modules))
	for _, m := range c.Modules {
		mods = append(mods, m)
	}
	body, err := ast.ParseBody(tc.Query)
	if err != nil {
		return nil, fmt.Errorf("parse query: %w", err)
	}
	return planner.New().
		WithQueries([]planner.QuerySet{{Name: "main", Queries: []ast.Body{body}}}).
		WithModules(mods).
		WithBuiltinDecls(ast.BuiltinMap).
		Plan()
}

func compileToPolicy(t *testing.T, module, query string) *ir.Policy {
	t.Helper()
	c, err := ast.CompileModules(map[string]string{"test.rego": module})
	if err != nil {
		t.Fatalf("compile modules: %v", err)
	}
	modules := make([]*ast.Module, 0, len(c.Modules))
	for _, m := range c.Modules {
		modules = append(modules, m)
	}
	plan, err := planner.New().
		WithQueries([]planner.QuerySet{{
			Name:    "main",
			Queries: []ast.Body{ast.MustParseBody(query)},
		}}).
		WithModules(modules).
		WithBuiltinDecls(ast.BuiltinMap).
		Plan()
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	return plan
}

func loadPlanCodec(t *testing.T) *protoroundtrip.Codec {
	t.Helper()
	c := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: []string{"../../v1/ir"},
		}),
	}
	files, err := c.Compile(context.Background(), "plan.proto")
	if err != nil {
		t.Fatalf("compile plan.proto: %v", err)
	}
	codec := protoroundtrip.NewCodec(files[0])

	codec.RegisterRoot(reflect.TypeOf(ir.Policy{}), "Policy")
	codec.RegisterRoot(reflect.TypeOf(ir.Static{}), "Static")
	codec.RegisterRoot(reflect.TypeOf(ir.Plans{}), "Plans")
	codec.RegisterRoot(reflect.TypeOf(ir.Funcs{}), "Funcs")
	codec.RegisterRoot(reflect.TypeOf(ir.BuiltinFunc{}), "BuiltinFunc")
	codec.RegisterRoot(reflect.TypeOf(ir.Plan{}), "Plan")
	codec.RegisterRoot(reflect.TypeOf(ir.Func{}), "Func")
	codec.RegisterRoot(reflect.TypeOf(ir.Block{}), "Block")
	codec.RegisterRoot(reflect.TypeOf(ir.StringConst{}), "StringConst")
	codec.RegisterRoot(reflect.TypeOf(ir.Operand{}), "Operand")

	codec.SkipGoFields(reflect.TypeOf(ir.BuiltinFunc{}), "Decl")
	codec.SetFieldNameOverride(reflect.TypeOf(ir.Func{}), map[string]string{
		"return": "result",
	})
	codec.SetFieldNameOverride(reflect.TypeOf(ir.CallStmt{}), map[string]string{
		"func": "function",
	})
	codec.SetFieldNameOverride(reflect.TypeOf(ir.MakeNumberRefStmt{}), map[string]string{
		"Index": "index",
	})

	locationT := reflect.TypeOf(ir.Location{})
	for kind, body := range ir.StmtKinds() {
		bodyT := reflect.TypeOf(body)
		for bodyT.Kind() == reflect.Pointer {
			bodyT = bodyT.Elem()
		}
		codec.RegisterEnvelope(bodyT, protoroundtrip.EnvelopeSpec{
			Envelope:      "Stmt",
			Oneof:         "kind",
			Case:          camelToSnake(kind),
			PromotedEmbed: locationT,
		})
		codec.SkipEmbedded(bodyT, locationT)
	}
	for kind, val := range ir.ValKinds() {
		valT := reflect.TypeOf(val)
		for valT.Kind() == reflect.Pointer {
			valT = valT.Elem()
		}
		codec.RegisterEnvelope(valT, protoroundtrip.EnvelopeSpec{
			Envelope: "Val",
			Oneof:    "kind",
			Case:     kind,
		})
	}

	return codec
}

// camelToSnake converts CamelCase to snake_case, handling acronym
// boundaries: "AssignVarStmt" → "assign_var_stmt", "HTTPSendStmt" →
// "http_send_stmt", "string_index" → "string_index" (unchanged).
func camelToSnake(s string) string {
	rs := []rune(s)
	var b strings.Builder
	b.Grow(len(rs))
	for i, r := range rs {
		if i > 0 && unicode.IsUpper(r) {
			prevLower := unicode.IsLower(rs[i-1])
			nextLower := i+1 < len(rs) && unicode.IsLower(rs[i+1])
			if prevLower || nextLower {
				b.WriteByte('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
