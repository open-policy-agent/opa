# OPA Debug API

This directory contains the OPA Debug API. The Debug API facilitates
programmatic debugging of Rego policies, on top of which 3rd parties can build
tools for debugging.

This API takes inspiration from the
[Debug Adapter Protocol (DAP)](https://microsoft.github.io/debug-adapter-protocol/),
and follows the conventions established therein for managing threads,
breakpoints, and variable scopes.

> [!TIP]
> The Debug API current actively supported in two clients
> [VS Code](https://github.com/open-policy-agent/vscode-opa) and
> [Neovim](https://github.com/rinx/nvim-dap-rego/tree/main). Both
> [Regal's Debug Adapter](https://www.openpolicyagent.org/projects/regal/debug-adapter) as the
> backend, which is based on this API.

> [!WARNING]
> The Debug API is experimental and subject to change.

## Creating a Debug Session

```go
debugger := debug.NewDebugger()

ctx := context.Background()
evalProps := debug.EvalProperties{
    Query: "data.example.allow = x",
    InputPath: "/path/to/input.json",
    LaunchProperties: LaunchProperties{
        DataPaths: []string{"/path/to/data.json", "/path/to/policy.rego"},
    },
}
session, err := s.debugger.LaunchEval(ctx, evalProps)
if err != nil {
    // handle error
}

// The session is launched in a paused state.
// Before resuming the session, here is the opportunity to set breakpoints

// Resume execution of all threads associated with the session
err = session.ResumeAll()
if err != nil {
    // handle error
}
```

## Managing Breakpoints

Breakpoints can be added, removed, and enumerated.

Breakpoints are added to file-and-row locations in a module, and are triggered when the policy evaluation reaches that location.
Breakpoints can be added at any time during policy evaluation.

```go
// Add a breakpoint
br, err := session.AddBreakpoint(location.Location{
    File: "/path/to/policy.rego",
    Row: 10,
})
if err != nil {
    // handle error
}

// ...

// Remove the breakpoint
_, err = session.RemoveBreakpoint(br.ID)
if err != nil {
    // handle error
}
```

## Stepping Through Policy Evaluation

When evaluation execution is paused, either immidiately after launching a session or when a breakpoint is hit, the session can be stepped through.

### Step Over

`StepOver()` executes the next expression in the current scope and then stops on the next expression in the same scope,
not stopping on expressions in sub-scopes; e.g. execution of referenced rule, called function, comprehension, or every expression.

```go
threads, err := session.Threads()
if err != nil {
    // handle error
}

if err := session.StepOver(threads[0].ID); err != nil {
    // handle error
}
```

#### Example 1

```
allow if {
  x := f(input) >-+
  x == 1        <-+
}

f(x) := y if {
  y := x + 1
}
```

#### Example 2

```
allow if {
  every x in l { >-+
    x < 10         |
  }                |
  input.x == 1   <-+
```

### Step In

`StepIn()` executes the next expression in the current scope and then stops on the next expression in the same scope or sub-scope;
stepping into any referenced rule, called function, comprehension, or every expression.

```go
if err := session.StepIn(threads[0].ID); err != nil {
    // handle error
}
```

#### Example 1

```
allow if {
  x := f(input) >-+
  x == 1          |
}                 |
                  |
f(x) := y if {  <-+
  y := x + 1
}
```

#### Example 2

```
allow if {
  every x in l { >-+
    x < 10       <-+
  }
  input.x == 1
}
```

### Step Out

`StepOut()` steps out of the current scope (rule, function, comprehension, every expression) and stops on the next expression in the parent scope.

```go
if err := session.StepOut(threads[0].ID); err != nil {
    // handle error
}
```

#### Example 1

```
allow if {
  x := f(input) <-+
  x == 1          |
}                 |
                  |
f(x) := y if {    |
  y := x + 1    >-+
}
```

#### Example 2

```
allow if {
  every x in l {
    x < 10       >-+
  }                |
  input.x == 1   <-+
}
```

## Stack Traces

`StackTrace()` returns the frames of a thread, ordered from the most recent frame to the least recent.
How trace events are grouped into frames is controlled by `LaunchProperties.StackTraceMode`, which accepts
three values:

| Value     | Constant                | Mode                                    |
| --------- | ----------------------- | --------------------------------------- |
| `""`      | `StackTraceModeDefault` | same as `query`; the zero value default |
| `"query"` | `StackTraceModeQuery`   | one frame per query scope               |
| `"event"` | `StackTraceModeEvent`   | one frame per consumed trace event      |

```go
launchProps := debug.LaunchProperties{
    StackTraceMode: debug.StackTraceModeQuery, // the default
}
```

### Query Mode

`StackTraceModeQuery` (the default) creates one frame per query scope: the base query, a rule or
function body, a comprehension, or an `every` expression and its body. Frames are pushed and popped as
evaluation descends into, and returns from, referenced rules, called functions, comprehensions, and every
expressions.

A frame is named for the query it represents, prefixed with the ID of that query to allow correlating
frames with the query IDs reported in OPA's traces. A frame is located on the expression that query is
currently evaluating. For a frame below the top of the stack, that is the expression from which the query
above it was entered. Expressions filtered out by `LaunchProperties.SkipOps` are not considered.

Stopped on a breakpoint on row 7 of:

```rego
package example         # 1
                        # 2
allow if {              # 3
	x := f(input.n)     # 4
	x > 1               # 5
	every y in input.l { # 6
		y < 10          # 7 <- breakpoint
	}                   # 8
}                       # 9
                        # 10
f(n) := n + 1           # 11
```

the stack trace has four frames:

```
4: lt(__local2__, 10)                                          policy.rego:7
3: every __local1__, __local2__ in __local5__ { lt(...) }      policy.rego:6
1: data.example.allow                                          policy.rego:6
0: data.example.allow = x                                      query:1
```

Note that frame names are rendered from the compiled policy, so rewritten local variables appear in their
generated form.

### Event Mode

`StackTraceModeEvent` creates one frame per trace event consumed by the debugger, named after the
event's pretty-printed trace line. Events filtered out by `LaunchProperties.SkipOps` also get a frame, and
the number of frames grows with the number of events consumed rather than with query nesting; the same
stop as above produces 29 frames.

## Fetching Variable Values

The current values of local and global variables are organized into scopes:

- `Local`: contains variables defined in the current rule, function, comprehension, or every expression.
- `Virtual Cache`: contains the state of the global Virtual Cache, where calculated return values for rules and functions are stored.
- `Input`: contains the input document.
- `Data`: contains the data document.
- `Result Set`: contains the result set of the current query. This scope is only available on the final expression of the query evaluation.

```go
scopes, err := session.Scopes(thread.ID)
if err != nil {
    // handle error
}

var localScope debug.Scope
for _, scope := range scopes {
    if scope.Name == "Local" {
        localScope = scope
        break
    }
}

variables, err := session.Variables(localScope.VariablesReference())
if err != nil {
    // handle error
}

// Enumerate and process variables
```
