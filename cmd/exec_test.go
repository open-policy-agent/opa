package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/cmd/internal/exec"
	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/v1/ast"
	loggingtest "github.com/open-policy-agent/opa/v1/logging/test"
	sdk_test "github.com/open-policy-agent/opa/v1/sdk/test"
	"github.com/open-policy-agent/opa/v1/util/test"
)

type execOutput struct {
	Result []execResultItem `json:"result"`
}

type execResultItem struct {
	DecisionID string              `json:"decision_id,omitempty"`
	Path       string              `json:"path"`
	Error      execResultItemError `json:"error,omitempty"`
	Result     *any                `json:"result,omitempty"`
}

type execResultItemError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (r execResultItemError) isEmpty() bool {
	return r.Code == "" && r.Message == ""
}

func toAnyPtr(a any) *any {
	return &a
}

func toStringSlice(a *any) []string {
	switch a := (*a).(type) {
	case []string:
		return a
	case []interface{}:
		strSlice := make([]string, len(a))
		for i := range a {
			strSlice[i] = a[i].(string)
		}
		return strSlice
	}

	return nil
}

func resultSliceEquals(t *testing.T, expected, output []execResultItem) {
	t.Helper()

	if len(expected) != len(output) {
		t.Fatalf("Expected %d results but got %d", len(expected), len(output))
	}

	for i := range output {
		if expected[i].Path != output[i].Path {
			t.Fatalf("Expected path %v but got %v", expected[i].Path, output[i].Path)
		}

		if expected[i].Error.isEmpty() {
			if !output[i].Error.isEmpty() {
				t.Fatalf("Expected no error but got %v", output[i].Error)
			}

			if !slices.Equal(toStringSlice(expected[i].Result), toStringSlice(output[i].Result)) {
				t.Fatalf("Expected result %v but got %v", expected[i].Result, output[i].Result)
			}

			if !uuidPattern.MatchString(output[i].DecisionID) {
				t.Fatalf("Expected decision ID to be a UUID but got %v", output[i].DecisionID)
			}
		} else {
			if expected[i].Error.Code != output[i].Error.Code {
				t.Fatalf("Expected error code %v but got %v", expected[i].Error.Code, output[i].Error.Code)
			}

			if expected[i].Error.Message != output[i].Error.Message {
				t.Fatalf("Expected error message %v but got %v", expected[i].Error.Message, output[i].Error.Message)
			}

			if output[i].DecisionID != "" {
				t.Fatalf("Expected no decision ID but got %v", output[i].DecisionID)
			}
		}
	}
}

var uuidPattern = regexp.MustCompile(`^[\da-f]{8}-[\da-f]{4}-[\da-f]{4}-[\da-f]{4}-[\da-f]{12}$`)

func TestExecBasic(t *testing.T) {

	files := map[string]string{
		"test.json":  `{"foo": 7}`,
		"test2.yaml": `bar: 8`,
		"test3.yml":  `baz: 9`,
		"ignore":     `garbage`, // do not recognize this filetype
	}

	test.WithTempFS(files, func(dir string) {

		s := sdk_test.MustNewServer(sdk_test.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"test.rego": `
				package system
				main contains "hello"
			`,
		}))

		defer s.Stop()

		var buf bytes.Buffer
		params := exec.NewParams(&buf)
		_ = params.OutputFormat.Set("json")
		params.ConfigOverrides = []string{
			"services.test.url=" + s.URL(),
			"bundles.test.resource=/bundles/bundle.tar.gz",
		}

		params.Paths = append(params.Paths, dir)
		err := runExec(params)
		if err != nil {
			t.Fatal(err)
		}

		var output execOutput
		if err := json.Unmarshal(bytes.ReplaceAll(buf.Bytes(), []byte(dir), nil), &output); err != nil {
			t.Fatal(err)
		}

		resultSliceEquals(t, []execResultItem{
			{
				Path:   "/test.json",
				Result: toAnyPtr([]string{"hello"}),
			},
			{
				Path:   "/test2.yaml",
				Result: toAnyPtr([]string{"hello"}),
			},
			{
				Path:   "/test3.yml",
				Result: toAnyPtr([]string{"hello"}),
			},
		}, output.Result)
	})
}

func TestExecDecisionOption(t *testing.T) {

	files := map[string]string{
		"test.json": `{"foo": 7}`,
	}

	test.WithTempFS(files, func(dir string) {

		s := sdk_test.MustNewServer(sdk_test.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"test.rego": `
				package foo
				
				main contains "hello"
			`,
		}))

		defer s.Stop()

		var buf bytes.Buffer
		params := exec.NewParams(&buf)
		_ = params.OutputFormat.Set("json")
		params.Decision = "foo/main"
		params.ConfigOverrides = []string{
			"services.test.url=" + s.URL(),
			"bundles.test.resource=/bundles/bundle.tar.gz",
		}

		params.Paths = append(params.Paths, dir)
		err := runExec(params)
		if err != nil {
			t.Fatal(err)
		}

		var output execOutput
		if err := json.Unmarshal(bytes.ReplaceAll(buf.Bytes(), []byte(dir), nil), &output); err != nil {
			t.Fatal(err)
		}

		resultSliceEquals(t, []execResultItem{
			{
				Path:   "/test.json",
				Result: toAnyPtr([]string{"hello"}),
			},
		}, output.Result)
	})

}

func TestExecBundleFlag(t *testing.T) {

	files := map[string]string{
		"files/test.json": `{"foo": 7}`,
		"bundle/x.rego": `package system
		
		main contains "hello"`,
	}

	test.WithTempFS(files, func(dir string) {

		var buf bytes.Buffer
		params := exec.NewParams(&buf)
		_ = params.OutputFormat.Set("json")
		params.BundlePaths = []string{dir + "/bundle/"}
		params.Paths = append(params.Paths, dir+"/files/")

		err := runExec(params)
		if err != nil {
			t.Fatal(err)
		}

		var output execOutput
		if err := json.Unmarshal(bytes.ReplaceAll(buf.Bytes(), []byte(dir), nil), &output); err != nil {
			t.Fatal(err)
		}

		resultSliceEquals(t, []execResultItem{
			{
				Path:   "/files/test.json",
				Result: toAnyPtr([]string{"hello"}),
			},
		}, output.Result)
	})
}

func TestExec_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note    string
		module  string
		expErrs []string
	}{
		{
			note: "v0, module",
			module: `package system
main["hello"] {
	input.foo == "bar"
}`,
			expErrs: []string{
				"test.rego:2: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:2: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "v1 module",
			module: `package system
main contains "hello" if {
	input.foo == "bar"
}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"test.json": `{"foo": "bar"}`,
			}

			test.WithTempFS(files, func(dir string) {
				s := sdk_test.MustNewServer(
					sdk_test.MockBundle("/bundles/bundle.tar.gz", map[string]string{"test.rego": tc.module}),
					sdk_test.RawBundles(true),
				)

				defer s.Stop()

				var buf bytes.Buffer
				params := exec.NewParams(&buf)
				_ = params.OutputFormat.Set("json")
				params.ConfigOverrides = []string{
					"services.test.url=" + s.URL(),
					"bundles.test.resource=/bundles/bundle.tar.gz",
				}

				params.Paths = append(params.Paths, dir)

				if len(tc.expErrs) > 0 {
					testLogger := loggingtest.New()
					params.Logger = testLogger

					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					go func(expectedErrors []string) {
						err := runExecWithContext(ctx, params)
						// Note(philipc): Catch the expected cancellation
						// errors, allowing unexpected test failures through.
						if err != context.Canceled {
							var errs ast.Errors
							if errors.As(err, &errs) {
								for _, expErr := range expectedErrors {
									found := false
									for _, e := range errs {
										if strings.Contains(e.Error(), expErr) {
											found = true
											break
										}
									}
									if !found {
										t.Errorf("Could not find expected error: %s in %v", expErr, errs)
										return
									}
								}
							} else {
								t.Error(err)
								return
							}
						}
					}(tc.expErrs)

					if !test.Eventually(t, 5*time.Second, func() bool {
						for _, expErr := range tc.expErrs {
							found := false
							for _, e := range testLogger.Entries() {
								if strings.Contains(e.Message, expErr) {
									found = true
									break
								}
							}
							if !found {
								return false
							}
						}
						return true
					}) {
						t.Fatalf("timed out waiting for logged errors:\n\n%v\n\ngot\n\n%v:", tc.expErrs, testLogger.Entries())
					}
				} else {
					err := runExec(params)
					if err != nil {
						t.Fatal(err)
					}

					var output execOutput
					if err := json.Unmarshal(bytes.ReplaceAll(buf.Bytes(), []byte(dir), nil), &output); err != nil {
						t.Fatal(err)
					}

					resultSliceEquals(t, []execResultItem{
						{
							Path:   "/test.json",
							Result: toAnyPtr([]string{"hello"}),
						},
					}, output.Result)
				}
			})
		})
	}
}

func TestExecCompatibleFlags(t *testing.T) {
	tests := []struct {
		note         string
		v0Compatible bool
		v1Compatible bool
		module       string
		expErrs      []string
	}{
		{
			note:         "v0, no keywords used",
			v0Compatible: true,
			module: `package system
main["hello"] {
	input.foo == "bar"
}`,
		},
		{
			note:         "v0, no keywords imported",
			v0Compatible: true,
			module: `package system
main contains "hello" if {
	input.foo == "bar"
}`,
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: string cannot be used for rule name",
			},
		},
		{
			note:         "v0, keywords imported",
			v0Compatible: true,
			module: `package system
import future.keywords
main contains "hello" if {
	input.foo == "bar"
}`,
		},
		{
			note:         "v0, rego.v1 imported",
			v0Compatible: true,
			module: `package system
import rego.v1
main contains "hello" if {
	input.foo == "bar"
}`,
		},

		{
			note:         "v1, no keywords used",
			v1Compatible: true,
			module: `package system
main["hello"] {
	input.foo == "bar"
}`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note:         "v1, no keywords imported",
			v1Compatible: true,
			module: `package system
main contains "hello" if {
	input.foo == "bar"
}`,
		},
		{
			note:         "v1, keywords imported",
			v1Compatible: true,
			module: `package system
import future.keywords
main contains "hello" if {
	input.foo == "bar"
}`,
		},
		{
			note:         "v1, rego.v1 imported",
			v1Compatible: true,
			module: `package system
import rego.v1
main contains "hello" if {
	input.foo == "bar"
}`,
		},

		// v0 takes precedence over v1
		{
			note:         "v0+v1, no keywords used",
			v0Compatible: true,
			v1Compatible: true,
			module: `package system
main["hello"] {
	input.foo == "bar"
}`,
		},
		{
			note:         "v0+v1, no keywords imported",
			v0Compatible: true,
			v1Compatible: true,
			module: `package system
main contains "hello" if {
	input.foo == "bar"
}`,
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: string cannot be used for rule name",
			},
		},
		{
			note:         "v0+v1, keywords imported",
			v0Compatible: true,
			v1Compatible: true,
			module: `package system
import future.keywords
main contains "hello" if {
	input.foo == "bar"
}`,
		},
		{
			note:         "v0+v1, rego.v1 imported",
			v0Compatible: true,
			v1Compatible: true,
			module: `package system
import rego.v1
main contains "hello" if {
	input.foo == "bar"
}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"test.json": `{"foo": "bar"}`,
			}

			test.WithTempFS(files, func(dir string) {
				s := sdk_test.MustNewServer(
					sdk_test.MockBundle("/bundles/bundle.tar.gz", map[string]string{"test.rego": tc.module}),
					sdk_test.RawBundles(true),
				)

				defer s.Stop()

				var buf bytes.Buffer
				params := exec.NewParams(&buf)
				params.V0Compatible = tc.v0Compatible
				params.V1Compatible = tc.v1Compatible
				_ = params.OutputFormat.Set("json")
				params.ConfigOverrides = []string{
					"services.test.url=" + s.URL(),
					"bundles.test.resource=/bundles/bundle.tar.gz",
				}

				params.Paths = append(params.Paths, dir)

				if len(tc.expErrs) > 0 {
					testLogger := loggingtest.New()
					params.Logger = testLogger

					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					go func(expectedErrors []string) {
						err := runExecWithContext(ctx, params)
						// Note(philipc): Catch the expected cancellation
						// errors, allowing unexpected test failures through.
						if err != context.Canceled {
							var errs ast.Errors
							if errors.As(err, &errs) {
								for _, expErr := range expectedErrors {
									found := false
									for _, e := range errs {
										if strings.Contains(e.Error(), expErr) {
											found = true
											break
										}
									}
									if !found {
										t.Errorf("Could not find expected error: %s in %v", expErr, errs)
										return
									}
								}
							} else {
								t.Error(err)
								return
							}
						}
					}(tc.expErrs)

					if !test.Eventually(t, 5*time.Second, func() bool {
						for _, expErr := range tc.expErrs {
							found := false
							for _, e := range testLogger.Entries() {
								if strings.Contains(e.Message, expErr) {
									found = true
									break
								}
							}
							if !found {
								return false
							}
						}
						return true
					}) {
						t.Fatalf("timed out waiting for logged errors:\n\n%v\n\ngot\n\n%v:", tc.expErrs, testLogger.Entries())
					}
				} else {
					err := runExec(params)
					if err != nil {
						t.Fatal(err)
					}

					var output execOutput
					if err := json.Unmarshal(bytes.ReplaceAll(buf.Bytes(), []byte(dir), nil), &output); err != nil {
						t.Fatal(err)
					}

					resultSliceEquals(t, []execResultItem{
						{
							Path:   "/test.json",
							Result: toAnyPtr([]string{"hello"}),
						},
					}, output.Result)
				}
			})
		})
	}
}

func TestExecWithBundleRegoVersion(t *testing.T) {
	tests := []struct {
		note    string
		files   map[string]string
		expErrs []string
	}{
		{
			note: "v0.x bundle, no keywords used",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package system
main["hello"] {
	input.foo == "bar"
}`,
			},
		},
		{
			note: "v0.x bundle, no keywords imported",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package system
main contains "hello" if {
	input.foo == "bar"
}`,
			},
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: string cannot be used for rule name",
			},
		},
		{
			note: "v0.x bundle, keywords imported",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package system
import future.keywords
main contains "hello" if {
	input.foo == "bar"
}`,
			},
		},
		{
			note: "v0.x bundle, rego.v1 imported",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package system
import rego.v1
main contains "hello" if {
	input.foo == "bar"
}`,
			},
		},
		{
			note: "v0 bundle, v1 per-file override",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"/policy2.rego": 1
	}
}`,
				"policy1.rego": `package system
p[42] {
	input.foo == "bar"
}`,
				"policy2.rego": `package system
main contains "hello" if {
	42 in p
}`,
			},
		},
		{
			note: "v0 bundle, v1 per-file override (glob)",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"*/policy2.rego": 1
	}
}`,
				"policy1.rego": `package system
p[42] {
	input.foo == "bar"
}`,
				"policy2.rego": `package system
main contains "hello" if {
	42 in p
}`,
			},
		},
		{
			note: "v0 bundle, v1 per-file override, incompatible",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"/policy2.rego": 1
	}
}`,
				"policy1.rego": `package system
p[42] {
	input.foo == "bar"
}`,
				"policy2.rego": `package system
main["hello"] {
	p[_] == 42
}`,
			},
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},

		{
			note: "v1.0 bundle, no keywords used",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package system
main["hello"] {
	input.foo == "bar"
}`,
			},
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "v1.0 bundle, no keywords imported",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package system
main contains "hello" if {
	input.foo == "bar"
}`,
			},
		},
		{
			note: "v1.0 bundle, keywords imported",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package system
import future.keywords
main contains "hello" if {
	input.foo == "bar"
}`,
			},
		},
		{
			note: "v1.0 bundle, rego.v1 imported",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package system
import rego.v1
main contains "hello" if {
	input.foo == "bar"
}`,
			},
		},
		{
			note: "v1 bundle, v0 per-file override",
			files: map[string]string{
				".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"/policy1.rego": 0
	}
}`,
				"policy1.rego": `package system
p[42] {
	input.foo == "bar"
}`,
				"policy2.rego": `package system
main contains "hello" if {
	42 in p
}`,
			},
		},
		{
			note: "v1 bundle, v0 per-file override (glob)",
			files: map[string]string{
				".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"*/policy1.rego": 0
	}
}`,
				"policy1.rego": `package system
p[42] {
	input.foo == "bar"
}`,
				"policy2.rego": `package system
main contains "hello" if {
	42 in p
}`,
			},
		},
		{
			note: "v1 bundle, v0 per-file override, incompatible",
			files: map[string]string{
				".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"/policy1.rego": 0
	}
}`,
				"policy1.rego": `package system
p contains 42 {
	input.foo == "bar"
}`,
				"policy2.rego": `package system
main contains "hello" if {
	42 in p
}`,
			},
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: number cannot be used for rule name",
				"rego_parse_error: set cannot be used for rule name",
			},
		},
	}

	bundleTypeCases := []struct {
		note string
		tar  bool
	}{
		{
			"bundle dir", false,
		},
		{
			"bundle tar", true,
		},
	}

	v1CompatibleFlagCases := []struct {
		note string
		used bool
	}{
		{
			"no --v1-compatible", false,
		},
		{
			"--v1-compatible", true,
		},
	}

	for _, bundleType := range bundleTypeCases {
		for _, v1CompatibleFlag := range v1CompatibleFlagCases {
			for _, tc := range tests {
				t.Run(fmt.Sprintf("%s, %s, %s", bundleType.note, v1CompatibleFlag.note, tc.note), func(t *testing.T) {
					files := map[string]string{
						"files/test.json": `{"foo": "bar"}`,
					}
					if bundleType.tar {
						files["bundle.tar.gz"] = ""
					} else {
						for k, v := range tc.files {
							files[k] = v
						}
					}

					test.WithTempFS(files, func(root string) {
						p := root
						if bundleType.tar {
							p = filepath.Join(root, "bundle.tar.gz")
							files := make([][2]string, 0, len(tc.files))
							for k, v := range tc.files {
								files = append(files, [2]string{k, v})
							}
							buf := archive.MustWriteTarGz(files)
							bf, err := os.Create(p)
							if err != nil {
								t.Fatalf("Unexpected error: %v", err)
							}
							_, err = bf.Write(buf.Bytes())
							if err != nil {
								t.Fatalf("Unexpected error: %v", err)
							}
						}

						var buf bytes.Buffer
						params := exec.NewParams(&buf)
						params.Paths = append(params.Paths, root+"/files/")
						params.BundlePaths = []string{p}
						params.V1Compatible = v1CompatibleFlag.used
						_ = params.OutputFormat.Set("json")

						if len(tc.expErrs) > 0 {
							testLogger := loggingtest.New()
							params.Logger = testLogger

							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()
							go func() {
								err := runExecWithContext(ctx, params)
								// we cancelled the context, so we expect that error
								if err != nil && err.Error() != "context canceled" {
									t.Error(err)
									return
								}
							}()

							if !test.Eventually(t, 5*time.Second, func() bool {
								for _, expErr := range tc.expErrs {
									found := false
									for _, e := range testLogger.Entries() {
										if strings.Contains(e.Message, expErr) {
											found = true
											break
										}
									}
									if !found {
										return false
									}
								}
								return true
							}) {
								t.Fatalf("timed out waiting for logged errors:\n\n%v\n\ngot\n\n%v:", tc.expErrs, testLogger.Entries())
							}
						} else {
							err := runExec(params)
							if err != nil {
								t.Fatal(err)
							}

							var output execOutput
							if err := json.Unmarshal(bytes.ReplaceAll(buf.Bytes(), []byte(root), nil), &output); err != nil {
								t.Fatal(err)
							}

							resultSliceEquals(t, []execResultItem{
								{
									Path:   "/files/test.json",
									Result: toAnyPtr([]string{"hello"}),
								},
							}, output.Result)
						}
					})
				})
			}
		}
	}
}

func TestInvalidConfig(t *testing.T) {
	var buf bytes.Buffer
	params := exec.NewParams(&buf)
	params.Fail = true
	params.FailDefined = true

	err := exec.Exec(context.TODO(), nil, params)
	if err == nil || err.Error() != "specify --fail or --fail-defined but not both" {
		t.Fatalf("Expected error '%s' but got '%s'", "specify --fail or --fail-defined but not both", err.Error())
	}
}

func TestInvalidConfigAllThree(t *testing.T) {
	var buf bytes.Buffer
	params := exec.NewParams(&buf)
	params.Fail = true
	params.FailDefined = true
	params.FailNonEmpty = true

	err := exec.Exec(context.TODO(), nil, params)
	if err == nil || err.Error() != "specify --fail or --fail-defined but not both" {
		t.Fatalf("Expected error '%s' but got '%s'", "specify --fail or --fail-defined but not both", err.Error())
	}
}

func TestInvalidConfigNonEmptyAndFail(t *testing.T) {
	var buf bytes.Buffer
	params := exec.NewParams(&buf)
	params.FailNonEmpty = true
	params.Fail = true

	err := exec.Exec(context.TODO(), nil, params)
	if err == nil || err.Error() != "specify --fail-non-empty or --fail but not both" {
		t.Fatalf("Expected error '%s' but got '%s'", "specify --fail-non-empty or --fail but not both", err.Error())
	}
}

func TestInvalidConfigNonEmptyAndFailDefined(t *testing.T) {
	var buf bytes.Buffer
	params := exec.NewParams(&buf)
	params.FailNonEmpty = true
	params.FailDefined = true

	err := exec.Exec(context.TODO(), nil, params)
	if err == nil || err.Error() != "specify --fail-non-empty or --fail-defined but not both" {
		t.Fatalf("Expected error '%s' but got '%s'", "specify --fail-non-empty or --fail-defined but not both", err.Error())
	}
}

func TestFailFlagCases(t *testing.T) {

	var tests = []struct {
		description  string
		files        map[string]string
		decision     string
		expectError  bool
		expected     []byte
		fail         bool
		failDefined  bool
		failNonEmpty bool
	}{
		{
			description: "--fail-defined with undefined result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package system
				import rego.v1
				
				test_fun := x if {
					x = false
					x
				}
				
				undefined_test if {
					test_fun
				}`,
			},
			expectError: false,
			expected: []byte(`{"result": [{
			"path": "/files/test.json",
			"error": {
				"code": "opa_undefined_error",
				"message": "/system/main decision was undefined"
			  }
		}]}`),
			failDefined: true,
		},
		{
			description: "--fail-defined with populated result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package system
				import rego.v1
				
				main contains "hello"`,
			},
			decision:    "",
			expectError: true,
			expected: []byte(`{"result": [{
			"path": "/files/test.json",
			"result": ["hello"]
		}]}`),
			failDefined: true,
		},
		{
			description: "--fail-defined with true boolean result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package fail.defined.flag
				import rego.v1

				some_function if {
					  input.foo == 7
				}
				
				default fail_test := false
				fail_test if {
					  some_function
				}`,
			},
			decision:    "fail/defined/flag/fail_test",
			expectError: true,
			expected: []byte(`{"result": [{
			"path": "/files/test.json",
			"result": true
		}]}`),
			failDefined: true,
		},
		{
			description: "--fail-defined with false boolean result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package fail.defined.flag
				import rego.v1

				default fail_test := false
				fail_test if {
					false
				}`,
			},
			decision:    "fail/defined/flag/fail_test",
			expectError: true,
			expected: []byte(`{"result": [{
			"path": "/files/test.json",
			"result": false
		}]}`),
			failDefined: true,
		},
		{
			description: "--fail with undefined result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package system
				import rego.v1

				test_fun := x if {
					x = false
					x
				}
		
				undefined_test if {
					test_fun
				}`,
			},
			expectError: true,
			expected: []byte(`{"result": [{
			"path": "/files/test.json",
			"error": {
				"code": "opa_undefined_error",
				"message": "/system/main decision was undefined"
			  }
		}]}`),
			fail: true,
		},
		{
			description: "--fail with populated result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package system
				import rego.v1
				
			main contains "hello"`,
			},
			expectError: false,
			expected: []byte(`{"result": [{
			"path": "/files/test.json",
			"result": ["hello"]
		}]}`),
			fail: true,
		},
		{
			description: "--fail with true boolean result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package fail.defined.flag
				import rego.v1

				some_function if {
					input.foo == 7
				}
				
				default fail_test := false
				fail_test if {
					some_function
				}`,
			},
			decision:    "fail/defined/flag/fail_test",
			expectError: false,
			expected: []byte(`{"result": [{
			"path": "/files/test.json",
			"result": true
		}]}`),
			fail: true,
		},
		{
			description: "--fail with false boolean result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package fail.defined.flag
				import rego.v1

				default fail_test := false
				fail_test if {
					false
				}`,
			},
			decision:    "fail/defined/flag/fail_test",
			expectError: false,
			expected: []byte(`{"result": [{
			"path": "/files/test.json",
			"result": false
		}]}`),
			fail: true,
		},
		{
			description: "--fail-non-empty with undefined result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package system
				import rego.v1

				test_fun := x if {
					x = false
					x
				}
		
				undefined_test if {
					test_fun
				}`,
			},
			expectError: false,
			expected: []byte(`{"result": [{
			"path": "/files/test.json",
			"error": {
				"code": "opa_undefined_error",
				"message": "/system/main decision was undefined"
			  }
		}]}`),
			failNonEmpty: true,
		},
		{
			description: "--fail-non-empty with populated result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package system
				import rego.v1

				main contains "hello"`,
			},
			decision:    "",
			expectError: true,
			expected: []byte(`{"result": [{
			"path": "/files/test.json",
			"result": ["hello"]
		}]}`),
			failNonEmpty: true,
		},
		{
			description: "--fail-non-empty with true boolean result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package fail.non.empty.flag
				import rego.v1

				some_function if {
					input.foo == 7
				}
				
				default fail_test := false
				fail_test if {
					some_function
				}`,
			},
			decision:    "fail/non/empty/flag/fail_test",
			expectError: true,
			expected: []byte(`{"result": [{
			"path": "/files/test.json",
			"result": true
		}]}`),
			failNonEmpty: true,
		},
		{
			description: "--fail-non-empty with false boolean result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package fail.non.empty.flag
				import rego.v1

				default fail_test := false
				fail_test if {
					false
				}`,
			},
			decision:    "fail/non/empty/flag/fail_test",
			expectError: true,
			expected: []byte(`{"result": [{
			"path": "/files/test.json",
			"result": false
		}]}`),
			failNonEmpty: true,
		},
		{
			description: "--fail-non-empty with an empty array",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package fail.non.empty.flag
				import rego.v1

				default fail_test := ["something", "hello"]
				fail_test := [] if {
					input.foo == 7
				}`,
			},
			decision:    "fail/non/empty/flag/fail_test",
			expectError: false,
			expected: []byte(`{"result": [{
			"path": "/files/test.json",
			"result": []
		}]}`),
			failNonEmpty: true,
		},
		{
			description: "--fail-non-empty for an empty set coming from a partial rule",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package fail.non.empty.flag
				import rego.v1

				fail_test contains message if {
				   false
				   message := "not gonna happen"
				}`,
			},
			decision:    "fail/non/empty/flag/fail_test",
			expectError: false,
			expected: []byte(`{"result": [{
			"path": "/files/test.json",
			"result": []
		}]}`),
			failNonEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			test.WithTempFS(tt.files, func(dir string) {
				var buf bytes.Buffer
				params := exec.NewParams(&buf)
				_ = params.OutputFormat.Set("json")
				params.BundlePaths = []string{dir + "/bundle/"}
				params.Paths = append(params.Paths, dir+"/files/")
				if tt.decision != "" {
					params.Decision = tt.decision
				}
				params.FailDefined = tt.failDefined
				params.Fail = tt.fail
				params.FailNonEmpty = tt.failNonEmpty

				err := runExec(params)
				if err != nil && !tt.expectError {
					t.Fatal("unexpected error in test")
				}
				if err == nil && tt.expectError {
					t.Fatal("expected error, but none occurred in test")
				}

				var output execOutput
				if err := json.Unmarshal(bytes.ReplaceAll(buf.Bytes(), []byte(dir), nil), &output); err != nil {
					t.Fatal(err)
				}

				var expected execOutput
				if err := json.Unmarshal(tt.expected, &expected); err != nil {
					t.Fatal(err)
				}

				resultSliceEquals(t, expected.Result, output.Result)
			})
		})
	}
}

func TestExecWithInvalidInputOptions(t *testing.T) {
	tests := []struct {
		description string
		files       map[string]string
		stdIn       bool
		input       string
		expectError bool
		expected    string
	}{
		{
			description: "path passed in as arg should not raise error",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package system
				import rego.v1

				test_fun := x if {
					x = false
					x
				}
		
				undefined_test if {
					test_fun
				}`,
			},
			expectError: false,
			expected:    "",
		},
		{
			description: "no paths passed in as args should raise error if --stdin-input flag not set",
			files: map[string]string{
				"bundle/x.rego": `package system
				import rego.v1

				test_fun := x if {
					x = false
					x
				}
		
				undefined_test if {
					test_fun
				}`,
			},
			expectError: true,
			expected:    "requires at least 1 path arg, or the --stdin-input flag",
		},
		{
			description: "should not raise error if --stdin-input flag is set when no paths passed in as args",
			files: map[string]string{
				"bundle/x.rego": `package system
				import rego.v1

				test_fun := x if {
					x = false
					x
				}
		
				undefined_test if {
					test_fun
				}`,
			},
			stdIn:       true,
			input:       `{"foo": 7}`,
			expectError: false,
			expected:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			test.WithTempFS(tt.files, func(dir string) {
				var buf bytes.Buffer
				params := exec.NewParams(&buf)
				_ = params.OutputFormat.Set("json")
				params.BundlePaths = []string{dir + "/bundle/"}
				if tt.stdIn {
					params.StdIn = true
					tempFile, err := os.CreateTemp(t.TempDir(), "test")
					if err != nil {
						t.Fatalf("unexpected error creating temp file: %q", err.Error())
					}
					if _, err := tempFile.WriteString(tt.input); err != nil {
						t.Fatalf("unexpeced error when writing to temp file: %q", err.Error())
					}
					if _, err := tempFile.Seek(0, 0); err != nil {
						t.Fatalf("unexpected error when rewinding temp file: %q", err.Error())
					}
					oldStdin := os.Stdin
					defer func() {
						os.Stdin = oldStdin
						os.Remove(tempFile.Name())
					}()
					os.Stdin = tempFile
				} else {
					if _, ok := tt.files["files/test.json"]; ok {
						params.Paths = append(params.Paths, dir+"/files/")
					}
				}

				err := runExec(params)
				if err != nil && !tt.expectError {
					t.Fatalf("unexpected error in test: %q", err.Error())
				}
				if err == nil && tt.expectError {
					t.Fatalf("expected error %q, but none occurred in test", tt.expected)
				}
				if err != nil && err.Error() != tt.expected {
					t.Fatalf("expected error %q, but got %q", tt.expected, err.Error())
				}
			})
		})
	}
}

func TestExecTimeoutWithMalformedRemoteBundle(t *testing.T) {
	test.WithTempFS(map[string]string{}, func(dir string) {
		// Note(philipc): We add the "raw bundles" flag so that we can stuff a
		// malformed bundle into the mock bundle server. Otherwise, the server
		// will just return 503 errors forever, because it won't be able to
		// build the bundle on its end.
		s := sdk_test.MustNewServer(
			sdk_test.RawBundles(true),
			sdk_test.MockBundle("/bundles/bundle.tar.gz", map[string]string{
				"example.rego": `
				package example

				p := bits.sand(42, 43)  # typo of bits.and
			`,
			}))

		defer s.Stop()

		var buf bytes.Buffer
		params := exec.NewParams(&buf)
		_ = params.OutputFormat.Set("json")
		params.ConfigOverrides = []string{
			"services.test.url=" + s.URL(),
			"bundles.test.resource=/bundles/bundle.tar.gz",
		}

		// Note(philipc): We can set this timeout almost arbitrarily high or
		// low-- the test will time out before it ever succeeds, due to the
		// faulty bundle.
		params.Timeout = time.Millisecond * 50

		params.Paths = append(params.Paths, dir)
		err := runExec(params)
		if err == nil {
			t.Fatalf("Expected error, got nil instead.")
		}

		exp := "exec error: timed out before OPA was ready."
		if !strings.HasPrefix(err.Error(), exp) {
			t.Fatalf("Expected error: %s, got %s", exp, err.Error())
		}
	})
}
