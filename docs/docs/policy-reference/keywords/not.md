---
sidebar_label: not
title: 'Rego Keyword Examples: not'
---

The `not` keyword is the primary means of expressing
[negation](../../policy-language#negation) in Rego. Similar to other keywords in
Rego, it can also make your policies more 'English-like' and thus easier to
read.

```rego
allow if {
    not input.user.external
}
```

## Examples

<PlaygroundExample dir={require.context('./_examples/not/undefined/')} />

<PlaygroundExample dir={require.context('./_examples/not/negation/')} />

## Improved Negation Semantics

The `future.keywords.not` import fixes a long-standing semantic issue with
negation in Rego.

### The problem with legacy negation

Without the import, the compiler expands a negated composite expression like
`not f(g(input.x))` into a series of sub-expressions evaluated _before_ the
`not`:

```
__local0__ = input.x
g(__local0__, __local1__)
not f(__local1__)
```

If any sub-expression fails — for example, `input.x` is undefined or `g`
produces an undefined result — the entire rule fails rather than the `not` succeeding.
This is unintuitive: the user's intent is "the condition does not hold," but
an undefined intermediate value causes a silent failure instead of the expected
`not` result.

### Implicit body wrapping

With `import future.keywords.not`, composite-expression negation wraps the full
compiler expansion in an implicit body:

```
not { __local0__ = input.x; g(__local0__, __local1__); f(__local1__) }
```

Now, if _any_ sub-expression is undefined or fails, the body is unsatisfiable
and the `not` expression succeeds; matching the intuition that "the condition does not hold."

```json
{
  "user": "cesar"
}
```

<RunSnippet id="input.negation1.json" />

```rego
package negation

import future.keywords.not

# Succeeds when input.role is undefined OR when lookup/admin fail
restricted if {
    not admin(lookup(input.user))
}

groups := {
    "admin": ["alice"],
    "user": ["bob"]
}

lookup(user) := group if {
    some group, members in groups
    user in members
}

admin(group) if group in ["admin", "sudo"]
```

<RunSnippet files="#input.negation1.json" command="data.negation.restricted"/>

:::important
Notice how if we remove the `future.keywords.not` import in the above policy, the `restricted` rule starts failing.
This is a consequence of the `lookup()` function failing with an `undefined` value.
:::

### Explicit negation bodies

The import also enables a `not` expression to take a curly-brace-enclosed body
instead of a single expression:

```json
{
  "servers": [
    {
      "name": "web1",
      "listener": {
        "port": 80,
        "protocol": "tcp"
      }
    },
    {
      "name": "web2",
      "listener": {
        "port": 443,
        "protocol": "tcp"
      }
    },
    {
      "name": "web3",
      "listener": {
        "port": 443,
        "protocol": "udp"
      }
    }
  ]
}
```

<RunSnippet id="input.negation2.json" />

```rego
package negation

import future.keywords.not

# Deny any server that doesn't listen on TCP on port 443
deny contains $"server {server.name} is misconfigured" if {
    some server in input.servers
    not {
        # If any of the following expressions fail, the 'not' succeeds
        listener := server.listener
        listener.port == 443
        listener.protocol == "tcp"
    }
}
```

<RunSnippet files="#input.negation2.json" command="data.negation.deny"/>

The `not` succeeds when the body is **unsatisfiable**; no combination of
variable bindings makes every expression in the body true.

Variables declared inside the body (`listener` above) are scoped locally and are not
visible outside the `not` block.
