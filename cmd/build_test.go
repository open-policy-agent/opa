package cmd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/loader"

	"github.com/open-policy-agent/opa/util/test"
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
			if f.Name == "/data.json" || strings.HasSuffix(f.Name, "/test.rego") {
				continue
			}
			t.Fatal("unexpected file:", f.Name)
		}
	})
}

func TestBuildRespectsCapabilities(t *testing.T) {
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
	}

	// add same tests for bundle-mode == true:
	for i := range tests {
		tc := tests[i]
		tc.bundleMode = true
		tc.note = tc.note + " (as bundle)"
		tests = append(tests, tc)
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"capabilities.json": tc.caps,
				"test.rego":         tc.policy,
			}

			test.WithTempFS(files, func(root string) {
				caps := newcapabilitiesFlag()
				if err := caps.Set(path.Join(root, "capabilities.json")); err != nil {
					t.Fatal(err)
				}
				params := newBuildParams()
				params.outputFile = path.Join(root, "bundle.tar.gz")
				params.capabilities = caps
				params.bundleMode = tc.bundleMode

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
			p { p }
		`,
	}

	test.WithTempFS(files, func(root string) {
		params := newBuildParams()
		params.outputFile = path.Join(root, "bundle.tar.gz")

		err := dobuild(params, []string{root})
		exp := fmt.Sprintf("1 error occurred: %s/test.rego:3: rego_recursion_error: rule data.test.p is recursive: data.test.p -> data.test.p",
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
			p { p }
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
			
			p[1]
			
			f(x) { p[x] }
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
			case f.Name == "/data.json" || strings.HasSuffix(f.Name, "/test.rego"): // expected
			default:
				t.Errorf("unexpected file: %s", f.Name)
			}
		}
		if !found {
			t.Error("plan.json not found")
		}
	})
}

func TestBuildPlanWithRegoEntrypointAnnotations(t *testing.T) {

	tests := []struct {
		note  string
		files map[string]string
		err   error
	}{
		{
			note: "annotated entrypoint",
			files: map[string]string{
				"test.rego": `
# METADATA
# entrypoint: true
package test

p[1]

f(x) { p[x] }
		`,
			},
			err: nil,
		},
		{
			note: "no annotated entrypoint",
			files: map[string]string{
				"test.rego": `

package test

p[1]

f(x) { p[x] }
		`,
			},
			err: fmt.Errorf("plan compilation requires at least one entrypoint"),
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

		expected := 4
		if len(files) != expected {
			t.Fatalf("expected %v files but got %v", expected, len(files))
		}
	})
}
