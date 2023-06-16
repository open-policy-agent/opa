package cmd

import (
	"bytes"
	"context"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
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
		var buf bytes.Buffer

		testParams := newTestCommandParams()
		testParams.output = &buf
		testParams.watch = true
		testParams.count = 1

		done := make(chan struct{})
		go func() {
			_, _ = opaTest([]string{root}, testParams)
			<-done
		}()

		time.Sleep(500 * time.Millisecond)

		// update the test
		f, _ := os.OpenFile(path.Join(root, "policy_test.rego"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		_, err := f.WriteString("package foo\n test_p { p == 2 }")
		if err != nil {
			t.Fatal(err)
		}
		f.Close()

		time.Sleep(500 * time.Millisecond)

		// update policy so test passes
		f, _ = os.OpenFile(path.Join(root, "policy.rego"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		_, err = f.WriteString("package foo\n p := 2")
		if err != nil {
			t.Fatal(err)
		}

		f.Close()

		time.Sleep(500 * time.Millisecond)

		// add new policy and test
		if err := os.WriteFile(path.Join(root, "policy2.rego"), []byte("package bar\n q := \"hello\""), 0644); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path.Join(root, "policy2_test.rego"), []byte("package bar\n test_q { q == \"hello\" }"), 0644); err != nil {
			t.Fatal(err)
		}

		time.Sleep(500 * time.Millisecond)

		testParams.stopChan <- syscall.SIGINT
		done <- struct{}{}

		expected := `********************************************************************************
Watching for changes ...
%ROOT%/policy_test.rego:
data.foo.test_p: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
********************************************************************************
Watching for changes ...
`

		r := regexp.MustCompile(`FAIL \(.*s\)`)
		actual := r.ReplaceAllString(buf.String(), "FAIL (%TIME%)")
		expected = strings.ReplaceAll(expected, "%ROOT%", root)

		if !strings.Contains(actual, expected) {
			t.Fatalf("Expected:\n\n%s\n\nGot:\n\n%s\n\n", expected, actual)
		}

		// verify output after policy update and new added policy
		expected = "PASS: 2/2\n********************************************************************************\nWatching for changes ..."

		if !strings.Contains(buf.String(), expected) {
			t.Fatalf("Expected:%s in result but \n\nGot:\n\n%s\n\n", expected, buf.String())
		}
	})
}

func TestWatchModeWithDataFile(t *testing.T) {

	files := map[string]string{
		"/policy.rego": "package foo\n test_p { data.y == 1 }",
		"/data.json":   `{"y": 1}`,
	}

	test.WithTempFS(files, func(root string) {
		var buf bytes.Buffer

		testParams := newTestCommandParams()
		testParams.output = &buf
		testParams.watch = true
		testParams.count = 1

		done := make(chan struct{})
		go func() {
			_, _ = opaTest([]string{root}, testParams)
			<-done
		}()

		time.Sleep(500 * time.Millisecond)

		// update the data
		f, _ := os.OpenFile(path.Join(root, "data.json"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		_, err := f.WriteString(`{"y": 2}`)
		if err != nil {
			t.Fatal(err)
		}
		f.Close()

		time.Sleep(500 * time.Millisecond)

		// update policy so test passes
		f, _ = os.OpenFile(path.Join(root, "policy.rego"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		_, err = f.WriteString("package foo\n test_p { data.y == 2 }")
		if err != nil {
			t.Fatal(err)
		}

		f.Close()

		time.Sleep(500 * time.Millisecond)

		testParams.stopChan <- syscall.SIGINT
		done <- struct{}{}

		expected := `********************************************************************************
Watching for changes ...
%ROOT%/policy.rego:
data.foo.test_p: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
********************************************************************************
Watching for changes ...
`

		r := regexp.MustCompile(`FAIL \(.*s\)`)
		actual := r.ReplaceAllString(buf.String(), "FAIL (%TIME%)")
		expected = strings.ReplaceAll(expected, "%ROOT%", root)

		if !strings.Contains(actual, expected) {
			t.Fatalf("Expected:\n\n%s\n\nGot:\n\n%s\n\n", expected, actual)
		}

		// verify output after policy update
		expected = "PASS: 1/1\n********************************************************************************\nWatching for changes ..."

		if !strings.Contains(buf.String(), expected) {
			t.Fatalf("Expected:%s in result but \n\nGot:\n\n%s\n\n", expected, buf.String())
		}
	})
}

func TestWatchModeWhenDataFileRemoved(t *testing.T) {
	files := map[string]string{
		"/policy.rego": "package foo\n test_p { data.y == 1 }",
		"/data.json":   `{"y": 1}`,
	}

	test.WithTempFS(files, func(root string) {
		var buf bytes.Buffer

		testParams := newTestCommandParams()
		testParams.output = &buf
		testParams.watch = true
		testParams.count = 1

		done := make(chan struct{})
		go func() {
			_, _ = opaTest([]string{root}, testParams)
			<-done
		}()

		time.Sleep(500 * time.Millisecond)

		// update the data
		f, _ := os.OpenFile(path.Join(root, "data.json"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		_, err := f.WriteString(`{"y": 2}`)
		if err != nil {
			t.Fatal(err)
		}
		f.Close()

		time.Sleep(500 * time.Millisecond)

		// update the data back to the original state, so the opa test passes
		f, _ = os.OpenFile(path.Join(root, "data.json"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		_, err = f.WriteString(`{"y": 1}`)
		if err != nil {
			t.Fatal(err)
		}
		f.Close()

		time.Sleep(500 * time.Millisecond)

		// remove the data file, check that test fails afterward
		err = os.Remove(path.Join(root, "data.json"))
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(500 * time.Millisecond)

		testParams.stopChan <- syscall.SIGINT
		done <- struct{}{}

		expected := `PASS: 1/1
********************************************************************************
Watching for changes ...
%ROOT%/policy.rego:
data.foo.test_p: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
********************************************************************************
Watching for changes ...
`

		r := regexp.MustCompile(`FAIL \(.*s\)`)
		actual := r.ReplaceAllString(buf.String(), "FAIL (%TIME%)")
		expected = strings.ReplaceAll(expected, "%ROOT%", root)

		if !strings.Contains(actual, expected) {
			t.Fatalf("Expected:\n\n%s\n\nGot:\n\n%s\n\n", expected, actual)
		}
	})
}
