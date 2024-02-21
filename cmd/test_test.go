package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/util/test"
)

func TestFilterTraceDefault(t *testing.T) {
	p := newTestCommandParams()
	p.verbose = false
	expected := `Enter data.testing.test_p = _
| Enter data.testing.test_p
| | Enter data.testing.p
| | | Enter data.testing.q
| | | | Enter data.testing.r
| | | | | Fail x = data.x
| | | | Fail data.testing.r[x]
| | | Fail data.testing.q.foo
| | Fail data.testing.p with data.x as "bar"
| Fail data.testing.test_p = _
`
	verifyFilteredTrace(t, &p, expected)
}

func TestFilterTraceVerbose(t *testing.T) {
	p := newTestCommandParams()
	p.verbose = true
	expected := `Enter data.testing.test_p = _
| Enter data.testing.test_p
| | Enter data.testing.p
| | | Note "test test"
| | | Enter data.testing.q
| | | | Note "got this far"
| | | | Enter data.testing.r
| | | | | Note "got this far2"
| | | | | Fail x = data.x
| | | | Fail data.testing.r[x]
| | | Fail data.testing.q.foo
| | Fail data.testing.p with data.x as "bar"
| Fail data.testing.test_p = _
`
	verifyFilteredTrace(t, &p, expected)
}

func TestFilterTraceExplainFails(t *testing.T) {
	p := newTestCommandParams()
	err := p.explain.Set(explainModeFails)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expected := `Enter data.testing.test_p = _
| Enter data.testing.test_p
| | Enter data.testing.p
| | | Enter data.testing.q
| | | | Enter data.testing.r
| | | | | Fail x = data.x
| | | | Fail data.testing.r[x]
| | | Fail data.testing.q.foo
| | Fail data.testing.p with data.x as "bar"
| Fail data.testing.test_p = _
`
	verifyFilteredTrace(t, &p, expected)
}

func TestFilterTraceExplainNotes(t *testing.T) {
	p := newTestCommandParams()
	err := p.explain.Set(explainModeNotes)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expected := `Enter data.testing.test_p = _
| Enter data.testing.test_p
| | Enter data.testing.p
| | | Note "test test"
| | | Enter data.testing.q
| | | | Note "got this far"
| | | | Enter data.testing.r
| | | | | Note "got this far2"
`
	verifyFilteredTrace(t, &p, expected)
}

func TestFilterTraceExplainFull(t *testing.T) {
	p := newTestCommandParams()
	err := p.explain.Set(explainModeFull)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expected := `Enter data.testing.test_p = _
| Eval data.testing.test_p = _
| Index data.testing.test_p (matched 1 rule, early exit)
| Enter data.testing.test_p
| | Eval data.testing.p with data.x as "bar"
| | Index data.testing.p (matched 1 rule, early exit)
| | Enter data.testing.p
| | | Eval data.testing.x
| | | Index data.testing.x (matched 1 rule, early exit)
| | | Enter data.testing.x
| | | | Eval data.testing.y
| | | | Index data.testing.y (matched 1 rule, early exit)
| | | | Enter data.testing.y
| | | | | Eval true
| | | | | Exit data.testing.y early
| | | | Exit data.testing.x early
| | | Eval trace("test test")
| | | Note "test test"
| | | Eval data.testing.q.foo
| | | Index data.testing.q (matched 1 rule)
| | | Enter data.testing.q
| | | | Eval trace("got this far")
| | | | Note "got this far"
| | | | Eval data.testing.r[x]
| | | | Index data.testing.r (matched 1 rule)
| | | | Enter data.testing.r
| | | | | Eval trace("got this far2")
| | | | | Note "got this far2"
| | | | | Eval x = data.x
| | | | | Fail x = data.x
| | | | | Redo trace("got this far2")
| | | | Fail data.testing.r[x]
| | | | Redo trace("got this far")
| | | Fail data.testing.q.foo
| | | Redo trace("test test")
| | | Redo data.testing.x
| | | Redo data.testing.x
| | | | Redo data.testing.y
| | | | | Redo true
| | Fail data.testing.p with data.x as "bar"
| Fail data.testing.test_p = _
`
	verifyFilteredTrace(t, &p, expected)
}

func TestThresholdRange(t *testing.T) {
	thresholds := []float64{-1, 101}
	for _, threshold := range thresholds {
		if isThresholdValid(threshold) {
			t.Fatalf("invalid threshold %2f shoul be reported", threshold)
		}
	}
}

func verifyFilteredTrace(t *testing.T, params *testCommandParams, expected string) {
	filtered := filterTrace(params, failTrace(t))

	var buff bytes.Buffer
	topdown.PrettyTrace(&buff, filtered)
	actual := buff.String()

	if actual != expected {
		t.Fatalf("Expected:\n\n%s\n\nGot:\n\n%s\n\n", expected, actual)
	}
}

func failTrace(t *testing.T) []*topdown.Event {
	t.Helper()
	mod := `
	package testing
	
	p {
		x  # Always true
		trace("test test")
		q["foo"]
	}
	
	x {
		y
	}
	
	y {
		true
	}
	
	q[x] {
		some x
		trace("got this far")
		r[x]
		trace("got this far1")
	}
	
	r[x] {
		trace("got this far2")
		x := data.x
	}
	
	test_p {
		p with data.x as "bar"
	}
	`

	tracer := topdown.NewBufferTracer()

	_, err := rego.New(
		rego.Module("test.rego", mod),
		rego.Trace(true),
		rego.QueryTracer(tracer),
		rego.Query("data.testing.test_p"),
	).Eval(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	return *tracer
}

// Assert that ignore flag is correctly used when the bundle flag is activated
func TestIgnoreFlag(t *testing.T) {
	files := map[string]string{
		"/test.rego":   "package test\n p := input.foo == 42\ntest_p {\n p with input.foo as 42\n}",
		"/broken.rego": "package foo\n bar {",
	}

	var exitCode int
	test.WithTempFS(files, func(root string) {
		testParams := newTestCommandParams()
		testParams.count = 1
		testParams.errOutput = io.Discard
		testParams.bundleMode = false
		testParams.ignore = []string{"broken.rego"}

		exitCode, _ = opaTest([]string{root}, testParams)
	})

	if exitCode > 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
}

// Assert that ignore flag is correctly used when the bundle flag is activated
func TestIgnoreFlagWithBundleFlag(t *testing.T) {
	files := map[string]string{
		"/test.rego":   "package test\n p := input.foo == 42\ntest_p {\n p with input.foo as 42\n}",
		"/broken.rego": "package foo\n bar {",
	}

	var exitCode int
	test.WithTempFS(files, func(root string) {
		testParams := newTestCommandParams()
		testParams.count = 1
		testParams.errOutput = io.Discard
		testParams.bundleMode = true
		testParams.ignore = []string{"broken.rego"}
		exitCode, _ = opaTest([]string{root}, testParams)
	})

	if exitCode > 0 {
		t.Fatalf("unexpected exit code: %d", exitCode)
	}
}

func testSchemasAnnotation(rego string) (int, error) {

	files := map[string]string{
		"test.rego": rego,
	}

	var exitCode int
	var err error
	test.WithTempFS(files, func(path string) {
		regoFilePath := filepath.Join(path, "test.rego")

		testParams := newTestCommandParams()
		testParams.count = 1
		testParams.errOutput = io.Discard

		exitCode, err = opaTest([]string{regoFilePath}, testParams)
	})
	return exitCode, err
}

// Assert that 'schemas' annotations with schema ref are ignored, but not inlined schemas
func TestSchemasAnnotation(t *testing.T) {
	policyWithSchemaRef := `
package test
# METADATA
# schemas:
#   - input: schema["input"]
p { 
	rego.metadata.rule() # presence of rego.metadata.* calls must not trigger unwanted schema evaluation
	input.foo == 42 # type mismatch with schema that should be ignored
}

test_p {
    p with input.foo as 42
}`

	exitCode, _ := testSchemasAnnotation(policyWithSchemaRef)
	if exitCode > 0 {
		t.Fatalf("unexpected error when schema ref is present")
	}
}
func TestSchemasAnnotationInline(t *testing.T) {
	policyWithInlinedSchema := `
package test
# METADATA
# schemas:
#   - input.foo: {"type": "boolean"}
p { 
	input.foo == 42 # type mismatch with schema that should NOT be ignored since it is an inlined schema format
}

test_p {
    p with input.foo as 42
}`

	exitCode, err := testSchemasAnnotation(policyWithInlinedSchema)
	// We expect an error here, as inlined schemas are always used for type checking

	if exitCode == 0 {
		t.Fatalf("didn't get expected error when inlined schema is present")
	} else if !strings.Contains(err.Error(), "rego_type_error: match error") {
		t.Fatalf("didn't get expected %s error when inlined schema is present; got: %v", ast.TypeErr, err)
	}
}

func testSchemasAnnotationWithJSONFile(rego string, schema string) (int, error) {

	files := map[string]string{
		"test.rego":        rego,
		"demo_schema.json": schema,
	}

	var exitCode int
	var err error
	test.WithTempFS(files, func(path string) {
		regoFilePath := filepath.Join(path, "test.rego")

		testParams := newTestCommandParams()
		testParams.count = 1
		testParams.schema.path = path
		testParams.errOutput = io.Discard

		exitCode, err = opaTest([]string{regoFilePath}, testParams)
	})
	return exitCode, err
}
func TestJSONSchemaSuccess(t *testing.T) {

	regoContents := `package test
# METADATA
# schemas:
#   - input: schema.demo_schema
p {
input.foo == 42
}

test_p {
p with input.foo as 42
}`

	schema := `{
		"$schema": "http://json-schema.org/draft-07/schema",
		"$id": "schema",
		"type": "object",
		"description": "The root schema comprises the entire JSON document.",
		"required": [
			"foo"
		],
		"properties": {
			"foo": {
				"$id": "#/properties/foo",
				"type": "number",
				"description": "foo"
			}         
		},
		"additionalProperties": false
   }`

	_, err := testSchemasAnnotationWithJSONFile(regoContents, schema)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestJSONSchemaFail(t *testing.T) {

	regoContents := `package test
# METADATA
# schemas:
#   - input: schema.demo_schema
p {
input.foo == 42
}

test_p {
p with input.foo as 42
}`

	schema := `{
"$schema": "http://json-schema.org/draft-07/schema",
"$id": "schema",
"type": "object",
"description": "The root schema comprises the entire JSON document.",
"required": [
	"foo"
],
"properties": {
	"foo": {
		"$id": "#/properties/foo",
		"type": "boolean",
		"description": "foo"
	}         
},
"additionalProperties": false
}`

	exitCode, err := testSchemasAnnotationWithJSONFile(regoContents, schema)
	if exitCode == 0 {
		t.Fatalf("didn't get expected error when schema is present and is defining a different type than being used.")
	} else if !strings.Contains(err.Error(), "rego_type_error: match error") {
		t.Fatalf("didn't get expected %s error when schema is defining a different type than being used; got: %v", ast.TypeErr, err)
	}
}

func TestWatchMode(t *testing.T) {

	files := map[string]string{
		"/policy.rego":      "package foo\n p := 1",
		"/policy_test.rego": "package foo\n test_p { p == 1 }",
	}

	test.WithTempFS(files, func(root string) {
		buf := test.BlockingWriter{}

		testParams := newTestCommandParams()
		testParams.output = &buf
		testParams.watch = true
		testParams.count = 1

		done := make(chan struct{})
		go func() {
			_, _ = opaTest([]string{root}, testParams)
			<-done
		}()

		expected := "Watching for changes ..."
		if !test.Eventually(t, 2*time.Second, func() bool {
			return strings.Contains(buf.String(), expected)
		}) {
			t.Fatalf("expected:\n\n%q\n\ngot:\n\n%q", expected, buf.String())
		}
		buf.Reset()

		// update the test
		f, _ := os.OpenFile(path.Join(root, "policy_test.rego"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		_, err := f.WriteString("package foo\n test_p { p == 2 }")
		if err != nil {
			t.Fatal(err)
		}
		f.Close()

		r := regexp.MustCompile(`FAIL \(.*s\)`)
		expected = `%ROOT%/policy_test.rego:
data.foo.test_p: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
********************************************************************************
Watching for changes ...
`
		if !test.Eventually(t, 2*time.Second, func() bool {
			actual := r.ReplaceAllString(buf.String(), "FAIL (%TIME%)")
			expected := strings.ReplaceAll(expected, "%ROOT%", root)
			return strings.Contains(actual, expected)
		}) {
			t.Fatalf("expected:\n\n%q\n\ngot:\n\n%q", expected, buf.String())
		}
		buf.Reset()

		// update policy so test passes
		f, _ = os.OpenFile(path.Join(root, "policy.rego"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		_, err = f.WriteString("package foo\n p := 2")
		if err != nil {
			t.Fatal(err)
		}

		f.Close()

		expected = `PASS: 1/1
********************************************************************************
Watching for changes ...
`
		if !test.Eventually(t, 2*time.Second, func() bool {
			expected := strings.ReplaceAll(expected, "%ROOT%", root)
			return strings.Contains(buf.String(), expected)
		}) {
			t.Fatalf("expected:\n\n%q\n\ngot:\n\n%q", expected, buf.String())
		}
		buf.Reset()

		// add new policy and test
		if err := os.WriteFile(path.Join(root, "policy2.rego"), []byte("package bar\n q := \"hello\""), 0644); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path.Join(root, "policy2_test.rego"), []byte("package bar\n test_q { q == \"hello\" }"), 0644); err != nil {
			t.Fatal(err)
		}

		expected = `PASS: 2/2
********************************************************************************
Watching for changes ...
`
		if !test.Eventually(t, 2*time.Second, func() bool {
			expected := strings.ReplaceAll(expected, "%ROOT%", root)
			return strings.Contains(buf.String(), expected)
		}) {
			t.Fatalf("expected:\n\n%q\n\ngot:\n\n%q", expected, buf.String())
		}
		buf.Reset()

		testParams.stopChan <- syscall.SIGINT
		done <- struct{}{}
	})
}

func TestWatchModeWithDataFile(t *testing.T) {

	files := map[string]string{
		"/policy.rego": "package foo\n test_p { data.y == 1 }",
		"/data.json":   `{"y": 1}`,
	}

	test.WithTempFS(files, func(root string) {
		buf := test.BlockingWriter{}

		testParams := newTestCommandParams()
		testParams.output = &buf
		testParams.watch = true
		testParams.count = 1

		done := make(chan struct{})
		go func() {
			_, _ = opaTest([]string{root}, testParams)
			<-done
		}()

		expected := "Watching for changes ..."
		if !test.Eventually(t, 2*time.Second, func() bool {
			return strings.Contains(buf.String(), expected)
		}) {
			t.Fatalf("expected:\n\n%q\n\ngot:\n\n%q", expected, buf.String())
		}
		buf.Reset()

		// update the data
		f, _ := os.OpenFile(path.Join(root, "data.json"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		_, err := f.WriteString(`{"y": 2}`)
		if err != nil {
			t.Fatal(err)
		}
		f.Close()

		r := regexp.MustCompile(`FAIL \(.*s\)`)
		expected = `%ROOT%/policy.rego:
data.foo.test_p: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
********************************************************************************
Watching for changes ...
`
		if !test.Eventually(t, 2*time.Second, func() bool {
			actual := r.ReplaceAllString(buf.String(), "FAIL (%TIME%)")
			expected := strings.ReplaceAll(expected, "%ROOT%", root)
			return strings.Contains(actual, expected)
		}) {
			t.Fatalf("expected:\n\n%q\n\ngot:\n\n%q", expected, buf.String())
		}
		buf.Reset()

		// update policy so test passes
		f, _ = os.OpenFile(path.Join(root, "policy.rego"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		_, err = f.WriteString("package foo\n test_p { data.y == 2 }")
		if err != nil {
			t.Fatal(err)
		}

		f.Close()

		expected = `PASS: 1/1
********************************************************************************
Watching for changes ...
`

		if !test.Eventually(t, 2*time.Second, func() bool {
			expected := strings.ReplaceAll(expected, "%ROOT%", root)
			return strings.Contains(buf.String(), expected)
		}) {
			t.Fatalf("expected:\n\n%q\n\ngot:\n\n%q", expected, buf.String())
		}
		buf.Reset()

		testParams.stopChan <- syscall.SIGINT
		done <- struct{}{}
	})
}

func TestWatchModeWhenDataFileRemoved(t *testing.T) {
	files := map[string]string{
		"/policy.rego": "package foo\n test_p { data.y == 1 }",
		"/data.json":   `{"y": 1}`,
	}

	test.WithTempFS(files, func(root string) {
		buf := test.BlockingWriter{}

		testParams := newTestCommandParams()
		testParams.output = &buf
		testParams.watch = true
		testParams.count = 1

		done := make(chan struct{})
		go func() {
			_, _ = opaTest([]string{root}, testParams)
			<-done
		}()

		expected := "Watching for changes ..."
		if !test.Eventually(t, 2*time.Second, func() bool {
			return strings.Contains(buf.String(), expected)
		}) {
			t.Fatalf("expected:\n\n%q\n\ngot:\n\n%q", expected, buf.String())
		}
		buf.Reset()

		// update the data
		f, _ := os.OpenFile(path.Join(root, "data.json"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		_, err := f.WriteString(`{"y": 2}`)
		if err != nil {
			t.Fatal(err)
		}
		f.Close()

		r := regexp.MustCompile(`FAIL \(.*s\)`)
		expected = `%ROOT%/policy.rego:
data.foo.test_p: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
********************************************************************************
Watching for changes ...
`
		if !test.Eventually(t, 2*time.Second, func() bool {
			actual := r.ReplaceAllString(buf.String(), "FAIL (%TIME%)")
			expected := strings.ReplaceAll(expected, "%ROOT%", root)
			return strings.Contains(actual, expected)
		}) {
			t.Fatalf("expected:\n\n%q\n\ngot:\n\n%q", expected, buf.String())
		}
		buf.Reset()

		// update the data back to the original state, so the opa test passes
		f, _ = os.OpenFile(path.Join(root, "data.json"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		_, err = f.WriteString(`{"y": 1}`)
		if err != nil {
			t.Fatal(err)
		}
		f.Close()

		expected = `PASS: 1/1
********************************************************************************
Watching for changes ...
`

		if !test.Eventually(t, 2*time.Second, func() bool {
			expected := strings.ReplaceAll(expected, "%ROOT%", root)
			return strings.Contains(buf.String(), expected)
		}) {
			t.Fatalf("expected:\n\n%q\n\ngot:\n\n%q", expected, buf.String())
		}
		buf.Reset()

		// remove the data file, check that test fails afterward
		err = os.Remove(path.Join(root, "data.json"))
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(500 * time.Millisecond)

		testParams.stopChan <- syscall.SIGINT
		done <- struct{}{}
	})
}

func TestWatchModeBrokenFileRecovery(t *testing.T) {

	tests := []struct {
		note           string
		fileName       string
		brokenFile     string
		fixedFile      string
		expectedOutput string
	}{
		{
			note:      "empty data file (EOF read by watcher)",
			fileName:  "data.json",
			fixedFile: `{"foo": "bar"}`,
			expectedOutput: `1 error occurred during loading: %ROOT%/data.json: EOF
********************************************************************************
Watching for changes ...`,
		},
		{
			note:       "broken policy",
			fileName:   "broken_policy.rego",
			brokenFile: "package foo\n bar {",
			fixedFile:  "package foo\n bar {true}",
			expectedOutput: `1 error occurred during loading: %ROOT%/broken_policy.rego:2: rego_parse_error: unexpected eof token
	 bar {
	     ^
********************************************************************************
Watching for changes ...`,
		},
	}

	files := map[string]string{
		"/policy.rego":      "package foo\n p := 1",
		"/policy_test.rego": "package foo\n test_p { p == 1 }",
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(files, func(root string) {
				buf := test.BlockingWriter{}

				testParams := newTestCommandParams()
				testParams.output = &buf
				testParams.watch = true
				testParams.count = 1
				testParams.errOutput = io.Discard

				done := make(chan struct{})
				go func() {
					_, _ = opaTest([]string{root}, testParams)
					<-done
				}()

				expected := "Watching for changes ..."
				if !test.Eventually(t, 2*time.Second, func() bool {
					return strings.Contains(buf.String(), expected)
				}) {
					t.Fatalf("expected:\n\n%q\n\ngot:\n\n%q", expected, buf.String())
				}
				buf.Reset()

				// create broken (possibly empty) file
				f, _ := os.OpenFile(path.Join(root, tc.fileName), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
				if len(tc.brokenFile) > 0 {
					_, err := f.WriteString(tc.brokenFile)
					if err != nil {
						t.Fatal(err)
					}
				}
				f.Close()

				if !test.Eventually(t, 2*time.Second, func() bool {
					expected := strings.ReplaceAll(tc.expectedOutput, "%ROOT%", root)
					return strings.Contains(buf.String(), expected)
				}) {
					t.Fatalf("expected:\n\n%q\n\ngot:\n\n%q", tc.expectedOutput, buf.String())
				}
				buf.Reset()

				// write data to empty file
				f, _ = os.OpenFile(path.Join(root, tc.fileName), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
				_, err := f.WriteString(tc.fixedFile)
				if err != nil {
					t.Fatal(err)
				}
				f.Close()

				expected = "Watching for changes ..."
				if !test.Eventually(t, 2*time.Second, func() bool {
					expected := strings.ReplaceAll(expected, "%ROOT%", root)
					return strings.Contains(buf.String(), expected)
				}) {
					t.Fatalf("expected:\n\n%q\n\ngot:\n\n%q", expected, buf.String())
				}
				buf.Reset()

				testParams.stopChan <- syscall.SIGINT
				done <- struct{}{}
			})
		})
	}
}

func testExitCode(rego string, skipExitZero bool) (int, error) {
	files := map[string]string{
		"test.rego": rego,
	}

	var exitCode int
	var err error
	test.WithTempFS(files, func(path string) {
		regoFilePath := filepath.Join(path, "test.rego")

		testParams := newTestCommandParams()
		testParams.count = 1
		testParams.skipExitZero = skipExitZero
		testParams.errOutput = io.Discard
		testParams.output = io.Discard

		exitCode, err = opaTest([]string{regoFilePath}, testParams)
	})
	return exitCode, err
}

func TestExitCode(t *testing.T) {
	testCases := map[string]struct {
		Test              string
		ExitZeroOnSkipped bool
		ExpectedExitCode  int
	}{
		"pass when no failed or skipped tests": {
			Test: `package foo
			test_pass { true }
			`,
			ExitZeroOnSkipped: false,
			ExpectedExitCode:  0,
		},
		"fail when failed tests": {
			Test: `package foo
			test_pass { true }
			test_fail { false }
			`,
			ExitZeroOnSkipped: false,
			ExpectedExitCode:  2,
		},
		"fail when skipped tests": {
			Test: `package foo
			test_pass { true }
			todo_test_skip { true }
			`,
			ExitZeroOnSkipped: false,
			ExpectedExitCode:  2,
		},
		"fail when failed tests and skipped tests": {
			Test: `package foo
			test_pass { true }
			test_fail { false }
			todo_test_skip { true }
			`,
			ExitZeroOnSkipped: false,
			ExpectedExitCode:  2,
		},
		"pass when skipped tests and exit zero on skipped": {
			Test: `package foo
			test_pass { true }
			todo_test_skip { true }
			`,
			ExitZeroOnSkipped: true,
			ExpectedExitCode:  0,
		},
		"fail when failed tests and exit zero on skipped": {
			Test: `package foo
			test_pass { true }
			test_fail { false }
			`,
			ExitZeroOnSkipped: true,
			ExpectedExitCode:  2,
		},
		"fail when failed tests, skipped tests and exit zero on skipped": {
			Test: `package foo
			test_pass { true }
			test_fail { false }
			todo_test_skip { true }
			`,
			ExitZeroOnSkipped: true,
			ExpectedExitCode:  2,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			exitCode, _ := testExitCode(tc.Test, tc.ExitZeroOnSkipped)

			if exitCode != tc.ExpectedExitCode {
				t.Errorf("Expected exit code to be %d but got %d", tc.ExpectedExitCode, exitCode)
			}
		})
	}
}

func TestCoverageThreshold(t *testing.T) {
	testCases := []struct {
		note              string
		modules           map[string]string
		threshold         float64
		verbose           bool
		expectedErrOutput string
		expectedExitCode  int
	}{
		{
			note: "coverage threshold met",
			modules: map[string]string{
				"test.rego": `package test
					p := 1
					test_p { p == 1 }`,
			},
			expectedExitCode: 0,
		},
		{
			note: "coverage threshold not met",
			modules: map[string]string{
				"test.rego": `package test
					p := 1 {
						1 == 1
					}
					q := 2
					r := 3
					test_q { q == 2 }`,
			},
			threshold:         100,
			expectedExitCode:  2,
			expectedErrOutput: "Code coverage threshold not met: got 40.00 instead of 100.00\n",
		},
		{
			note: "coverage threshold not met (verbose)",
			modules: map[string]string{
				"test.rego": `package test
					p := 1 {
						1 == 1
					}
					q := 2
					r := 3
					test_q { q == 2 }`,
			},
			threshold:        100,
			expectedExitCode: 2,
			verbose:          true,
			expectedErrOutput: `Code coverage threshold not met: got 40.00 instead of 100.00
Lines not covered:
	%ROOT%/test.rego:2-3
	%ROOT%/test.rego:6
`,
		},
		{
			note: "coverage threshold not met (verbose, multiple files)",
			modules: map[string]string{
				"policy1.rego": `package test
					p := 1 {
						1 == 1
					}
					q := 2
					r := 3`,
				"policy2.rego": `package test
					s := 4 {
						1 == 1
						2 == 2
					}
					t := 5
					u := 6
					v := 7`,
				"test.rego": `package test
					test_q { q == 2 }
					test_t { t == 5 }`,
			},
			threshold:        100,
			expectedExitCode: 2,
			verbose:          true,
			expectedErrOutput: `Code coverage threshold not met: got 33.33 instead of 100.00
Lines not covered:
	%ROOT%/policy1.rego:2-3
	%ROOT%/policy1.rego:6
	%ROOT%/policy2.rego:2-4
	%ROOT%/policy2.rego:7-8
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(tc.modules, func(root string) {
				var buf bytes.Buffer

				testParams := newTestCommandParams()
				testParams.threshold = tc.threshold
				testParams.verbose = tc.verbose
				testParams.count = 1
				testParams.errOutput = &buf

				exitCode, _ := opaTest([]string{root}, testParams)
				if exitCode != tc.expectedExitCode {
					t.Fatalf("unexpected exit code: %d", exitCode)
				}

				if len(tc.expectedErrOutput) == 0 && buf.Len() > 0 {
					t.Fatalf("expected no error output but got:\n\n%q", buf.String())
				}

				expectedErrOutput := strings.ReplaceAll(tc.expectedErrOutput, "%ROOT%", root)
				if buf.String() != expectedErrOutput {
					t.Fatalf("expected error output to contain:\n\n%q\n\nbut got:\n\n%q", expectedErrOutput, buf.String())
				}
			})
		})
	}
}

type loadType int

const (
	loadFile loadType = iota
	loadBundle
	loadTarball
)

func (t loadType) String() string {
	return [...]string{"file", "bundle", "bundle tarball"}[t]
}

func TestWithV1CompatibleFlag(t *testing.T) {
	tests := []struct {
		note         string
		v1Compatible bool
		files        map[string]string
		expErr       string
	}{
		{
			note: "0.x module, no imports",
			files: map[string]string{
				"/test.rego": `package test

l1 := {1, 3, 5}
l2 contains v if {
	v := l1[_]
}

test_l if {
	l1 == l2
}`,
			},
			expErr: "rego_parse_error",
		},
		{
			note: "0.x module, rego.v1 imported",
			files: map[string]string{
				"/test.rego": `package test

import rego.v1

l1 := {1, 3, 5}
l2 contains v if {
	v := l1[_]
}

test_l if {
	l1 == l2
}`,
			},
		},
		{
			note: "0.x module, future.keywords imported",
			files: map[string]string{
				"/test.rego": `package test

import future.keywords

l1 := {1, 3, 5}
l2 contains v if {
	v := l1[_]
}

test_l if {
	l1 == l2
}`,
			},
		},

		{
			note:         "1.0 compatible module, no imports",
			v1Compatible: true,
			files: map[string]string{
				"/test.rego": `package test

l1 := {1, 3, 5}
l2 contains v if {
	v := l1[_]
}

test_l if {
	l1 == l2
}`,
			},
		},
		{
			note:         "1.0 compatible module, rego.v1 imported",
			v1Compatible: true,
			files: map[string]string{
				"/test.rego": `package test

import rego.v1

l1 := {1, 3, 5}
l2 contains v if {
	v := l1[_]
}

test_l if {
	l1 == l2
}`,
			},
		},
		{
			note:         "1.0 compatible module, future.keywords imported",
			v1Compatible: true,
			files: map[string]string{
				"/test.rego": `package test

import future.keywords

l1 := {1, 3, 5}
l2 contains v if {
	v := l1[_]
}

test_l if {
	l1 == l2
}`,
			},
		},
	}

	loadTypes := []loadType{loadFile, loadBundle, loadTarball}

	for _, tc := range tests {
		for _, loadType := range loadTypes {
			t.Run(fmt.Sprintf("%s (%s)", tc.note, loadType), func(t *testing.T) {
				var files map[string]string
				if loadType != loadTarball {
					files = tc.files
				}
				test.WithTempFS(files, func(root string) {
					if loadType == loadTarball {
						f, err := os.Create(filepath.Join(root, "bundle.tar.gz"))
						if err != nil {
							t.Fatal(err)
						}

						testBundle := bundle.Bundle{
							Data: map[string]interface{}{},
						}
						for k, v := range tc.files {
							testBundle.Modules = append(testBundle.Modules, bundle.ModuleFile{
								Path: k,
								Raw:  []byte(v),
							})
						}

						if err := bundle.Write(f, testBundle); err != nil {
							t.Fatal(err)
						}
					}

					var buf bytes.Buffer
					var errBuf bytes.Buffer

					testParams := newTestCommandParams()
					testParams.v1Compatible = tc.v1Compatible
					testParams.bundleMode = loadType == loadBundle
					testParams.count = 1
					testParams.output = &buf
					testParams.errOutput = &errBuf

					var paths []string
					if loadType == loadTarball {
						paths = []string{filepath.Join(root, "bundle.tar.gz")}
					} else {
						paths = []string{root}
					}

					exitCode, _ := opaTest(paths, testParams)
					if tc.expErr != "" {
						if exitCode == 0 {
							t.Fatalf("expected non-zero exit code")
						}

						if actual := errBuf.String(); !strings.Contains(actual, tc.expErr) {
							t.Fatalf("expected error output to contain:\n\n%q\n\nbut got:\n\n%q", tc.expErr, actual)
						}
					} else {
						if exitCode != 0 {
							t.Fatalf("unexpected exit code: %d", exitCode)
						}

						if errBuf.Len() > 0 {
							t.Fatalf("expected no error output but got:\n\n%q", buf.String())
						}

						expected := "PASS: 1/1"
						if actual := buf.String(); !strings.Contains(actual, expected) {
							t.Fatalf("expected output to contain:\n\n%s\n\nbut got:\n\n%q", expected, actual)
						}
					}
				})
			})
		}
	}
}
