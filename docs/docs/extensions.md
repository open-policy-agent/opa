---
title: Extending OPA
sidebar_position: 8
---

OPA can be extended with custom built-in functions and plugins that
implement functionality like support for new protocols. This page explains how
to customize and extend OPA in different ways.

## Custom Built-in Functions in Go

Read this section if you want to extend OPA with custom built-in functions.

:::info
This section assumes you are embedding OPA as a library and executing policies
via the `github.com/open-policy-agent/opa/rego` package. If you are NOT embedding OPA
as a library and instead want to customize the OPA runtime, read this section
anyway because it provides useful information on implementing built-in functions.
For a complete example that shows how to add custom built-in functions to the
OPA runtime, see the [Adding Built-in Functions to the OPA Runtime](#adding-built-in-functions-to-the-opa-runtime) appendix.
:::

OPA supports built-in functions for simple operations like string manipulation
and arithmetic as well as more complex operations like JWT verification and
executing HTTP requests. If you need to extend OPA with custom built-in
functions for use cases or integrations that are not supported out-of-the-box
you can supply the function definitions when you prepare queries.

Using custom built-in functions involves providing a declaration and
implementation. The declaration tells OPA the function's type signature and the
implementation provides the callback that OPA can execute during query
evaluation.

To get started you need to import three packages:

```
import "github.com/open-policy-agent/opa/ast"
import "github.com/open-policy-agent/opa/types"
import "github.com/open-policy-agent/opa/rego"
```

The `ast` and `types` packages contain the types for declarations and runtime
objects passed to your implementation. Here is a trivial example that shows the
process:

```golang
r := rego.New(
    rego.Query(`x = hello("bob")`),
    rego.Function1(
        &rego.Function{
            Name: "hello",
            Decl: types.NewFunction(types.Args(types.S), types.S),
        },
        func(_ rego.BuiltinContext, a *ast.Term) (*ast.Term, error) {
            if str, ok := a.Value.(ast.String); ok {
                return ast.StringTerm("hello, " + string(str)), nil
            }
            return nil, nil
        }),
)

query, err := r.PrepareForEval(ctx)
if err != nil {
    // handle error.
}
```

At this point you can execute the `query`:

```golang
rs, err := query.Eval(ctx)
if err != nil {
    // handle error.
}

// Do something with result.
fmt.Println(rs[0].Bindings["x"])
```

If you executed this code you the output would be:

```rego
"hello, bob"
```

The example above highlights a few important points.

- The `rego` package includes variants of `rego.Function1` for accepting
  different numbers of operands (e.g., `rego.Function2`, `rego.Function3`, etc.)
- The `rego.Function#Name` struct field specifies the operator that queries can
  refer to.
- The `rego.Function#Decl` struct field specifies the function's type signature.
  In the example above the function accepts a string and returns a string.
- The function indicates it's undefined by returning `nil` for the first return
  argument.

The following example shows another use case. Imagine you want to expose GitHub repository
metadata to your policies. One option is to implement a custom built-in
function to fetch the data for specific repositories on-the-fly.

```golang
r := rego.New(
    rego.Query(`github.repo("open-policy-agent", "opa")`),
    rego.Function2(
        &rego.Function{
            Name: "github.repo",
            Decl: types.NewFunction(types.Args(types.S, types.S), types.A),
            Memoize: true,
            Nondeterministic: true,
        },
        func(bctx rego.BuiltinContext, a, b *ast.Term) (*ast.Term, error) {
            // see implementation below.
        },
    ),
)
```

Built-in function names can include `.` characters. Consider namespacing your
built-in functions to avoid collisions. This declaration indicates the function
accepts two strings and returns a value of type `any`. The `any` type is the
union of all types in Rego.

:::info
`types.S` and `types.A` are shortcuts for constructing Rego types. If you need
to define use-case specific types (e.g., a list of objects that have fields
`foo`, `bar`, and `baz`, you will need to construct them using the `types`
packages APIs.)
:::

The declaration also sets `rego.Function#Memoize` to true to enable memoization
across multiple calls in the same query. If your built-in function performs I/O,
you should enable memoization as it ensures function evaluation is
deterministic.

Since this built-in could have non-deterministic results, depending on network
conditions, the declaration also sets `rego.Function#Nondeterministic` to true.
This provides basic safety information to the runtime, so that the function
isn't accidentally run during bundle builds or partial evaluation. If your
builtin can have non-deterministic results, you should mark it appropriately
to avoid surprises.

The implementation wraps the Go standard library to perform HTTP requests to
GitHub's API:

```golang
func(bctx rego.BuiltinContext, a, b *ast.Term) (*ast.Term, error) {
    var org, repo string

    if err := ast.As(a.Value, &org); err != nil {
        return nil, err
    } else if err := ast.As(b.Value, &repo); err != nil {
        return nil, err
    }

    req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%v/%v", org, repo), nil)
    if err != nil {
        return nil, err
    }

    resp, err := http.DefaultClient.Do(req.WithContext(bctx.Context))
    if err != nil {
        return nil, err
    }

    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf(resp.Status)
    }

    v, err := ast.ValueFromReader(resp.Body)
    if err != nil {
        return nil, err
    }

    return ast.NewTerm(v), nil
}
```

The implementation is careful to use the context passed to the built-in function
when executing the HTTP request. See the appendix at the end of this page for
the complete example.

:::danger
Custom built-in functions **must not** be used for effecting changes in
external systems as OPA does not guarantee that the statement will be executed due
to automatic performance optimizations that are applied during policy evaluation.
:::

## Custom Plugins for OPA Runtime

Read this section if you want to customize or extend the OPA runtime/executable
with custom behaviour.

OPA defines a plugin interface that allows you to customize certain behaviour
like decision logging or add new behaviour like different query APIs. To
implement a custom plugin you must implement two interfaces:

- [`Factory`](https://pkg.go.dev/github.com/open-policy-agent/opa/plugins#Factory)
  to instantiate your plugin.
- [`Plugin`](https://pkg.go.dev/github.com/open-policy-agent/opa/plugins#Plugin)
  to provide your plugin behavior.

You can register your factory with OPA by calling
[`RegisterPlugin`](https://pkg.go.dev/github.com/open-policy-agent/opa/runtime#RegisterPlugin)
inside your main function.

### Plugin Status

The plugin may (optionally) report its current status to the plugin Manager via the `plugins.Manager#UpdatePluginStatus`
API.

:::info
If no status is provided the plugin is assumed to be working OK.
:::

Typically the plugin should report `StatusNotReady` at creation time and update to `StatusOK` (or `StatusErr`) when
appropriate.

### Plugin Configuration Validation

OPA passes the plugin's configuration to the factory's `Validate` function as raw
bytes and doesn't check its keys, so a misspelled option is silently ignored. To have
OPA [warn about unrecognized options](./configuration#configuration-validation)
under your plugin's configuration, register the keys it recognizes with
[`config.RegisterConfigSpec`](https://pkg.go.dev/github.com/open-policy-agent/opa/v1/config#RegisterConfigSpec).
[`config.SpecsFromStruct`](https://pkg.go.dev/github.com/open-policy-agent/opa/v1/config#SpecsFromStruct)
derives them from the JSON field names of your config struct, so they stay in sync
with what `Validate` decodes:

```golang
import "github.com/open-policy-agent/opa/v1/config"

func init() {
    config.RegisterConfigSpec(config.SpecsFromStruct[Config]("plugins", PluginName)...)
}
```

With this in place, `plugins.println_decision_logger.stdrr: true` in the example
below is reported as
`unknown configuration option "plugins.println_decision_logger.stdrr" encountered`.
Nested structs are checked too, and a map or slice of structs matches any entry.
Maps and slices of other types accept any keys.

Unrecognized options are warnings only. Return an error from `Validate` to reject
a configuration.

#### Validating with Rego

Like OPA does for its own configuration, a plugin can validate its configuration
and inject defaults with a Rego policy. Create one with
[`config.NewPolicy`](https://pkg.go.dev/github.com/open-policy-agent/opa/v1/config#NewPolicy)
and evaluate it in `Validate` with
[`config.EvalPolicyInto`](https://pkg.go.dev/github.com/open-policy-agent/opa/v1/config#EvalPolicyInto),
which decodes the processed configuration into your config struct. Keep the
policy in a `.rego` file in your plugin's package directory, next to the Go
file that embeds it with
[`go:embed`](https://pkg.go.dev/embed), so it is compiled into your binary.
See [Validating the Example with Rego](#validating-the-example-with-rego) for
a complete example.

The first argument to `config.NewPolicy` is only a label for the module: it
appears in error messages, such as a Rego compile error. It isn't a path and
isn't looked up anywhere, so a descriptive name for your plugin's policy works.
The package path that the policy's rules live under is taken from the module's
own `package` declaration.

The policy is evaluated with `input.config` set to the plugin's raw configuration
and defines these rules in its package:

| Rule        | Description                                                               |
| ----------- | ------------------------------------------------------------------------- |
| `processed` | Required. The configuration with defaults injected.                       |
| `errors`    | Optional. A set of messages; if non-empty, they are returned as an error. |
| `warnings`  | Optional. A set of messages, returned for the plugin to log.              |

The module can import `data.opa.config.util` for the helpers OPA's own validation
policies use (`value`, `absent`, `not_object` and `not_positive_number`).

### Putting It Together

The example below shows how you can implement a custom [Decision Logger](./management-decision-logs)
that writes events to a stream (e.g., stdout/stderr).

```golang
import (
    "encoding/json"

    "github.com/open-policy-agent/opa/plugins/logs"
)

const PluginName = "println_decision_logger"

type Config struct {
    Stderr bool `json:"stderr"` // false => stdout, true => stderr
}

type PrintlnLogger struct {
    manager *plugins.Manager
    mtx     sync.Mutex
    config  Config
}

func (p *PrintlnLogger) Start(ctx context.Context) error {
    p.manager.UpdatePluginStatus(PluginName, &plugins.Status{State: plugins.StateOK})
    return nil
}

func (p *PrintlnLogger) Stop(ctx context.Context) {
    p.manager.UpdatePluginStatus(PluginName, &plugins.Status{State: plugins.StateNotReady})
}

func (p *PrintlnLogger) Reconfigure(ctx context.Context, config any) {
    p.mtx.Lock()
    defer p.mtx.Unlock()
    p.config = config.(Config)
}


// Log is called by the decision logger when a record (event) should be emitted. The logs.EventV1 fields
// map 1:1 to those described in https://www.openpolicyagent.org/docs/latest/management-decision-logs
func (p *PrintlnLogger) Log(ctx context.Context, event logs.EventV1) error {
    p.mtx.Lock()
    defer p.mtx.Unlock()
    w := os.Stdout
    if p.config.Stderr {
        w = os.Stderr
    }
    bs, err := json.Marshal(event)
    if err != nil {
        p.manager.UpdatePluginStatus(PluginName, &plugins.Status{State: plugins.StateErr})
        return nil
    }
    _, err = fmt.Fprintln(w, string(bs))
    if err != nil {
        p.manager.UpdatePluginStatus(PluginName, &plugins.Status{State: plugins.StateErr})
    }
    return nil
}
```

Next, implement a factory function that instantiates your plugin:

```golang
import (
    "github.com/open-policy-agent/opa/plugins"
    "github.com/open-policy-agent/opa/util"
)

type Factory struct{}

func (Factory) New(m *plugins.Manager, config any) plugins.Plugin {

    m.UpdatePluginStatus(PluginName, &plugins.Status{State: plugins.StateNotReady})

    return &PrintlnLogger{
        manager: m,
        config:  config.(Config),
    }
}

func (Factory) Validate(_ *plugins.Manager, config []byte) (any, error) {
    parsedConfig := Config{}
    return parsedConfig, util.Unmarshal(config, &parsedConfig)
}
```

Finally, register your factory with OPA and call `cmd.RootCommand.Execute`. The
latter starts OPA and does not return.

```golang
import (
    "github.com/open-policy-agent/opa/cmd"
    "github.com/open-policy-agent/opa/runtime"
)

func main() {
    runtime.RegisterPlugin(PluginName, Factory{})

    if err := cmd.RootCommand.Execute(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}
```

At this point you can build an OPA executable including your plugin.

```
go build -o opa++
```

Define an OPA configuration file that will use your plugin:

**opa-config.yaml**:

```yaml
decision_logs:
  plugin: println_decision_logger
plugins:
  println_decision_logger:
    stderr: false
```

Start OPA with the configuration file:

```bash
./opa++ run --server --config-file opa-config.yaml
```

Exercise the plugin via the OPA API:

```
curl localhost:8181/v1/data
```

If everything worked you will see the Go struct representation of the decision
log event written to stdout.

The source code for this example can be found
[in the contrib repository](https://github.com/open-policy-agent/contrib/tree/main/decision_logger_plugin_example).

:::info
If there is a mask policy set (see [Decision Logger](./management-decision-logs)
for details) the `Event` received by the demo plugin will potentially be different
than the example documented.
:::

### Validating the Example with Rego

The `Validate` function above only decodes the configuration, so
`stderr: "yes"` fails with a generic decoding error and a misspelled option such
as `stdrr` is silently ignored. The same plugin can instead validate its
configuration with a Rego policy, as described in
[Validating with Rego](#validating-with-rego).

**validate.rego**:

```rego
package println_decision_logger.config

# Default to stdout when "stderr" is not set.
processed := object.union({"stderr": false}, input.config)

errors contains "stderr must be a boolean" if {
    stderr := input.config.stderr
    not is_boolean(stderr)
}

warnings contains $"unknown configuration option \"{key}\"" if {
    some key, _ in input.config
    key != "stderr"
}
```

Embed the policy and evaluate it in `Validate`, in place of the plain decode:

```golang
import (
    "context"
    _ "embed"

    "github.com/open-policy-agent/opa/v1/config"
    "github.com/open-policy-agent/opa/v1/plugins"
)

//go:embed validate.rego
var validationModule string

var validationPolicy *config.Policy

func init() {
    p, err := config.NewPolicy("println_decision_logger_config_validation", validationModule)
    if err != nil {
        panic(err)
    }
    validationPolicy = p
}

func (Factory) Validate(m *plugins.Manager, bs []byte) (any, error) {
    var parsedConfig Config
    warnings, err := config.EvalPolicyInto(context.TODO(), validationPolicy, bs, &parsedConfig)
    if err != nil {
        return nil, err
    }
    for _, w := range warnings {
        m.Logger().Warn("println_decision_logger: %s", w)
    }
    return parsedConfig, nil
}
```

With this in place, an invalid value is rejected at startup:

```yaml
plugins:
  println_decision_logger:
    stderr: "yes"
```

```
error: stderr must be a boolean
```

and an unrecognized option is logged as a warning while the plugin still starts:

```yaml
plugins:
  println_decision_logger:
    stdrr: true
```

```
{"level":"warning","msg":"println_decision_logger: unknown configuration option \"stdrr\""}
```

## Custom Storage Backends

OPA's default in-memory storage can be replaced with custom implementations.

Custom storage backends must implement the [`storage.Store`](https://pkg.go.dev/github.com/open-policy-agent/opa/v1/storage#Store) interface. Register your backend by calling [`v1/runtime.RegisterStorageBackend`](https://pkg.go.dev/github.com/open-policy-agent/opa/v1/runtime#RegisterStorageBackend) before OPA runtime initialization.

### Example

```go
package main

import (
    "github.com/open-policy-agent/opa/cmd"
    "github.com/open-policy-agent/opa/v1/runtime"
)

func init() {
    runtime.RegisterStorageBackend(func(
        ctx context.Context,
        logger logging.Logger,
        registerer prometheus.Registerer,
        config []byte,
        id string,
    ) (storage.Store, error) {
        return myCustomStore, nil
    })
}

func main() {
    if err := cmd.RootCommand.Execute(); err != nil {
        os.Exit(1)
    }
}
```

If your storage needs resource cleanup (close connections, flush buffers, etc.), implement the [`storage.Closer`](https://pkg.go.dev/github.com/open-policy-agent/opa/v1/storage#Closer) interface. The `Close()` method will be called during graceful shutdown

## Setting the OPA Runtime Version

The OPA runtime version is set statically at build-time. The following global variables
are exported by the `github.com/open-policy-agent/opa/version` package and can be
set at build-time:

| Name        | Description                                             |
| ----------- | ------------------------------------------------------- |
| `Version`   | Human-readable/semantic version of the OPA runtime.     |
| `Vcs`       | Git SHA that the OPA runtime was built from.            |
| `Timestamp` | Date/time when the OPA runtime was built.               |
| `Hostname`  | Hostname of the system where the OPA runtime was built. |

These values can be set on the command-line when building OPA from source:

```
go build \
  -ldflags=" \
  -X github.com/open-policy-agent/opa/v1/version.Version=MY_VERSION\
  -X github.com/open-policy-agent/opa/v1/version.Vcs=MY_COMMIT_HASH \
  -X github.com/open-policy-agent/opa/v1/version.Hostname=MY_HOSTNAME \
  -X github.com/open-policy-agent/opa/v1/version.Timestamp=MY_TIMESTAMP" \
  -o opa++
```

## Appendix

### Custom Built-in Function in Go

```golang
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"

    "github.com/open-policy-agent/opa/ast"
    "github.com/open-policy-agent/opa/rego"
    "github.com/open-policy-agent/opa/types"
)

func main() {

    r := rego.New(
        rego.Query(`github.repo("open-policy-agent", "opa")`),
        rego.Function2(
            &rego.Function{
                Name:    "github.repo",
                Decl:    types.NewFunction(types.Args(types.S, types.S), types.A),
                Memoize: true,
                Nondeterministic: true,
            },
            func(bctx rego.BuiltinContext, a, b *ast.Term) (*ast.Term, error) {

                var org, repo string

                if err := ast.As(a.Value, &org); err != nil {
                    return nil, err
                } else if err := ast.As(b.Value, &repo); err != nil {
                    return nil, err
                }

                req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%v/%v", org, repo), nil)
                if err != nil {
                    return nil, err
                }

                resp, err := http.DefaultClient.Do(req.WithContext(bctx.Context))
                if err != nil {
                    return nil, err
                }

                defer resp.Body.Close()

                if resp.StatusCode != http.StatusOK {
                    return nil, fmt.Errorf(resp.Status)
                }

                v, err := ast.ValueFromReader(resp.Body)
                if err != nil {
                    return nil, err
                }

                return ast.NewTerm(v), nil
            },
        ),
    )

    rs, err := r.Eval(context.Background())
    if err != nil {
        log.Fatal(err)
    } else if len(rs) == 0 {
        fmt.Println("undefined")
    } else {
        bs, _ := json.MarshalIndent(rs[0].Expressions[0].Value, "", "  ")
        fmt.Println(string(bs))
    }
}
```

### Adding Built-in Functions to the OPA Runtime

```golang
package main

import (
    "fmt"
    "net/http"
    "os"

    "github.com/open-policy-agent/opa/ast"
    "github.com/open-policy-agent/opa/cmd"
    "github.com/open-policy-agent/opa/rego"
    "github.com/open-policy-agent/opa/types"

)

func main() {

    rego.RegisterBuiltin2(
        &rego.Function{
            Name:    "github.repo",
            Decl:    types.NewFunction(types.Args(types.S, types.S), types.A),
            Memoize: true,
            Nondeterministic: true,
        },
        func(bctx rego.BuiltinContext, a, b *ast.Term) (*ast.Term, error) {

            var org, repo string

            if err := ast.As(a.Value, &org); err != nil {
                return nil, err
            } else if err := ast.As(b.Value, &repo); err != nil {
                return nil, err
            }

            req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%v/%v", org, repo), nil)
            if err != nil {
                return nil, err
            }

            resp, err := http.DefaultClient.Do(req.WithContext(bctx.Context))
            if err != nil {
                return nil, err
            }

            defer resp.Body.Close()

            if resp.StatusCode != http.StatusOK {
                return nil, fmt.Errorf(resp.Status)
            }

            v, err := ast.ValueFromReader(resp.Body)
            if err != nil {
                return nil, err
            }

            return ast.NewTerm(v), nil
        },
    )

    if err := cmd.RootCommand.Execute(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}
```
