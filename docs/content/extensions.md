---
title: Extending OPA
kind: documentation
weight: 70
---

OPA can be extended with custom built-in functions and plugins that
implement functionality like support for new protocols.

> Support for Go plugins was deprecated in OPA v0.14.0. If you want to customize
> the OPA daemon we recommend you build OPA from source.

This page explains how to customize and extend OPA in different ways.

## Custom Built-in Functions in Go

Read this section if you want to extend OPA with custom built-in functions and
you are using the `github.com/open-policy-agent/opa/rego` package to execute
policies inside of software written in Go.

OPA supports built-in functions for simple operations like string manipulation
and arithmetic as well as more complex operations like JWT verification and
executing HTTP requests. If you need to to extend OPA with custom built-in
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
	))

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

```live:trivial:query:read_only
"hello, bob"
```

The example above highlights a few important points.

* The `rego` package includes variants of `rego.Function1` for passing accepting
  different numbers of operands (e.g., `rego.Function2`, `rego.Function3`, etc.)
* The `rego.Function#Name` struct field specifies the operator that queries can
  refer to.
* The `rego.Function#Decl` struct field specifies the function's type signature.
  In the example above the function accepts a string and returns a string.
* The function indicates it's undefined by returning `nil` for the first return
  argument.

Let's look at another exmaple. Imagine you want to expose GitHub repository
metadata to your policices. One option is to implement a custom built-in
function to fetch the data for specific repositories on-the-fly.

```golang
r := rego.New(
	rego.Query(`github.repo("open-policy-agent", "opa")`),
	rego.Function2(
		&rego.Function{
			Name: "github.repo",
			Decl: types.NewFunction(types.Args(types.S, types.S), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, a, b *ast.Term) (*ast.Term, error) {
			// see implementation below.
		},
	),
)
```

Built-in function names can included `.` characters. Consider namespacing your
built-in functions to avoid collisions. This declaration indicates the function
accepts two strings and returns a value of type `any`. The `any` type is the
union of all types in Rego.

> `types.S` and `types.A` are shortcuts for constructing Rego types. If you need
> to define use-case specific types (e.g., a list of objects that have fields
> `foo`, `bar`, and `baz`, you will need to construct them using the `types`
> packages APIs.)

The declaration also sets `rego.Function#Memoize` to true to enable memoization
across multiple calls in the same query. If your built-in function performs I/O,
you should enable memoization as it ensures function evaluation is
deterministic.

The implementation wraps the Go standard library to perform HTTP requests to
GitHub's API:

```golang
func(bctx rego.BuiltinContext, a, b *ast.Term) (*ast.Term, error) {
	var org, repo string

	if err := ast.As(a.Value, &org); err != nil {
		return nil, err
	} else if ast.As(b.Value, &repo); err != nil {
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

## Custom Plugins for OPA Daemon

Read this section if you want to customize or extend the OPA daemon.

OPA defines a plugin interface that allows you to customize certain behaviour
like decision logging or add new behaviour like different query APIs. To
implement a custom plugin you must implement two interfaces:

- [github.com/open-policy-agent/opa/plugins#Factory](https://godoc.org/github.com/open-policy-agent/opa/plugins#Factory)
  to instantiate your plugin.
- [github.com/open-policy-agent/opa/plugins#Plugin](https://godoc.org/github.com/open-policy-agent/opa/plugins#Plugin)
  to provide your plugin behavior.

You can register your factory with OPA by calling
[github.com/open-policy-agent/opa/runtime#RegisterPlugin](https://godoc.org/github.com/open-policy-agent/opa/runtime#RegisterPlugin)
inside your main function.

### Putting It Together

The example below shows how you can implement a custom [Decision Logger](../management/#decision-logs)
that writes events to a stream (e.g., stdout/stderr).

```golang
import (
	"github.com/open-policy-agent/opa/plugins/logs"
)

type Config struct {
	Stderr bool `json:"stderr"` // false => stdout, true => stderr
}

type PrintlnLogger struct {
	mtx sync.Mutex
	config Config
}

func (p *PrintlnLogger) Start(ctx context.Context) error {
	// No-op.
	return nil
}

func (p *PrintlnLogger) Stop(ctx context.Context) {
	// No-op.
}

func (p *PrintlnLogger) Reconfigure(ctx context.Context, config interface{}) {
    p.mtx.Lock()
    defer p.mtx.Unlock()
    p.config = config.(Config)
}

func (p *PrintlnLogger) Log(ctx context.Context, event logs.EventV1) error {
    p.mtx.Lock()
    defer p.mtx.Unlock()
    w := os.Stdout
    if p.config.Stderr {
        w = os.Stderr
    }
    fmt.Fprintln(w, event) // ignoring errors!
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

func (Factory) New(_ *plugins.Manager, config interface{}) plugins.Plugin {
	return &PrintlnLogger{
		config: config.(Config),
	}
}

func (Factory) Validate(_ *plugins.Manager, config []byte) (interface{}, error) {
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
	runtime.RegisterPlugin("println_decision_logger", Factory{})

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

**config.yaml**:

```yaml
decision_logs:
  plugin: println_decision_logger
plugins:
  println_decision_logger:
    stderr: false
```

Start OPA with the configuration file:

```bash
./opa++ run --server --config-file config.yaml
```

Exercise the plugin via the OPA API:

```
curl localhost:8181/v1/data
```

If everything worked you will see the Go struct representation of the decision
log event written to stdout.

The source code for this example can be found
[here](https://github.com/open-policy-agent/contrib/tree/master/decision_logger_plugin_example).

> If there is a mask policy set (see [Decision
  Logger](../management/#decision-logs) for details) the `Event` received by the
  demo plugin will potentially be different than the example documented.

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
			},
			func(bctx rego.BuiltinContext, a, b *ast.Term) (*ast.Term, error) {

				var org, repo string

				if err := ast.As(a.Value, &org); err != nil {
					return nil, err
				} else if ast.As(b.Value, &repo); err != nil {
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
