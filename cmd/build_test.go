package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/loader"
	"github.com/open-policy-agent/opa/v1/util"
	"github.com/open-policy-agent/opa/v1/util/test"
)

func TestBuildProducesBundle(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package test
			p = 1
		`,
	}

	test.WithTempFS(files, func(root string) {
		params := newBuildParams()
		params.outputFile = path.Join(root, "bundle.tar.gz")

		err := dobuild(params, []string{root})
		if err != nil {
			t.Fatal(err)
		}

		_, err = loader.NewFileLoader().AsBundle(params.outputFile)
		if err != nil {
			t.Fatal(err)
		}

		// Check that manifest is not written given no input manifest and no other flags
		f, err := os.Open(params.outputFile)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		gr, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}

		tr := tar.NewReader(gr)

		for {
			f, err := tr.Next()
			if err == io.EOF {
				break
			} else if err != nil {
				t.Fatal(err)
			}
			if f.Name == "/.manifest" || f.Name == "/data.json" || strings.HasSuffix(f.Name, "/test.rego") {
				continue
			}
			t.Fatal("unexpected file:", f.Name)
		}
	})
}

func TestBuildRespectsCapabilities(t *testing.T) {
	//nolint:prealloc // test slice is extended dynamically, initial values are clearer as slice literal
	tests := []struct {
		note       string
		caps       string
		policy     string
		err        string
		bundleMode bool // build with "-b" flag
	}{
		{
			note: "builtin defined in caps",
			caps: `{
			"builtins": [
				{
					"name": "is_foo",
					"decl": {
						"args": [
							{
								"type": "string"
							}
						],
						"result": {
							"type": "boolean"
						},
						"type": "function"
					}
				}
			]
		}`,
			policy: `package test
p { is_foo("bar") }`,
		},
		{
			note: "future kw NOT defined in caps",
			caps: func() string {
				c := ast.CapabilitiesForThisVersion()
				c.FutureKeywords = []string{"in"}
				c.Features = []string{}
				j, err := json.Marshal(c)
				if err != nil {
					panic(err)
				}
				return string(j)
			}(),
			policy: `package test
import future.keywords.if
import future.keywords.in
p if "opa" in input.tools`,
			err: "rego_parse_error: unexpected keyword, must be one of [in]",
		},
		{
			note: "future kw NOT defined in caps, rego-v1 feature",
			caps: func() string {
				c := ast.CapabilitiesForThisVersion()
				c.FutureKeywords = []string{"in"}
				c.Features = []string{ast.FeatureRegoV1}
				j, err := json.Marshal(c)
				if err != nil {
					panic(err)
				}
				return string(j)
			}(),
			policy: `package test
import future.keywords.if
import future.keywords.in
p if "opa" in input.tools`,
		},
		{
			note: "future kw are defined in caps",
			caps: func() string {
				c := ast.CapabilitiesForThisVersion()
				c.FutureKeywords = []string{"in", "if"}
				j, err := json.Marshal(c)
				if err != nil {
					panic(err)
				}
				return string(j)
			}(),
			policy: `package test
import future.keywords.if
import future.keywords.in
p if "opa" in input.tools`,
		},
		{
			note: "rego.v1 imported AND defined in capabilities",
			caps: func() string {
				c := ast.CapabilitiesForThisVersion()
				c.Features = []string{ast.FeatureRegoV1Import}
				j, err := json.Marshal(c)
				if err != nil {
					panic(err)
				}
				return string(j)
			}(),
			policy: `package test
import rego.v1`,
		},
		{
			note: "rego.v1 imported AND rego-v1 in capabilities",
			caps: func() string {
				c := ast.CapabilitiesForThisVersion()
				c.Features = []string{ast.FeatureRegoV1}
				j, err := json.Marshal(c)
				if err != nil {
					panic(err)
				}
				return string(j)
			}(),
			policy: `package test
import rego.v1`,
		},
	}

	// add same tests for bundle-mode == true:
	for i := range tests {
		tc := tests[i]
		tc.bundleMode = true
		tc.note += " (as bundle)"
		tests = append(tests, tc)
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"capabilities.json": tc.caps,
				"test.rego":         tc.policy,
			}

			test.WithTempFS(files, func(root string) {
				caps := newCapabilitiesFlag()
				if err := caps.Set(path.Join(root, "capabilities.json")); err != nil {
					t.Fatal(err)
				}
				params := newBuildParams()
				params.outputFile = path.Join(root, "bundle.tar.gz")
				params.capabilities = caps
				params.bundleMode = tc.bundleMode
				// Test capabilities are all pre-v1
				params.v0Compatible = true

				err := dobuild(params, []string{root})
				switch {
				case err != nil && tc.err != "":
					if !strings.Contains(err.Error(), tc.err) {
						t.Fatalf("expected err %v, got %v", tc.err, err)
					}
					return // don't read back bundle below
				case err != nil && tc.err == "":
					t.Fatalf("unexpected error: %v", err)
				case err == nil && tc.err != "":
					t.Fatalf("expected error %v, got nil", tc.err)
				}

				// check that the resulting bundle is readable
				_, err = loader.NewFileLoader().AsBundle(params.outputFile)
				if err != nil {
					t.Fatal(err)
				}
			})
		})
	}
}

func TestBuildFilesystemModeIgnoresTarGz(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package test
			p = 1
		`,
	}

	test.WithTempFS(files, func(root string) {
		params := newBuildParams()
		params.outputFile = path.Join(root, "bundle.tar.gz")

		err := dobuild(params, []string{root})
		if err != nil {
			t.Fatal(err)
		}

		_, err = loader.NewFileLoader().AsBundle(params.outputFile)
		if err != nil {
			t.Fatal(err)
		}

		// Just run the build again to simulate the user doing back-to-back builds.
		err = dobuild(params, []string{root})
		if err != nil {
			t.Fatal(err)
		}

	})
}

func TestBuildErrorDoesNotWriteFile(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package test
			
			p if { p }
		`,
	}

	test.WithTempFS(files, func(root string) {
		params := newBuildParams()
		params.outputFile = path.Join(root, "bundle.tar.gz")

		err := dobuild(params, []string{root})
		exp := fmt.Sprintf("1 error occurred: %s/test.rego:4: rego_recursion_error: rule data.test.p is recursive: data.test.p -> data.test.p",
			root)
		if err == nil || err.Error() != exp {
			t.Fatalf("expected recursion error %q but got: %q", exp, err)
		}

		if _, err := os.Stat(params.outputFile); !os.IsNotExist(err) {
			t.Fatalf("expected stat \"not found\" error, got %v", err)
		}
	})
}

func TestBuildErrorVerifyNonBundle(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package test
			
			p if { p }
		`,
	}

	test.WithTempFS(files, func(root string) {
		params := newBuildParams()
		params.outputFile = path.Join(root, "bundle.tar.gz")
		params.pubKey = "secret"

		err := dobuild(params, []string{root})
		if err == nil {
			t.Fatal("expected error but got nil")
		}

		exp := "enable bundle mode (ie. --bundle) to verify or sign bundle files or directories"
		if err.Error() != exp {
			t.Fatalf("expected error message %v but got %v", exp, err.Error())
		}
	})
}

func TestBuildVerificationConfigError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot be run as root")
	}

	files := map[string]string{
		"public.pem": "foo",
	}

	test.WithTempFS(files, func(rootDir string) {
		// simulate error while reading file
		err := os.Chmod(filepath.Join(rootDir, "public.pem"), 0111)
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}

		_, err = buildVerificationConfig(filepath.Join(rootDir, "public.pem"), "default", "", "", nil)
		if err == nil {
			t.Fatal("Expected error but got nil")
		}
	})
}

func TestBuildSigningConfigError(t *testing.T) {
	tests := []struct {
		note                    string
		key, plugin, claimsFile string
		expErr                  bool
	}{
		{
			note: "key+plugin+claimsFile unset",
		},
		{
			note:   "key+claimsFile unset",
			plugin: "plugin",
			expErr: true,
		},
		{
			note:       "key+plugin unset",
			claimsFile: "claims",
			expErr:     true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			_, err := buildSigningConfig(tc.key, defaultTokenSigningAlg, tc.claimsFile, tc.plugin)
			switch {
			case tc.expErr && err == nil:
				t.Fatal("Expected error but got nil")
			case !tc.expErr && err != nil:
				t.Fatalf("Expected no error but got %v", err)
			}
		})
	}
}

func TestBuildPlanWithPruneUnused(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package test
			
			p contains 1
			
			f(x) if { p[x] }
		`,
	}

	test.WithTempFS(files, func(root string) {
		params := newBuildParams()
		if err := params.target.Set("plan"); err != nil {
			t.Fatal(err)
		}
		params.pruneUnused = true
		params.entrypoints.v = []string{"test"}
		params.outputFile = path.Join(root, "bundle.tar.gz")

		err := dobuild(params, []string{root})
		if err != nil {
			t.Fatal(err)
		}

		_, err = loader.NewFileLoader().AsBundle(params.outputFile)
		if err != nil {
			t.Fatal(err)
		}

		// Check that manifest is not written given no input manifest and no other flags
		f, err := os.Open(params.outputFile)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		gr, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}

		tr := tar.NewReader(gr)

		found := false // for plan.json

		for {
			f, err := tr.Next()
			if err == io.EOF {
				break
			} else if err != nil {
				t.Fatal(err)
			}
			switch {
			case f.Name == "/plan.json":
				found = true
			case f.Name == "/.manifest" || f.Name == "/data.json" || strings.HasSuffix(f.Name, "/test.rego"): // expected
			default:
				t.Errorf("unexpected file: %s", f.Name)
			}
		}
		if !found {
			t.Error("plan.json not found")
		}
	})
}

func TestBuildPlanWithPrintStatements(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package test

			p if { print("hello") }
		`,
	}

	test.WithTempFS(files, func(root string) {
		params := newBuildParams()
		if err := params.target.Set("plan"); err != nil {
			t.Fatal(err)
		}
		params.entrypoints.v = []string{"test"}
		params.outputFile = path.Join(root, "bundle.tar.gz")

		err := dobuild(params, []string{root})
		if err != nil {
			t.Fatal(err)
		}

		_, err = loader.NewFileLoader().AsBundle(params.outputFile)
		if err != nil {
			t.Fatal(err)
		}

		f, err := os.Open(params.outputFile)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		gr, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}

		tr := tar.NewReader(gr)
		var found bool

		for {
			f, err := tr.Next()
			if err == io.EOF {
				break
			} else if err != nil {
				t.Fatal(err)
			}
			if f.Name == "/plan.json" {
				found = true
				plan, err := io.ReadAll(tr)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(plan), "internal.print") {
					t.Error("expected plan.json to contain reference to internal.print built-in function")
				}
			}
		}

		if !found {
			t.Error("plan.json not found")
		}
	})
}

func TestBuildPlanWithRegoEntrypointAnnotations(t *testing.T) {

	tests := []struct {
		note         string
		files        map[string]string
		err          error
		v0Compatible bool
	}{
		{
			note: "annotated entrypoint",
			files: map[string]string{
				"test.rego": `
# METADATA
# entrypoint: true
package test

p contains 1

f(x) if { p[x] }
		`,
			},
			err: nil,
		},
		{
			note:         "set generation with annotated entrypoint (v0)",
			v0Compatible: true,
			files: map[string]string{
				"test.rego": `
package test

# METADATA
# entrypoint: true
p[x] {
	{"a", "b"}[x]
}
		`,
			},
			err: nil,
		},
		{
			note: "set generation with annotated entrypoint (contains if)",
			files: map[string]string{
				"test.rego": `
package test

# METADATA
# entrypoint: true
p contains x if {
	{"a", "b"}[x]
}
		`,
			},
			err: nil,
		},
		{
			note:         "object generation with annotated entrypoint (v0)",
			v0Compatible: true,
			files: map[string]string{
				"test.rego": `
package test

# METADATA
# entrypoint: true
p[i] := x {
	x := ["a", "b"][i]
}
		`,
			},
			err: nil,
		},
		{
			note: "object generation with annotated entrypoint (if)",
			files: map[string]string{
				"test.rego": `
package test

# METADATA
# entrypoint: true
p[i] if {
	{"a", "b"}[i]
}
		`,
			},
			err: nil,
		},
		{
			note: "dots in head with annotated entrypoint",
			files: map[string]string{
				"test.rego": `
package test

# METADATA
# entrypoint: true
p.a.b if {
	true
}
		`,
			},
			err: nil,
		},
		{
			note: "dots in head object generation with annotated entrypoint",
			files: map[string]string{
				"test.rego": `
package test

# METADATA
# entrypoint: true
p.a.b[i] := x if {
	x := ["a", "b"][i]
}
		`,
			},
			err: nil,
		},
		{
			note: "no annotated entrypoint",
			files: map[string]string{
				"test.rego": `
package test

p contains 1

f(x) if { p[x] }
`,
			},
			err: errors.New("plan compilation requires at least one entrypoint"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(tc.files, func(root string) {
				params := newBuildParams()
				if err := params.target.Set("plan"); err != nil {
					t.Fatal(err)
				}
				params.pruneUnused = true
				params.outputFile = path.Join(root, "bundle.tar.gz")
				params.v0Compatible = tc.v0Compatible

				// Build should fail if entrypoint is not discovered from annotations.
				err := dobuild(params, []string{root})
				if err != nil {
					if tc.err == nil || tc.err.Error() != err.Error() {
						t.Fatal(err)
					}
					return // Bail out if this was an expected test failure.
				}

				// Attempt to load up the built bundle.
				_, err = loader.NewFileLoader().AsBundle(params.outputFile)
				if err != nil {
					t.Fatal(err)
				}
			})
		})
	}
}

func TestBuildWasmWithAnnotations(t *testing.T) {
	tests := []struct {
		note        string
		files       map[string]string
		entrypoints []string
		manifest    string
	}{
		{
			note: "last rule is annotated entrypoint",
			files: map[string]string{
				"test.rego": `
package test

# METADATA
# title: P1
p1 := 1

# METADATA
# title: P2
# entrypoint: true
p2 := 2
`,
			},
			manifest: `
{
	"revision":"",
	"rego_version": %REGO_VERSION%,
	"roots":[""],
	"wasm":[{
		"entrypoint":"test/p2",
		"module":"/policy.wasm",
		"annotations":[{
			"scope":"document",
			"title":"P2",
			"entrypoint":true
		}]
	}]
}
`,
		},
		{
			note: "last rule is (not annotated) entrypoint",
			files: map[string]string{
				"test.rego": `
package test

# METADATA
# title: P1
p1 := 1

# METADATA
# title: P2
p2 := 2
`,
			},
			entrypoints: []string{"test/p2"},
			manifest: `
{
	"revision":"",
	"rego_version": %REGO_VERSION%,
	"roots":[""],
	"wasm":[{
		"entrypoint":"test/p2",
		"module":"/policy.wasm",
		"annotations":[{
			"scope":"rule",
			"title":"P2"
		}]
	}]
}
`,
		},
		{
			note: "rules in multiple files are entrypoints",
			files: map[string]string{
				"test1.rego": `
package test

# METADATA
# title: P1
p1 := 1

# METADATA
# title: P2
# entrypoint: true
p2 := 2
`,
				"test2.rego": `
package test

# METADATA
# title: P3
p3 := 3

# METADATA
# title: P4
p4 := 4
`,
				"test3.rego": `
package test.foo

# METADATA
# title: BAR
# entrypoint: true
bar := "baz"
`,
			},
			entrypoints: []string{"test/p3"},
			manifest: `
{
	"revision":"",
	"rego_version": %REGO_VERSION%,
	"roots":[""],
	"wasm":[{
		"entrypoint":"test/p3",
		"annotations":[{"scope":"rule","title":"P3"}],
		"module":"/policy.wasm"
	},{
		"entrypoint":"test/foo/bar",
		"module":"/policy.wasm",
		"annotations":[{
			"scope":"document",
			"title":"BAR",
			"entrypoint":true
		}]
	},{
		"entrypoint":"test/p2",
		"module":"/policy.wasm",
		"annotations":[{
			"scope":"document",
			"title":"P2",
			"entrypoint":true
		}]
	}]
}
`,
		},
		{
			note: "rule with multiple metadata blocks",
			files: map[string]string{
				"test.rego": `
package test

# METADATA
# title: P doc
# scope: document
# entrypoint: true

# METADATA
# title: P
p := 1
`,
			},
			manifest: `
{
	"revision":"",
	"rego_version": %REGO_VERSION%,
	"roots":[""],
	"wasm":[{
		"entrypoint":"test/p",
		"module":"/policy.wasm",
		"annotations":[{
			"scope":"document",
			"title":"P doc",
			"entrypoint":true
		},{
			"scope":"rule",
			"title":"P"
		}]
	}]
}
`,
		},

		// Package annotations are not injected into manifest, as package definition is always retained in Rego source.
		{
			note: "package is annotated entrypoint",
			files: map[string]string{
				"test.rego": `
# METADATA
# title: PKG
# entrypoint: true
package test

# METADATA
# title: P1
p1 := 1

# METADATA
# title: P2
p2 := 2
`,
			},
			manifest: `
{
	"revision":"",
	"rego_version": %REGO_VERSION%,
	"roots":[""],
	"wasm":[{
		"entrypoint":"test",
		"module":"/policy.wasm"
	}]
}
`,
		},
		{
			note: "package is (not annotated) entrypoint",
			files: map[string]string{
				"test.rego": `
package test

# METADATA
# title: P1
p1 := 1

# METADATA
# title: P2
p2 := 2
`,
			},
			entrypoints: []string{"test"},
			manifest: `
{
	"revision":"",
	"rego_version": %REGO_VERSION%,
	"roots":[""],
	"wasm":[{
		"entrypoint":"test",
		"module":"/policy.wasm"
	}]
}
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(tc.files, func(root string) {
				params := newBuildParams()
				if err := params.target.Set("wasm"); err != nil {
					t.Fatal(err)
				}
				params.pruneUnused = true
				params.outputFile = path.Join(root, "bundle.tar.gz")
				params.entrypoints.v = tc.entrypoints

				// Build should fail if entrypoint is not discovered from annotations.
				err := dobuild(params, []string{root})
				if err != nil {
					t.Fatal(err)
				}

				_, err = loader.NewFileLoader().AsBundle(params.outputFile)
				if err != nil {
					t.Fatal(err)
				}

				// Check that manifest has expected content
				f, err := os.Open(params.outputFile)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()

				gr, err := gzip.NewReader(f)
				if err != nil {
					t.Fatal(err)
				}

				tr := tar.NewReader(gr)

				expManifest := strings.ReplaceAll(tc.manifest, "%REGO_VERSION%",
					strconv.Itoa(ast.DefaultRegoVersion.Int()))

				found := false
				for {
					f, err := tr.Next()
					if err == io.EOF {
						break
					} else if err != nil {
						t.Fatal(err)
					}
					if f.Name == "/.manifest" {
						found = true
						data, err := io.ReadAll(tr)
						if err != nil {
							t.Fatal(err)
						}
						manifest := util.MustUnmarshalJSON(data)
						if !reflect.DeepEqual(manifest, util.MustUnmarshalJSON([]byte(expManifest))) {
							t.Fatalf("expected manifest\n\n%v\n\nbut got\n\n%v", expManifest, string(util.MustMarshalJSON(manifest)))
						}
						break
					}
				}

				if !found {
					t.Fatal("no manifest found in bundle")
				}
			})
		})
	}
}

func TestBuildBundleModeIgnoreFlag(t *testing.T) {

	files := map[string]string{
		"/a/b/d/data.json":                              `{"e": "f"}`,
		"/policy.rego":                                  "package foo\n p = 1",
		"/policy_test.rego":                             "package foo\n test_p { p }",
		"/roles/policy.rego":                            "package bar\n p = 1",
		"/roles/policy_test.rego":                       "package bar\n test_p { p }",
		"/deeper/dir/path/than/others/policy.rego":      "package baz\n p = 1",
		"/deeper/dir/path/than/others/policy_test.rego": "package baz\n test_p { p }",
	}

	test.WithTempFS(files, func(root string) {
		params := newBuildParams()
		params.outputFile = path.Join(root, "bundle.tar.gz")
		params.bundleMode = true
		params.ignore = []string{"*_test.rego"}

		err := dobuild(params, []string{root})
		if err != nil {
			t.Fatal(err)
		}

		_, err = loader.NewFileLoader().AsBundle(params.outputFile)
		if err != nil {
			t.Fatal(err)
		}

		// Check that test files are not included in the output bundle
		f, err := os.Open(params.outputFile)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		gr, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}

		tr := tar.NewReader(gr)

		files := []string{}

		for {
			f, err := tr.Next()
			if err == io.EOF {
				break
			} else if err != nil {
				t.Fatal(err)
			}

			files = append(files, filepath.Base(f.Name))
		}

		// We additionally expect a manifest file
		expected := 5
		if len(files) != expected {
			t.Fatalf("expected %v files but got %v", expected, len(files))
		}
	})
}

func TestBuildBundleModeWithManifestRegoVersion(t *testing.T) {
	tests := []struct {
		note         string
		roots        []string
		files        map[string]string
		expManifest  string
		expErrs      []string
		v0Compatible bool
		v1Compatible bool
		capabilities *ast.Capabilities
	}{
		{
			note: "v0 bundle rego-version",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"test.rego": `package test

p[42] {
	input.x == 1
}`,
			},
			expManifest: `{"revision":"","roots":[""],"rego_version":0}`,
		},
		{
			note: "v1 bundle rego-version",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"test.rego": `package test

p contains 42 if {
	input.x == 1
}`,
			},
			expManifest: `{"revision":"","roots":[""],"rego_version":1}`,
		},
		{
			note: "v0 bundle rego-version, v1 per-file override",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"*/test2.rego": 1
	}
}`,
				"test1.rego": `package test

p[1] {
	input.x == 1
}`,
				"test2.rego": `package test

p contains 2 if {
	input.x == 1
}`,
			},
			expManifest: `{"revision":"","roots":[""],"rego_version":0,"file_rego_versions":{"%ROOT%/test2.rego":1}}`,
		},
		{
			note: "v0 bundle rego-version, v1 per-file override, missing v1 keywords in v1 file",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"*/test2.rego": 1
	}
}`,
				"test1.rego": `package test

p[1] {
	input.x == 1
}`,
				"test2.rego": `package test

p[2] {
	input.x == 1
}`,
			},
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "v0 bundle rego-version, v1 per-file override, v1 keywords but no v1 imports in v0 file",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"*/test2.rego": 1
	}
}`,
				"test1.rego": `package test

p contains 1 if {
	input.x == 1
}`,
				"test2.rego": `package test

p contains 2 if {
	input.x == 1
}`,
			},
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: number cannot be used for rule name",
			},
		},
		{
			note:         "multiple bundles with different rego-versions, v0-compatible",
			v0Compatible: true,
			roots:        []string{"bundle1", "bundle2"},
			files: map[string]string{
				"bundle1/.manifest": `{
	"roots": ["test1"],
	"rego_version": 0,
	"file_rego_versions": {
		"*/test2.rego": 1
	}
}`,
				"bundle1/test1.rego": `package test1
p[1] {
	input.x == 1
}`,
				"bundle1/test2.rego": `package test1
p contains 2 if {
	input.x == 1
}`,
				"bundle2/.manifest": `{
	"roots": ["test2"],
	"rego_version": 1,
	"file_rego_versions": {
		"*/test4.rego": 0
	}
}`,
				"bundle2/test3.rego": `package test2
p contains 3 if {
	input.x == 1
}`,
				"bundle2/test4.rego": `package test2
p[4] {
	input.x == 1
}`,
			},
			expManifest: `{"revision":"","roots":["test1","test2"],"rego_version":0,"file_rego_versions":{"%ROOT%/bundle1/test2.rego":1,"%ROOT%/bundle2/test3.rego":1}}`,
		},
		{
			note:         "multiple bundles with different rego-versions, v0-compatible, no rego_v1 capabilities feature",
			v0Compatible: true,
			roots:        []string{"bundle1", "bundle2"},
			files: map[string]string{
				"bundle1/.manifest": `{
	"roots": ["test1"],
	"rego_version": 0,
	"file_rego_versions": {
		"*/test2.rego": 1
	}
}`,
				"bundle1/test1.rego": `package test1
p[1] {
	input.x == 1
}`,
				"bundle1/test2.rego": `package test1
p contains 2 if {
	input.x == 1
}`,
				"bundle2/.manifest": `{
	"roots": ["test2"],
	"rego_version": 1,
	"file_rego_versions": {
		"*/test4.rego": 0
	}
}`,
				"bundle2/test3.rego": `package test2
p contains 3 if {
	input.x == 1
}`,
				"bundle2/test4.rego": `package test2
p[4] {
	input.x == 1
}`,
			},
			capabilities: capsWithoutFeat(ast.RegoV0, ast.FeatureRegoV1),
			expErrs: []string{
				// capabilities doesn't include rego_v1 feature, which must be respected
				"rego_parse_error: illegal capabilities: rego_v1 feature required for parsing v1 Rego",
			},
		},
		{
			note:         "multiple bundles with different rego-versions, v0-compatible, rego_v1 capabilities feature",
			v0Compatible: true,
			roots:        []string{"bundle1", "bundle2"},
			files: map[string]string{
				"bundle1/.manifest": `{
	"roots": ["test1"],
	"rego_version": 0,
	"file_rego_versions": {
		"*/test2.rego": 1
	}
}`,
				"bundle1/test1.rego": `package test1
p[1] {
	input.x == 1
}`,
				"bundle1/test2.rego": `package test1
p contains 2 if {
	input.x == 1
}`,
				"bundle2/.manifest": `{
	"roots": ["test2"],
	"rego_version": 1,
	"file_rego_versions": {
		"*/test4.rego": 0
	}
}`,
				"bundle2/test3.rego": `package test2
p contains 3 if {
	input.x == 1
}`,
				"bundle2/test4.rego": `package test2
p[4] {
	input.x == 1
}`,
				"capabilities.json": func() string {
					caps := ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0))
					caps.Features = append(caps.Features, ast.FeatureRegoV1)
					bs, err := json.Marshal(caps)
					if err != nil {
						t.Fatal(err)
					}
					return string(bs)
				}(),
			},
			expManifest: `{"revision":"","roots":["test1","test2"],"rego_version":0,"file_rego_versions":{"%ROOT%/bundle1/test2.rego":1,"%ROOT%/bundle2/test3.rego":1}}`,
		},
		{
			note:         "multiple bundles with different rego-versions, v1-compatible",
			v1Compatible: true,
			roots:        []string{"bundle1", "bundle2"},
			files: map[string]string{
				"bundle1/.manifest": `{
	"roots": ["test1"],
	"rego_version": 0,
	"file_rego_versions": {
		"*/test2.rego": 1
	}
}`,
				"bundle1/test1.rego": `package test1
p[1] {
	input.x == 1
}`,
				"bundle1/test2.rego": `package test1
p contains 2 if {
	input.x == 1
}`,
				"bundle2/.manifest": `{
	"roots": ["test2"],
	"rego_version": 1,
	"file_rego_versions": {
		"*/test4.rego": 0
	}
}`,
				"bundle2/test3.rego": `package test2
p contains 3 if {
	input.x == 1
}`,
				"bundle2/test4.rego": `package test2
p[4] {
	input.x == 1
}`,
			},
			expManifest: `{"revision":"","roots":["test1","test2"],"rego_version":1,"file_rego_versions":{"%ROOT%/bundle1/test1.rego":0,"%ROOT%/bundle2/test4.rego":0}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(tc.files, func(root string) {
				params := newBuildParams()
				params.outputFile = path.Join(root, "bundle.tar.gz")
				params.bundleMode = true
				params.v0Compatible = tc.v0Compatible
				params.v1Compatible = tc.v1Compatible

				if tc.capabilities != nil {
					params.capabilities = newCapabilitiesFlag()
					params.capabilities.C = tc.capabilities
				}

				if _, ok := tc.files["capabilities.json"]; ok {
					_ = params.capabilities.Set(path.Join(root, "capabilities.json"))
				}

				var roots []string
				if len(tc.roots) == 0 {
					roots = []string{root}
				} else {
					for _, r := range tc.roots {
						roots = append(roots, path.Join(root, r))
					}
				}
				err := dobuild(params, roots)
				if tc.expErrs != nil {
					if err == nil {
						t.Fatal("expected error but got none")
					}
					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("expected error:\n\n%q\n\nbut got:\n\n%v", expErr, err)
						}
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}

					_, err = loader.NewFileLoader().AsBundle(params.outputFile)
					if err != nil {
						t.Fatal(err)
					}

					f, err := os.Open(params.outputFile)
					if err != nil {
						t.Fatal(err)
					}
					defer func() {
						_ = f.Close()
					}()

					gr, err := gzip.NewReader(f)
					if err != nil {
						t.Fatal(err)
					}

					tr := tar.NewReader(gr)

					for {
						f, err := tr.Next()
						if err == io.EOF {
							break
						} else if err != nil {
							t.Fatal(err)
						}

						if f.Name == "/.manifest" {
							b, err := io.ReadAll(tr)
							if err != nil {
								t.Fatal(err)
							}
							expManifest := strings.ReplaceAll(tc.expManifest, "%ROOT%", root)
							if !strings.Contains(string(b), expManifest) {
								t.Fatalf("expected manifest:\n\n%v\n\nbut got:\n\n%v", expManifest, string(b))
							}
						}
					}
				}
			})
		})
	}
}

func capsWithoutFeat(regoVersion ast.RegoVersion, feat ...string) *ast.Capabilities {
	caps := ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(regoVersion))

	feats := make([]string, 0, len(caps.Features))
	for _, f := range caps.Features {
		skip := slices.Contains(feat, f)
		if !skip {
			feats = append(feats, f)
		}
	}
	caps.Features = feats

	return caps
}

func TestBuildBundleFromOtherBundles(t *testing.T) {
	type bundleInfo map[string]string

	tests := []struct {
		note         string
		v0Compatible bool
		v1Compatible bool
		capabilities *ast.Capabilities
		bundles      map[string]bundleInfo
		expBundle    bundleInfo
		expErrs      []string
	}{
		{
			note: "single bundle",
			bundles: map[string]bundleInfo{
				"bundle.tar.gz": {
					"policy.rego": `package test

p := input.x == 1
`,
				},
			},
			expBundle: bundleInfo{
				"/data.json": `{}
`,
				"/.manifest": `{"revision":"","roots":[""],"rego_version":%DEFAULT_REGO_VERSION%}
`,
				"%ROOT%/bundle.tar.gz/policy.rego": `package test

p := input.x == 1
`,
			},
		},
		{
			note:         "single bundle, --v1-compatible",
			v1Compatible: true,
			bundles: map[string]bundleInfo{
				"bundle.tar.gz": {
					"policy.rego": `package test
p if {
	input.x == 1
}`,
				},
			},
			expBundle: bundleInfo{
				"/data.json": `{}
`,
				"/.manifest": `{"revision":"","roots":[""],"rego_version":1}
`,
				"%ROOT%/bundle.tar.gz/policy.rego": `package test

p if {
	input.x == 1
}
`,
			},
		},
		{
			note: "single v0 bundle",
			bundles: map[string]bundleInfo{
				"bundle.tar.gz": {
					".manifest": `{"rego_version": 0}`,
					"policy.rego": `package test
p {
	input.x == 1
}`,
				},
			},
			expBundle: bundleInfo{
				"/data.json": `{}
`,
				"/.manifest": `{"revision":"","roots":[""],"rego_version":0}
`,
				"%ROOT%/bundle.tar.gz/policy.rego": `package test

p {
	input.x == 1
}
`,
			},
		},
		{
			note:         "single v0 bundle, --v1-compatible",
			v1Compatible: true,
			bundles: map[string]bundleInfo{
				"bundle.tar.gz": {
					".manifest": `{"rego_version": 0}`,
					"policy.rego": `package test
p {
	input.x == 1
}`,
				},
			},
			// We don't expect parse/compile errors, as the bundle rego-version is 0, which overrides the --v1-compatible flag.
			expBundle: bundleInfo{
				"/data.json": `{}
`,
				"/.manifest": `{"revision":"","roots":[""],"rego_version":0}
`,
				"%ROOT%/bundle.tar.gz/policy.rego": `package test

p {
	input.x == 1
}
`,
			},
		},
		{
			note:         "single v1 bundle, --v0-compatible",
			v1Compatible: true,
			bundles: map[string]bundleInfo{
				"bundle.tar.gz": {
					".manifest": `{"rego_version": 1}`,
					"policy.rego": `package test
p if {
	input.x == 1
}`,
				},
			},
			// We don't expect parse/compile errors, as the bundle rego-version is 0, which overrides the --v1-compatible flag.
			expBundle: bundleInfo{
				"/data.json": `{}
`,
				"/.manifest": `{"revision":"","roots":[""],"rego_version":1}
`,
				"%ROOT%/bundle.tar.gz/policy.rego": `package test

p if {
	input.x == 1
}
`,
			},
		},
		{
			note: "single v0 bundle, v1 per-file override",
			bundles: map[string]bundleInfo{
				"bundle.tar.gz": {
					".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"/policy_1.rego": 1
	}
}`,
					"policy_0.rego": `package test
p {
	input.x == 1
}`,
					"policy_1.rego": `package test
q contains 1 if {
	input.x == 1
}`,
				},
			},
			expBundle: bundleInfo{
				"/data.json": `{}
`,
				"/.manifest": `{"revision":"","roots":[""],"rego_version":0,"file_rego_versions":{"%ROOT%/bundle.tar.gz/policy_1.rego":1}}
`,
				"%ROOT%/bundle.tar.gz/policy_0.rego": `package test

p {
	input.x == 1
}
`,
				"%ROOT%/bundle.tar.gz/policy_1.rego": `package test

q contains 1 if {
	input.x == 1
}
`,
			},
		},
		{
			note:         "single v0 bundle, v1 per-file override, --v1-compatible",
			v1Compatible: true,
			bundles: map[string]bundleInfo{
				"bundle.tar.gz": {
					".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"/policy_1.rego": 1
	}
}`,
					"policy_0.rego": `package test
p {
	input.x == 1
}`,
					"policy_1.rego": `package test
q contains 1 if {
	input.x == 1
}`,
				},
			},
			expBundle: bundleInfo{
				"/data.json": `{}
`,
				"/.manifest": `{"revision":"","roots":[""],"rego_version":0,"file_rego_versions":{"%ROOT%/bundle.tar.gz/policy_1.rego":1}}
`,
				"%ROOT%/bundle.tar.gz/policy_0.rego": `package test

p {
	input.x == 1
}
`,
				"%ROOT%/bundle.tar.gz/policy_1.rego": `package test

q contains 1 if {
	input.x == 1
}
`,
			},
		},
		{
			note: "single v1 bundle, v0 per-file override",
			bundles: map[string]bundleInfo{
				"bundle.tar.gz": {
					".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"/policy_0.rego": 0
	}
}`,
					"policy_0.rego": `package test
p {
	input.x == 1
}`,
					"policy_1.rego": `package test
q contains 1 if {
	input.x == 1
}`,
				},
			},
			expBundle: bundleInfo{
				"/data.json": `{}
`,
				"/.manifest": `{"revision":"","roots":[""],"rego_version":1,"file_rego_versions":{"%ROOT%/bundle.tar.gz/policy_0.rego":0}}
`,
				"%ROOT%/bundle.tar.gz/policy_0.rego": `package test

p {
	input.x == 1
}
`,
				"%ROOT%/bundle.tar.gz/policy_1.rego": `package test

q contains 1 if {
	input.x == 1
}
`,
			},
		},
		{
			note:         "single v1 bundle, v0 per-file override, --v0-compatible",
			v0Compatible: true,
			bundles: map[string]bundleInfo{
				"bundle.tar.gz": {
					".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"/policy_0.rego": 0
	}
}`,
					"policy_0.rego": `package test
p {
	input.x == 1
}`,
					"policy_1.rego": `package test
q contains 1 if {
	input.x == 1
}`,
				},
			},
			expBundle: bundleInfo{
				"/data.json": `{}
`,
				"/.manifest": `{"revision":"","roots":[""],"rego_version":1,"file_rego_versions":{"%ROOT%/bundle.tar.gz/policy_0.rego":0}}
`,
				"%ROOT%/bundle.tar.gz/policy_0.rego": `package test

p {
	input.x == 1
}
`,
				"%ROOT%/bundle.tar.gz/policy_1.rego": `package test

q contains 1 if {
	input.x == 1
}
`,
			},
		},
		{
			note:         "single v1 bundle, v0 per-file override, --v0-compatible, no rego_v1 capabilities feature",
			v0Compatible: true,
			capabilities: capsWithoutFeat(ast.RegoV0, ast.FeatureRegoV1),
			bundles: map[string]bundleInfo{
				"bundle.tar.gz": {
					".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"/policy_0.rego": 0
	}
}`,
					"policy_0.rego": `package test
p {
	input.x == 1
}`,
					"policy_1.rego": `package test
q contains 1 if {
	input.x == 1
}`,
				},
			},
			expErrs: []string{
				// capabilities doesn't include rego_v1 feature, which must be respected
				"rego_parse_error: illegal capabilities: rego_v1 feature required for parsing v1 Rego",
			},
		},
		{
			note:         "v0 bundle + v1 bundle, --v0-compatible",
			v0Compatible: true,
			bundles: map[string]bundleInfo{
				"bundle_v0.tar.gz": {
					".manifest": `{"roots": ["test1"], "rego_version": 0}`,
					"policy.rego": `package test1
p {
	input.x == 1
}`,
				},
				"bundle_v1.tar.gz": {
					".manifest": `{"roots": ["test2"], "rego_version": 1}`,
					"policy.rego": `package test2
q contains 1 if {
	input.x == 1
}`,
				},
			},
			expBundle: bundleInfo{
				"/data.json": `{}
`,
				"/.manifest": `{"revision":"","roots":["test1","test2"],"rego_version":0,"file_rego_versions":{"%ROOT%/bundle_v1.tar.gz/policy.rego":1}}
`,
				"%ROOT%/bundle_v0.tar.gz/policy.rego": `package test1

p {
	input.x == 1
}
`,
				"%ROOT%/bundle_v1.tar.gz/policy.rego": `package test2

q contains 1 if {
	input.x == 1
}
`,
			},
		},
		{
			note:         "v0 bundle + v1 bundle, --v0-compatible, no rego_v1 capabilities feature",
			v0Compatible: true,
			capabilities: capsWithoutFeat(ast.RegoV0, ast.FeatureRegoV1),
			bundles: map[string]bundleInfo{
				"bundle_v0.tar.gz": {
					".manifest": `{"roots": ["test1"], "rego_version": 0}`,
					"policy.rego": `package test1
p {
	input.x == 1
}`,
				},
				"bundle_v1.tar.gz": {
					".manifest": `{"roots": ["test2"], "rego_version": 1}`,
					"policy.rego": `package test2
q contains 1 if {
	input.x == 1
}`,
				},
			},
			expErrs: []string{
				// capabilities inferred from --v0-compatible doesn't include rego_v1 feature, which must be respected
				"rego_parse_error: illegal capabilities: rego_v1 feature required for parsing v1 Rego",
			},
		},
		{
			note:         "v0 bundle + v1 bundle, --v1-compatible",
			v1Compatible: true,
			bundles: map[string]bundleInfo{
				"bundle_v0.tar.gz": {
					".manifest": `{"roots": ["test1"], "rego_version": 0}`,
					"policy.rego": `package test1
p {
	input.x == 1
}`,
				},
				"bundle_v1.tar.gz": {
					".manifest": `{"roots": ["test2"], "rego_version": 1}`,
					"policy.rego": `package test2
q contains 1 if {
	input.x == 1
}`,
				},
			},
			expBundle: bundleInfo{
				"/data.json": `{}
`,
				// We get a v1 bundle with a v0 per-file override
				"/.manifest": `{"revision":"","roots":["test1","test2"],"rego_version":1,"file_rego_versions":{"%ROOT%/bundle_v0.tar.gz/policy.rego":0}}
`,
				"%ROOT%/bundle_v0.tar.gz/policy.rego": `package test1

p {
	input.x == 1
}
`,
				"%ROOT%/bundle_v1.tar.gz/policy.rego": `package test2

q contains 1 if {
	input.x == 1
}
`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(nil, func(root string) {
				var roots []string
				for name, files := range tc.bundles {
					p := filepath.Join(root, name)
					roots = append(roots, p)
					filePairs := make([][2]string, 0, len(files))
					for k, v := range files {
						filePairs = append(filePairs, [2]string{k, v})
					}
					buf := archive.MustWriteTarGz(filePairs)
					bf, err := os.Create(p)
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}
					_, err = bf.Write(buf.Bytes())
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}
				}

				params := newBuildParams()
				params.outputFile = path.Join(root, "bundle.tar.gz")
				params.bundleMode = true
				params.v0Compatible = tc.v0Compatible
				params.v1Compatible = tc.v1Compatible

				if tc.capabilities != nil {
					params.capabilities.C = tc.capabilities
				}

				err := dobuild(params, roots)
				if tc.expErrs != nil {
					if err == nil {
						t.Fatal("expected error but got none")
					}
					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("expected error:\n\n%q\n\nbut got:\n\n%v", expErr, err)
						}
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}

					_, err = loader.NewFileLoader().AsBundle(params.outputFile)
					if err != nil {
						t.Fatal(err)
					}

					f, err := os.Open(params.outputFile)
					if err != nil {
						t.Fatal(err)
					}
					defer func() {
						_ = f.Close()
					}()

					gr, err := gzip.NewReader(f)
					if err != nil {
						t.Fatal(err)
					}

					tr := tar.NewReader(gr)

					for {
						f, err := tr.Next()
						if err == io.EOF {
							break
						} else if err != nil {
							t.Fatal(err)
						}

						found := false
						for expName, expVal := range tc.expBundle {
							expName = strings.ReplaceAll(expName, "%ROOT%", root)
							if f.Name == expName {
								found = true
								b, err := io.ReadAll(tr)
								if err != nil {
									t.Fatal(err)
								}
								expVal = strings.ReplaceAll(expVal, "%ROOT%", root)
								expVal = strings.ReplaceAll(expVal, "%DEFAULT_REGO_VERSION%",
									strconv.Itoa(ast.DefaultRegoVersion.Int()))
								if string(b) != expVal {
									t.Fatalf("expected %v:\n\n%v\n\nbut got:\n\n%v", expName, expVal, string(b))
								}
								break
							}
						}
						if !found {
							t.Fatalf("unexpected file in bundle: %v", f.Name)
						}
					}
				}
			})
		})
	}
}

func TestBuild_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note     string
		files    map[string]string
		expFiles map[string]string
		expErrs  []string
	}{
		{
			note: "v0 module",
			files: map[string]string{
				"test.rego": `package test
				p[x] {
					x := 42
				}`,
			},
			expErrs: []string{
				"test.rego:2: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:2: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "v1 module",
			files: map[string]string{
				"test.rego": `package test
				
				p contains x if {
					x := 42
				}`,
			},
			expFiles: map[string]string{
				".manifest": `{"revision":"","roots":[""],"rego_version":1}
`,
				"test.rego": `package test

p contains x if {
	x := 42
}
`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(tc.files, func(root string) {
				params := newBuildParams()
				params.outputFile = path.Join(root, "bundle.tar.gz")

				err := dobuild(params, []string{root})

				if len(tc.expErrs) > 0 {
					if err == nil {
						t.Fatal("expected error but got nil")
					}
					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("expected error:\n\n%v\n\ngot:\n\n%v", expErr, err)
						}
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}

					fl := loader.NewFileLoader()
					_, err = fl.AsBundle(params.outputFile)
					if err != nil {
						t.Fatal(err)
					}

					// Check that manifest is not written given no input manifest and no other flags
					f, err := os.Open(params.outputFile)
					if err != nil {
						t.Fatal(err)
					}
					defer f.Close()

					gr, err := gzip.NewReader(f)
					if err != nil {
						t.Fatal(err)
					}

					tr := tar.NewReader(gr)

					foundFiles := map[string]struct{}{}
					for {
						f, err := tr.Next()
						if err == io.EOF {
							break
						} else if err != nil {
							t.Fatal(err)
						}
						foundFiles[path.Base(f.Name)] = struct{}{}
						expectedFile := tc.expFiles[path.Base(f.Name)]
						if expectedFile != "" {
							data, err := io.ReadAll(tr)
							if err != nil {
								t.Fatal(err)
							}
							actualFile := string(data)
							if actualFile != expectedFile {
								t.Fatalf("expected file %s to be:\n\n%v\n\ngot:\n\n%v", f.Name, expectedFile, actualFile)
							}
						}
					}

					for expectedFile := range tc.expFiles {
						if _, ok := foundFiles[expectedFile]; !ok {
							t.Fatalf("expected file %s not found in bundle, got: %v", expectedFile, foundFiles)
						}
					}
				}
			})
		})
	}
}

func TestBuildWithRegoV1Capability(t *testing.T) {
	tests := []struct {
		note         string
		v0Compatible bool
		capabilities *ast.Capabilities
		files        map[string]string
		expFiles     map[string]string
		expErrs      []string
	}{
		{
			note:         "v0 module, v0-compatible, no capabilities",
			v0Compatible: true,
			files: map[string]string{
				"test.rego": `package test
				p[x] {
					x := 42
				}`,
			},
			expFiles: map[string]string{
				".manifest": `{"revision":"","roots":[""],"rego_version":0}
`,
				"test.rego": `package test

p[x] {
	x := 42
}
`,
			},
		},
		{
			note:         "v0 module, v0-compatible, v0 capabilities",
			v0Compatible: true,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0)),
			files: map[string]string{
				"test.rego": `package test
				p[x] {
					x := 42
				}`,
			},
			expFiles: map[string]string{
				".manifest": `{"revision":"","roots":[""],"rego_version":0}
`,
				"test.rego": `package test

p[x] {
	x := 42
}
`,
			},
		},
		{
			note:         "v0 module, v0-compatible, v1 capabilities",
			v0Compatible: true,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV1)),
			files: map[string]string{
				"test.rego": `package test
				p[x] {
					x := 42
				}`,
			},
			expFiles: map[string]string{
				".manifest": `{"revision":"","roots":[""],"rego_version":0}
`,
				"test.rego": `package test

p[x] {
	x := 42
}
`,
			},
		},

		{
			note: "v0 module, not v0-compatible, no capabilities",
			files: map[string]string{
				"test.rego": `package test
				p[x] {
					x := 42
				}`,
			},
			expErrs: []string{
				"test.rego:2: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:2: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note:         "v0 module, not v0-compatible, v0 capabilities",
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0)),
			files: map[string]string{
				"test.rego": `package test
				p[x] {
					x := 42
				}`,
			},
			expErrs: []string{
				"test.rego:2: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:2: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note:         "v0 module, not v0-compatible, v0 capabilities without rego_v1 feature",
			capabilities: capsWithoutFeat(ast.RegoV0, ast.FeatureRegoV1),
			files: map[string]string{
				"test.rego": `package test
				p[x] {
					x := 42
				}`,
			},
			expErrs: []string{
				"rego_parse_error: illegal capabilities: rego_v1 feature required for parsing v1 Rego",
			},
		},
		{
			note:         "v0 module, not v0-compatible, v1 capabilities",
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV1)),
			files: map[string]string{
				"test.rego": `package test
				p[x] {
					x := 42
				}`,
			},
			expErrs: []string{
				"test.rego:2: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:2: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},

		{
			note:         "v1 module, v0-compatible, no capabilities",
			v0Compatible: true,
			files: map[string]string{
				"test.rego": `package test
				
				p contains x if {
					x := 42
				}`,
			},
			expErrs: []string{
				"test.rego:3: rego_parse_error: var cannot be used for rule name",
			},
		},
		{
			note:         "v1 module, v0-compatible, v0 capabilities",
			v0Compatible: true,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0)),
			files: map[string]string{
				"test.rego": `package test
				
				p contains x if {
					x := 42
				}`,
			},
			expErrs: []string{
				"test.rego:3: rego_parse_error: var cannot be used for rule name",
			},
		},
		{
			note:         "v1 module, v0-compatible, v1 capabilities",
			v0Compatible: true,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV1)),
			files: map[string]string{
				"test.rego": `package test
				
				p contains x if {
					x := 42
				}`,
			},
			expErrs: []string{
				"test.rego:3: rego_parse_error: var cannot be used for rule name",
			},
		},

		{
			note: "v1 module, not v0-compatible, no capabilities",
			files: map[string]string{
				"test.rego": `package test
				
				p contains x if {
					x := 42
				}`,
			},
			expFiles: map[string]string{
				".manifest": `{"revision":"","roots":[""],"rego_version":1}
`,
				"test.rego": `package test

p contains x if {
	x := 42
}
`,
			},
		},
		{
			note:         "v1 module, not v0-compatible, v0 capabilities",
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0)),
			files: map[string]string{
				"test.rego": `package test
				
				p contains x if {
					x := 42
				}`,
			},
		},
		{
			note:         "v1 module, not v0-compatible, v0 capabilities without rego_v1 feature",
			capabilities: capsWithoutFeat(ast.RegoV0, ast.FeatureRegoV1),
			files: map[string]string{
				"test.rego": `package test
				
				p contains x if {
					x := 42
				}`,
			},
			expErrs: []string{
				"rego_parse_error: illegal capabilities: rego_v1 feature required for parsing v1 Rego",
			},
		},
		{
			note:         "v1 module, not v0-compatible, v1 capabilities",
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV1)),
			files: map[string]string{
				"test.rego": `package test
				
				p contains x if {
					x := 42
				}`,
			},
			expFiles: map[string]string{
				".manifest": `{"revision":"","roots":[""],"rego_version":1}
`,
				"test.rego": `package test

p contains x if {
	x := 42
}
`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(tc.files, func(root string) {
				params := newBuildParams()
				params.outputFile = path.Join(root, "bundle.tar.gz")
				params.v0Compatible = tc.v0Compatible
				params.capabilities.C = tc.capabilities

				err := dobuild(params, []string{root})

				if len(tc.expErrs) > 0 {
					if err == nil {
						t.Fatal("expected error but got nil")
					}
					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("expected error:\n\n%v\n\ngot:\n\n%v", expErr, err)
						}
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}

					fl := loader.NewFileLoader()
					_, err = fl.AsBundle(params.outputFile)
					if err != nil {
						t.Fatal(err)
					}

					// Check that manifest is not written given no input manifest and no other flags
					f, err := os.Open(params.outputFile)
					if err != nil {
						t.Fatal(err)
					}
					defer f.Close()

					gr, err := gzip.NewReader(f)
					if err != nil {
						t.Fatal(err)
					}

					tr := tar.NewReader(gr)

					foundFiles := map[string]struct{}{}
					for {
						f, err := tr.Next()
						if err == io.EOF {
							break
						} else if err != nil {
							t.Fatal(err)
						}
						foundFiles[path.Base(f.Name)] = struct{}{}
						expectedFile := tc.expFiles[path.Base(f.Name)]
						if expectedFile != "" {
							data, err := io.ReadAll(tr)
							if err != nil {
								t.Fatal(err)
							}
							actualFile := string(data)
							if actualFile != expectedFile {
								t.Fatalf("expected file %s to be:\n\n%v\n\ngot:\n\n%v", f.Name, expectedFile, actualFile)
							}
						}
					}

					for expectedFile := range tc.expFiles {
						if _, ok := foundFiles[expectedFile]; !ok {
							t.Fatalf("expected file %s not found in bundle, got: %v", expectedFile, foundFiles)
						}
					}
				}
			})
		})
	}
}

func TestBuildWithCompatibleFlags(t *testing.T) {
	tests := []struct {
		note          string
		v0Compatible  bool
		v1Compatible  bool
		files         map[string]string
		expectedFiles map[string]string
		expectedErr   string
	}{
		{
			note:         "v0 compatibility: policy with no rego.v1 or future.keywords imports",
			v0Compatible: true,
			files: map[string]string{
				"test.rego": `package test
				allow if {
					1 < 2
				}`,
			},
			expectedErr: "rego_parse_error",
		},
		{
			note:         "v0 compatibility: policy with rego.v1 imports",
			v0Compatible: true,
			files: map[string]string{
				"test.rego": `package test
				import rego.v1
				allow if {
					1 < 2
				}`,
			},
			// Imports are preserved
			expectedFiles: map[string]string{
				".manifest": `{"revision":"","roots":[""],"rego_version":0}
`,
				"test.rego": `package test

import rego.v1

allow if {
	1 < 2
}
`,
			},
		},
		{
			note:         "v0 compatibility: policy with future.keywords imports",
			v0Compatible: true,
			files: map[string]string{
				"test.rego": `package test
				import future.keywords.if
				allow if {
					1 < 2
				}`,
			},
			// Imports are preserved
			expectedFiles: map[string]string{
				".manifest": `{"revision":"","roots":[""],"rego_version":0}
`,
				"test.rego": `package test

import future.keywords.if

allow if {
	1 < 2
}
`,
			},
		},
		{
			note:         "v1 compatibility: policy with no rego.v1 or future.keywords imports",
			v1Compatible: true,
			files: map[string]string{
				"test.rego": `package test
				allow if {
					1 < 2
				}`,
			},
			// Imports are not added in
			expectedFiles: map[string]string{
				".manifest": `{"revision":"","roots":[""],"rego_version":1}
`,
				"test.rego": `package test

allow if {
	1 < 2
}
`,
			},
		},
		{
			note:         "v1 compatibility: policy with rego.v1 import",
			v1Compatible: true,
			files: map[string]string{
				"test.rego": `package test
				import rego.v1
				allow if {
					1 < 2
				}`,
			},
			// the rego.v1 import is kept to maximize compatibility surface
			expectedFiles: map[string]string{
				".manifest": `{"revision":"","roots":[""],"rego_version":1}
`,
				"test.rego": `package test

import rego.v1

allow if {
	1 < 2
}
`,
			},
		},
		{
			note:         "v1 compatibility: policy with future.keywords import",
			v1Compatible: true,
			files: map[string]string{
				"test.rego": `package test
				import future.keywords.if
				allow if {
					1 < 2
				}`,
			},
			// future.keywords imports are kept to maximize compatibility surface
			expectedFiles: map[string]string{
				".manifest": `{"revision":"","roots":[""],"rego_version":1}
`,
				"test.rego": `package test

import future.keywords.if

allow if {
	1 < 2
}
`,
			},
		},
		{
			note:         "v1 compatibility: policy with rego.v1 and future.keywords imports",
			v1Compatible: true,
			files: map[string]string{
				"test.rego": `package test
				import rego.v1
				import future.keywords.if
				allow if {
					1 < 2
				}`,
			},
			// future.keywords are dropped as these are covered by rego.v1
			expectedFiles: map[string]string{
				".manifest": `{"revision":"","roots":[""],"rego_version":1}
`,
				"test.rego": `package test

import rego.v1

allow if {
	1 < 2
}
`,
			},
		},
		{
			note:         "v1 compatibility: missing keywords",
			v1Compatible: true,
			files: map[string]string{
				"test.rego": `package test
				allow[1] {
					1 < 2
				}`,
			},
			expectedErr: "rego_parse_error",
		},
		// v0 takes precedence over v1
		{
			note:         "v0+v1 compatibility: policy with no rego.v1 or future.keywords imports",
			v0Compatible: true,
			v1Compatible: true,
			files: map[string]string{
				"test.rego": `package test
				allow if {
					1 < 2
				}`,
			},
			expectedErr: "rego_parse_error",
		},
		{
			note:         "v0+v1 compatibility: policy with rego.v1 imports",
			v0Compatible: true,
			v1Compatible: true,
			files: map[string]string{
				"test.rego": `package test
				import rego.v1
				allow if {
					1 < 2
				}`,
			},
			// Imports are preserved
			expectedFiles: map[string]string{
				".manifest": `{"revision":"","roots":[""],"rego_version":0}
`,
				"test.rego": `package test

import rego.v1

allow if {
	1 < 2
}
`,
			},
		},
		{
			note:         "v0+v1 compatibility: policy with future.keywords imports",
			v0Compatible: true,
			v1Compatible: true,
			files: map[string]string{
				"test.rego": `package test
				import future.keywords.if
				allow if {
					1 < 2
				}`,
			},
			// Imports are preserved
			expectedFiles: map[string]string{
				".manifest": `{"revision":"","roots":[""],"rego_version":0}
`,
				"test.rego": `package test

import future.keywords.if

allow if {
	1 < 2
}
`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(tc.files, func(root string) {
				params := newBuildParams()
				params.outputFile = path.Join(root, "bundle.tar.gz")
				params.v0Compatible = tc.v0Compatible
				params.v1Compatible = tc.v1Compatible

				err := dobuild(params, []string{root})

				if tc.expectedErr != "" {
					if err == nil {
						t.Fatal("expected error but got nil")
					}
					if !strings.Contains(err.Error(), tc.expectedErr) {
						t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}

					fl := loader.NewFileLoader()
					if tc.v1Compatible {
						fl = fl.WithRegoVersion(ast.RegoV1)
					}
					_, err = fl.AsBundle(params.outputFile)
					if err != nil {
						t.Fatal(err)
					}

					// Check that manifest is not written given no input manifest and no other flags
					f, err := os.Open(params.outputFile)
					if err != nil {
						t.Fatal(err)
					}
					defer f.Close()

					gr, err := gzip.NewReader(f)
					if err != nil {
						t.Fatal(err)
					}

					tr := tar.NewReader(gr)

					foundFiles := map[string]struct{}{}
					for {
						f, err := tr.Next()
						if err == io.EOF {
							break
						} else if err != nil {
							t.Fatal(err)
						}
						foundFiles[path.Base(f.Name)] = struct{}{}
						expectedFile := tc.expectedFiles[path.Base(f.Name)]
						if expectedFile != "" {
							data, err := io.ReadAll(tr)
							if err != nil {
								t.Fatal(err)
							}
							actualFile := string(data)
							if actualFile != expectedFile {
								t.Fatalf("expected file %s to be:\n\n%v\n\ngot:\n\n%v", f.Name, expectedFile, actualFile)
							}
						}
					}

					for expectedFile := range tc.expectedFiles {
						if _, ok := foundFiles[expectedFile]; !ok {
							t.Fatalf("expected file %s not found in bundle, got: %v", expectedFile, foundFiles)
						}
					}
				}
			})
		})
	}
}

func TestBuildOptimizedLevel2(t *testing.T) {
	tests := []struct {
		note          string
		files         map[string]string
		expectedFiles map[string]string
	}{
		{
			note: "Parse package path correctly",
			files: map[string]string{
				"test.rego": `package demo["test-policy"]

# METADATA
# entrypoint: true
default allow := false

allow if {
	some value in input
}
`,
			},
			expectedFiles: map[string]string{
				"/optimized/demo/test-policy.rego": `package demo["test-policy"]

default allow := false

allow if __local2__1 = input[__local1__1]
`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(tc.files, func(root string) {
				params := newBuildParams()
				params.outputFile = path.Join(root, "bundle.tar.gz")
				params.optimizationLevel = 2

				err := dobuild(params, []string{root})

				if err != nil {
					t.Fatal(err)
				}

				f, err := os.Open(params.outputFile)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()

				gr, err := gzip.NewReader(f)
				if err != nil {
					t.Fatal(err)
				}

				tr := tar.NewReader(gr)

				foundFiles := map[string]struct{}{}
				for {
					f, err := tr.Next()
					if err == io.EOF {
						break
					} else if err != nil {
						t.Fatal(err)
					}
					foundFiles[f.Name] = struct{}{}
					expectedFile := tc.expectedFiles[f.Name]
					if expectedFile != "" {
						data, err := io.ReadAll(tr)
						if err != nil {
							t.Fatal(err)
						}
						actualFile := string(data)
						if actualFile != expectedFile {
							t.Fatalf("expected file %s to be:\n\n%v\n\ngot:\n\n%v", f.Name, expectedFile, actualFile)
						}
					}
				}

				for expectedFile := range tc.expectedFiles {
					if _, ok := foundFiles[expectedFile]; !ok {
						t.Fatalf("expected file %s not found in bundle, got: %v", expectedFile, foundFiles)
					}
				}
			})
		})
	}
}

func TestBuildOptimizedWithRegoVersion(t *testing.T) {
	tests := []struct {
		note                string
		v1Compatible        bool
		regoV1ImportCapable bool
		files               map[string]string
		expectedFiles       map[string]string
	}{
		{
			note:                "v0, no future keywords",
			v1Compatible:        false,
			regoV1ImportCapable: true,
			files: map[string]string{
				"test.rego": `package test
# METADATA
# entrypoint: true
p[v] {
	v := input.v
}
`,
			},
			expectedFiles: map[string]string{
				"/.manifest": `{"revision":"","roots":[""],"rego_version":0}
`,
				// rego.v1 import added to optimized support module
				"/optimized/test.rego": `package test

import rego.v1

p contains __local0__1 if {
	__local0__1 = input.v
}
`,
			},
		},
		{
			note:                "v0, No future keywords, not rego.v1 import capable",
			v1Compatible:        false,
			regoV1ImportCapable: false,
			files: map[string]string{
				"test.rego": `package test
# METADATA
# entrypoint: true
p[v] {
	v := input.v
}
`,
			},
			expectedFiles: map[string]string{
				"/.manifest": `{"revision":"","roots":[""],"rego_version":0}
`,
				// rego.v1 import NOT added to optimized support module
				"/optimized/test.rego": `package test

p[__local0__1] {
	__local0__1 = input.v
}
`,
			},
		},
		{
			note:                "v1, No imports",
			v1Compatible:        true,
			regoV1ImportCapable: true,
			files: map[string]string{
				"test.rego": `package test
# METADATA
# entrypoint: true
p[k] contains v if {
	k := "foo"
	v := input.v
}
`,
			},
			expectedFiles: map[string]string{
				"/.manifest": `{"revision":"","roots":[""],"rego_version":1}
`,
				"/optimized/test/p.rego": `package test.p

foo contains __local1__1 if {
	__local1__1 = input.v
}
`,
			},
		},
		{
			note:                "v1, rego.v1 imported",
			v1Compatible:        true,
			regoV1ImportCapable: true,
			files: map[string]string{
				"test.rego": `package test
import rego.v1
# METADATA
# entrypoint: true
p[k] contains v if {
	k := "foo"
	v := input.v
}
`,
			},
			// Note: the rego.v1 import isn't added to the optimized module.
			// This is ok, as the bundle was built with the --v1-compatible flag,
			// and is tagged with a rego-version to inform the consumer.
			expectedFiles: map[string]string{
				"/.manifest": `{"revision":"","roots":[""],"rego_version":1}
`,
				"/optimized/test/p.rego": `package test.p

foo contains __local1__1 if {
	__local1__1 = input.v
}
`,
			},
		},
		{
			note:                "v1, future.keywords imported",
			v1Compatible:        true,
			regoV1ImportCapable: true,
			files: map[string]string{
				"test.rego": `package test
import future.keywords
# METADATA
# entrypoint: true
p[k] contains v if {
	k := "foo"
	v := input.v
}
`,
			},
			expectedFiles: map[string]string{
				"/.manifest": `{"revision":"","roots":[""],"rego_version":1}
`,
				"/optimized/test/p.rego": `package test.p

foo contains __local1__1 if {
	__local1__1 = input.v
}
`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(tc.files, func(root string) {
				params := newBuildParams()
				params.outputFile = path.Join(root, "bundle.tar.gz")
				params.v0Compatible = !tc.v1Compatible
				params.v1Compatible = tc.v1Compatible
				params.optimizationLevel = 1

				if !tc.regoV1ImportCapable {
					caps := newCapabilitiesFlag()
					caps.C = ast.CapabilitiesForThisVersion()
					caps.C.Features = []string{
						ast.FeatureRefHeadStringPrefixes,
						ast.FeatureRefHeads,
					}
					params.capabilities = caps
				}

				err := dobuild(params, []string{root})

				if err != nil {
					t.Fatal(err)
				}

				f, err := os.Open(params.outputFile)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()

				gr, err := gzip.NewReader(f)
				if err != nil {
					t.Fatal(err)
				}

				tr := tar.NewReader(gr)

				foundFiles := map[string]struct{}{}
				for {
					f, err := tr.Next()
					if err == io.EOF {
						break
					} else if err != nil {
						t.Fatal(err)
					}
					foundFiles[f.Name] = struct{}{}
					expectedFile := tc.expectedFiles[f.Name]
					if expectedFile != "" {
						data, err := io.ReadAll(tr)
						if err != nil {
							t.Fatal(err)
						}
						actualFile := string(data)
						if actualFile != expectedFile {
							t.Fatalf("expected file %s to be:\n\n%v\n\ngot:\n\n%v", f.Name, expectedFile, actualFile)
						}
					}
				}

				for expectedFile := range tc.expectedFiles {
					if _, ok := foundFiles[expectedFile]; !ok {
						t.Fatalf("expected file %s not found in bundle, got: %v", expectedFile, foundFiles)
					}
				}
			})
		})
	}
}

// TestBuildWithFollowSymlinks tests that the build command follows symlinks when building a bundle.
// This test uses a local tmp filesystem to create a directory with a symlink to a file in it's root
// and a local file in the bundle directory, and verifies that the built bundle contains both the symlink
// and the regular file.
// There's probably some common utilities that could be extracted at some point but for now this code is
// local to the test until we need to reuse it elsewhere.
func TestBuildWithFollowSymlinks(t *testing.T) {
	rootDir := t.TempDir()
	bundleDir := path.Join(rootDir, "bundle")
	err := os.Mkdir(bundleDir, 0777)
	if err != nil {
		t.Fatal(err)
	}

	// create a regular file in our temp bundle directory
	err = os.WriteFile(filepath.Join(bundleDir, "foo.rego"), []byte("package foo\none = 1"), 0777)
	if err != nil {
		t.Fatal(err)
	}

	// create a regular file in the root directory of our tmp directory that we will symlink into the bundle directory later
	err = os.WriteFile(filepath.Join(rootDir, "bar.rego"), []byte("package foo\ntwo = 2"), 0777)
	if err != nil {
		t.Fatal(err)
	}

	// create a symlink in the bundle directory to the file in the root directory
	err = os.Symlink(filepath.Join(rootDir, "bar.rego"), filepath.Join(bundleDir, "bar.rego"))
	if err != nil {
		t.Fatal(err)
	}

	params := newBuildParams()
	params.outputFile = path.Join(rootDir, "test.tar.gz")
	params.bundleMode = true
	params.followSymlinks = true
	params.v1Compatible = true

	err = dobuild(params, []string{bundleDir})
	if err != nil {
		t.Fatal(err)
	}

	// verify that the bundle is a loadable bundle
	_, err = loader.NewFileLoader().AsBundle(params.outputFile)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(params.outputFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}

	tr := tar.NewReader(gr)

	// map of file name -> file content
	expectedFiles := map[string]string{
		bundleDir + "/foo.rego": "package foo\n\none := 1",
		bundleDir + "/bar.rego": "package foo\n\ntwo := 2",
		"/.manifest":            `{"revision":"","roots":[""],"rego_version":1}`,
		"/data.json":            "{}",
	}

	foundFiles := make(map[string]string, 4)
	for f, err := tr.Next(); err != io.EOF; f, err = tr.Next() {
		if err != nil {
			t.Fatal(err)
		}

		// ensure that all the files are regular files in the bundle
		// and that no symlinks were copied
		if mode := f.FileInfo().Mode(); !mode.IsRegular() {
			t.Fatalf("expected regular file for file %s but got %s", f.FileInfo().Name(), mode.String())
		}
		// read the file content
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("failed to read file %s: %v", f.FileInfo().Name(), err)
		}
		foundFiles[f.Name] = string(data)
	}

	if len(foundFiles) != 4 {
		t.Fatalf("expected four files in bundle but got %d", len(foundFiles))
	}

	for name, contents := range foundFiles {
		// trim added whitespace because it's annoying and makes the test less readable
		contents := strings.Trim(contents, "\n")
		// check that the file content matches the expected content
		expectedContent, ok := expectedFiles[name]
		if !ok {
			t.Fatalf("unexpected file %s in bundle", name)
		}

		if contents != expectedContent {
			t.Fatalf("expected file %s to contain:\n\n%v\n\ngot:\n\n%v", name, expectedContent, contents)
		}
	}
}

// TestBuildWithFollowSymlinksEntireDir tests that the build command can build a bundle from a symlinked directory.
// This test uses a local tmp filesystem to create a directory with a local file in the bundle directory, and
// verifies that the built bundle contains the files from the symlinked directory.
func TestBuildWithFollowSymlinksEntireDir(t *testing.T) {
	rootDir := t.TempDir()
	defer func() {
		if err := os.RemoveAll(rootDir); err != nil {
			t.Fatal(err)
		}
	}()
	bundleDir := path.Join(rootDir, "src")
	err := os.Mkdir(bundleDir, 0777)
	if err != nil {
		t.Fatal(err)
	}

	// create a regular file in our temp bundle directory
	err = os.WriteFile(filepath.Join(bundleDir, "foo.rego"), []byte("package foo\none = 1"), 0777)
	if err != nil {
		t.Fatal(err)
	}

	symlinkDir := path.Join(rootDir, "symlink")
	err = os.Mkdir(symlinkDir, 0777)
	if err != nil {
		t.Fatal(err)
	}

	// create a symlink in the symlink directory to the src directory
	err = os.Symlink(bundleDir, filepath.Join(symlinkDir, "linked"))
	if err != nil {
		t.Fatal(err)
	}

	params := newBuildParams()
	params.outputFile = path.Join(rootDir, "test.tar.gz")
	params.bundleMode = true
	params.followSymlinks = true
	params.v1Compatible = true

	err = dobuild(params, []string{symlinkDir + "/linked/"})
	if err != nil {
		t.Fatal(err)
	}

	// verify that the bundle is a loadable bundle
	_, err = loader.NewFileLoader().AsBundle(params.outputFile)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(params.outputFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}

	tr := tar.NewReader(gr)

	// map of file name -> file content
	expectedFiles := map[string]string{
		path.Join(symlinkDir, "linked", "foo.rego"): "package foo\n\none := 1",
		"/.manifest": `{"revision":"","roots":[""],"rego_version":1}`,
		"/data.json": "{}",
	}

	foundFiles := make(map[string]string, 3)
	for f, err := tr.Next(); err != io.EOF; f, err = tr.Next() {
		if err != nil {
			t.Fatal(err)
		}

		// ensure that all the files are regular files in the bundle
		// and that no symlinks were copied
		if mode := f.FileInfo().Mode(); !mode.IsRegular() {
			t.Fatalf("expected regular file for file %s but got %s", f.FileInfo().Name(), mode.String())
		}
		// read the file content
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("failed to read file %s: %v", f.FileInfo().Name(), err)
		}
		foundFiles[f.Name] = string(data)
	}

	if len(foundFiles) != 3 {
		t.Fatalf("expected three files in bundle but got %d", len(foundFiles))
	}

	for name, contents := range foundFiles {
		// trim added whitespace because it's annoying and makes the test less readable
		contents := strings.Trim(contents, "\n")
		// check that the file content matches the expected content
		expectedContent, ok := expectedFiles[name]
		if !ok {
			t.Fatalf("unexpected file %s in bundle", name)
		}

		if contents != expectedContent {
			t.Fatalf("expected file %s to contain:\n\n%v\n\ngot:\n\n%v", name, expectedContent, contents)
		}
	}
}

func TestBuildManifestWarning(t *testing.T) {
	testCases := map[string]struct {
		files          map[string]string
		bundleMode     bool
		buildArgs      []string
		expectedStderr func(root string) string
	}{
		"warns when manifest ignored": {
			files: map[string]string{
				"bundle/.manifest": `{"revision":"1.0.0","roots":["foo"]}`,
				"bundle/data.json": `{"data": "value"}`,
			},
			bundleMode: false,
			buildArgs:  []string{"bundle"},
			expectedStderr: func(root string) string {
				return fmt.Sprintf("Warning: .manifest file found in %q but -b flag not specified. Manifest will be ignored.\n", path.Join(root, "bundle"))
			},
		},
		"no warning when bundle mode enabled": {
			files: map[string]string{
				"bundle/.manifest":     `{"revision":"1.0.0","roots":["foo"]}`,
				"bundle/foo/data.json": `{"data": "value"}`,
			},
			bundleMode: true,
			buildArgs:  []string{"bundle"},
			expectedStderr: func(root string) string {
				return ""
			},
		},
		"no warning when no manifest exists": {
			files: map[string]string{
				"bundle/data.json": `{"data": "value"}`,
			},
			bundleMode: false,
			buildArgs:  []string{"bundle"},
			expectedStderr: func(root string) string {
				return ""
			},
		},
		"warns for multiple bundles with manifests": {
			files: map[string]string{
				"bundle1/.manifest":     `{"revision":"1.0.0","roots":["foo"]}`,
				"bundle1/foo/data.json": `{"data1": "value1"}`,
				"bundle2/.manifest":     `{"revision":"2.0.0","roots":["bar"]}`,
				"bundle2/bar/data.json": `{"data2": "value2"}`,
				"bundle3/baz/data.json": `{"data3": "value3"}`,
			},
			bundleMode: false,
			buildArgs:  []string{"bundle1", "bundle2", "bundle3"},
			expectedStderr: func(root string) string {
				return fmt.Sprintf(`Warning: .manifest file found in %q but -b flag not specified. Manifest will be ignored.
Warning: .manifest file found in %q but -b flag not specified. Manifest will be ignored.
`,
					path.Join(root, "bundle1"), path.Join(root, "bundle2"))
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var stderr bytes.Buffer
			test.WithTempFS(tc.files, func(root string) {
				params := newBuildParams()
				params.outputFile = path.Join(root, "output.tar.gz")
				params.bundleMode = tc.bundleMode
				params.stderr = &stderr

				args := make([]string, 0, len(tc.buildArgs))
				for _, arg := range tc.buildArgs {
					args = append(args, path.Join(root, arg))
				}

				err := dobuild(params, args)
				if err != nil {
					t.Fatal(err)
				}

				stderrOutput := stderr.String()
				expectedStderr := tc.expectedStderr(root)

				if stderrOutput != expectedStderr {
					t.Fatalf("Expected stderr:\n%q\nGot:\n%q", expectedStderr, stderrOutput)
				}
			})
		})
	}
}
