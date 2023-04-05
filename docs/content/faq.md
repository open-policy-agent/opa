---
title: Frequently Asked Questions
navtitle: FAQ
kind: support
weight: 1
---

## How do I make user attributes stored in LDAP/AD available to OPA for making decisions?

[This best-practice guide](../external-data) explains three options: JSON Web Tokens, synchronization with LDAP/AD, and calling into LDAP/AD during policy evaluation.

## How does OPA do conflict resolution? {#conflict-resolution}

In Rego (OPA's policy language), you can write statements that both allow and
deny a request, such as

```live:conflict_resolution:module:read_only
package foo
allow { input.name == "alice" }
deny { input.name == "alice" }
```

Neither `allow` nor `deny` are keywords in Rego so if you want to treat them
as contradictory, you control which one takes precedence explicitly.  When you ask for
a policy decision from OPA, you specify both the policy name (`foo`) and the
virtual document that names the decision within foo.  Typically in this scenario,
you create a virtual document called `authz` and define it so that `allow`
overrides `deny` or vice versa.  Then when asking for a policy decision, you
ask for `foo/authz`.

```live:conflict_resolution_deny_by_default:module:read_only
# deny everything by default
default authz := false

# deny overrides allow
authz {
    allow
    not deny
}
```

If instead you want to resolve conflicts using a first-match strategy (where
the first statement applicable makes the decision), see the FAQ entry on
[statement order](#statement-order).

## Does Statement Order Matter? {#statement-order}

The order in which statements occur does not matter in Rego.  Reorder any two statements
and the policy means exactly the same thing.  For example, the following two statements
mean the same thing whichever order you write them in.

```live:unordered:module:openable
package unordered

ratelimit := 4 { input.name == "alice" }
ratelimit := 5 { input.name == "bob" }
```

```live:unordered:input
{
  "name": "bob"
}
```

```live:unordered:query:hidden
ratelimit
```

```live:unordered:output
```

Sometimes, though, you want the statement order to matter.  For example, you might put more specific statements before more general statements so that the more specific statements take precedence (e.g. for [conflict resolution](#conflict-resolution)).  Rego lets you do that using the `else` keyword.  For example, if you want to make the first statement above take precedence, you would write the following Rego.

```live:ordered:module:openable
package ordered

ratelimit := 4 {
    input.owner == "bob"
} else := 5 {
    input.name == "alice"
}
```

```live:ordered:input
{
  "name": "alice",
  "owner": "bob"
}
```

```live:ordered:query:hidden
ratelimit
```

```live:ordered:output
```

## Which Equality Operator Should I Use?

Rego supports three kinds of equality: assignment (`:=`), comparison (`==`), and unification `=`. We recommend using assignment (`:=`) and comparison (`==`) whenever possible for policies that are easier to read and write.

```live:equality:query:read_only
# Assignment: declare local variable x and give it value 7
# If x appears before this statement in the rule, compiler throws error.
x := 7
y := {"a", "b", "c"}

# Comparison: check if two values are the same.
#   Do not assign variables--variables must be "safe".
x == 7
x == y
y == [1, 2, [3]]

# Unification: assign variables to values that make the
#   equality true
x = 7               # causes x to be assigned 7
[x, 2] = [3, y]     # x is assigned 3 and y is assigned 2
```

## Collaboration Using Import

OPA lets multiple teams contribute independent policies that you can then combine to make an overall decision.  Each team writes their policy in a separate `package`, then you write one more policy that imports all the teams policies and makes a decision.

For example, suppose there is a network team, a storage team, and a compute team.  Suppose they each write their own policy:

```live:collab_compute:module:read_only
package compute
allow { ... }
```

```live:collab_network:module:read_only
package network
allow { ... }
```

```live:collab_storage:module:read_only
package storage
allow { ... }
```

Now the cloud team, who is in charge of the overall decision, writes another policy that combines the decisions for each of the team policies.  In the example below, all 3 teams must allow for the overall decision to be allowed.

```live:collab_main:module:read_only
package main
import data.compute
import data.storage
import data.network

# allow if all 3 teams allow
allow {
    compute.allow
    storage.allow
    network.allow
}
```

The cloud team could have a more sophisticated scheme for combining policies, e.g. using just the compute policy for compute-only resources or requiring the compute policy to allow the compute-relevant portions of resource.  Remember that `allow` is not special--it is just another boolean that the policy author can use to make decisions.

## Functions Versus Rules

Rego lets you factor out common logic in 2 different and complementary ways.

One is the *function*, which is conceptually identical to functions from most programming languages.  It takes any input and returns any output.  Importantly, a function can take infinitely many inputs, e.g. any string.

```live:functions:module:openable
package functions

trim_and_split(s) := result {
     t := trim(s, " ")
     result := split(t, ".")
}
```

```live:functions:query
trim_and_split("  hello.world  ")
```

```live:functions:output
```

The other way to factor out common logic is with a *rule*.  Rules differ in that (i) they support automatic iteration and (ii) they are only defined for finitely many inputs.  (Those obviously go hand-in-hand.)  For example, you could define a rule that maps an application to the hostnames that app is running on:

```live:rules:module:openable
package rules

app_to_hostnames[app_name] := hostnames {
  app := apps[_]
  app_name := app.name
  hostnames := [hostname | name := app.servers[_]
                           s := sites[_].servers[_]
                           s.name == name
                           hostname := s.hostname]
}

apps := [
  {
    "name": "web",
    "servers": ["s1", "s2"],
  },
  {
    "name": "mysql",
    "servers": ["s3"],
  },
  {
    "name": "mongodb",
    "servers": ["s4"],
  },
]

sites := [
  {
    "servers": [
      {
        "name": "s1",
        "hostname": "hydrogen",
      },
      {
        "name": "s3",
        "hostname": "helium",
      },
      {
        "name": "s4",
        "hostname": "nitrogen",
      },
    ],
  },
  {
    "servers": [
      {
        "name": "s2",
        "hostname": "carbon",
      },
    ],
  },
]
```

And then we can iterate over all the key/value pairs of that app-to-hostname mapping (just like we could iterate over all key/value pairs of a hardcoded JSON object).  You can also iterate over just the keys or just the values or you can look up the value for a key or lookup all the keys for a single value.

```live:rules/all_pairs:query
# iterate over all key/value pairs
app_to_hostnames[app]
```

```live:rules/all_pairs:output
```

```live:rules/all_values:query
# iterate over all values
app_to_hostnames[_]
```

```live:rules/all_values:output
```

```live:rules/all_keys:query
# iterate over all keys
app_to_hostnames[x] = _
```

```live:rules/all_keys:output
```

```live:rules/lookup_key:query
# lookup the value for key "web"
app_to_hostnames["web"]
```

```live:rules/lookup_key:output
```

```live:rules/lookup_value:query
# lookup keys where value includes "carbon"
app_to_hostnames[k][_] == "carbon"
```

```live:rules/lookup_value:output
```

Obviously with the `trim_and_split` function we cannot ask for all the inputs/outputs since there are infinitely many.  We can't provide 1 input and ask for all the other inputs that make the function return true, again, because there could be infinitely many.  The only thing we can do with a function is provide it all the inputs and ask for the output.

Functions allow you to factor out common logic that has infinitely-many input/output pairs; rules allow you to factor out common logic with finitely many input/outputs and allow you to iterate over them in the same way as native JSON objects.

To achieve automatic iteration, there is an additional syntactic requirement on a rule that is NOT present for a function: `safety`.  See the FAQ entry on safety for technical details.  Every rule must be `safe`, which guarantees that OPA can figure out a finite list of possible values for every variable in the body and head of a rule.

We recommend using rules where possible and using functions when rules do not work.

## Safety

The compiler will sometimes throw errors that say a rule is not `safe`.  The goal of safety is to ensure that every rule has finitely many inputs/outputs.  Safety ensures that every variable has finitely many possible values, so that OPA can iterate over them to find those values that make the rule true.  Technically:

```
Safety: every variable appearing in the head or in a builtin or inside a negation must appear in a non-negated, non-builtin expression in the body of the rule.
```

Examples:

```live:safety:module:read_only
# Unsafe: x in head does not appear in body.
#   There are infinitely many values that make p true
p[x] { some y; q[y]; r[y] }

# Safe.  q and r are both rules
#   Both q and r are finite; therefore p is also finite.
p[x] := y { some x, y; q[x]; r[y] }

# Unsafe: y appears inside a builtin (+) but not in the body.
#   y has infinitely many possible values; so too does x.
p[x] { some y; x := y + 7 }

# Safe: the only values for y are those in q.
#   Since q is a rule and finite so is p finite.
p[x] { some y; x := y + 7; q[y]}

# Unsafe: x appears inside a negation
#  If q is finite, all the x's not in q are infinite.
p[x] { some x; not q[x] }

# Safe: x appears inside of r so p is no larger than r
#  Since r is finite, so too is p
p[x] { some x; not q[x]; r[x] }
```

Safety has one implication about negation: you don't iterate over values NOT in a rule like `q`.  Instead, you iterate over values in another rule like `r` and then use negation to CHECK whether if that value is NOT in `q`.

Embedded terms like `not p[q[_]]` sometimes produce difficult to decipher error messages.  We recommend pulling the embedded terms out into the rule--the meaning is the same and often creates easier to read error messages:

```live:safety/nested:query:read_only
x := q[_]
not p[x]
```

## JSON Web Tokens (JWTs)

[JSON Web Tokens (JWTs)](https://jwt.io/) are an industry standard for exchanging information between services.  Often they are used to represent information about the users logged into a system.  OPA has special-purpose code for dealing with JWTs.

All JWTs with OPA come in as strings.  That string is a JSON Web Token encoded with JWS Compact Serialization. JWE and JWS JSON Serialization are not supported.

You can verify tokens are properly signed.

```live:jwt_verify:query:read_only
# RS256 signature
io.jwt.verify_rs256(string, certificate)

# PS256 signature
io.jwt.verify_ps256(string, certificate)

# ES256 signature
io.jwt.verify_es256(string, certificate)

# HS256 signature
io.jwt.verify_hs256(string, certificate)
```

You can decode JWTs and use the contents of the JWT to make policy decisions.

```live:jwt_decode:module:hidden
package jwt_decode
```

```live:jwt_decode:input
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyIjoiYWxpY2UiLCJhenAiOiJhbGljZSIsInN1Ym9yZGluYXRlcyI6W10sImhyIjpmYWxzZX0.rz3jTY033z-NrKfwrK89_dcLF7TN4gwCMj-fVBDyLoM"
}
```

```live:jwt_decode:query
io.jwt.decode(input.token)
```

```live:jwt_decode:output
```

> If nested signing was used, the header, payload and signature will represent the most deeply nested token.

You can decode **and** verify using `io.jwt.decode_verify`.

```live:jwt_decode/verify:query
io.jwt.decode_verify(input.token, {
  "secret": "secret",
  "alg": "hs256",
})
```

```live:jwt_decode/verify:output
```

See the [Policy Reference](../policy-reference#tokens) for additional verification constraints.

To get certificates into the policy, you can either hardcode them or provide them as environmental variables to OPA and then use the `opa.runtime` builtin to retrieve those variables.

```live:runtime:query:read_only
# all runtime information
runtime := opa.runtime()

# environment variables provided when OPA started
runtime.env

# the env variable PROD_CERTIFICATE
runtime.env.PROD_CERTIFICATE
```

## How do I Write Policies Securely?

Depending on the use case and the integration with OPA that you are using, the style of policy you choose can impact your overall security posture.  Below we show three styles of authoring policy and compare them.

**Default allow**.  This style of policy allows every request by default.  The rules you write dictate which requests should be rejected.

```rego
# entry point is 'deny'
default deny := false
deny { ... }
deny { ... }
```

If you assume all of the rules you write are correct, then you know that every rejection the policy produces should truly be rejected.  However, there could be requests that are allowed that you may not truly want allowed, but you simply neglected to write the rule for.  For operations, this is often a useful style of policy authoring because it allows you to incrementally tighten the controls for a system from wherever that system starts.  For security, this style is less appropriate because it allows unknown bad actions to occur.

**Default deny**.  This style of policy rejects every request by default.  The rules you write dictate which requests should be allowed.

```rego
# entry point is 'allow'
default allow := false
allow { ... }
allow { ... }
```

If you assume your rules are correct, the only requests that are accepted are known to be safe.  Any statements you leave out reject requests that in actuality are safe but which you did not know were safe.  For operations, these policies are less suitable for incrementally improving the policy posture of a system because the initial policy must explicitly allow all of the behaviors that are necessary for the system to operate correctly.  For security, these policies ensure that any request that is allowed is known to be safe (because there is a rule saying it is safe).

**Default allow with deny override**.  This style of policy rejects every request by default.  You write  rules that dictate which requests should be allowed, and optionally you write other rules that dictate which of those allowed requests should be rejected.

```rego
# entry point is 'authz'
default authz := false
authz {
  allow
  not deny
}
allow { ... }
deny { ... }
```

This hybrid approach to policy authoring combines the two previous styles.  These policies allow relatively coarse grained parts of the request space and then carve out of each part what should actually be denied.  Any deny statements that you forget lead to security problems; any allow statements you forget lead to operational problems.  But since this approach allows you to implement either of the other two, it is a common pattern across use cases.

**Non-boolean policies**. The examples above focus on policies with boolean decisions.  Policies that make non-boolean decisions typically have similar tradeoffs.  Are you enumerating the conditions under which requests are permitted (e.g. the list of clusters to which an app SHOULD be deployed) or are you enumerating the conditions under which requests are prohibited (e.g. the list of clusters to which an app SHOULD NOT be deployed).  While the details differ, the concepts are often similar.
