package refactor

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestMoveRenamePackage(t *testing.T) {
	module := ast.MustParseModule(`package lib.foo
import rego.v1

default allow = false

allow if {
        input.message == "hello"
}`)

	modules := map[string]*ast.Module{
		"policy.rego": module,
	}

	mappings := map[string]string{
		"data.lib.foo": "data.baz.bar",
	}

	result, err := New().Move(MoveQuery{
		Modules:       modules,
		SrcDstMapping: mappings,
	})
	if err != nil {
		t.Fatal(err)
	}

	actual := result.Result["policy.rego"]

	expected := ast.MustParseModule(`package baz.bar
import rego.v1

default allow = false

allow if {
        input.message == "hello"
}`)

	if !expected.Equal(actual) {
		t.Fatalf("Expected module:\n%v\n\nGot:\n%v\n", expected, actual)
	}
}

func TestMoveRenamePackagePrefix(t *testing.T) {
	module1 := ast.MustParseModule(`package lib.foo
import rego.v1

default allow = false

allow if {
        input.message == "hello"
}`)

	module2 := ast.MustParseModule(`package lib.bar
import rego.v1

allow if {
        input.message == "world"
}`)

	modules := map[string]*ast.Module{
		"policy1.rego": module1,
		"policy2.rego": module2,
	}

	mappings := map[string]string{
		"data.lib": "data.hidden",
	}

	result, err := New().Move(MoveQuery{
		Modules:       modules,
		SrcDstMapping: mappings,
	})
	if err != nil {
		t.Fatal(err)
	}

	actual1 := result.Result["policy1.rego"]
	actual2 := result.Result["policy2.rego"]

	expected1 := ast.MustParseModule(`package hidden.foo
import rego.v1

default allow = false

allow if {
        input.message == "hello"
}`)

	expected2 := ast.MustParseModule(`package hidden.bar
import rego.v1

allow if {
        input.message == "world"
}`)

	if !expected1.Equal(actual1) {
		t.Fatalf("Expected module:\n%v\n\nGot:\n%v\n", expected1, actual1)
	}

	if !expected2.Equal(actual2) {
		t.Fatalf("Expected module:\n%v\n\nGot:\n%v\n", expected2, actual2)
	}
}

func TestMovePrefixInjection(t *testing.T) {
	module1 := ast.MustParseModule(`package a.b
import rego.v1

p if { data.x.q }`)

	module2 := ast.MustParseModule(`package x

q = true`)

	modules := map[string]*ast.Module{
		"policy1.rego": module1,
		"policy2.rego": module2,
	}

	mappings := map[string]string{
		"data": "data.deadbeef",
	}

	result, err := New().Move(MoveQuery{
		Modules:       modules,
		SrcDstMapping: mappings,
	})
	if err != nil {
		t.Fatal(err)
	}

	actual1 := result.Result["policy1.rego"]
	actual2 := result.Result["policy2.rego"]

	expected1 := ast.MustParseModule(`package deadbeef.a.b
import rego.v1

p if {
	data.deadbeef.x.q
}`)

	expected2 := ast.MustParseModule(`package deadbeef.x

q = true`)

	if !expected1.Equal(actual1) {
		t.Fatalf("Expected module:\n%v\n\nGot:\n%v\n", expected1, actual1)
	}

	if !expected2.Equal(actual2) {
		t.Fatalf("Expected module:\n%v\n\nGot:\n%v\n", expected2, actual2)
	}
}

func TestMoveWithKeyword(t *testing.T) {
	module1 := ast.MustParseModule(`package a.b
import rego.v1
import data.x.q as r

p if{ r with data.foo as 7 }`)

	module2 := ast.MustParseModule(`package x
import rego.v1

q if { data.foo == 7 }`)

	modules := map[string]*ast.Module{
		"policy1.rego": module1,
		"policy2.rego": module2,
	}

	mappings := map[string]string{
		"data": "data.deadbeef",
	}

	result, err := New().Move(MoveQuery{
		Modules:       modules,
		SrcDstMapping: mappings,
	})
	if err != nil {
		t.Fatal(err)
	}

	actual1 := result.Result["policy1.rego"]
	actual2 := result.Result["policy2.rego"]

	expected1 := ast.MustParseModule(`package deadbeef.a.b
import rego.v1
import data.deadbeef.x.q as r

p if {
	r with data.deadbeef.foo as 7
}`)

	expected2 := ast.MustParseModule(`package deadbeef.x
import rego.v1

q if {
	data.deadbeef.foo == 7
}`)

	if !expected1.Equal(actual1) {
		t.Fatalf("Expected module:\n%v\n\nGot:\n%v\n", expected1, actual1)
	}

	if !expected2.Equal(actual2) {
		t.Fatalf("Expected module:\n%v\n\nGot:\n%v\n", expected2, actual2)
	}
}

func TestMovePrefixTooShort(t *testing.T) {
	module1 := ast.MustParseModule(`package foo.bar

p = 7`)

	module2 := ast.MustParseModule(`package a

p = data.foo`)

	modules := map[string]*ast.Module{
		"policy1.rego": module1,
		"policy2.rego": module2,
	}

	mappings := map[string]string{
		"data.foo.bar": "data.baz",
	}

	_, err := New().Move(MoveQuery{
		Modules:       modules,
		SrcDstMapping: mappings,
	})
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	errMsg := "cannot rewrite `data.foo`: constant prefix `data.foo` of `data.foo` is too short"
	if !strings.Contains(err.Error(), errMsg) {
		t.Fatalf("Expected error message %v but got %v", errMsg, err.Error())
	}
}

func TestMovePrefixEmpty(t *testing.T) {
	module1 := ast.MustParseModule(`package foo.bar.v1
import rego.v1

helper_1 if {
	to_number(split(input.baz, ".")[1]) >= 1
}

helper_2 if {
	to_number(split(data.bar, ".")[1]) >= 1
}`)

	modules := map[string]*ast.Module{
		"policy1.rego": module1,
	}

	mappings := map[string]string{
		"data.foo": "data.hidden.name[\"hello:0.1\"]",
		"data.bar": "data.hello",
	}

	result, err := New().Move(MoveQuery{
		Modules:       modules,
		SrcDstMapping: mappings,
	})
	if err != nil {
		t.Fatal(err)
	}

	actual := result.Result["policy1.rego"]

	expected := ast.MustParseModule(`package hidden.name["hello:0.1"].bar.v1
import rego.v1

helper_1 if{
	to_number(split(input.baz, ".")[1]) >= 1
}

helper_2 if {
	to_number(split(data.hello, ".")[1]) >= 1
}`)

	if !expected.Equal(actual) {
		t.Fatalf("Expected module:\n%v\n\nGot:\n%v\n", expected, actual)
	}
}

func TestMoveConflictingRulesNoValidation(t *testing.T) {
	module1 := ast.MustParseModule(`package a.b
import rego.v1

p contains 1`)

	module2 := ast.MustParseModule(`package b

p = 7`)

	modules := map[string]*ast.Module{
		"policy1.rego": module1,
		"policy2.rego": module2,
	}

	mappings := map[string]string{
		"data.a": "data",
	}

	_, err := New().Move(MoveQuery{
		Modules:       modules,
		SrcDstMapping: mappings,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestMoveConflictingRulesWithValidation(t *testing.T) {
	module1 := ast.MustParseModule(`package a.b
import rego.v1

p contains 1`)

	module2 := ast.MustParseModule(`package b

p = 7`)

	modules := map[string]*ast.Module{
		"policy1.rego": module1,
		"policy2.rego": module2,
	}

	mappings := map[string]string{
		"data.a": "data",
	}

	_, err := New().Move(MoveQuery{
		Modules:       modules,
		SrcDstMapping: mappings,
	}.WithValidation(true))
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	errMsg := "rego_type_error: conflicting rules data.b.p found"
	if !strings.Contains(err.Error(), errMsg) {
		t.Fatalf("Expected error message %v but got %v", errMsg, err.Error())
	}
}

func TestMoveBadSourceMapping(t *testing.T) {
	module := ast.MustParseModule(`package lib.foo
import rego.v1

default allow = false

allow if {
        input.message == "hello"
}`)

	modules := map[string]*ast.Module{
		"policy.rego": module,
	}

	mappings := map[string]string{
		"data.lib.": "data.hidden",
	}

	_, err := New().Move(MoveQuery{
		Modules:       modules,
		SrcDstMapping: mappings,
	})
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	mappings = map[string]string{
		"data.lib": "data.hidden.",
	}

	_, err = New().Move(MoveQuery{
		Modules:       modules,
		SrcDstMapping: mappings,
	})
	if err == nil {
		t.Fatal("Expected error but got nil")
	}
}
