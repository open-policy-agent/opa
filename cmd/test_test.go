package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

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
	verifyFilteredTrace(t, p, expected)
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
	verifyFilteredTrace(t, p, expected)
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
	verifyFilteredTrace(t, p, expected)
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
	verifyFilteredTrace(t, p, expected)
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
	verifyFilteredTrace(t, p, expected)
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

func testSchemasAnnotation(mod string) error {
	query := "data.test.test_p"

	files := map[string]string{
		"test.rego": mod,
	}

	var err error
	test.WithTempFS(files, func(path string) {
		_, err = rego.New(
			rego.Module("test.rego", mod),
			rego.Trace(true),
			rego.Query(query),
		).Eval(context.Background())
	})

	return err
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

	err := testSchemasAnnotation(policyWithSchemaRef)
	if err != nil {
		t.Fatalf("unexpected error when schema ref is present: %v", err)
	}

	policyWithInlinedSchema := `
package test
# METADATA
# schemas:
#   - input.foo: {"type": "boolean"}
p { 
	rego.metadata.rule() # presence of rego.metadata.* calls must not trigger unwanted schema evaluation 
	input.foo == 42 # type mismatch with schema that should be ignored
}

test_p {
    p with input.foo as 42
}`

	err = testSchemasAnnotation(policyWithInlinedSchema)
	if err == nil {
		t.Fatalf("didn't get expected error when inlined schema is present")
	} else if !strings.Contains(err.Error(), "rego_type_error: match error") {
		t.Fatalf("didn't get expected %s error when inlined schema is present; got: %v", ast.TypeErr, err)
	}
}
