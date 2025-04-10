package cmd

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/format"
	"github.com/open-policy-agent/opa/v1/util/test"
)

func TestDoMoveRenamePackage(t *testing.T) {
	cases := []struct {
		note         string
		v0Compatible bool
		module       string
		expected     *ast.Module
	}{
		{
			note:         "v0",
			v0Compatible: true,
			module: `package lib.foo

				# this is a comment
				default allow = false

				allow {
					input.message == "hello"    # this is a comment too
				}`,
			expected: ast.MustParseModuleWithOpts(`package baz.bar

				# this is a comment
				default allow = false

				allow {
					input.message == "hello"    # this is a comment too
				}`, ast.ParserOptions{RegoVersion: ast.RegoV0}),
		},
		{
			note: "v1",
			module: `package lib.foo

				# this is a comment
				default allow = false

				allow if {
					input.message == "hello"    # this is a comment too
				}`,
			expected: ast.MustParseModuleWithOpts(`package baz.bar

				# this is a comment
				default allow = false

				allow if {
					input.message == "hello"    # this is a comment too
				}`, ast.ParserOptions{RegoVersion: ast.RegoV1}),
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"policy.rego": tc.module,
			}

			test.WithTempFS(files, func(path string) {

				mappings := []string{"data.lib.foo:data.baz.bar"}

				params := moveCommandParams{
					mapping:      newrepeatedStringFlag(mappings),
					v0Compatible: tc.v0Compatible,
				}

				var buf bytes.Buffer

				err := doMove(params, []string{path}, &buf)
				if err != nil {
					t.Fatal(err)
				}

				var formatted []byte
				if tc.v0Compatible {
					formatted = format.MustAstWithOpts(tc.expected, format.Opts{RegoVersion: ast.RegoV0})
				} else {
					formatted = format.MustAstWithOpts(tc.expected, format.Opts{RegoVersion: ast.RegoV1})
				}

				if !bytes.Equal(formatted, buf.Bytes()) {
					t.Fatalf("Expected module:\n%v\n\nGot:\n%v\n", string(formatted), buf.String())
				}
			})
		})
	}
}

func TestDoMoveOverwriteFile(t *testing.T) {
	cases := []struct {
		note         string
		v0Compatible bool
		module       string
		expected     *ast.Module
	}{
		{
			note:         "v0",
			v0Compatible: true,
			module: `package lib.foo

				import data.x.q

				default allow := false

				allow {
					input.message == "hello"
				}
				`,
			expected: ast.MustParseModuleWithOpts(`package baz.bar

				import data.hidden.q

				default allow := false

				allow {
					input.message == "hello"
				}`, ast.ParserOptions{RegoVersion: ast.RegoV0}),
		},
		{
			note: "v1",
			module: `package lib.foo

				import data.x.q

				default allow := false

				allow if {
					input.message == "hello"
				}
				`,
			expected: ast.MustParseModuleWithOpts(`package baz.bar

				import data.hidden.q

				default allow := false

				allow if {
					input.message == "hello"
				}`, ast.ParserOptions{RegoVersion: ast.RegoV1}),
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"policy.rego": tc.module,
			}

			test.WithTempFS(files, func(path string) {

				mappings := []string{"data.lib.foo:data.baz.bar", "data.x: data.hidden"}

				params := moveCommandParams{
					mapping:      newrepeatedStringFlag(mappings),
					overwrite:    true,
					v0Compatible: tc.v0Compatible,
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

				var actual *ast.Module
				if tc.v0Compatible {
					actual = ast.MustParseModuleWithOpts(string(data), ast.ParserOptions{RegoVersion: ast.RegoV0})
				} else {
					actual = ast.MustParseModuleWithOpts(string(data), ast.ParserOptions{RegoVersion: ast.RegoV1})
				}

				if !tc.expected.Equal(actual) {
					t.Fatalf("Expected module:\n%v\n\nGot:\n%v\n", tc.expected, actual)
				}
			})
		})
	}
}

func TestParseSrcDstMap(t *testing.T) {
	actual, err := parseSrcDstMap([]string{"data.lib.foo:data.baz.bar", "data:data.acme"})
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]string{"data.lib.foo": "data.baz.bar", "data": "data.acme"}

	if !maps.Equal(actual, expected) {
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
