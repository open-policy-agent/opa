//go:build go1.27

package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/util"
	"github.com/open-policy-agent/opa/v1/util/test"
)

func TestOracleFindDefinition(t *testing.T) {
	cases := []struct {
		note         string
		v0Compatible bool
		onDiskModule string
		stdin        string
		paths        []string
	}{
		{
			note:         "v0",
			v0Compatible: true,
			onDiskModule: `package test

p { r }

r = true`,
			stdin: `package test

p { q }

q = true`,
			paths: []string{
				"test.rego:10",
				"test.rego:15",
				"test.rego:18",
			},
		},
		{
			note: "v1",
			onDiskModule: `package test

p if { r }

r = true`,
			stdin: `package test

p if { q }

q = true`,
			paths: []string{
				"test.rego:10",
				"test.rego:15",
				"test.rego:21",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			stdin := bytes.NewBufferString(tc.stdin)

			files := map[string]string{
				"test.rego":    tc.onDiskModule,
				"document.txt": "this should not be included",
				"ignore.json":  `{"neither": "should this"}`,
			}

			test.WithTempFS(files, func(rootDir string) {

				params := newFindDefinitionParams()
				params.bundlePaths = repeatedStringFlag{
					v:     []string{rootDir},
					isSet: true,
				}
				params.stdinBuffer = true
				params.v0Compatible = tc.v0Compatible

				stdout := bytes.NewBuffer(nil)

				err := dofindDefinition(params, stdin, stdout, []string{path.Join(rootDir, tc.paths[0])})
				expectJSON(t, err, stdout, `{"error": {"code": "oracle_no_match_found"}}`)

				err = dofindDefinition(params, stdin, stdout, []string{path.Join(rootDir, tc.paths[1])})
				expectJSON(t, err, stdout, `{"error": {"code": "oracle_no_definition_found"}}`)

				err = dofindDefinition(params, stdin, stdout, []string{path.Join(rootDir, tc.paths[2])})
				expectJSON(t, err, stdout, fmt.Sprintf(`{"result": {
			"file": %q,
			"row": 5,
			"col": 1
		}}`, path.Join(rootDir, "test.rego")))
			})
		})
	}

}

func TestOracleFindDefinitionJSONOutputBytes(t *testing.T) {
	onDiskModule := `package test

p if { r }

r = true`
	stdin := bytes.NewBufferString(`package test

p if { q }

q = true`)

	files := map[string]string{
		"test.rego":    onDiskModule,
		"document.txt": "this should not be included",
		"ignore.json":  `{"neither": "should this"}`,
	}

	test.WithTempFS(files, func(rootDir string) {
		params := newFindDefinitionParams()
		params.bundlePaths = repeatedStringFlag{
			v:     []string{rootDir},
			isSet: true,
		}
		params.stdinBuffer = true

		stdout := bytes.NewBuffer(nil)

		err := dofindDefinition(params, stdin, stdout, []string{path.Join(rootDir, "test.rego:10")})
		if err != nil {
			t.Fatal(err)
		}

		exp := `{
  "error": {
    "code": "oracle_no_match_found"
  }
}
`
		if diff := cmp.Diff(exp, stdout.String()); diff != "" {
			t.Errorf("unexpected result (-want, +got):\n%s", diff)
		}
	})
}

// The oracle parses the stdin buffer itself, so keywords that are gated behind
// capabilities must reach it through the --capabilities flag.
func TestOracleFindDefinitionCapabilities(t *testing.T) {
	module := `package test

import future.keywords.or

p if {
	q or r
}

q := 1

r := 2`

	restricted := ast.CapabilitiesForThisVersion()
	restricted.FutureKeywords = slices.DeleteFunc(restricted.FutureKeywords, func(kw string) bool {
		return kw == "or"
	})

	capabilities, err := json.Marshal(restricted)
	if err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"test.rego":         module,
		"capabilities.json": string(capabilities),
	}

	// offset of 'q' in the 'q or r' expression
	pos := strings.Index(module, "q or r")

	rootDir := test.TempDir(t, files)

	params := newFindDefinitionParams()
	params.bundlePaths = repeatedStringFlag{
		v:     []string{rootDir},
		isSet: true,
	}
	params.stdinBuffer = true

	arg := fmt.Sprintf("%s:%d", path.Join(rootDir, "test.rego"), pos)
	stdout := bytes.NewBuffer(nil)

	err = dofindDefinition(params, bytes.NewBufferString(module), stdout, []string{arg})
	expectJSON(t, err, stdout, fmt.Sprintf(`{"result": {
		"file": %q,
		"row": 9,
		"col": 1
	}}`, path.Join(rootDir, "test.rego")))

	if err := params.capabilities.Set(path.Join(rootDir, "capabilities.json")); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()

	err = dofindDefinition(params, bytes.NewBufferString(module), stdout, []string{arg})
	if err == nil || !strings.Contains(err.Error(), "rego_parse_error") {
		t.Fatal("expected parse error with restricted capabilities but got:", err, "result:", stdout.String())
	}
}

// The rego-version must be left undefined unless explicitly asked for, so that the
// buffer inherits it from the file it shadows, e.g. one parsed as v0 because of a
// bundle manifest.
func TestFindDefinitionParamsParserOptions(t *testing.T) {
	tests := []struct {
		note         string
		v0Compatible bool
		v1Compatible bool
		exp          ast.RegoVersion
	}{
		{note: "no flags", exp: ast.RegoUndefined},
		{note: "v0", v0Compatible: true, exp: ast.RegoV0},
		{note: "v1", v1Compatible: true, exp: ast.RegoV1},
		{note: "v0 takes precedence over v1", v0Compatible: true, v1Compatible: true, exp: ast.RegoV0},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			params := newFindDefinitionParams()
			params.v0Compatible = tc.v0Compatible
			params.v1Compatible = tc.v1Compatible

			if act := params.parserOptions().RegoVersion; act != tc.exp {
				t.Fatalf("expected rego-version %v but got %v", tc.exp, act)
			}
		})
	}
}

func expectJSON(t *testing.T, err error, buffer *bytes.Buffer, exp string) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	var x any
	if err := util.UnmarshalJSON(buffer.Bytes(), &x); err != nil {
		t.Fatal(err)
	}
	var y any
	if err := util.UnmarshalJSON([]byte(exp), &y); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(x, y) {
		t.Fatalf("expected %v but got %v", y, x)
	}
	buffer.Reset()
}

func TestOracleParseFilenameOffset(t *testing.T) {

	tests := []struct {
		input    string
		wantFile string
		wantPos  int
	}{
		{
			input:    "x.rego:10",
			wantFile: "x.rego",
			wantPos:  10,
		},
		{
			input:    "/x.rego:10",
			wantFile: "/x.rego",
			wantPos:  10,
		},
		{
			input:    "x.rego:0x10",
			wantFile: "x.rego",
			wantPos:  16,
		},
		{
			input:    "file://x.rego:10",
			wantFile: "x.rego",
			wantPos:  10,
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			filename, pos, err := parseFilenameOffset(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantFile != filename || tc.wantPos != pos {
				t.Fatalf("expected %v:%v but got %v:%v", tc.wantFile, tc.wantPos, filename, pos)
			}
		})
	}

}

func TestOracleParseFilenameOffsetError(t *testing.T) {

	tests := []struct {
		input   string
		wantErr error
	}{
		{
			input:   "x.rego",
			wantErr: errors.New("expected <filename>:<offset> argument"),
		},
		{
			input:   "x.rego:",
			wantErr: errors.New("invalid syntax"),
		},
		{
			input:   "x.rego:3.14",
			wantErr: errors.New("invalid syntax"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			_, _, err := parseFilenameOffset(tc.input)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr.Error()) {
				t.Fatalf("expected %v but got %v", tc.wantErr, err)
			}
		})
	}

}
