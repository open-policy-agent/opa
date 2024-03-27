package cmd

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/cmd/internal/exec"
	loggingtest "github.com/open-policy-agent/opa/logging/test"
	sdk_test "github.com/open-policy-agent/opa/sdk/test"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
)

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
				main["hello"]
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

		output := util.MustUnmarshalJSON(bytes.ReplaceAll(buf.Bytes(), []byte(dir), nil))

		exp := util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/test.json",
			"result": ["hello"]
		}, {
			"path": "/test2.yaml",
			"result": ["hello"]
		}, {
			"path": "/test3.yml",
			"result": ["hello"]
		}]}`))

		if !reflect.DeepEqual(output, exp) {
			t.Fatal("Expected:", exp, "Got:", output)
		}
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
				main["hello"]
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

		output := util.MustUnmarshalJSON(bytes.ReplaceAll(buf.Bytes(), []byte(dir), nil))

		exp := util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/test.json",
			"result": ["hello"]
		}]}`))

		if !reflect.DeepEqual(output, exp) {
			t.Fatal("Expected:", exp, "Got:", output)
		}

	})

}

func TestExecBundleFlag(t *testing.T) {

	files := map[string]string{
		"files/test.json": `{"foo": 7}`,
		"bundle/x.rego": `package system

		main["hello"]`,
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

		output := util.MustUnmarshalJSON(bytes.ReplaceAll(buf.Bytes(), []byte(dir), nil))

		exp := util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/files/test.json",
			"result": ["hello"]
		}]}`))

		if !reflect.DeepEqual(output, exp) {
			t.Fatal("Expected:", exp, "Got:", output)
		}

	})
}

func TestExecV1Compatible(t *testing.T) {
	tests := []struct {
		note         string
		v1Compatible bool
		module       string
		expErrs      []string
	}{
		{
			note: "v0.x, no keywords used",
			module: `package system
main["hello"] {
	input.foo == "bar"
}`,
		},
		{
			note: "v0.x, no keywords imported",
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
			note: "v0.x, keywords imported",
			module: `package system
import future.keywords
main contains "hello" if {
	input.foo == "bar"
}`,
		},
		{
			note: "v0.x, rego.v1 imported",
			module: `package system
import rego.v1
main contains "hello" if {
	input.foo == "bar"
}`,
		},

		{
			note:         "v1.0, no keywords used",
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
			note:         "v1.0, no keywords imported",
			v1Compatible: true,
			module: `package system
main contains "hello" if {
	input.foo == "bar"
}`,
		},
		{
			note:         "v1.0, keywords imported",
			v1Compatible: true,
			module: `package system
import future.keywords
main contains "hello" if {
	input.foo == "bar"
}`,
		},
		{
			note:         "v1.0, rego.v1 imported",
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
					go func() {
						err := runExecWithContext(ctx, params)
						// Note(philipc): Catch the expected cancellation
						// errors, allowing unexpected test failures through.
						if err != context.Canceled {
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

					output := util.MustUnmarshalJSON(bytes.ReplaceAll(buf.Bytes(), []byte(dir), nil))

					exp := util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/test.json",
			"result": ["hello"]
		}]}`))

					if !reflect.DeepEqual(output, exp) {
						t.Fatal("Expected:", exp, "Got:", output)
					}
				}
			})
		})
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
		expected     interface{}
		fail         bool
		failDefined  bool
		failNonEmpty bool
	}{
		{
			description: "--fail-defined with undefined result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package system

		test_fun := x {
			x = false
			x
		}

		undefined_test {
			test_fun
		}`,
			},
			expectError: false,
			expected: util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/files/test.json",
			"error": {
				"code": "opa_undefined_error",
				"message": "/system/main decision was undefined"
			  }
		}]}`)),
			failDefined: true,
		},
		{
			description: "--fail-defined with populated result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package system

		main["hello"]`,
			},
			decision:    "",
			expectError: true,
			expected: util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/files/test.json",
			"result": ["hello"]
		}]}`)),
			failDefined: true,
		},
		{
			description: "--fail-defined with true boolean result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package fail.defined.flag

               some_function {
                       input.foo == 7
               }

               default fail_test := false
               fail_test {
                       some_function
               }`,
			},
			decision:    "fail/defined/flag/fail_test",
			expectError: true,
			expected: util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/files/test.json",
			"result": true
		}]}`)),
			failDefined: true,
		},
		{
			description: "--fail-defined with false boolean result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package fail.defined.flag

		default fail_test := false
		fail_test {
			false
		}`,
			},
			decision:    "fail/defined/flag/fail_test",
			expectError: true,
			expected: util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/files/test.json",
			"result": false
		}]}`)),
			failDefined: true,
		},
		{
			description: "--fail with undefined result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package system

		test_fun := x {
			x = false
			x
		}

		undefined_test {
			test_fun
		}`,
			},
			expectError: true,
			expected: util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/files/test.json",
			"error": {
				"code": "opa_undefined_error",
				"message": "/system/main decision was undefined"
			  }
		}]}`)),
			fail: true,
		},
		{
			description: "--fail with populated result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package system

		main["hello"]`,
			},
			expectError: false,
			expected: util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/files/test.json",
			"result": ["hello"]
		}]}`)),
			fail: true,
		},
		{
			description: "--fail with true boolean result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package fail.defined.flag

               some_function {
                       input.foo == 7
               }

               default fail_test := false
               fail_test {
                       some_function
               }`,
			},
			decision:    "fail/defined/flag/fail_test",
			expectError: false,
			expected: util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/files/test.json",
			"result": true
		}]}`)),
			fail: true,
		},
		{
			description: "--fail with false boolean result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package fail.defined.flag

		default fail_test := false
		fail_test {
			false
		}`,
			},
			decision:    "fail/defined/flag/fail_test",
			expectError: false,
			expected: util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/files/test.json",
			"result": false
		}]}`)),
			fail: true,
		},
		{
			description: "--fail-non-empty with undefined result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package system

		test_fun := x {
			x = false
			x
		}

		undefined_test {
			test_fun
		}`,
			},
			expectError: false,
			expected: util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/files/test.json",
			"error": {
				"code": "opa_undefined_error",
				"message": "/system/main decision was undefined"
			  }
		}]}`)),
			failNonEmpty: true,
		},
		{
			description: "--fail-non-empty with populated result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package system

		main["hello"]`,
			},
			decision:    "",
			expectError: true,
			expected: util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/files/test.json",
			"result": ["hello"]
		}]}`)),
			failNonEmpty: true,
		},
		{
			description: "--fail-non-empty with true boolean result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package fail.non.empty.flag

               some_function {
                       input.foo == 7
               }

               default fail_test := false
               fail_test {
                       some_function
               }`,
			},
			decision:    "fail/non/empty/flag/fail_test",
			expectError: true,
			expected: util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/files/test.json",
			"result": true
		}]}`)),
			failNonEmpty: true,
		},
		{
			description: "--fail-non-empty with false boolean result",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package fail.non.empty.flag

		default fail_test := false
		fail_test {
			false
		}`,
			},
			decision:    "fail/non/empty/flag/fail_test",
			expectError: true,
			expected: util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/files/test.json",
			"result": false
		}]}`)),
			failNonEmpty: true,
		},
		{
			description: "--fail-non-empty with an empty array",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package fail.non.empty.flag

		default fail_test := ["something", "hello"]
		fail_test := [] if {
			input.foo == 7
		}`,
			},
			decision:    "fail/non/empty/flag/fail_test",
			expectError: false,
			expected: util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/files/test.json",
			"result": []
		}]}`)),
			failNonEmpty: true,
		},
		{
			description: "--fail-non-empty for an empty set coming from a partial rule",
			files: map[string]string{
				"files/test.json": `{"foo": 7}`,
				"bundle/x.rego": `package fail.non.empty.flag

		fail_test[message] {
		   false
		   message := "not gonna happen"
		}`,
			},
			decision:    "fail/non/empty/flag/fail_test",
			expectError: false,
			expected: util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/files/test.json",
			"result": []
		}]}`)),
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

				output := util.MustUnmarshalJSON(bytes.ReplaceAll(buf.Bytes(), []byte(dir), nil))

				if !reflect.DeepEqual(output, tt.expected) {
					t.Errorf("Expected %v, got: %v", tt.expected, output)
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
