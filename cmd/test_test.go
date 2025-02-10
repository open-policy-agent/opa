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

	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/util/test"
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
	
	p if {
		x  # Always true
		trace("test test")
		q["foo"]
	}
	
	x if {
		y
	}
	
	y if {
		true
	}
	
	q contains x if {
		some x
		trace("got this far")
		r[x]
		trace("got this far1")
	}
	
	r contains x if {
		trace("got this far2")
		x := data.x
	}
	
	test_p if {
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

func TestPrettyTraceWithLocalVars(t *testing.T) {
	tests := []struct {
		note        string
		includeVars bool
		files       map[string]string
		expected    string
	}{
		{
			note:        "without vars",
			includeVars: false,
			files: map[string]string{
				"test.rego": `package test

test_p if {
	x := 1
	y := 2
	z := 3
	x == z + y
} 
`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_p: FAIL (%.*%)

  query:1 %.*%           Enter data.test.test_p = _  
  query:1 %.*%           | Eval data.test.test_p = _  
  query:1 %.*%           | Index data.test.test_p (matched 1 rule, early exit)  
  %.*%/test.rego:3       | Enter data.test.test_p  
  %.*%/test.rego:4       | | Eval x = 1  
  %.*%/test.rego:5       | | Eval y = 2  
  %.*%/test.rego:6       | | Eval z = 3  
  %.*%/test.rego:7       | | Eval plus(z, y, __local3__)  
  %.*%/test.rego:7       | | Eval x = __local3__  
  %.*%/test.rego:7       | | Fail x = __local3__  
  %.*%/test.rego:7       | | Redo plus(z, y, __local3__)  
  %.*%/test.rego:6       | | Redo z = 3  
  %.*%/test.rego:5       | | Redo y = 2  
  %.*%/test.rego:4       | | Redo x = 1  
  query:1 %.*%           | Fail data.test.test_p = _  

SUMMARY
--------------------------------------------------------------------------------
%.*%/test.rego:
data.test.test_p: FAIL (%.*%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note:        "with vars",
			includeVars: true,
			files: map[string]string{
				"test.rego": `package test

test_p if {
	x := 1
	y := 2
	z := 3
	x == z + y
} 
`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_p: FAIL (%.*%)

  query:1 %.*%           Enter data.test.test_p = _                                  {}  
  query:1 %.*%           | Eval data.test.test_p = _                                 {}  
  query:1 %.*%           | Index data.test.test_p (matched 1 rule, early exit)       {}  
  %.*%/test.rego:3       | Enter data.test.test_p                                    {}  
  %.*%/test.rego:4       | | Eval x = 1                                              {}  
  %.*%/test.rego:5       | | Eval y = 2                                              {}  
  %.*%/test.rego:6       | | Eval z = 3                                              {}  
  %.*%/test.rego:7       | | Eval plus(z, y, __local3__)                             {y: 2, z: 3}  
  %.*%/test.rego:7       | | Eval x = __local3__                                     {__local3__: 5, x: 1}  
  %.*%/test.rego:7       | | Fail x = __local3__                                     {__local3__: 5, x: 1}  
  %.*%/test.rego:7       | | Redo plus(z, y, __local3__)                             {__local3__: 5, y: 2, z: 3}  
  %.*%/test.rego:6       | | Redo z = 3                                              {z: 3}  
  %.*%/test.rego:5       | | Redo y = 2                                              {y: 2}  
  %.*%/test.rego:4       | | Redo x = 1                                              {x: 1}  
  query:1   %.*%         | Fail data.test.test_p = _                                 {}  

  %.*%/test.rego:7:
    	x == z + y
    	|    |   |
    	|    |   2
    	|    z + y: 5
    	|    z: 3
    	1

SUMMARY
--------------------------------------------------------------------------------
%.*%/test.rego:
data.test.test_p: FAIL (%.*%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(tc.files, func(root string) {
				buf := new(bytes.Buffer)
				testParams := newTestCommandParams()
				testParams.count = 1
				testParams.output = buf
				testParams.errOutput = io.Discard
				testParams.bundleMode = true
				testParams.verbose = true
				testParams.varValues = tc.includeVars
				_ = testParams.explain.Set(explainModeFull)

				_, err := opaTest([]string{root}, testParams)
				if err != nil {
					t.Fatalf("Unexpected error: %s", err)
				}

				actual := buf.String()
				if !stringsMatch(t, tc.expected, actual) {
					t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", tc.expected, actual)
				}
			})
		})
	}
}

func TestFailVarValues(t *testing.T) {
	tests := []struct {
		note     string
		files    map[string]string
		expected string
	}{
		{
			note: "simple",
			files: map[string]string{
				"/test.rego": `package test

test_foo if {
	x := 1
	y := 2
	z := 3
	x == y + z
}
`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:7:
    	x == y + z
    	|    |   |
    	|    |   3
    	|    y + z: 5
    	|    y: 2
    	1

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "simple (not)",
			files: map[string]string{
				"/test.rego": `package test

test_foo if {
	x := 5
	y := 2
	z := 3
	not x == y + z
}
`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:7:
    	not x == y + z
    	    |    |   |
    	    |    |   3
    	    |    y + z: 5
    	    |    y: 2
    	    5

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "array",
			files: map[string]string{
				"/test.rego": `package test

test_foo if {
	x := 1
	y := [1, 2, 3]
	z := 3
	x == y[2] + z
}
`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:7:
    	x == y[2] + z
    	|    |      |
    	|    |      3
    	|    y[2] + z: 6
    	|    y[2]: 3
    	|    y: [1, 2, 3]
    	1

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "array, var key",
			files: map[string]string{
				"/test.rego": `package test

test_foo if {
	x := 1
	y := [1, 2, 3]
	z := 3
	i := 2
	x == y[i] + z
}
`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:8:
    	x == y[i] + z
    	|    | |    |
    	|    | |    3
    	|    | 2
    	|    y[i] + z: 6
    	|    y[i]: 3
    	|    y: [1, 2, 3]
    	1

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "array containing vars",
			files: map[string]string{
				"/test.rego": `package test

test_foo if {
	x := 1
	y := 2
	z := 3
	[x, y, z] == [4, 5, 6]
}
`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:7:
    	[x, y, z] == [4, 5, 6]
    	 |  |  |
    	 |  |  3
    	 |  2
    	 1

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "array containing refs",
			files: map[string]string{
				"/test.rego": `package test

a := 1

b := 2

test_foo if {
	[a, data.test.b, data.c] == [4, 5, 6]
}
`,
				"data.json": `{"c": 3}`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:8:
    	[a, data.test.b, data.c] == [4, 5, 6]
    	 |  |            |
    	 |  |            3
    	 |  2
    	 1

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "array containing refs, undefined",
			files: map[string]string{
				"/test.rego": `package test

a := 1

b := data.b

test_foo if {
	[a, b, data.c] == [4, 5, 6]
}
`,
				"data.json": `{"c": 3}`,
			},
			// Note: each dynamic array element is broken out into a separate "co-expression" by the compiler.
			// Since we failed on the 2nd element (b), we don't have value for the 3rd element (data.c).
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:8:
    	[a, b, data.c] == [4, 5, 6]
    	 |  |
    	 |  undefined
    	 1

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "nested collections containing vars",
			files: map[string]string{
				"/test.rego": `package test

test_foo if {
	x := 1
	y := 2
	z := 3
	[x, {y, {"a": z}}] == [4, {5, {"a": 6}}]
}
`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:7:
    	[x, {y, {"a": z}}] == [4, {5, {"a": 6}}]
    	 |   |        |
    	 |   |        3
    	 |   2
    	 1

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "single line expression containing tabs",
			files: map[string]string{
				"/test.rego": `package test

	test_foo if {
		x := 1
		y := 2
		z := 3
		x == y +	z
	}
`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:7:
    		x == y +	z
    		|    |  	|
    		|    |  	3
    		|    y +	z: 5
    		|    y: 2
    		1

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "single line expression containing tabs #2",
			files: map[string]string{
				"/test.rego": `package test

	test_foo if {
		x := 1
		y := 2
		z := 3
		x	== y +	z
	}
`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:7:
    		x	== y +	z
    		|	   |  	|
    		|	   |  	3
    		|	   y +	z: 5
    		|	   y: 2
    		1

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "multi-line expression containing tabs",
			files: map[string]string{
				"/test.rego": `package test

	test_foo if {
		x := 1
		y := 2
		z := 3
		obj := {
			"foo_": 1,
			"bar__": 42,
			"baz": 3,
		}
		obj == {
			"foo_":		x,
			"bar__":	y,
			"baz":		z,
		}
	}
`,
			},
			// We can't deal with tabs in a consistent manner when they occur on multiple lines
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:12:
    		obj == {
    			"foo_":		x,
    			"bar__":	y,
    			"baz":		z,
    		}
    
    Where:
    
    obj: {"bar__": 42, "baz": 3, "foo_": 1}
    x: 1
    y: 2
    z: 3

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "composite rule",
			files: map[string]string{
				"/test.rego": `package test

p contains v if {
	some v in numbers.range(1, 3)
}

test_p if {
	p == {4, 5, 6}
}`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_p: FAIL (%TIME%)

  %ROOT%/test.rego:8:
    	p == {4, 5, 6}
    	|
    	{1, 2, 3}

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_p: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "composite rule with ref-head",
			files: map[string]string{
				"/test.rego": `package test

p.q contains v if {
	some v in numbers.range(1, 3)
}

test_p if {
	p.q == {4, 5, 6}
}`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_p: FAIL (%TIME%)

  %ROOT%/test.rego:8:
    	p.q == {4, 5, 6}
    	|
    	{1, 2, 3}

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_p: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "composite rule with ref-head, partial ref",
			files: map[string]string{
				"/test.rego": `package test

p.q contains v if {
	some v in numbers.range(1, 3)
}

test_p if {
	p == {
		"q": {4, 5, 6}
	}
}`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_p: FAIL (%TIME%)

  %ROOT%/test.rego:8:
    	p == {
    		"q": {4, 5, 6}
    	}
    	|
    	{"q": {1, 2, 3}}

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_p: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "composite rules with ref-head, composite value",
			files: map[string]string{
				"/test.rego": `package test

p.q contains v if {
	some v in numbers.range(1, 3)
}

p.r := "foo"

test_p if {
	p == {
		"q": {4, 5, 6},
		"r": "bar"
	}
}`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_p: FAIL (%TIME%)

  %ROOT%/test.rego:10:
    	p == {
    		"q": {4, 5, 6},
    		"r": "bar"
    	}
    	|
    	{"q": {1, 2, 3}, "r": "foo"}

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_p: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "refs in different compiled sub-expressions",
			files: map[string]string{
				"/test.rego": `package test

a := 1
b := 2
c := 3

test_p if {
	# This expression is split into multiple final expressions by the compiler, each containing a rule ref
	a == b + c
}
`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_p: FAIL (%TIME%)

  %ROOT%/test.rego:9:
    	a == b + c
    	|    |   |
    	|    |   3
    	|    b + c: 5
    	|    b: 2
    	1

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_p: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "rule not defined",
			files: map[string]string{
				"/test.rego": `package test

p if {
	input.x == 1
}

test_p if {
	p with input.x as 2
}`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_p: FAIL (%TIME%)

  %ROOT%/test.rego:8:
    	p with input.x as 2
    	|
    	undefined

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_p: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "rule defined (not)",
			files: map[string]string{
				"/test.rego": `package test

p if {
	input.x == 1
}

test_p if {
	not p with input.x as 1
}`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_p: FAIL (%TIME%)

  %ROOT%/test.rego:8:
    	not p with input.x as 1
    	    |
    	    true

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_p: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "data ref",
			files: map[string]string{
				"/test.rego": `package test

test_foo if {
	y := 1
	data.x == y
}
`,
				"data.json": `{"x": 2}`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:5:
    	data.x == y
    	|         |
    	|         1
    	2

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "data + virtual extent ref",
			files: map[string]string{
				"/test.rego": `package test

foo.x := 1

test_foo if {
	y := {"x": 1, "y": 42}
	foo == y
}
`,
				"data.json": `{"test": {"foo": {"y": 2}}}`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:7:
    	foo == y
    	|      |
    	|      {"x": 1, "y": 42}
    	{"x": 1, "y": 2}

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "in (array)",
			files: map[string]string{
				"/test.rego": `package test

test_foo if {
	l := ["a", "b", "c"]
	x := "q"
	x in l
}
`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:6:
    	x in l
    	|    |
    	|    ["a", "b", "c"]
    	"q"

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "in (set)",
			files: map[string]string{
				"/test.rego": `package test

test_foo if {
	l := {"a", "b", "c"}
	x := "q"
	x in l
}
`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:6:
    	x in l
    	|    |
    	|    {"a", "b", "c"}
    	"q"

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "comprehension (array)",
			files: map[string]string{
				"/test.rego": `package test

test_foo if {
	l := ["a", "b", "c"]
	[x | x := l[_]] == ["d", "e", "f"]
}
`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:5:
    	[x | x := l[_]] == ["d", "e", "f"]
    	|
    	["a", "b", "c"]

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "comprehension (set)",
			files: map[string]string{
				"/test.rego": `package test

test_foo if {
	l := ["a"]
	{x | x := l[_]} == {"b"}
}
`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:5:
    	{x | x := l[_]} == {"b"}
    	|
    	{"a"}

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "comprehension (object)",
			files: map[string]string{
				"/test.rego": `package test

test_foo if {
	l := ["a", "b", "c"]
	{k: x | x := l[k]} == {3: "d", 4: "e", 5: "f"}
}
`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:5:
    	{k: x | x := l[k]} == {3: "d", 4: "e", 5: "f"}
    	|
    	{0: "a", 1: "b", 2: "c"}

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "every",
			files: map[string]string{
				"/test.rego": `package test

test_foo if {
	l := [1, 2, 3]
	every x in l {
		x == 1 
	}
}`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:6:
    		x == 1
    		|
    		2

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "comprehension inside every",
			files: map[string]string{
				"/test.rego": `package test

test_foo if {
	l := [1, 2, 3]
	every x in l {
		[v | v := x] == [42]
	}
}`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:6:
    		[v | v := x] == [42]
    		|
    		[1]

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "nested every",
			files: map[string]string{
				"/test.rego": `package test

test_foo if {
	l := [[1, 2], [3, 4], [5, 6]]
	every x in l {
		every y in x {
			y < 4
		}
	}
}`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:7:
    			y < 4
    			|
    			4

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "nested every with comprehension",
			files: map[string]string{
				"/test.rego": `package test

test_foo if {
	l := [[1, 2], [3, 4], [5, 6]]
	every x in l {
		every y in x {
			[v | v := y] == [42]
		}
	}
}`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:7:
    			[v | v := y] == [42]
    			|
    			[1]

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "ref equality",
			files: map[string]string{
				"/test.rego": `package test

a := 1
b := 2

test_foo if {
	a == b
}`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:7:
    	a == b
    	|    |
    	|    2
    	1

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "ref equality (data)",
			files: map[string]string{
				"/test.rego": `package test

test_foo if {
	data.a == data.b
}`,
				"data.json": `{"a": 1, "b": 2}`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:4:
    	data.a == data.b
    	|         |
    	|         2
    	1

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "with, containing local vars",
			files: map[string]string{
				"/test.rego": `package test

p := input.x

test_p if {
	a := 1
	p == 2 with input.x as a
}`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_p: FAIL (%TIME%)

  %ROOT%/test.rego:7:
    	p == 2 with input.x as a
    	|                      |
    	|                      1
    	1

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_p: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "with, containing ref",
			files: map[string]string{
				"/test.rego": `package test

p := input.x

testInput := {"x": 1}

test_p if {
	p == 2 with input as testInput
}`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_p: FAIL (%TIME%)

  %ROOT%/test.rego:8:
    	p == 2 with input as testInput
    	|                    |
    	|                    {"x": 1}
    	1

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_p: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "negated rule ref",
			files: map[string]string{
				"/test.rego": `package test

a if {true}

test_foo if {
	not a
}`,
				"data.json": `{"a": true}`,
			},
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:6:
    	not a
    	    |
    	    true

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
		{
			note: "negated data ref",
			files: map[string]string{
				"/test.rego": `package test

test_foo if {
	not data.a
}`,
				"data.json": `{"a": true}`,
			},
			// Because of the negated expr, the compiler will have opted out of rewriting the expression to
			// capture the value of data.a in a local variable, and since data.a isn't in the local bindings
			// or in the virtual cache, we don't know if it's undefined or unknown, and therefore can't report
			// on a value.
			expected: `FAILURES
--------------------------------------------------------------------------------
data.test.test_foo: FAIL (%TIME%)

  %ROOT%/test.rego:4:
    	not data.a

SUMMARY
--------------------------------------------------------------------------------
%ROOT%/test.rego:
data.test.test_foo: FAIL (%TIME%)
--------------------------------------------------------------------------------
FAIL: 1/1
`,
		},
	}

	r := regexp.MustCompile(`FAIL \(.*s\)`)
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(tc.files, func(root string) {
				buf := new(bytes.Buffer)
				testParams := newTestCommandParams()
				testParams.count = 1
				testParams.output = buf
				testParams.errOutput = io.Discard
				testParams.bundleMode = true
				testParams.varValues = true
				_ = testParams.explain.Set(explainModeFull)

				_, err := opaTest([]string{root}, testParams)
				if err != nil {
					t.Fatalf("Unexpected error: %s", err)
				}

				actual := r.ReplaceAllString(buf.String(), "FAIL (%TIME%)")
				expected := strings.ReplaceAll(tc.expected, "%ROOT%", root)

				if !stringsMatch(t, expected, actual) {
					t.Fatalf("Expected output to be:\n\n%s\n\ngot:\n\n%s", expected, actual)
				}
			})
		})
	}
}

// Assert that ignore flag is correctly used when the bundle flag is activated
func TestIgnoreFlag(t *testing.T) {
	files := map[string]string{
		"/test.rego": `package test

p := input.foo == 42
test_p if {
	p with input.foo as 42
}`,
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
		"/test.rego": `package test

p := input.foo == 42
test_p if {
	p with input.foo as 42
}`,
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
p if { 
	rego.metadata.rule() # presence of rego.metadata.* calls must not trigger unwanted schema evaluation
	input.foo == 42 # type mismatch with schema that should be ignored
}

test_p if {
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
p if { 
	input.foo == 42 # type mismatch with schema that should NOT be ignored since it is an inlined schema format
}

test_p if {
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
p if {
	input.foo == 42
}

test_p if {
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
p if {
	input.foo == 42
}

test_p if {
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
		"/policy.rego": `package foo
p := 1`,
		"/policy_test.rego": `package foo

test_p if { 
	p == 1
}`,
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
		_, err := f.WriteString("package foo\n test_p if { p == 2 }")
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

		if err := os.WriteFile(path.Join(root, "policy2_test.rego"), []byte("package bar\n test_q if { q == \"hello\" }"), 0644); err != nil {
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

func TestWatchMode_v0(t *testing.T) {

	files := map[string]string{
		"/policy.rego": `package foo
p := 1`,
		"/policy_test.rego": `package foo

test_p { 
	p == 1
}`,
	}

	test.WithTempFS(files, func(root string) {
		buf := test.BlockingWriter{}

		testParams := newTestCommandParams()
		testParams.output = &buf
		testParams.watch = true
		testParams.count = 1
		testParams.v0Compatible = true

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
		"/policy.rego": `package foo

test_p if { 
	data.y == 1
}`,
		"/data.json": `{"y": 1}`,
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
		_, err = f.WriteString("package foo\n test_p if { data.y == 2 }")
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
		"/policy.rego": `package foo

test_p if { 
	data.y == 1 
}`,
		"/data.json": `{"y": 1}`,
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
			fixedFile:  "package foo\n bar if {true}",
			expectedOutput: `1 error occurred during loading: %ROOT%/broken_policy.rego:2: rego_parse_error: unexpected eof token
	 bar {
	     ^
********************************************************************************
Watching for changes ...`,
		},
	}

	files := map[string]string{
		"/policy.rego": `package foo
p := 1`,
		"/policy_test.rego": `package foo

test_p if { 
	p == 1
}`,
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
			
			test_pass if { true }
			`,
			ExitZeroOnSkipped: false,
			ExpectedExitCode:  0,
		},
		"fail when failed tests": {
			Test: `package foo
			
			test_pass if { true }
			test_fail if { false }
			`,
			ExitZeroOnSkipped: false,
			ExpectedExitCode:  2,
		},
		"fail when skipped tests": {
			Test: `package foo
			
			test_pass if { true }
			todo_test_skip if { true }
			`,
			ExitZeroOnSkipped: false,
			ExpectedExitCode:  2,
		},
		"fail when failed tests and skipped tests": {
			Test: `package foo
			
			test_pass if { true }
			test_fail if { false }
			todo_test_skip if { true }
			`,
			ExitZeroOnSkipped: false,
			ExpectedExitCode:  2,
		},
		"pass when skipped tests and exit zero on skipped": {
			Test: `package foo
			
			test_pass if { true }
			todo_test_skip if { true }
			`,
			ExitZeroOnSkipped: true,
			ExpectedExitCode:  0,
		},
		"fail when failed tests and exit zero on skipped": {
			Test: `package foo
			
			test_pass if { true }
			test_fail if { false }
			`,
			ExitZeroOnSkipped: true,
			ExpectedExitCode:  2,
		},
		"fail when failed tests, skipped tests and exit zero on skipped": {
			Test: `package foo
			
			test_pass if { true }
			test_fail if { false }
			todo_test_skip if { true }
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
					test_p if { p == 1 }`,
			},
			expectedExitCode: 0,
		},
		{
			note: "coverage threshold not met",
			modules: map[string]string{
				"test.rego": `package test

					p := 1 if {
						1 == 1
					}
					q := 2
					r := 3
					test_q if { q == 2 }`,
			},
			threshold:         100,
			expectedExitCode:  2,
			expectedErrOutput: "Code coverage threshold not met: got 40.00 instead of 100.00\n",
		},
		{
			note: "coverage threshold not met (verbose)",
			modules: map[string]string{
				"test.rego": `package test

					p := 1 if {
						1 == 1
					}
					q := 2
					r := 3
					test_q if { q == 2 }`,
			},
			threshold:        100,
			expectedExitCode: 2,
			verbose:          true,
			expectedErrOutput: `Code coverage threshold not met: got 40.00 instead of 100.00
Lines not covered:
	%ROOT%/test.rego:3-4
	%ROOT%/test.rego:7
`,
		},
		{
			note: "coverage threshold not met (verbose, multiple files)",
			modules: map[string]string{
				"policy1.rego": `package test
					
					p := 1 if {
						1 == 1
					}
					q := 2
					r := 3`,
				"policy2.rego": `package test
					
					s := 4 if {
						1 == 1
						2 == 2
					}
					t := 5
					u := 6
					v := 7`,
				"test.rego": `package test
					
					test_q if { q == 2 }
					test_t if { t == 5 }`,
			},
			threshold:        100,
			expectedExitCode: 2,
			verbose:          true,
			expectedErrOutput: `Code coverage threshold not met: got 33.33 instead of 100.00
Lines not covered:
	%ROOT%/policy1.rego:3-4
	%ROOT%/policy1.rego:7
	%ROOT%/policy2.rego:3-5
	%ROOT%/policy2.rego:8-9
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

func TestRun_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note    string
		files   map[string]string
		expErrs []string
	}{
		{
			note: "v0 module",
			files: map[string]string{
				"/test.rego": `package test

l1 := {1, 3, 5}
l2[v] {
	v := l1[_]
}

test_l {
	l1 == l2
}`,
			},
			expErrs: []string{
				"test.rego:4: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:4: rego_parse_error: `contains` keyword is required for partial set rules",
				"test.rego:8: rego_parse_error: `if` keyword is required before rule body",
			},
		},
		{
			note: "v1 module",
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
					if len(tc.expErrs) > 0 {
						if exitCode == 0 {
							t.Fatalf("expected non-zero exit code")
						}

						for _, expErr := range tc.expErrs {
							if actual := errBuf.String(); !strings.Contains(actual, expErr) {
								t.Fatalf("expected error output to contain:\n\n%q\n\nbut got:\n\n%q", expErr, actual)
							}
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

func TestRunWithRegoV1Capability(t *testing.T) {
	tests := []struct {
		note         string
		v0Compatible bool
		capabilities *ast.Capabilities
		files        map[string]string
		expErrs      []string
	}{
		{
			note:         "v0 module, v0-compatible, no capabilities",
			v0Compatible: true,
			files: map[string]string{
				"/test.rego": `package test

l1 := {1, 3, 5}
l2[v] {
	v := l1[_]
}

test_l {
	l1 == l2
}`,
			},
		},
		{
			note:         "v0 module, v0-compatible, v0 capabilities",
			v0Compatible: true,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0)),
			files: map[string]string{
				"/test.rego": `package test

l1 := {1, 3, 5}
l2[v] {
	v := l1[_]
}

test_l {
	l1 == l2
}`,
			},
		},
		{
			note:         "v0 module, v0-compatible, v1 capabilities",
			v0Compatible: true,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV1)),
			files: map[string]string{
				"/test.rego": `package test

l1 := {1, 3, 5}
l2[v] {
	v := l1[_]
}

test_l {
	l1 == l2
}`,
			},
		},

		{
			note: "v0 module, not v0-compatible, no capabilities",
			files: map[string]string{
				"/test.rego": `package test

l1 := {1, 3, 5}
l2[v] {
	v := l1[_]
}

test_l {
	l1 == l2
}`,
			},
			expErrs: []string{
				"test.rego:4: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:4: rego_parse_error: `contains` keyword is required for partial set rules",
				"test.rego:8: rego_parse_error: `if` keyword is required before rule body",
			},
		},
		{
			note:         "v0 module, not v0-compatible, v0 capabilities",
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0)),
			files: map[string]string{
				"/test.rego": `package test

l1 := {1, 3, 5}
l2[v] {
	v := l1[_]
}

test_l {
	l1 == l2
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
				"/test.rego": `package test

l1 := {1, 3, 5}
l2[v] {
	v := l1[_]
}

test_l {
	l1 == l2
}`,
			},
			expErrs: []string{
				"test.rego:4: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:4: rego_parse_error: `contains` keyword is required for partial set rules",
				"test.rego:8: rego_parse_error: `if` keyword is required before rule body",
			},
		},

		{
			note:         "v1 module, v0-compatible, no capabilities",
			v0Compatible: true,
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
			expErrs: []string{
				"test.rego:4: rego_parse_error: var cannot be used for rule name",
			},
		},
		{
			note:         "v1 module, v0-compatible, v0 capabilities",
			v0Compatible: true,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0)),
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
			expErrs: []string{
				"test.rego:4: rego_parse_error: var cannot be used for rule name",
			},
		},
		{
			note:         "v1 module, v0-compatible, v1 capabilities",
			v0Compatible: true,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV1)),
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
			expErrs: []string{
				"test.rego:4: rego_parse_error: var cannot be used for rule name",
			},
		},

		{
			note: "v1 module, not v0-compatible, no capabilities",
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
			note:         "v1 module, not v0-compatible, v0 capabilities",
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0)),
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
			expErrs: []string{
				"rego_parse_error: illegal capabilities: rego_v1 feature required for parsing v1 Rego",
			},
		},
		{
			note:         "v1 module, not v0-compatible, v1 capabilities",
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV1)),
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
					testParams.bundleMode = loadType == loadBundle
					testParams.count = 1
					testParams.output = &buf
					testParams.errOutput = &errBuf
					testParams.v0Compatible = tc.v0Compatible
					testParams.capabilities.C = tc.capabilities

					var paths []string
					if loadType == loadTarball {
						paths = []string{filepath.Join(root, "bundle.tar.gz")}
					} else {
						paths = []string{root}
					}

					exitCode, _ := opaTest(paths, testParams)
					if len(tc.expErrs) > 0 {
						if exitCode == 0 {
							t.Fatalf("expected non-zero exit code")
						}

						for _, expErr := range tc.expErrs {
							if actual := errBuf.String(); !strings.Contains(actual, expErr) {
								t.Fatalf("expected error output to contain:\n\n%q\n\nbut got:\n\n%q", expErr, actual)
							}
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

func TestRun_CompatibleFlags(t *testing.T) {
	tests := []struct {
		note         string
		v0Compatible bool
		v1Compatible bool
		files        map[string]string
		expErr       string
	}{
		{
			note:         "v0 module, no imports",
			v0Compatible: true,
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
			note:         "v0 module, rego.v1 imported",
			v0Compatible: true,
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
			note:         "v0 module, future.keywords imported",
			v0Compatible: true,
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
			note:         "v1 compatible module, no imports",
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
			note:         "v1 compatible module, rego.v1 imported",
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
			note:         "v1 compatible module, future.keywords imported",
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

		// v0 takes precedence over v1
		{
			note:         "v0+v1 module, no imports",
			v0Compatible: true,
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
			expErr: "rego_parse_error",
		},
		{
			note:         "v0+v1 module, rego.v1 imported",
			v0Compatible: true,
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
			note:         "v0+v1 module, future.keywords imported",
			v0Compatible: true,
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
					testParams.v0Compatible = tc.v0Compatible
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

func TestWithBundleRegoVersion(t *testing.T) {
	tests := []struct {
		note   string
		files  map[string]string
		expErr string
	}{
		{
			note: "v0.x bundle, no imports",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test

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
			note: "v0.x bundle, rego.v1 imported",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test

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
			note: "v0.x bundle, future.keywords imported",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test

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
			note: "v0 bundle, v1 per-file override",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"/policy2.rego": 1
	}
}`,
				"policy1.rego": `package test
l1 := {1, 3, 5}
l2[v] {
	v := l1[_]
}`,
				"policy2.rego": `package test
test_l if {
	l1 == l2
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
				"policy1.rego": `package test
l1 := {1, 3, 5}
l2[v] {
	v := l1[_]
}`,
				"policy2.rego": `package test
test_l if {
	l1 == l2
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
				"policy1.rego": `package test
l1 := {1, 3, 5}
l2[v] {
	v := l1[_]
}`,
				"policy2.rego": `package test
test_l {
	l1 == l2
}`,
			},
			expErr: "rego_parse_error",
		},

		{
			note: "v1.0 bundle, no imports",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test

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
			note: "v1.0 bundle, rego.v1 imported",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test

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
			note: "v1.0 bundle, future.keywords imported",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test

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
			note: "v1 bundle, v0 per-file override",
			files: map[string]string{
				".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"/policy1.rego": 0
	}
}`,
				"policy1.rego": `package test
l1 := {1, 3, 5}
l2[v] {
	v := l1[_]
}`,
				"policy2.rego": `package test
test_l if {
	l1 == l2
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
				"policy1.rego": `package test
l1 := {1, 3, 5}
l2[v] {
	v := l1[_]
}`,
				"policy2.rego": `package test
test_l if {
	l1 == l2
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
				"policy1.rego": `package test
l1 := {1, 3, 5}
l2 contains v if {
	v := l1[_]
}`,
				"policy2.rego": `package test
test_l if {
	l1 == l2
}`,
			},
			expErr: "rego_parse_error",
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
					files := map[string]string{}
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
						var errBuf bytes.Buffer

						testParams := newTestCommandParams()
						testParams.v1Compatible = v1CompatibleFlag.used
						testParams.bundleMode = true
						testParams.count = 1
						testParams.output = &buf
						testParams.errOutput = &errBuf

						exitCode, _ := opaTest([]string{p}, testParams)
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
}

// Assert that a failing test doesn't cause a panic.
// https://github.com/open-policy-agent/opa/issues/7205
func TestTestBenchFailingTest(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test
			test_fail if false`,
	}

	test.WithTempFS(files, func(path string) {
		fp := filepath.Join(path, "test.rego")
		tp := newTestCommandParams()
		tp.benchmark = true
		tp.count = 1

		exitCode, err := opaTest([]string{fp}, tp)
		if exitCode == 0 {
			t.Fatalf("Expected exit code 0, got %d", exitCode)
		}
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})
}
