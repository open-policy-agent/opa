# ll - A Modern Structured Logging Library for Go

`ll` is a high-performance, production-ready logging library for Go, designed to provide **hierarchical namespaces**, **structured logging**, **middleware pipelines**, **conditional logging**, and support for multiple output formats, including text, JSON, colorized logs, and compatibility with Go’s `slog`. It’s ideal for applications requiring fine-grained log control, extensibility, and scalability.

## Key Features

- **Hierarchical Namespaces**: Organize logs with fine-grained control over subsystems (e.g., "app/db").
- **Structured Logging**: Add key-value metadata for machine-readable logs.
- **Middleware Pipeline**: Customize log processing with error-based rejection.
- **Conditional Logging**: Optimize performance by skipping unnecessary log operations.
- **Multiple Output Formats**: Support for text, JSON, colorized logs, and `slog` integration.
- **Debugging Utilities**: Inspect variables (`Dbg`), binary data (`Dump`), and stack traces (`Stack`).
- **Thread-Safe**: Built for concurrent use with mutex-protected state.
- **Performance Optimized**: Minimal allocations and efficient namespace caching.

## Installation

Install `ll` using Go modules:

```bash
go get github.com/olekukonko/ll
```

Ensure you have Go 1.21 or later for optimal compatibility.

## Getting Started

Here’s a quick example to start logging with `ll`:


```go
package main

import (
  "github.com/olekukonko/ll"
)

func main() {
  // Create a logger with namespace "app"
  logger := ll.New("")

  // enable output
  logger.Enable()

  // Basic log
  logger.Info("Welcome") // Output: [app] INFO: Application started

  logger = logger.Namespace("app")

  // Basic log
  logger.Info("start at :8080") // Output: [app] INFO: Application started

  //Output
  //INFO: Welcome
  //[app] INFO: start at :8080
}

```

```go
package main

import (
    "github.com/olekukonko/ll"
    "github.com/olekukonko/ll/lh"
    "os"
)

func main() {
    // Chaining
    logger := ll.New("app").Enable().Handler(lh.NewTextHandler(os.Stdout))

    // Basic log
    logger.Info("Application started") // Output: [app] INFO: Application started

    // Structured log with fields
    logger.Fields("user", "alice", "status", 200).Info("User logged in")
    // Output: [app] INFO: User logged in [user=alice status=200]

    // Conditional log
    debugMode := false
    logger.If(debugMode).Debug("Debug info") // No output (debugMode is false)
}
```

## Core Features

### 1. Hierarchical Namespaces

Namespaces allow you to organize logs hierarchically, enabling precise control over logging for different parts of your application. This is especially useful for large systems with multiple components.

**Benefits**:
- **Granular Control**: Enable/disable logs for specific subsystems (e.g., "app/db" vs. "app/api").
- **Scalability**: Manage log volume in complex applications.
- **Readability**: Clear namespace paths improve traceability.

**Example**:
```go
logger := ll.New("app").Enable().Handler(lh.NewTextHandler(os.Stdout))

// Child loggers
dbLogger := logger.Namespace("db")
apiLogger := logger.Namespace("api").Style(lx.NestedPath)

// Namespace control
logger.NamespaceEnable("app/db")   // Enable DB logs
logger.NamespaceDisable("app/api") // Disable API logs

dbLogger.Info("Query executed")     // Output: [app/db] INFO: Query executed
apiLogger.Info("Request received")  // No output
```

### 2. Structured Logging

Add key-value metadata to logs for machine-readable output, making it easier to query and analyze logs in tools like ELK or Grafana.

**Example**:
```go
logger := ll.New("app").Enable().Handler(lh.NewTextHandler(os.Stdout))

// Variadic fields
logger.Fields("user", "bob", "status", 200).Info("Request completed")
// Output: [app] INFO: Request completed [user=bob status=200]

// Map-based fields
logger.Field(map[string]interface{}{"method": "GET"}).Info("Request")
// Output: [app] INFO: Request [method=GET]
```

### 3. Middleware Pipeline

Customize log processing with a middleware pipeline. Middleware functions can enrich, filter, or transform logs, using an error-based rejection mechanism (non-nil errors stop logging).

**Example**:
```go
logger := ll.New("app").Enable().Handler(lh.NewTextHandler(os.Stdout))

// Enrich logs with app metadata
logger.Use(ll.FuncMiddleware(func(e *lx.Entry) error {
    if e.Fields == nil {
        e.Fields = make(map[string]interface{})
    }
    e.Fields["app"] = "myapp"
    return nil
}))

// Filter low-level logs
logger.Use(ll.FuncMiddleware(func(e *lx.Entry) error {
    if e.Level < lx.LevelWarn {
        return fmt.Errorf("level too low")
    }
    return nil
}))

logger.Info("Ignored") // No output (filtered)
logger.Warn("Warning") // Output: [app] WARN: Warning [app=myapp]
```

### 4. Conditional Logging

Optimize performance by skipping expensive log operations when conditions are false, ideal for production environments.

**Example**:
```go
logger := ll.New("app").Enable().Handler(lh.NewTextHandler(os.Stdout))

featureEnabled := true
logger.If(featureEnabled).Fields("action", "update").Info("Feature used")
// Output: [app] INFO: Feature used [action=update]

logger.If(false).Info("Ignored") // No output, no processing
```

### 5. Multiple Output Formats

`ll` supports various output formats, including human-readable text, colorized logs, JSON, and integration with Go’s `slog` package.

**Example**:
```go
logger := ll.New("app").Enable()

// Text output
logger.Handler(lh.NewTextHandler(os.Stdout))
logger.Info("Text log") // Output: [app] INFO: Text log

// JSON output
logger.Handler(lh.NewJSONHandler(os.Stdout, time.RFC3339Nano))
logger.Info("JSON log") // Output: {"timestamp":"...","level":"INFO","message":"JSON log","namespace":"app"}

// Slog integration
slogText := slog.NewTextHandler(os.Stdout, nil)
logger.Handler(lh.NewSlogHandler(slogText))
logger.Info("Slog log") // Output: level=INFO msg="Slog log" namespace=app class=Text
```

### 6. Debugging Utilities

`ll` provides powerful tools for debugging, including variable inspection, binary data dumps, and stack traces.

#### Core Debugging Methods

1. **Dbg - Contextual Inspection**
   Inspects variables with file and line context, preserving variable names and handling all Go types.
   ```go
   x := 42
   user := struct{ Name string }{"Alice"}
   ll.Dbg(x)    // Output: [file.go:123] x = 42
   ll.Dbg(user) // Output: [file.go:124] user = [Name:Alice]
   ```

2. **Dump - Binary Inspection**
   Displays a hex/ASCII view of data, optimized for strings, bytes, and complex types (with JSON fallback).
   ```go
   ll.Handler(lh.NewColorizedHandler(os.Stdout))
   ll.Dump("hello\nworld") // Output: Hex/ASCII dump (see example/dump.png)
   ```

3. **Stack - Stack Inspection**
   Logs a stack trace for debugging critical errors.
   ```go
   ll.Handler(lh.NewColorizedHandler(os.Stdout))
   ll.Stack("Critical error") // Output: [app] ERROR: Critical error [stack=...] (see example/stack.png)
   ```

#### Performance Tracking
Measure execution time for performance analysis.
```go
// Automatic measurement
defer ll.Measure(func() { time.Sleep(time.Millisecond) })()
// Output: [app] INFO: function executed [duration=~1ms]

// Explicit benchmarking
start := time.Now()
time.Sleep(time.Millisecond)
ll.Benchmark(start) // Output: [app] INFO: benchmark [start=... end=... duration=...]
```

**Performance Notes**:
- `Dbg` calls are disabled at compile-time when not enabled.
- `Dump` optimizes for primitive types, strings, and bytes with zero-copy paths.
- Stack traces are configurable via `StackSize`.

## Real-World Example: Web Server

A practical example of using `ll` in a web server with structured logging, middleware, and `slog` integration:

```go
package main

import (
    "github.com/olekukonko/ll"
    "github.com/olekukonko/ll/lh"
    "log/slog"
    "net/http"
    "os"
    "time"
)

func main() {
    // Initialize logger with slog handler
    slogHandler := slog.NewJSONHandler(os.Stdout, nil)
    logger := ll.New("server").Enable().Handler(lh.NewSlogHandler(slogHandler))

    // HTTP child logger
    httpLogger := logger.Namespace("http").Style(lx.NestedPath)

    // Middleware for request ID
    httpLogger.Use(ll.FuncMiddleware(func(e *lx.Entry) error {
        if e.Fields == nil {
            e.Fields = make(map[string]interface{})
        }
        e.Fields["request_id"] = "req-" + time.Now().String()
        return nil
    }))

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        httpLogger.Fields("method", r.Method, "path", r.URL.Path).Info("Request received")
        w.Write([]byte("Hello, world!"))
        httpLogger.Fields("duration_ms", time.Since(start).Milliseconds()).Info("Request completed")
    })

    logger.Info("Starting server on :8080")
    http.ListenAndServe(":8080", nil)
}
```

**Sample Output (JSON via slog)**:
```json
{"level":"INFO","msg":"Starting server on :8080","namespace":"server"}
{"level":"INFO","msg":"Request received","namespace":"server/http","class":"Text","method":"GET","path":"/","request_id":"req-..."}
{"level":"INFO","msg":"Request completed","namespace":"server/http","class":"Text","duration_ms":1,"request_id":"req-..."}
```

## Why Choose `ll`?

- **Granular Control**: Hierarchical namespaces for precise log management.
- **Performance**: Conditional logging and optimized concatenation reduce overhead.
- **Extensibility**: Middleware pipeline for custom log processing.
- **Structured Output**: Machine-readable logs with key-value metadata.
- **Flexible Formats**: Text, JSON, colorized, and `slog` support.
- **Debugging Power**: Advanced tools like `Dbg`, `Dump`, and `Stack` for deep inspection.
- **Thread-Safe**: Safe for concurrent use in high-throughput applications.

## Comparison with Other Libraries

| Feature                  | `ll`                     | `log` (stdlib) | `slog` (stdlib) | `zap`             |
|--------------------------|--------------------------|----------------|-----------------|-------------------|
| Hierarchical Namespaces  | ✅                      | ❌             | ❌              | ❌                |
| Structured Logging       | ✅ (Fields, Context)     | ❌             | ✅              | ✅                |
| Middleware Pipeline      | ✅                      | ❌             | ❌              | ✅ (limited)      |
| Conditional Logging      | ✅ (If, IfOne, IfAny)   | ❌             | ❌              | ❌                |
| Slog Compatibility       | ✅                      | ❌             | ✅ (native)     | ❌                |
| Debugging (Dbg, Dump)    | ✅                      | ❌             | ❌              | ❌                |
| Performance (disabled logs) | High (conditional)    | Low            | Medium          | High              |
| Output Formats           | Text, JSON, Color, Slog | Text           | Text, JSON      | JSON, Text       |

## Benchmarks

`ll` is optimized for performance, particularly for disabled logs and structured logging:
- **Disabled Logs**: 30% faster than `slog` due to efficient conditional checks.
- **Structured Logging**: 2x faster than `log` with minimal allocations.
- **Namespace Caching**: Reduces overhead for hierarchical lookups.

See `ll_bench_test.go` for detailed benchmarks on namespace creation, cloning, and field building.

## Testing and Stability

The `ll` library includes a comprehensive test suite (`ll_test.go`) covering:
- Logger configuration, namespaces, and conditional logging.
- Middleware, rate limiting, and sampling.
- Handler output formats (text, JSON, slog).
- Debugging utilities (`Dbg`, `Dump`, `Stack`).

Recent improvements:
- Fixed sampling middleware for reliable behavior at edge cases (0.0 and 1.0 rates).
- Enhanced documentation across `conditional.go`, `field.go`, `global.go`, `ll.go`, `lx.go`, and `ns.go`.
- Added `slog` compatibility via `lh.SlogHandler`.

## Contributing

Contributions are welcome! To contribute:
1. Fork the repository: `github.com/olekukonko/ll`.
2. Create a feature branch: `git checkout -b feature/your-feature`.
3. Commit changes: `git commit -m "Add your feature"`.
4. Push to the branch: `git push origin feature/your-feature`.
5. Open a pull request with a clear description.

Please include tests in `ll_test.go` and update documentation as needed. Follow the Go coding style and run `go test ./...` before submitting.

## License

`ll` is licensed under the MIT License. See [LICENSE](LICENSE) for details.

## Resources

- **Source Code**: [github.com/olekukonko/ll](https://github.com/olekukonko/ll)
- **Issue Tracker**: [github.com/olekukonko/ll/issues](https://github.com/olekukonko/ll/issues)
- **GoDoc**: [pkg.go.dev/github.com/olekukonko/ll](https://pkg.go.dev/github.com/olekukonko/ll)