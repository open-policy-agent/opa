package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/format"
	"github.com/open-policy-agent/opa/util/test"
)

func TestDoMoveRenamePackage(t *testing.T) {

	files := map[string]string{
		"policy.rego": `package lib.foo

# this is a comment
default allow = false

allow {
        input.message == "hello"    # this is a comment too
}`,
	}

	test.WithTempFS(files, func(path string) {

		mappings := []string{"data.lib.foo:data.baz.bar"}

		params := moveCommandParams{
			mapping: newrepeatedStringFlag(mappings),
		}

		var buf bytes.Buffer

		err := doMove(params, []string{path}, &buf)
		if err != nil {
			t.Fatal(err)
		}

		expected := ast.MustParseModule(`package baz.bar

# this is a comment
default allow = false

allow {
        input.message == "hello"    # this is a comment too
}`)

		formatted := format.MustAst(expected)

		if !reflect.DeepEqual(formatted, buf.Bytes()) {
			t.Fatalf("Expected module:\n%v\n\nGot:\n%v\n", string(formatted), buf.String())
		}
	})
}

func TestDoMoveOverwriteFile(t *testing.T) {

	files := map[string]string{
		"policy.rego": `package lib.foo

import data.x.q

default allow = false
`,
	}

	test.WithTempFS(files, func(path string) {

		mappings := []string{"data.lib.foo:data.baz.bar", "data.x: data.hidden"}

		params := moveCommandParams{
			mapping:   newrepeatedStringFlag(mappings),
			overwrite: true,
		}

		var buf bytes.Buffer

		err := doMove(params, []string{path}, &buf)
		if err != nil {
			t.Fatal(err)
		}

		data, err := os.ReadFile(filepath.Join(path, "policy.rego"))
		if err != nil {
			t.Fatal(err)
		}

		actual := ast.MustParseModule(string(data))

		expected := ast.MustParseModule(`package baz.bar
		
		import data.hidden.q
	
		default allow = false`)

		if !expected.Equal(actual) {
			t.Fatalf("Expected module:\n%v\n\nGot:\n%v\n", expected, actual)
		}
	})
}

func TestParseSrcDstMap(t *testing.T) {
	actual, err := parseSrcDstMap([]string{"data.lib.foo:data.baz.bar", "data:data.acme"})
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]string{"data.lib.foo": "data.baz.bar", "data": "data.acme"}

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Expected mapping %v but got %v", expected, actual)
	}

	_, err = parseSrcDstMap([]string{"data.lib.foo:data.baz.bar", "data::data.acme"})
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	_, err = parseSrcDstMap([]string{"data.lib.foo:data.baz.bar", "data%data.acme:foo:bar"})
	if err == nil {
		t.Fatal("Expected error but got nil")
	}
}

func TestDoMoveNoMapping(t *testing.T) {
	err := doMove(moveCommandParams{}, []string{}, os.Stdout)
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	msg := "specify at least one mapping of the form <from>:<to>"
	if err.Error() != msg {
		t.Fatalf("Expected error %v but got %v", msg, err.Error())
	}
}

func TestValidateMoveArgs(t *testing.T) {
	err := validateMoveArgs([]string{})
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	msg := "specify at least one path containing policy files"
	if err.Error() != msg {
		t.Fatalf("Expected error %v but got %v", msg, err.Error())
	}

	err = validateMoveArgs([]string{"foo"})
	if err != nil {
		t.Fatal(err)
	}
}
