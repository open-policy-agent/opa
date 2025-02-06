// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package debug

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/ast/location"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
	"github.com/open-policy-agent/opa/v1/types"
	"github.com/open-policy-agent/opa/v1/util/test"
)

func TestDebuggerEvalStepOver(t *testing.T) {
	tests := []struct {
		note   string
		module string
		brRow  int
		brHits int
		expRow int
	}{
		{
			note: "rule",
			module: `package test
import rego.v1

p if {
	q    # breakpoint
	true # step-over to here
}

q := y if {
	y := 1 * 2
}
`,
			brRow:  5,
			expRow: 6,
		},
		{
			note: "partial rule (success)",
			module: `package test
import rego.v1

p contains 1 if { # breakpoint
	true
}

p contains 2 if { # step-over to here, the next partial rule 'p'
	true
}
`,
			brRow:  4,
			brHits: 2, // First enter, then exit
			expRow: 8,
		},
		{
			note: "partial rule (fail)",
			module: `package test
import rego.v1

p contains 1 if {
	false         # breakpoint
}

p contains 2 if { # step-over to here, the next partial rule 'p'
	true
}
`,
			brRow:  5,
			brHits: 2, // First eval, then fail
			expRow: 8,
		},
		{
			note: "function",
			module: `package test
import rego.v1

p if {
	f(1) # breakpoint
	true # step-over to here
}

f(x) := y if {
	y := x * 2
}
`,
			brRow:  5,
			expRow: 6,
		},
		{
			note: "every",
			module: `package test
import rego.v1

p if {
	every x in [1, 2, 3] { # breakpoint
		f(x)
	}
	true                   # step-over to here
}

f(x) := y if {
	y := x * 2
}
`,
			brRow:  5,
			brHits: 4, // every "header" is composed of multiple expressions that will all cause a breakpoint hit.
			expRow: 8,
		},
		{
			note: "comprehension",
			module: `package test
import rego.v1

p if {
	l := [x |                   # breakpoint
		x := ["a", "b", "c"][_]
	]
	true                        # step-over to here
}

f(x) := y if {
	y := x * 2
}
`,
			brRow:  5,
			expRow: 8,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			modName := "test1.rego"
			files := map[string]string{
				modName: tc.module,
			}

			test.WithTempFS(files, func(rootDir string) {
				eh := newTestEventHandler()
				d := NewDebugger(SetEventHandler(eh.HandleEvent))

				launchProps := LaunchEvalProperties{
					LaunchProperties: LaunchProperties{
						BundlePaths: []string{rootDir},
					},
					Query: "x = data.test.p",
				}

				// There is only one thread
				thr := ThreadID(1)

				s, err := d.LaunchEval(ctx, launchProps)
				if err != nil {
					t.Fatalf("Unexpected error launching debgug session: %v", err)
				}

				if _, err := s.AddBreakpoint(location.Location{File: path.Join(rootDir, modName), Row: tc.brRow}); err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				expBrHits := 1
				if tc.brHits > 0 {
					expBrHits = tc.brHits
				}

				for range expBrHits {
					if err := s.ResumeAll(); err != nil {
						t.Fatalf("Unexpected error resuming threads: %v", err)
					}

					// wait for breakpoint
					if e := eh.WaitFor(ctx, StoppedEventType); e == nil {
						t.Fatal("Expected stopped event")
					} else if e.stackEvent.Location.Row != tc.brRow {
						t.Fatalf("Expected to stop on row 5, got %d", e.stackEvent.Location.Row)
					}
				}

				// Release the event handler, so we don't block the debugger
				eh.IgnoreAll(ctx)

				// step over
				if err := s.StepOver(thr); err != nil {
					t.Fatalf("Unexpected error stepping over: %v", err)
				}

				if frame := topOfStack(t, s); frame == nil {
					t.Fatal("Expected frame")
				} else if frame.location.Row != tc.expRow {
					t.Fatalf("Expected to stop on row %d, got %d", tc.expRow, frame.location.Row)
				}
			})
		})
	}
}

func TestDebuggerEvalStepIn(t *testing.T) {
	tests := []struct {
		note   string
		module string
		brRow  int
		brHits int
		expRow int
	}{
		{
			note: "rule",
			module: `package test
import rego.v1

p if {
	q          # breakpoint
	true
}

q := y if {    # step-in to here
	y := 2 * 2
}
`,
			brRow:  5,
			expRow: 9,
		},
		{
			note: "function",
			module: `package test
import rego.v1

p if {
	f(1)       # breakpoint
	true
}

f(x) := y if { # step-in to here
	y := x * 2
}
`,
			brRow:  5,
			expRow: 9,
		},
		{
			note: "every",
			module: `package test
import rego.v1

p if {
	every x in [1, 2, 3] { # breakpoint
		x < 4              # step-in to here
	}
	true
}
`,
			brRow:  5,
			brHits: 4, // every "header" is composed of multiple expressions that will all cause a breakpoint hit.
			expRow: 6,
		},
		{
			note: "comprehension",
			module: `package test
import rego.v1

p if {
	l := [x |                   # breakpoint
		x := ["a", "b", "c"][_] # step-in to here
	]
	count(l) == 3
}
`,
			brRow:  5,
			expRow: 6,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			modName := "test1.rego"
			files := map[string]string{
				modName: tc.module,
			}

			test.WithTempFS(files, func(rootDir string) {
				eh := newTestEventHandler()
				d := NewDebugger(SetEventHandler(eh.HandleEvent))

				launchProps := LaunchEvalProperties{
					LaunchProperties: LaunchProperties{
						BundlePaths: []string{rootDir},
					},
					Query: "x = data.test.p",
				}

				// There is only one thread
				thr := ThreadID(1)

				s, err := d.LaunchEval(ctx, launchProps)
				if err != nil {
					t.Fatalf("Unexpected error launching debgug session: %v", err)
				}

				if _, err := s.AddBreakpoint(location.Location{File: path.Join(rootDir, modName), Row: tc.brRow}); err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				expBrHits := 1
				if tc.brHits > 0 {
					expBrHits = tc.brHits
				}
				for range expBrHits {
					if err := s.ResumeAll(); err != nil {
						t.Fatalf("Unexpected error resuming threads: %v", err)
					}

					// wait for breakpoint
					if e := eh.WaitFor(ctx, StoppedEventType); e == nil {
						t.Fatal("Expected stopped event")
					} else if e.stackEvent.Location.Row != tc.brRow {
						t.Fatalf("Expected to stop on row 5, got %d", e.stackEvent.Location.Row)
					}
				}

				// Release the event handler, so we don't block the debugger
				eh.IgnoreAll(ctx)

				// step into function
				if err := s.StepIn(thr); err != nil {
					t.Fatalf("Unexpected error stepping in: %v", err)
				}

				if frame := topOfStack(t, s); frame == nil {
					t.Fatal("Expected frame")
				} else if frame.location.Row != tc.expRow {
					t.Fatalf("Expected to stop on row %d, got %d", tc.expRow, frame.location.Row)
				}
			})
		})
	}
}

func TestDebuggerEvalStepOut(t *testing.T) {
	tests := []struct {
		note   string
		module string
		brRow  int
		brHits int
		expRow int
	}{
		{
			note: "rule",
			module: `package test
import rego.v1

p if {
	q
	true       # step-out to here
}

q := y if {
	true       # breakpoint
	y := 2 * 2
}
`,
			brRow:  10,
			expRow: 6,
		},
		{
			note: "function",
			module: `package test
import rego.v1

p if {
	f(1)
	true       # step-out to here
}

f(x) := y if {
	true       # breakpoint
	y := x * 2
}
`,
			brRow:  10,
			expRow: 6,
		},
		{
			note: "every",
			module: `package test
import rego.v1

p if {
	every x in [1, 2, 3] {
		true
		true               # breakpoint
		x < 4
	}
	true                   # step-out to here
}
`,
			brRow:  7,
			brHits: 3, // every expr enumeration
			expRow: 10,
		},
		{
			note: "comprehension",
			module: `package test
import rego.v1

p if {
	l := [x |
		true
		true                    # breakpoint
		x := ["a", "b", "c"][_]
	]
	true                        # step-out to here
}
`,
			brRow:  7,
			expRow: 10,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			modName := "test1.rego"
			files := map[string]string{
				modName: tc.module,
			}

			test.WithTempFS(files, func(rootDir string) {
				eh := newTestEventHandler()
				d := NewDebugger(SetEventHandler(eh.HandleEvent))

				launchProps := LaunchEvalProperties{
					LaunchProperties: LaunchProperties{
						BundlePaths: []string{rootDir},
					},
					Query: "x = data.test.p",
				}

				// There is only one thread
				thr := ThreadID(1)

				s, err := d.LaunchEval(ctx, launchProps)
				if err != nil {
					t.Fatalf("Unexpected error launching debgug session: %v", err)
				}

				if _, err := s.AddBreakpoint(location.Location{File: path.Join(rootDir, modName), Row: tc.brRow}); err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				// wait for breakpoint
				expBrHits := 1
				if tc.brHits > 0 {
					expBrHits = tc.brHits
				}
				for range expBrHits {
					if err := s.ResumeAll(); err != nil {
						t.Fatalf("Unexpected error resuming threads: %v", err)
					}

					if e := eh.WaitFor(ctx, StoppedEventType); e == nil {
						t.Fatal("Expected stopped event")
					} else if e.stackEvent.Location.Row != tc.brRow {
						t.Fatalf("Expected to stop on row 10, got %d", e.stackEvent.Location.Row)
					}
				}

				// Release the event handler, so we don't block the debugger
				eh.IgnoreAll(ctx)

				// step out of function
				if err := s.StepOut(thr); err != nil {
					t.Fatalf("Unexpected error stepping in: %v", err)
				}

				if frame := topOfStack(t, s); frame == nil {
					t.Fatal("Expected frame")
				} else if frame.location.Row != tc.expRow {
					t.Fatalf("Expected to stop on row %d, got %d", tc.expRow, frame.location.Row)
				}
			})
		})
	}
}

func TestDebuggerEvalPrint(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancel()

	files := map[string]string{
		"test1.rego": `package test
import rego.v1

p if {
	print("hello")
}
`,
	}

	test.WithTempFS(files, func(rootDir string) {
		eh := newTestEventHandler()
		d := NewDebugger(SetEventHandler(eh.HandleEvent))

		launchProps := LaunchEvalProperties{
			LaunchProperties: LaunchProperties{
				BundlePaths: []string{rootDir},
				EnablePrint: true,
			},
			Query: "x = data.test.p",
		}

		s, err := d.LaunchEval(ctx, launchProps)
		if err != nil {
			t.Fatalf("Unexpected error launching debgug session: %v", err)
		}

		if err := s.ResumeAll(); err != nil {
			t.Fatalf("Unexpected error resuming threads: %v", err)
		}

		// print output
		e := eh.WaitFor(ctx, StdoutEventType)
		if e.Message != "hello" {
			t.Fatalf("Expected message to be 'hello', got %q", e.Message)
		}

		// result output
		exp := `[
  {
    "expressions": [
      {
        "value": true,
        "text": "x = data.test.p",
        "location": {
          "row": 1,
          "col": 1
        }
      }
    ],
    "bindings": {
      "x": true
    }
  }
]`
		e = eh.WaitFor(ctx, StdoutEventType)
		if e.Message != exp {
			t.Fatalf("Expected message to be:\n\n%s\n\ngot:\n\n%s", exp, e.Message)
		}
	})
}

func TestFiles(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancel()

	files := map[string]string{
		"mod.rego": `package test
import rego.v1

p if {
	input.foo == "a"
	input.bar == "b"
	data.baz == "c"
	data.qux == "d"
}
`,
		"input.json": `{
	"foo": "a",
	"bar": "b"
}`,
		"input.yaml": `
foo: a
bar: b
`,
		"data.json": `{
	"baz": "c",
	"qux": "d"
}`,
		"data.yaml": `
baz: c
qux: d
`,
	}

	for _, ext := range []string{"json", "yaml"} {
		t.Run(ext, func(t *testing.T) {
			test.WithTempFS(files, func(rootDir string) {
				eh := newTestEventHandler()
				d := NewDebugger(SetEventHandler(eh.HandleEvent))

				launchProps := LaunchEvalProperties{
					LaunchProperties: LaunchProperties{
						DataPaths: []string{
							path.Join(rootDir, "mod.rego"),
							path.Join(rootDir, "data."+ext),
						},
						EnablePrint: true,
					},
					Query:     "x = data.test.p",
					InputPath: path.Join(rootDir, "input."+ext),
				}

				s, err := d.LaunchEval(ctx, launchProps)
				if err != nil {
					t.Fatalf("Unexpected error launching debgug session: %v", err)
				}

				if err := s.ResumeAll(); err != nil {
					t.Fatalf("Unexpected error resuming threads: %v", err)
				}

				// result output
				exp := `[
  {
    "expressions": [
      {
        "value": true,
        "text": "x = data.test.p",
        "location": {
          "row": 1,
          "col": 1
        }
      }
    ],
    "bindings": {
      "x": true
    }
  }
]`
				e := eh.WaitFor(ctx, StdoutEventType)
				if e.Message != exp {
					t.Fatalf("Expected message to be:\n\n%s\n\ngot:\n\n%s", exp, e.Message)
				}
			})
		})
	}
}

func topOfStack(t *testing.T, s Session) *stackFrame {
	t.Helper()
	stk, err := s.StackTrace(ThreadID(1))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	frame, ok := stk[0].(*stackFrame)
	if !ok {
		t.Fatalf("Expected stackFrame, got %T", stk[0])
	}
	return frame
}

func TestDebuggerAutomaticStop(t *testing.T) {
	tests := []struct {
		note          string
		props         LaunchProperties
		expEventType  EventType
		expEventIndex int
	}{
		{
			note:          "No automatic stop",
			expEventType:  TerminatedEventType,
			expEventIndex: -1,
		},
		{
			note: "Stop on entry",
			props: LaunchProperties{
				StopOnEntry: true,
			},
			expEventType:  StoppedEventType,
			expEventIndex: 1,
		},
		{
			note: "Stop on end of trace",
			props: LaunchProperties{
				StopOnResult: true,
			},
			expEventType:  StoppedEventType,
			expEventIndex: -1,
		},
		{
			note: "Stop on fail",
			props: LaunchProperties{
				StopOnFail: true,
			},
			expEventType:  ExceptionEventType,
			expEventIndex: 3,
		},
	}

	testEvents := []*topdown.Event{
		{ // 0
			Op:      topdown.EvalOp,
			QueryID: 0,
		},
		{ // 1
			Op:      topdown.EnterOp,
			QueryID: 1,
			Location: &location.Location{
				File: "test.rego",
				Row:  1,
			},
		},
		{ // 2
			Op:      topdown.EvalOp,
			QueryID: 1,
			Location: &location.Location{
				File: "test.rego",
				Row:  2,
			},
		},
		{ // 3
			Op:      topdown.FailOp,
			QueryID: 1,
			Location: &location.Location{
				File: "test.rego",
				Row:  2,
			},
		},
		{ // 4
			Op:      topdown.RedoOp,
			QueryID: 1,
			Location: &location.Location{
				File: "test.rego",
				Row:  2,
			},
		},
		{ // 5
			Op:      topdown.ExitOp,
			QueryID: 1,
			Location: &location.Location{
				File: "test.rego",
				Row:  1,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			stk := newTestStack(testEvents...)
			eh := newTestEventHandler()
			_, s, _ := setupDebuggerSession(ctx, stk, tc.props, eh.HandleEvent, nil, nil, nil)

			if err := s.start(); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if err := s.ResumeAll(); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			test.EventuallyOrFatal(t, 5*time.Second, func() bool {
				e := eh.Next(10 * time.Millisecond)
				if e != nil && e.Type == tc.expEventType {
					i, _ := stk.Current()
					return i == tc.expEventIndex
				}
				return false
			})
		})
	}
}

func TestDebuggerStopOnBreakpoint(t *testing.T) {
	tests := []struct {
		note            string
		breakpoint      location.Location
		events          []*topdown.Event
		expEventIndices []int
	}{
		{
			note:       "breakpoint on line with single event",
			breakpoint: location.Location{File: "test.rego", Row: 1},
			events: []*topdown.Event{
				{ // 0
					Op:      topdown.EvalOp,
					QueryID: 0,
				},
				{ // 1
					Op:      topdown.EnterOp,
					QueryID: 1,
					Location: &location.Location{
						File: "test.rego",
						Row:  1,
					},
				},
				{ // 2
					Op:      topdown.EvalOp,
					QueryID: 1,
					Location: &location.Location{
						File: "test.rego",
						Row:  2,
					},
				},
			},
			expEventIndices: []int{1},
		},
		{
			note:       "breakpoint on line with single event (2)",
			breakpoint: location.Location{File: "test.rego", Row: 2},
			events: []*topdown.Event{
				{ // 0
					Op:      topdown.EvalOp,
					QueryID: 0,
				},
				{ // 1
					Op:      topdown.EnterOp,
					QueryID: 1,
					Location: &location.Location{
						File: "test.rego",
						Row:  1,
					},
				},
				{ // 2
					Op:      topdown.EvalOp,
					QueryID: 1,
					Location: &location.Location{
						File: "test.rego",
						Row:  2,
					},
				},
			},
			expEventIndices: []int{2},
		},
		{
			note:       "breakpoint on line with multiple consecutive events",
			breakpoint: location.Location{File: "test.rego", Row: 2},
			events: []*topdown.Event{
				{ // 0
					Op:      topdown.EvalOp,
					QueryID: 0,
				},
				{ // 1
					Op:      topdown.EnterOp,
					QueryID: 1,
					Location: &location.Location{
						File: "test.rego",
						Row:  1,
					},
				},
				{ // 2
					Op:      topdown.EvalOp,
					QueryID: 1,
					Location: &location.Location{
						File: "test.rego",
						Row:  2,
					},
				},
				{ // 3
					Op:      topdown.UnifyOp,
					QueryID: 1,
					Location: &location.Location{
						File: "test.rego",
						Row:  2,
					},
				},
				{ // 4
					Op:      topdown.EvalOp,
					QueryID: 1,
					Location: &location.Location{
						File: "test.rego",
						Row:  3,
					},
				},
			},
			expEventIndices: []int{2, 3},
		},
		{
			note:       "breakpoint on reoccurring line",
			breakpoint: location.Location{File: "test.rego", Row: 2},
			events: []*topdown.Event{
				{ // 0
					Op: topdown.EvalOp,
				},
				{ // 1
					Op: topdown.EnterOp,
					Location: &location.Location{
						File: "test.rego",
						Row:  1,
					},
				},
				{ // 2
					Op: topdown.EvalOp,
					Location: &location.Location{
						File: "test.rego",
						Row:  2,
					},
				},
				{ // 3
					Op: topdown.EvalOp,
					Location: &location.Location{
						File: "test.rego",
						Row:  3,
					},
				},
				{ // 4
					Op: topdown.RedoOp,
					Location: &location.Location{
						File: "test.rego",
						Row:  2,
					},
				},
				{ // 5
					Op: topdown.RedoOp,
					Location: &location.Location{
						File: "test.rego",
						Row:  1,
					},
				},
			},
			expEventIndices: []int{2, 4},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			stk := newTestStack(tc.events...)
			eh := newTestEventHandler()
			_, s, _ := setupDebuggerSession(ctx, stk, LaunchProperties{}, eh.HandleEvent, nil, nil, nil)

			bp, err := s.AddBreakpoint(tc.breakpoint)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if bp.Location().File != tc.breakpoint.File {
				t.Errorf("Expected breakpoint file %s, got %s", tc.breakpoint.File, bp.Location().File)
			}

			if bp.Location().Row != tc.breakpoint.Row {
				t.Errorf("Expected breakpoint row %d, got %d", tc.breakpoint.Row, bp.Location().Row)
			}

			if err := s.start(); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if err := s.ResumeAll(); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var stoppedAt []int
			test.EventuallyOrFatal(t, 5*time.Second, func() bool {
				for {
					e := eh.NextBlocking()
					if e == nil || e.Type == TerminatedEventType {
						return true
					}
					if e.Type == StoppedEventType {
						stoppedAt = append(stoppedAt, e.stackIndex)
						if err := s.Resume(e.Thread); err != nil {
							t.Fatalf("Unexpected error resuming: %v", err)
						}
					}
				}
			})

			if !slices.Equal(stoppedAt, tc.expEventIndices) {
				t.Errorf("Expected to stop at event indices %v, got %v", tc.expEventIndices, stoppedAt)
			}
		})
	}
}

func TestDebuggerStepIn(t *testing.T) {
	tests := []struct {
		note            string
		events          []*topdown.Event
		expEventIndices []int
	}{
		{
			note: "single query",
			events: []*topdown.Event{
				{ // 0
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
				{ // 1
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
				{ // 2
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
			},
			expEventIndices: []int{0, 1, 2},
		},
		{
			note: "nested queries",
			events: []*topdown.Event{
				{ // 0
					Op:      topdown.EvalOp,
					QueryID: 0,
				},
				{ // 1
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
				{ // 2
					Op:      topdown.EvalOp,
					QueryID: 2,
				},
				{ // 3
					Op:      topdown.RedoOp,
					QueryID: 2,
				},
				{ // 4
					Op:      topdown.RedoOp,
					QueryID: 1,
				},
				{ // 5
					Op:      topdown.RedoOp,
					QueryID: 0,
				},
			},
			expEventIndices: []int{0, 1, 2, 3, 4, 5},
		},
		{
			note: "sequential queries",
			events: []*topdown.Event{
				{ // 0
					Op:      topdown.EvalOp,
					QueryID: 0,
				},
				{ // 1
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
				{ // 2
					Op:      topdown.EvalOp,
					QueryID: 2,
				},
				{ // 3
					Op:      topdown.RedoOp,
					QueryID: 2,
				},
				{ // 4
					Op:      topdown.RedoOp,
					QueryID: 1,
				},
				{ // 5
					Op:      topdown.RedoOp,
					QueryID: 0,
				},
				{ // 6
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
				{ // 7
					Op:      topdown.EvalOp,
					QueryID: 2,
				},
			},
			expEventIndices: []int{0, 1, 2, 3, 4, 5, 6, 7},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			stk := newTestStack(tc.events...)
			eh := newTestEventHandler()
			_, s, thr := setupDebuggerSession(ctx, stk, LaunchProperties{}, eh.HandleEvent, nil, nil, nil)

			var stoppedAt []int
			doneCh := make(chan struct{})
			defer close(doneCh)
			go func() {
				for {
					e := eh.NextBlocking()

					if e == nil || e.Type == TerminatedEventType {
						break
					}

					if e.Type == StoppedEventType {
						stoppedAt = append(stoppedAt, e.stackIndex)
					}
				}
				doneCh <- struct{}{}
			}()

			go func() {
				for {
					if err := s.StepIn(thr.id); err != nil {
						t.Errorf("Unexpected error stepping in: %v", err)
						break
					}
				}
			}()

			select {
			case <-time.After(5 * time.Second):
				t.Fatal("Timed out waiting for debugger to finish")
			case <-doneCh:
			}

			if !slices.Equal(stoppedAt, tc.expEventIndices) {
				t.Errorf("Expected to stop at event indices %v, got %v", tc.expEventIndices, stoppedAt)
			}
		})
	}
}

func TestDebuggerStepOver(t *testing.T) {
	tests := []struct {
		note            string
		events          []*topdown.Event
		expEventIndices []int
	}{
		{
			note: "single query",
			events: []*topdown.Event{
				{ // 0
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
				{ // 1
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
				{ // 2
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
			},
			expEventIndices: []int{0, 1, 2},
		},
		{
			note: "nested queries",
			events: []*topdown.Event{
				{ // 0
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
				{ // 1
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
				{ // 2
					Op:      topdown.EvalOp,
					QueryID: 2,
				},
				{ // 3
					Op:      topdown.RedoOp,
					QueryID: 2,
				},
				{ // 4
					Op:      topdown.RedoOp,
					QueryID: 1,
				},
				{ // 5
					Op:      topdown.RedoOp,
					QueryID: 1,
				},
			},
			expEventIndices: []int{0, 1, 4, 5},
		},
		{
			note: "multiple nested queries",
			events: []*topdown.Event{
				{ // 0
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
				{ // 1
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
				{ // 2
					Op:      topdown.EvalOp,
					QueryID: 2,
				},
				{ // 3
					Op:      topdown.EvalOp,
					QueryID: 3,
				},
				{ // 4
					Op:      topdown.RedoOp,
					QueryID: 3,
				},
				{ // 5
					Op:      topdown.RedoOp,
					QueryID: 2,
				},
				{ // 6
					Op:      topdown.RedoOp,
					QueryID: 1,
				},
				{ // 7
					Op:      topdown.RedoOp,
					QueryID: 1,
				},
			},
			expEventIndices: []int{0, 1, 6, 7},
		},
		{
			note: "sequential queries",
			events: []*topdown.Event{
				{ // 0
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
				{ // 1
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
				{ // 2
					Op:      topdown.EvalOp,
					QueryID: 2,
				},
				{ // 3
					Op:      topdown.RedoOp,
					QueryID: 2,
				},
				{ // 4
					Op:      topdown.RedoOp,
					QueryID: 1,
				},
				{ // 5
					Op:      topdown.EvalOp,
					QueryID: 2,
				},
				{ // 6
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
			},
			expEventIndices: []int{0, 1, 4, 6},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			stk := newTestStack(tc.events...)
			eh := newTestEventHandler()
			_, s, thr := setupDebuggerSession(ctx, stk, LaunchProperties{}, eh.HandleEvent, nil, nil, nil)

			var stoppedAt []int
			doneCh := make(chan struct{})
			defer close(doneCh)
			go func() {
				for {
					e := eh.NextBlocking()

					if e == nil || e.Type == TerminatedEventType {
						break
					}

					if e.Type == StoppedEventType {
						stoppedAt = append(stoppedAt, e.stackIndex)
					}
				}
				doneCh <- struct{}{}
			}()

			go func() {
				for {
					if err := s.StepOver(thr.id); err != nil {
						t.Errorf("Unexpected error stepping over: %v", err)
						break
					}
				}
			}()

			select {
			case <-time.After(5 * time.Second):
				t.Fatal("Timed out waiting for debugger to finish")
			case <-doneCh:
			}

			if !slices.Equal(stoppedAt, tc.expEventIndices) {
				t.Errorf("Expected to stop at event indices %v, got %v", tc.expEventIndices, stoppedAt)
			}
		})
	}
}

func TestDebuggerStepOut(t *testing.T) {
	tests := []struct {
		note            string
		events          []*topdown.Event
		expEventIndices []int
	}{
		{
			note: "single query",
			events: []*topdown.Event{
				{ // 0
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
				{ // 1
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
				{ // 2
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
			},
			// We always expect to stop on the first stack event
			expEventIndices: []int{0},
		},
		{
			note: "single query to step out of",
			events: []*topdown.Event{
				{ // 0
					Op:      topdown.EvalOp,
					QueryID: 2,
				},
				{ // 1
					Op:      topdown.EvalOp,
					QueryID: 2,
				},
				{ // 2
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
				{ // 3
					Op:      topdown.RedoOp,
					QueryID: 1,
				},
			},

			expEventIndices: []int{0, 2},
		},
		{
			note: "multiple queries to step out of",
			events: []*topdown.Event{
				{ // 0
					Op:      topdown.EvalOp,
					QueryID: 3,
				},
				{ // 1
					Op:      topdown.EvalOp,
					QueryID: 3,
				},
				{ // 2
					Op:      topdown.EvalOp,
					QueryID: 2,
				},
				{ // 3
					Op:      topdown.RedoOp,
					QueryID: 2,
				},
				{ // 4
					Op:      topdown.RedoOp,
					QueryID: 1,
				},
				{ // 5
					Op:      topdown.EvalOp,
					QueryID: 1,
				},
			},

			expEventIndices: []int{0, 2, 4},
		},
		{
			note: "step-out also steps-over",
			events: []*topdown.Event{
				{ // 0
					Op:      topdown.EvalOp,
					QueryID: 3,
				},
				// Extra query to step over
				{ // 1
					Op:      topdown.EvalOp,
					QueryID: 4,
				},
				{ // 2
					Op:      topdown.RedoOp,
					QueryID: 4,
				},
				{ // 3
					Op:      topdown.EvalOp,
					QueryID: 3,
				},
				{ // 4
					Op:      topdown.EvalOp,
					QueryID: 2,
				},
				{ // 5
					Op:      topdown.RedoOp,
					QueryID: 2,
				},
			},
			expEventIndices: []int{0, 4},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			stk := newTestStack(tc.events...)
			eh := newTestEventHandler()
			_, s, thr := setupDebuggerSession(ctx, stk, LaunchProperties{}, eh.HandleEvent, nil, nil, nil)

			var stoppedAt []int
			doneCh := make(chan struct{})
			defer close(doneCh)
			go func() {
				for {
					e := eh.NextBlocking()

					if e == nil || e.Type == TerminatedEventType {
						break
					}

					if e.Type == StoppedEventType {
						stoppedAt = append(stoppedAt, e.stackIndex)
					}
				}
				doneCh <- struct{}{}
			}()

			go func() {
				for {
					if err := s.StepOut(thr.id); err != nil {
						t.Errorf("Unexpected error stepping over: %v", err)
						break
					}
				}
			}()

			select {
			case <-time.After(5 * time.Second):
				t.Fatal("Timed out waiting for debugger to finish")
			case <-doneCh:
			}

			if !slices.Equal(stoppedAt, tc.expEventIndices) {
				t.Errorf("Expected to stop at event indices %v, got %v", tc.expEventIndices, stoppedAt)
			}
		})
	}
}

func TestDebuggerStackTrace(t *testing.T) {
	tests := []struct {
		note     string
		events   []*topdown.Event
		expTrace []*stackFrame
	}{
		{
			note:     "empty stack",
			expTrace: []*stackFrame{},
		},
		{
			note: "single stack frame, no event node",
			events: []*topdown.Event{
				{
					Op:      topdown.EvalOp,
					QueryID: 1,
					Location: &location.Location{
						File: "test.rego",
						Row:  42,
					},
				},
			},
			expTrace: []*stackFrame{
				{
					id:     1,
					name:   "#1: 1 Eval, test.rego:42",
					thread: 1,
					location: &location.Location{
						File: "test.rego",
						Row:  42,
					},
				},
			},
		},
		{
			note: "single stack frame, event node",
			events: []*topdown.Event{
				{
					Op:      topdown.EvalOp,
					Node:    ast.MustParseExpr("data.test.p[x]"),
					QueryID: 1,
					Location: &location.Location{
						File: "test.rego",
						Row:  42,
					},
				},
			},
			expTrace: []*stackFrame{
				{
					id:     1,
					name:   "#1: 1 | Eval data.test.p[x]",
					thread: 1,
					location: &location.Location{
						File: "test.rego",
						Row:  42,
					},
				},
			},
		},
		{
			note: "multiple stack frames",
			events: []*topdown.Event{
				{
					Op:      topdown.EvalOp,
					Node:    ast.MustParseExpr("y := data.test.p[x]"),
					QueryID: 5,
					Location: &location.Location{
						File: "test.rego",
						Row:  2,
					},
				},
				{
					Op:      topdown.UnifyOp,
					Node:    ast.MustParseExpr("y = 1"),
					QueryID: 5,
					Location: &location.Location{
						File: "test.rego",
						Row:  3,
					},
				},
			},
			// Reversed order
			expTrace: []*stackFrame{
				{
					id:     2,
					name:   "#2: 5 | Unify y = 1",
					thread: 1,
					location: &location.Location{
						File: "test.rego",
						Row:  3,
					},
				},
				{
					id:     1,
					name:   "#1: 5 | Eval assign(y, data.test.p[x])",
					thread: 1,
					location: &location.Location{
						File: "test.rego",
						Row:  2,
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
			defer cancel()

			stk := newTestStack(tc.events...)
			eh := newTestEventHandler()
			_, s, thr := setupDebuggerSession(ctx, stk, LaunchProperties{}, eh.HandleEvent, nil, nil, nil)

			if err := s.start(); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if err := s.ResumeAll(); err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if e := eh.WaitFor(ctx, TerminatedEventType); e == nil {
				t.Fatal("Run never terminated")
			}

			trace, err := s.StackTrace(thr.id)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(trace) != len(tc.expTrace) {
				t.Fatalf("Expected %d stack frames, got %d", len(tc.expTrace), len(trace))
			}

			if len(trace) != len(tc.expTrace) {
				t.Errorf("Expected stack trace:\n\n%v\n\ngot:\n\n%v", tc.expTrace, trace)
			}
			for i := range trace {
				if !trace[i].Equal(tc.expTrace[i]) {
					t.Errorf("Expected stack frame (%d):\n\n%v\n\ngot:\n\n%v", i, tc.expTrace[i], trace[i])
				}
			}
		})
	}
}

func TestDebuggerScopeVariables(t *testing.T) {
	tests := []struct {
		note         string
		input        *ast.Term
		locals       map[ast.Var]ast.Value
		virtualCache map[string]ast.Value
		data         map[string]interface{}
		result       *rego.ResultSet
		expScopes    map[string]scopeInfo
	}{
		{
			note: "no variables",
			expScopes: map[string]scopeInfo{
				"Locals": {
					name:           "Locals",
					namedVariables: 0,
				},
				"Input (not provided)": {
					name:           "Input (not provided)",
					namedVariables: 0,
				},
				"Data (not provided)": {
					name:           "Data (not provided)",
					namedVariables: 0,
				},
			},
		},
		{
			note: "input (object)",
			input: ast.ObjectTerm(
				ast.Item(ast.StringTerm("x"), ast.NumberTerm("1")),
				ast.Item(ast.StringTerm("y"), ast.BooleanTerm(true))),
			expScopes: map[string]scopeInfo{
				"Locals": {
					name:           "Locals",
					namedVariables: 0,
				},
				"Input": {
					name:           "Input",
					namedVariables: 1,
					variables: map[string]varInfo{
						"input": {
							typ: "object",
							val: `{"x": 1, "y": true}`,
							children: map[string]varInfo{
								`"x"`: {
									typ: "number",
									val: "1",
								},
								`"y"`: {
									typ: "boolean",
									val: "true",
								},
							},
						},
					},
				},
				"Data (not provided)": {
					name:           "Data (not provided)",
					namedVariables: 0,
				},
			},
		},
		{
			note: "input (array)",
			input: ast.ArrayTerm(
				ast.StringTerm("foo"),
				ast.NumberTerm("1"),
				ast.BooleanTerm(true)),
			expScopes: map[string]scopeInfo{
				"Locals": {
					name:           "Locals",
					namedVariables: 0,
				},
				"Input": {
					name:           "Input",
					namedVariables: 1,
					variables: map[string]varInfo{
						"input": {
							typ: "array",
							val: `["foo", 1, true]`,
							children: map[string]varInfo{
								"0": {
									typ: "string",
									val: `"foo"`,
								},
								"1": {
									typ: "number",
									val: "1",
								},
								"2": {
									typ: "boolean",
									val: "true",
								},
							},
						},
					},
				},
				"Data (not provided)": {
					name:           "Data (not provided)",
					namedVariables: 0,
				},
			},
		},
		{
			note: "local vars",
			locals: map[ast.Var]ast.Value{
				ast.Var("x"): ast.Number("42"),
				ast.Var("y"): ast.Boolean(true),
				ast.Var("z"): ast.String("foo"),
				ast.Var("obj"): ast.NewObject(
					ast.Item(ast.StringTerm("a"), ast.NumberTerm("1")),
					ast.Item(ast.StringTerm("b"), ast.NumberTerm("2"))),
				ast.Var("arr"): ast.NewArray(ast.NumberTerm("1"), ast.NumberTerm("2"), ast.NumberTerm("3")),
			},
			expScopes: map[string]scopeInfo{
				"Locals": {
					name:           "Locals",
					namedVariables: 5,
					variables: map[string]varInfo{
						"x": {
							typ: "number",
							val: "42",
						},
						"y": {
							typ: "boolean",
							val: "true",
						},
						"z": {
							typ: "string",
							val: `"foo"`,
						},
						"obj": {
							typ: "object",
							val: `{"a": 1, "b": 2}`,
							children: map[string]varInfo{
								`"a"`: {
									typ: "number",
									val: "1",
								},
								`"b"`: {
									typ: "number",
									val: "2",
								},
							},
						},
						"arr": {
							typ: "array",
							val: "[1, 2, 3]",
							children: map[string]varInfo{
								"0": {
									typ: "number",
									val: "1",
								},
								"1": {
									typ: "number",
									val: "2",
								},
								"2": {
									typ: "number",
									val: "3",
								},
							},
						},
					},
				},
				"Input (not provided)": {
					name:           "Input (not provided)",
					namedVariables: 0,
				},
				"Data (not provided)": {
					name:           "Data (not provided)",
					namedVariables: 0,
				},
			},
		},
		{
			note: "local var with long text description",
			locals: map[ast.Var]ast.Value{
				ast.Var("x"): ast.String(strings.Repeat("x", 1000)),
			},
			expScopes: map[string]scopeInfo{
				"Locals": {
					name:           "Locals",
					namedVariables: 1,
					variables: map[string]varInfo{
						"x": {
							typ: "string",
							val: fmt.Sprintf(`"%s...`, strings.Repeat("x", 97)),
						},
					},
				},
				"Input (not provided)": {
					name:           "Input (not provided)",
					namedVariables: 0,
				},
				"Data (not provided)": {
					name:           "Data (not provided)",
					namedVariables: 0,
				},
			},
		},
		{
			note: "result",
			result: &rego.ResultSet{
				rego.Result{
					Expressions: []*rego.ExpressionValue{
						{
							Value: ast.Boolean(true),
							Text:  "x = data.test.allow",
						},
					},
					Bindings: map[string]interface{}{
						"x": ast.Boolean(true),
					},
				},
			},
			expScopes: map[string]scopeInfo{
				"Locals": {
					name:           "Locals",
					namedVariables: 0,
				},
				"Input (not provided)": {
					name:           "Input (not provided)",
					namedVariables: 0,
				},
				"Data (not provided)": {
					name:           "Data (not provided)",
					namedVariables: 0,
				},
				"Result Set": {
					name:           "Result Set",
					namedVariables: 1,
					variables: map[string]varInfo{
						"0": {
							typ: "object",
							val: `{"bindings": {"x": true}, "expressions": [{"text": "x = data.test.allow", "value": true}]}`,
							children: map[string]varInfo{
								`"bindings"`: {
									typ: "object",
									val: `{"x": true}`,
									children: map[string]varInfo{
										`"x"`: {
											typ: "boolean",
											val: "true",
										},
									},
								},
								`"expressions"`: {
									typ: "array",
									val: `[{"text": "x = data.test.allow", "value": true}]`,
									children: map[string]varInfo{
										"0": {
											typ: "object",
											val: `{"text": "x = data.test.allow", "value": true}`,
											children: map[string]varInfo{
												`"text"`: {
													typ: "string",
													val: `"x = data.test.allow"`,
												},
												`"value"`: {
													typ: "boolean",
													val: "true",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			note: "virtual cache",
			virtualCache: map[string]ast.Value{
				"data.foo": ast.String("bar"),
				"data.baz": ast.Number("42"),
			},
			expScopes: map[string]scopeInfo{
				"Locals": {
					name:           "Locals",
					namedVariables: 0,
				},
				"Input (not provided)": {
					name:           "Input (not provided)",
					namedVariables: 0,
				},
				"Data (not provided)": {
					name:           "Data (not provided)",
					namedVariables: 0,
				},
				"Virtual Cache": {
					name:           "Virtual Cache",
					namedVariables: 2,
					variables: map[string]varInfo{
						"data.foo": {
							typ: "string",
							val: `"bar"`,
						},
						"data.baz": {
							typ: "number",
							val: "42",
						},
					},
				},
			},
		},
		{
			note: "data",
			data: map[string]interface{}{
				"foo": "bar",
				"baz": 42,
			},
			expScopes: map[string]scopeInfo{
				"Locals": {
					name:           "Locals",
					namedVariables: 0,
				},
				"Input (not provided)": {
					name:           "Input (not provided)",
					namedVariables: 0,
				},
				"Data": {
					name:           "Data",
					namedVariables: 1,
					variables: map[string]varInfo{
						"data": {
							typ: "object",
							val: `{"baz": 42, "foo": "bar"}`,
							children: map[string]varInfo{
								`"foo"`: {
									typ: "string",
									val: `"bar"`,
								},
								`"baz"`: {
									typ: "number",
									val: "42",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			locals := ast.NewValueMap()
			for k, v := range tc.locals {
				locals.Put(k, v)
			}

			e := topdown.Event{
				Op:     topdown.EvalOp,
				Locals: locals,
			}

			e.WithInput(tc.input)
			events := []*topdown.Event{&e}

			stk := newTestStack(events...)

			if tc.result != nil {
				stk.result = *tc.result
			}

			stk.Next() // Move forward to the first event
			eh := newTestEventHandler()

			var vc topdown.VirtualCache
			if tc.virtualCache != nil {
				vc = topdown.NewVirtualCache()
				for k, v := range tc.virtualCache {
					vc.Put(ast.MustParseRef(k), ast.NewTerm(v))
				}
			}

			var store storage.Store
			if tc.data != nil {
				store = inmem.NewFromObject(tc.data)
			}

			_, s, thr := setupDebuggerSession(ctx, stk, LaunchProperties{}, eh.HandleEvent, vc, store, nil)

			trace, err := s.StackTrace(thr.id)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if len(trace) != 1 {
				t.Fatalf("Expected 1 stack frame, got %d", len(trace))
			}

			scopes, err := s.Scopes(trace[0].ID())
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(scopes) != len(tc.expScopes) {
				t.Fatalf("Expected %d scopes, got %d", len(tc.expScopes), len(scopes))
			}

			for i, scope := range scopes {
				expScope, ok := tc.expScopes[scope.Name()]
				if !ok {
					t.Errorf("Unexpected scope: %s", scopes[i].Name())
					continue
				}

				if scope.Name() != expScope.name {
					t.Errorf("Expected scope name %s, got %s", expScope.name, scope.Name())
				}

				if scope.NamedVariables() != expScope.namedVariables {
					t.Errorf("Expected %d named variables, got %d", expScope.namedVariables, scope.NamedVariables())
				}

				if scope.NamedVariables() > 0 && scope.VariablesReference() == 0 {
					t.Errorf("Expected non-zero variables reference")
				}

				if expScope.namedVariables > 0 {
					vars, err := s.Variables(scope.VariablesReference())
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}

					if len(vars) != expScope.namedVariables {
						t.Fatalf("Expected nuber of variables to equal named variables for scope (%d), got %d", expScope.namedVariables, len(vars))
					}

					assertVariables(t, s, vars, expScope.variables)
				}
			}
		})
	}
}

type varInfo struct {
	typ      string
	val      string
	children map[string]varInfo
}

type scopeInfo struct {
	name           string
	namedVariables int
	variables      map[string]varInfo
}

func assertVariables(t *testing.T, s Session, variables []Variable, exp map[string]varInfo) {
	for _, v := range variables {
		expVar, ok := exp[v.Name()]
		if !ok {
			t.Errorf("Unexpected variable: %s", v.Name())
			continue
		}

		if v.Type() != expVar.typ {
			t.Errorf("Expected variable type %s, got %s", expVar.typ, v.Type())
		}

		if v.Value() != expVar.val {
			t.Errorf("Expected variable value %s, got %s", expVar.val, v.Value())
		}

		if len(expVar.children) != 0 {
			if v.VariablesReference() == 0 {
				t.Errorf("Expected non-zero variables reference")
			}

			vars, err := s.Variables(v.VariablesReference())
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			assertVariables(t, s, vars, expVar.children)
		} else {
			if v.VariablesReference() != 0 {
				t.Errorf("Expected zero variables reference")
			}
		}
	}
}

func setupDebuggerSession(ctx context.Context, stk stack, launchProperties LaunchProperties, eh EventHandler,
	vc topdown.VirtualCache, store storage.Store, l logging.Logger) (*debugger, *session, *thread) {
	if l == nil {
		l = logging.NewNoOpLogger()
	}

	opts := []DebuggerOption{SetLogger(l)}
	if eh != nil {
		opts = append(opts, SetEventHandler(eh))
	}

	varManager := newVariableManager()
	d := newDebugger(opts...)
	t := newThread(1, "test", stk, varManager, vc, store, l)
	s := newSession(ctx, d, varManager, launchProperties, []*thread{t})

	return d, s, t
}

type testEventHandler struct {
	ch chan *Event
}

func newTestEventHandler() *testEventHandler {
	return &testEventHandler{
		ch: make(chan *Event),
	}
}

func (eh *testEventHandler) HandleEvent(event Event) {
	eh.ch <- &event
}

func (eh *testEventHandler) Next(duration time.Duration) *Event {
	select {
	case e := <-eh.ch:
		return e
	case <-time.After(duration):
		return nil
	}
}

func (eh *testEventHandler) NextBlocking() *Event {
	return <-eh.ch
}

func (eh *testEventHandler) WaitFor(ctx context.Context, eventType EventType) *Event {
	for {
		select {
		case e := <-eh.ch:
			if e.Type == eventType {
				return e
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (eh *testEventHandler) IgnoreAll(ctx context.Context) {
	go func() {
		for {
			select {
			case <-eh.ch:
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (eh *testEventHandler) Do(task func() error) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eh.IgnoreAll(ctx)

	return task()
}

type testStack struct {
	events []*topdown.Event
	index  int
	result rego.ResultSet
	closed bool
}

func newTestStack(events ...*topdown.Event) *testStack {
	return &testStack{
		events: events,
		index:  -1,
	}
}

func (ts *testStack) Enabled() bool {
	return true
}

func (ts *testStack) TraceEvent(_ topdown.Event) {
}

func (ts *testStack) Config() topdown.TraceConfig {
	return topdown.TraceConfig{}
}

func (ts *testStack) Current() (int, *topdown.Event) {
	if ts.index < 0 || ts.index >= len(ts.events) {
		return -1, nil
	}
	return ts.index, ts.events[ts.index]
}

func (ts *testStack) Event(i int) *topdown.Event {
	if i >= 0 && i < len(ts.events) {
		return ts.events[i]
	}
	return nil
}

func (ts *testStack) Next() (int, *topdown.Event) {
	if ts.closed || ts.index >= len(ts.events)-1 {
		ts.index++
		return -1, nil
	}
	ts.index++
	return ts.Current()
}

func (ts *testStack) Result() rego.ResultSet {
	return ts.result
}

func (ts *testStack) Close() error {
	ts.closed = true
	return nil
}

func TestDebuggerCustomBuiltIn(t *testing.T) {
	ctx := context.Background()

	decl := &rego.Function{
		Name:        "my.builtin",
		Description: "My built-in",
		Decl: types.NewFunction(
			types.Args(types.S, types.S),
			types.S,
		),
	}

	fn := func(_ rego.BuiltinContext, a, b *ast.Term) (*ast.Term, error) {
		aStr, err := builtins.StringOperand(a.Value, 1)
		if err != nil {
			return nil, err
		}

		bStr, err := builtins.StringOperand(b.Value, 2)
		if err != nil {
			return nil, err
		}

		return ast.StringTerm(fmt.Sprintf("%s+%s", aStr, bStr)), nil
	}

	props := LaunchEvalProperties{
		Query: `x := my.builtin("hello", "world")`,
	}

	exp := `[{"expressions":[{"value":true,"text":"x := my.builtin(\"hello\", \"world\")","location":{"row":1,"col":1}}],"bindings":{"x":"\"hello\"+\"world\""}}]`

	eh := newTestEventHandler()

	d := NewDebugger(SetEventHandler(eh.HandleEvent))

	s, err := d.LaunchEval(ctx, props, RegoOption(rego.Function2(decl, fn)))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := s.ResumeAll(); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// wait for result
	if e := eh.WaitFor(ctx, TerminatedEventType); e == nil {
		t.Fatal("Expected terminated event")
	}

	ts, err := s.Threads()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	res := ts[0].(*thread).stack.Result()
	bs, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	actual := string(bs)
	if actual != exp {
		t.Fatalf("Expected:\n\n%v\n\nbut got:\n\n%v", exp, actual)
	}
}
