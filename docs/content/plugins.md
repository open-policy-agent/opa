---
title: Plugins (Experimental)
navtitle: Plugins
kind: documentation
weight: 14
---

OPA can be extended with custom built-in functions and plugins that
implement functionality like support for new protocols.

> This page focuses on how to build [Go
> plugins](https://golang.org/pkg/plugin/) that can be loaded when OPA
> starts however the steps are similar if you are embedding OPA as a
> library or building from source.

## Building Go Plugins

At minimum, your Go plugin must implement the following:

```golang
package main

func Init() error {
    // your init function
}
```

When OPA starts, it will invoke the `Init` function which can:

* Register custom built-in functions.
* Register custom OPA plugins (e.g., decision loggers, servers, etc.)
* ...or do anything else.

See the sections below for examples.

To build your plugin into a shared object file (`.so`), you will
(minimally) run the following command:

```bash
go build -buildmode=plugin -o=plugin.so plugin.go
```

This will produce a file named `plugin.so` that you can pass to OPA
with the `--plugin-dir` flag. OPA will load all of the `.so` files out
of the directory you give it.

```bash
opa --plugin-dir=/path/to/plugins run
```

**NOTE:** You must build your plugin against the same version of the
  OPA that will eventually load the shared object file. If you build
  your plugin against a different version of the OPA source, the OPA
  will fail to start. You will see an error message like:

```
Error: plugin.Open("plugin/logger"): plugin was built with a different version of package github.com/open-policy-agent/opa/ast
```

## Built-in Functions

To implement custom built-in functions your `Init` function should call:

- `ast.RegisterBuiltin` to declare the built-in function.
- `topdown.RegisterFunctionalBuiltin[X]` to register the built-in function implementation (where X is replaced by the number of parameters your function receives.)

For example:

```golang
package main

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

var HelloBuiltin = &ast.Builtin{
	Name: "hello",
	Decl: types.NewFunction(
		types.Args(types.S),
		types.S,
	),
}

func HelloImpl(a ast.Value) (ast.Value, error) {
	s, err := builtins.StringOperand(a, 1)
	if err != nil {
		return nil, err
	}
	return ast.String("hello, " + string(s)), nil
}

func Init() error {
	ast.RegisterBuiltin(HelloBuiltin)
	topdown.RegisterFunctionalBuiltin1(HelloBuiltin.Name, HelloImpl)
	return nil
}
```

If you build this file into a shared object and start OPA with it you can call it like other built-in functions:

```
> hello("bob")
"hello, bob"
```

For more details on implementing built-in functions, see the [OPA Go Documentation](https://godoc.org/github.com/open-policy-agent/opa/topdown#example-RegisterFunctionalBuiltin1).

## Custom Plugins

OPA defines a plugin interface that allows you to customize certain
behaviour like decision logging or add new behaviour like different
query APIs. To implement a custom plugin you must implement two
interfaces:

- [github.com/open-policy-agent/opa/plugins#Factory](https://godoc.org/github.com/open-policy-agent/opa/plugins#Factory) to instantiate your plugin.
- [github.com/open-policy-agent/opa/plugins#Plugin](https://godoc.org/github.com/open-policy-agent/opa/plugins#Plugin) to provide your plugin behavior.

You can register your factory with OPA by calling [github.com/open-policy-agent/opa/runtime#RegisterPlugin](https://godoc.org/github.com/open-policy-agent/opa/runtime#RegisterPlugin) inside your `Init` function.

### Putting It Together

The example below shows how you can implement a custom [Decision Logger](../decision-logs)
that writes events to a stream (e.g., stdout/stderr).

```golang
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

Finally, register your factory with OPA:

```golang
func Init() {
    runtime.RegisterPlugin("println_decision_logger", Factory{})
}
```

To test your plugin, build a shared object file:

```
go build -buildmode=plugin -o=plugin.so main.go
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

Start OPA with the plugin directory and configuration file:

```bash
opa --plugin-dir $PWD run --server --config-file config.yaml
```

Exercise the plugin via the OPA API:

```
curl localhost:8181/v1/data
```

If everything worked you will see the Go struct representation of the decision
log event written to stdout.

The source code for this example can be found [here](https://github.com/open-policy-agent/contrib/tree/master/decision_logger_plugin_example).
