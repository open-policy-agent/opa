package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/open-policy-agent/opa/internal/planner"
	"github.com/open-policy-agent/opa/v1/ast"
)

const committedSchemaPath = "../../../v1/ir/plan.schema.json"

func TestSchemaDoesNotDrift(t *testing.T) {
	got, err := reflectSchema()
	if err != nil {
		t.Fatalf("reflectSchema: %v", err)
	}
	want, err := os.ReadFile(filepath.FromSlash(committedSchemaPath))
	if err != nil {
		t.Fatalf("read %s: %v", committedSchemaPath, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale; run `make generate` to update", committedSchemaPath)
	}
}

func TestSchemaValidatesRealPlans(t *testing.T) {
	schemaBytes, err := reflectSchema()
	if err != nil {
		t.Fatalf("reflectSchema: %v", err)
	}
	var schemaDoc any
	if err := json.Unmarshal(schemaBytes, &schemaDoc); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("plan.schema.json", schemaDoc); err != nil {
		t.Fatalf("add schema resource: %v", err)
	}
	schema, err := compiler.Compile("plan.schema.json")
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}

	cases := []struct {
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

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			c, err := ast.CompileModules(map[string]string{"test.rego": tc.module})
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
					Queries: []ast.Body{ast.MustParseBody(tc.query)},
				}}).
				WithModules(modules).
				WithBuiltinDecls(ast.BuiltinMap).
				Plan()
			if err != nil {
				t.Fatalf("plan: %v", err)
			}

			bs, err := json.Marshal(plan)
			if err != nil {
				t.Fatalf("marshal plan: %v", err)
			}
			var planDoc any
			if err := json.Unmarshal(bs, &planDoc); err != nil {
				t.Fatalf("unmarshal plan: %v", err)
			}
			if err := schema.Validate(planDoc); err != nil {
				t.Fatalf("plan does not validate:\n%s\n\nplan JSON:\n%s",
					strings.TrimSpace(err.Error()), string(bs))
			}
		})
	}
}
