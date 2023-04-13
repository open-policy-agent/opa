// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// nolint: goconst // string duplication is for test readability.
package profiler

import (
	"context"
	_ "encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/types"
)

func TestProfilerLargeArray(t *testing.T) {
	profiler := New()
	module := `package test

foo {
	p
	bar
	not baz
	bee
}

bee {
	nums = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i", "b", "c", "d", "e", "f", "g", "h", "i"]
	num = nums[_]
	contains(num, "test")
}

bar {
	a := 1
	b := 2
	a != b
}

baz {
	true
	false
	true
}

p {
	a := 1
	b := 2
	c := 3
	x = a + b * c
}
`

	_, err := ast.ParseModule("test.rego", module)
	if err != nil {
		t.Fatal(err)
	}

	eval := rego.New(
		rego.Module("test.rego", module),
		rego.Query("data.test.foo"),
		rego.QueryTracer(profiler),
	)

	ctx := context.Background()
	_, err = eval.Eval(ctx)

	if err != nil {
		t.Fatal(err)
	}

	report := profiler.ReportByFile()

	fr, ok := report.Files["test.rego"]
	if !ok {
		t.Fatal("Expected file report for test.rego")
	}

	if len(fr.Result) != 16 {
		t.Fatalf("Expected file report length to be 16 instead got %v", len(fr.Result))
	}

	expectedNumEval := []int{1, 1, 2, 1, 1, 1, 1633, 1, 1, 1, 1, 1, 1, 1, 1, 3}
	expectedNumRedo := []int{1, 1, 0, 0, 1, 1633, 0, 1, 1, 1, 1, 0, 1, 1, 1, 3}
	expectedRow := []int{4, 5, 6, 7, 11, 12, 13, 17, 18, 19, 23, 24, 29, 30, 31, 32}
	expectedNumGenExpr := []int{1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 3}

	for idx, actualExprStat := range fr.Result {
		if actualExprStat.NumEval != expectedNumEval[idx] {
			t.Fatalf("Index %v: Expected number of evals %v but got %v", idx, expectedNumEval[idx], actualExprStat.NumEval)
		}

		if actualExprStat.NumRedo != expectedNumRedo[idx] {
			t.Fatalf("Index %v: Expected number of redos %v but got %v", idx, expectedNumRedo[idx], actualExprStat.NumRedo)
		}

		if actualExprStat.Location.Row != expectedRow[idx] {
			t.Fatalf("Index %v: Expected row %v but got %v", idx, expectedRow[idx], actualExprStat.Location.Row)
		}

		if actualExprStat.NumGenExpr != expectedNumGenExpr[idx] {
			t.Fatalf("Index %v: Expected number of generated expressions %v but got %v", idx, expectedNumGenExpr[idx], actualExprStat.NumGenExpr)
		}
	}
}

func TestProfileCheckExprDuration(t *testing.T) {
	profiler := New()

	ast.RegisterBuiltin(&ast.Builtin{
		Name: "test.sleep",
		Decl: types.NewFunction(
			types.Args(types.S),
			types.NewNull(),
		),
	})

	topdown.RegisterBuiltinFunc("test.sleep", func(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
		d, _ := time.ParseDuration(string(operands[0].Value.(ast.String)))
		time.Sleep(d)
		return iter(ast.NullTerm())
	})

	module := `package test

	foo {
	  test.sleep("100ms")
	}`

	_, err := ast.ParseModule("test.rego", module)
	if err != nil {
		t.Fatal(err)
	}

	eval := rego.New(
		rego.Module("test.rego", module),
		rego.Query("data.test.foo"),
		rego.QueryTracer(profiler),
	)

	ctx := context.Background()
	_, err = eval.Eval(ctx)

	if err != nil {
		t.Fatal(err)
	}

	report := profiler.ReportByFile()

	fr, ok := report.Files["test.rego"]
	if !ok {
		t.Fatal("Expected file report for test.rego")
	}

	if len(fr.Result) != 1 {
		t.Fatalf("Expected file report length to be 1 instead got %v", len(fr.Result))
	}

	if string(fr.Result[0].Location.Text) != "test.sleep(\"100ms\")" {
		t.Fatalf("Expected text is test.sleep(\"100ms\") but got %v", string(fr.Result[0].Location.Text))
	}

	if fr.Result[0].ExprTimeNs <= 50*time.Millisecond.Nanoseconds() {
		t.Fatalf("Expected eval time is at least 100 msec but got %v", fr.Result[0].ExprTimeNs)
	}

}

func TestProfilerReportTopNResultsNoCriteria(t *testing.T) {
	profiler := New()
	module := `package test

foo {
	bar
	not baz
	bee
}

bee {
	nums = ["a", "b", "c", "d"]
	num = nums[_]
	contains(num, "test")
}

bar {
	a := 1
	b := 2
	a != b
}

baz {
	true
	false
	true
}`

	_, err := ast.ParseModule("test.rego", module)
	if err != nil {
		t.Fatal(err)
	}

	eval := rego.New(
		rego.Module("test.rego", module),
		rego.Query("data.test.foo"),
		rego.QueryTracer(profiler),
	)

	ctx := context.Background()
	_, err = eval.Eval(ctx)

	if err != nil {
		t.Fatal(err)
	}

	stats := profiler.ReportTopNResults(0, []string{})

	expectedResLen := 12
	if len(stats) != expectedResLen {
		t.Fatalf("Expected %v stats instead got %v", expectedResLen, len(stats))
	}
}

func TestProfilerReportTopNResultsOneCriteria(t *testing.T) {
	profiler := New()
	module := `package test

foo {
	bar
	not baz
	bee
}

bee {
	nums = ["a", "b", "c", "d"]
	num = nums[_]
	contains(num, "test")
}

bar {
	a := 1
	b := 2
	a != b
}

baz {
	true
	false
	true
}`

	_, err := ast.ParseModule("test.rego", module)
	if err != nil {
		t.Fatal(err)
	}

	eval := rego.New(
		rego.Module("test.rego", module),
		rego.Query("data.test.foo"),
		rego.QueryTracer(profiler),
	)

	ctx := context.Background()
	_, err = eval.Eval(ctx)

	if err != nil {
		t.Fatal(err)
	}

	stats := profiler.ReportTopNResults(5, []string{"total_time_ns"})

	expectedResLen := 5
	if len(stats) != expectedResLen {
		t.Fatalf("Expected %v stats instead got %v", expectedResLen, len(stats))
	}

	var i int
	for i = 0; i < len(stats)-1; i++ {
		if stats[i].ExprTimeNs < stats[i+1].ExprTimeNs {
			t.Fatalf("Results not sorted in decreasing order of evaluation times")
		}
	}
}

func TestProfilerReportTopNResultsTwoCriteria(t *testing.T) {
	profiler := New()
	module := `package test

foo {
	bar
	not baz
	bee
}

bee {
	nums = ["a", "b", "c", "d"]
	num = nums[_]
	contains(num, "test")
}

bar {
	a := 1
	b := 2
	a != b
}

baz {
	true
	false
	true
}`

	_, err := ast.ParseModule("test.rego", module)
	if err != nil {
		t.Fatal(err)
	}

	eval := rego.New(
		rego.Module("test.rego", module),
		rego.Query("data.test.foo"),
		rego.QueryTracer(profiler),
	)

	ctx := context.Background()
	_, err = eval.Eval(ctx)

	if err != nil {
		t.Fatal(err)
	}

	stats := profiler.ReportTopNResults(5, []string{"num_eval", "total_time_ns"})

	expectedResLen := 5
	if len(stats) != expectedResLen {
		t.Fatalf("Expected %v stats instead got %v", expectedResLen, len(stats))
	}

	var i int
	for i = 0; i < len(stats)-1; i++ {
		if stats[i].NumEval < stats[i+1].NumEval {
			t.Fatalf("Results not sorted in decreasing order of number of evaluations")
		}

		if stats[i].NumEval == stats[i+1].NumEval {
			if stats[i].ExprTimeNs < stats[i+1].ExprTimeNs {
				t.Fatalf("Results not sorted in decreasing order of evaluation times")
			}
		}
	}
}

func TestProfilerReportTopNResultsThreeCriteria(t *testing.T) {
	profiler := New()
	module := `package test

foo {
	bar
	not baz
	bee
}

bee {
	nums = ["a", "b", "c", "d"]
	num = nums[_]
	contains(num, "test")
}

bar {
	a := 1
	b := 2
	a != b
}

baz {
	true
	false
	true
}`

	_, err := ast.ParseModule("test.rego", module)
	if err != nil {
		t.Fatal(err)
	}

	eval := rego.New(
		rego.Module("test.rego", module),
		rego.Query("data.test.foo"),
		rego.QueryTracer(profiler),
	)

	ctx := context.Background()
	_, err = eval.Eval(ctx)

	if err != nil {
		t.Fatal(err)
	}

	stats := profiler.ReportTopNResults(10, []string{"num_eval", "num_redo", "total_time_ns"})

	expectedResLen := 10
	if len(stats) != expectedResLen {
		t.Fatalf("Expected %v stats instead got %v", expectedResLen, len(stats))
	}

	var i int
	for i = 0; i < len(stats)-1; i++ {
		if stats[i].NumEval < stats[i+1].NumEval {
			t.Fatalf("Results not sorted in decreasing order of number of evaluations")
		}

		if stats[i].NumEval == stats[i+1].NumEval {

			if stats[i].NumRedo < stats[i+1].NumRedo {
				t.Fatalf("Results not sorted in decreasing order of number of redos")
			}

			if stats[i].NumRedo == stats[i+1].NumRedo {
				if stats[i].ExprTimeNs < stats[i+1].ExprTimeNs {
					t.Fatalf("Results not sorted in decreasing order of evaluation times")
				}
			}
		}
	}
}

func TestProfilerWithPartialEval(t *testing.T) {
	profiler := New()

	module := `package test

default foo = false

foo = true {
	op = allowed_operations[_]
	input.method = op.method
	input.resource = op.resource
}

allowed_operations = [
	{"method": "PUT", "resource": "policy"},
]`

	_, err := ast.ParseModule("test.rego", module)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	pq, err := rego.New(
		rego.Module("test.rego", module),
		rego.Query("data.test.foo"),
	).PrepareForEval(ctx, rego.WithPartialEval())
	if err != nil {
		t.Fatal(err)
	}

	_, err = pq.Eval(ctx, rego.EvalQueryTracer(profiler))
	if err != nil {
		t.Fatal(err)
	}

	report := profiler.ReportByFile()

	if len(report.Files) != 1 {
		t.Fatalf("Expected file report length to be 1 instead got %v", len(report.Files))
	}

	fr := report.Files[""]

	if len(fr.Result) != 2 {
		t.Fatalf("Expected 2 results for file but instead got %v", len(fr.Result))
	}

	expectedNumEval := []int{2, 1}
	expectedNumRedo := []int{2, 1}
	expectedNumGenExpr := []int{1, 1}
	expectedLocation := []string{"???", "data.partial.__result__"}

	for idx, actualExprStat := range fr.Result {
		if actualExprStat.NumEval != expectedNumEval[idx] {
			t.Fatalf("Index %v: Expected number of evals %v but got %v", idx, expectedNumEval[idx], actualExprStat.NumEval)
		}

		if actualExprStat.NumRedo != expectedNumRedo[idx] {
			t.Fatalf("Index %v: Expected number of redos %v but got %v", idx, expectedNumRedo[idx], actualExprStat.NumRedo)
		}

		if actualExprStat.NumGenExpr != expectedNumGenExpr[idx] {
			t.Fatalf("Index %v: Expected number of generated expressions %v but got %v", idx, expectedNumGenExpr[idx], actualExprStat.NumGenExpr)
		}

		if string(actualExprStat.Location.Text) != expectedLocation[idx] {
			t.Fatalf("Index %v: Expected location %v but got %v", idx, expectedLocation[idx], string(actualExprStat.Location.Text))
		}
	}
}

func TestProfilerTraceConfig(t *testing.T) {
	ct := topdown.QueryTracer(New())
	conf := ct.Config()

	expected := topdown.TraceConfig{
		PlugLocalVars: false,
	}

	if !reflect.DeepEqual(expected, conf) {
		t.Fatalf("Expected config: %+v, got %+v", expected, conf)
	}
}
