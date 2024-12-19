---
title: v0 Backwards Compatibility
kind: documentation
weight: 121
---

## Running OPA in v0.x compatibility mode

The v1.0 release of OPA comes with functionality to run in a backwards
compatible, v0.x mode. This is used by running OPA with the `--v0-compatible`
flag or using the v0.x compatible options in Go integrations. When enabled, OPA
instances and Go integrations will behave as they do in pre v1.0 releases.

### When to use v0.x compatibility mode

**Use of v0.x compatibility mode is not recommended for most users**. This
mode is intended to help users with large volumes of third party Rego stay up to
date while performing a longer term migration to a OPA v1.0 compatible OPA
feature set. Examples of when this applies:

- You run a service for customers who supply their own Rego.
- You use OPA as part of a managed platform and need to run a mix of v0.x and
  v1.0 OPAs based on customer demands.

Users with control over their Rego and OPA deployments are instead encouraged
to migrate their Rego to be compatible with OPA v1.0 using the below tooling options:

1. The `rego.v1` import makes OPA apply all restrictions that are enforced by default in OPA v1.0.
   If a Rego module imports `rego.v1`, it means applicable `future.keywords` imports are implied. It is illegal to import both `rego.v1` and `future.keywords` in the same module.
2. The `--v0-v1` flag on the `opa fmt` command will rewrite existing modules to use the `rego.v1` import instead of `future.keywords` imports.
3. The `--v0-v1` flag on the `opa check` command will check that either the `rego.v1` import or applicable `future.keywords` imports are present if any of the `in`, `every`, `if` and `contains` keywords are used in a module.

### v0.x compatibility mode in the OPA binary

The `--v0-compatible` flag is supported on the following commands in OPA v1.0.x
releases:

- `bench`: supports Rego v0.x syntax modules, use of `import rego.v1` is optional.
- `build`: supports Rego v0.x syntax modules, use of `import rego.v1` is optional.
- `deps`: supports Rego v0.x syntax modules, use of `import rego.v1` is optional.
- `check`*: supports Rego v0.x syntax modules, use of `import rego.v1` is optional.
- `eval`: supports Rego v0.x syntax modules, use of `import rego.v1` is optional.
- `exec`: supports Rego v0.x syntax modules, use of `import rego.v1` is optional.
- `fmt`*: formats modules to be compatible with OPA v0.x syntax. See note about
  `--v0-v1` flag below.
- `inspect`: supports Rego v0.x syntax modules, use of `import rego.v1` is optional.
- `parse`: supports Rego v0.x syntax modules, use of `import rego.v1` is optional.
- `run`: supports modules (including discovery bundle) using Rego v0.x syntax, use of `import rego.v1` is optional. Binds server listeners to all interfaces by default, rather than localhost.
- `test`: supports Rego v0.x syntax modules, use of `import rego.v1` is optional.

Note (*): the `check` and `fmt` commands also support the `--v0-v1` flag,
which will check/format Rego modules as if compatible with the Rego syntax of
_both_ the old 0.x OPA version and current OPA v1.0.

Note (*): Pre v1.0 versions of OPA also support a comparable `--v1-compatible`
flag which can be used to produce and consume Rego v1 bundles. See
[Upgrading to v1.0](./v0-upgrade) for more information on how to use this flag
as part of an upgrade to OPA v1.0.

### v0.x compatibility mode in Rego package

There are three ways to enable v0.x compatibility mode in the [Rego package](https://pkg.go.dev/github.com/open-policy-agent/opa/rego):

1. Set the Rego version on modules
2. Set the Rego version on bundle manifests
3. Use the SetRegoVersion Rego argument

1 & 2 are preferred as they are more granular and make it easier to run a
mix of v0.x and v1.0 compatible Rego in the same OPA instance and thus better
support a gradual upgrade path.

The `SetRegoVersion` method on [Module](https://pkg.go.dev/github.com/open-policy-agent/opa/ast#Module.SetRegoVersion?)
can be used like this:

```go
m := ast.Module{
	Package: regoCode,
}

m.SetRegoVersion(ast.RegoV0)
```

Similarly, the [Bundle Manifest](https://pkg.go.dev/github.com/open-policy-agent/opa/bundle#Manifest.SetRegoVersion) Rego version
can be set like this:

```go
b := Bundle{
    // ...
}
b.SetRegoVersion(ast.RegoV0)
```

If you cannot set the Rego version on modules or bundle manifests, you
can use the [`SetRegoVersion`](https://pkg.go.dev/github.com/open-policy-agent/opa/rego#SetRegoVersion) Rego argument to control the Rego version used when
evaluating policies.

Users are encouraged to use the more granular options where possible to better
allow them to upgrade Rego used in their system to Rego v1 gradually.

In the example below, `SetRegoVersion` is used as a Rego argument instructing
the supplied Rego to be handled as v0.x syntax:

```go
// Only to be used if the above are not suitable.
r := rego.New(
	rego.Query("data.foo.bar"),
	rego.Module("policy.rego", regoCode),
	rego.SetRegoVersion(ast.RegoV1), // <---
)
```

Finally, another option is to import the `v0` package instead. The program

below imports the v0 package instead:

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/open-policy-agent/opa/rego"
    // rather than the v1 import, which is:
	// "github.com/open-policy-agent/opa/v1/rego"
)

func main() {
	module := `package example
messages[msg] {
	msg := "foo"
}
`

	r := v0rego.New(
		rego.Query("data.example.messages"),
		rego.Module("example.rego", module),
	)

	rs, _ = rv0.Eval(context.TODO())
	bs, _ = json.Marshal(rs)

	fmt.Println(string(bs))
}
```

{{< danger >}}
**Note**: Using v0 packages and v1 packages in the same program is considered an
anti-pattern and is not recommended or supported. Any interoperability between
the two packages is not guaranteed and should be considered unsupported.
{{< /danger >}}

### v0.x compatibility mode in the OPA Go SDK

In OPA 1.0, the recommended

[SDK package](https://pkg.go.dev/github.com/open-policy-agent/opa/v1/sdk)
import for most users is `github.com/open-policy-agent/opa/v1/sdk`.

Those who need to support v0 bundles should set the Rego version on bundle
manifests as outlined above wherever possible. For users unable to do this, use
of a v0 import of the SDK package is required. For example:

```go
package main

import (
	"bytes"
	"context"
	"fmt"

	"github.com/open-policy-agent/opa/sdk" // <-- import v0 sdk package
)

func main() {
	opa, _ := sdk.New(ctx, sdk.Options{
		ID:     "opa-1",
		Config: bytes.NewReader(config),
	})

	defer opa.Stop(ctx)

    // ...
}
```

Users in this scenario should look to version bundles as soon as possible to
allow them to use a v1 SDK instead.
