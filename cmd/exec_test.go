package cmd

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/cmd/internal/exec"
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

func TestFailFlagCases(t *testing.T) {

	var tests = []struct {
		description string
		files       map[string]string
		decision    string
		expectError bool
		expected    interface{}
		fail        bool
		failDefined bool
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
			expected: util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/files/test.json",
			"result": ["hello"]
		}]}`)),
			fail: true,
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
			decision: "fail/defined/flag/fail_test",
			expected: util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/files/test.json",
			"result": true
		}]}`)),
			fail: true,
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
			decision: "fail/defined/flag/fail_test",
			expected: util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/files/test.json",
			"result": false
		}]}`)),
			fail: true,
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
