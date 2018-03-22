# Frequently Asked Questions

## How does OPA do conflict resolution?

In Rego (OPA's policy language), you can write statements that both allow and
deny a request, such as

```
package foo
allow { input.name = "alice" }
deny { input.name = "alice" }
```

Neither `allow` nor `deny` are keywords in Rego so if you want to treat them
as contradictory, you control which one takes precedence explicitly.  When you ask for
a policy decision from OPA, you specify both the policy name (`foo`) and the
virtual document that names the decision within foo.  Typically in this scenario,
you create a virtual document called `authz` and define it so that `allow`
overrides `deny` or vice versa.  Then when asking for a policy decision, you
ask for `foo/authz`.

```
# deny everything by default
default authz = false

# deny overrides allow
authz {
    allow
    not deny
}
```

If instead you want to resolve conflicts using a first-match strategy (where
the first statement applicable makes the decision), see the FAQ entry on
[statement order](#statement-order).


## Does Statement Order Matter?

The order in which statements occur does not matter in Rego.  Reorder any two statements
and the policy means exactly the same thing.  For example, the following two statements
mean the same thing whichever order you write them in.

```
ratelimit = 4 { input.name = "alice" }
ratelimit = 5 { input.owner = "bob" }
```

Sometimes, though, you want the statement order to matter.  For example, you might put more specific statements before more general statements so that the more specific statements take precedence (e.g. for [conflict resolution](conflict-resolution)).  Rego lets you do that using the `else` keyword.  For example, if you want to make the first statement above take precedence, you would write the following Rego.

```
ratelimit = 4 {
    input.name = "alice"
} else = 5 {
    input.owner = "bob"
}
```

