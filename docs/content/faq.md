---
title: Frequently Asked Questions
navtitle: FAQ
kind: documentation
weight: 17
---

## How do I make user attributes stored in LDAP/AD available to OPA for making decisions?

[This best-practice guide](../guides-identity) explains three options: JSON Web Tokens, synchronization with LDAP/AD, and calling into LDAP/AD during policy evaluation.

## How does OPA do conflict resolution? {#conflict-resolution}

In Rego (OPA's policy language), you can write statements that both allow and
deny a request, such as

```ruby
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

```ruby
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


## Does Statement Order Matter? {#statement-order}

The order in which statements occur does not matter in Rego.  Reorder any two statements
and the policy means exactly the same thing.  For example, the following two statements
mean the same thing whichever order you write them in.

```ruby
ratelimit = 4 { input.name = "alice" }
ratelimit = 5 { input.owner = "bob" }
```

Sometimes, though, you want the statement order to matter.  For example, you might put more specific statements before more general statements so that the more specific statements take precedence (e.g. for [conflict resolution](#conflict-resolution)).  Rego lets you do that using the `else` keyword.  For example, if you want to make the first statement above take precedence, you would write the following Rego.

```ruby
ratelimit = 4 {
    input.name == "alice"
} else = 5 {
    input.owner == "bob"
}
```

## Which Equality Operator Should I Use?

Rego supports three kinds of equality: assignment (`:=`), comparison (`==`), and unification `=`.  Both assignment (`:=`) and comparison (`==`) are only available inside of rules (and in the REPL), and we recommend using them whenever possible for policies that are easier to read and write.

```ruby
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
# Note: = is the only option outside of rule bodies
x = 7               # causes x to be assigned 7
[x, 2] = [3, y]     # x is assigned 3 and y is assigned 2
```

## Collaboration Using Import
OPA lets multiple teams contribute independent policies that you can then combine to make an overall decision.  Each team writes their policy in a separate `package`, then you write one more policy that imports all the teams policies and makes a decision.

For example, suppose there is a network team, a storage team, and a compute team.  Suppose they each write their own policy:

```ruby
package compute
allow { ... }
```

```ruby
package network
allow { ... }
```

```ruby
package storage
allow { ... }
```

Now the cloud team, who is in charge of the overall decision, writes another policy that combines the decisions for each of the team policies.  In the example below, all 3 teams must allow for the overall decision to be allowed.

```ruby
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

## High Performance Policy Decisions

For low-latency/high-performance use-cases, e.g. microservice API authorization, policy evaluation has a budget on the order of 1 millisecond.  Not all use cases require that kind of performance, and OPA is powerful enough that you can write policies that take much longer than 1 millisecond to evaluate.  But for high-performance use cases, there is a fragment of the policy language that has been engineered to evaluate quickly.  Even as the size of the policies grow, the performance for this fragment can be nearly constant-time.

**Linear fragment** The *linear fragment* of the language is all of those policies where evaluation amounts to walking over the policy once.  This means there is no search required to make a policy decision.  Any variables you use can be assigned at most one value.

For example, the following rule has one local variable `user`, and that variable can only be assigned one value.  Intuitively, evaluating this rule requires checking each of the conditions in the body, and if there were N of these rules, evaluation would only require walking over each of them as well.

```ruby
allow {
  some user
  input.method == "GET"
  input.path = ["accounts", user]
  input.user == user
}
```

**Use objects instead of arrays**.  One common mistake people make is using arrays when they could use objects.  For example, below is an array of ID/first-name/last-names where ID is unique, and you're looking up the first-name/last-name given the ID.

```ruby
# DO NOT DO THIS.
# Array of objects where each object has a unique identifier
d = [{"id": "a123", "first": "alice", "last": "smith"},
     {"id": "a456", "first": "bob", "last": "jones"},
     {"id": "a789", "first": "clarice", "last": "johnson"}
     ]
# search through all elements of the array to find the ID
d[i].id == "a789"
d[i].first ...
```

Instead, use a dictionary where the key is the ID and the value is the first-name/last-name.  Given the ID, you can lookup the name information directly.

```ruby
# DO THIS INSTEAD OF THE ABOVE
# Use object whose keys are the IDs for the objects.
#   Looking up an object given its ID requires NO search
d = {"a123": {"first": "alice", "last": "smith"},
     "a456": {"first": "bob", "last": "jones"},
     "a789": {"first": "clarice", "last": "johnson"}
    }
# no search required
d["a789"].first ...
```


**Use simple equality statements for indexing**.  The linear-time fragment ensures that the cost of evaluation is no larger than the size of the policy.  OPA lets you write non-linear policies, because sometimes you need to, and because sometimes it's convenient.  The blog on [partial evaluation](https://blog.openpolicyagent.org/partial-evaluation-162750eaf422) describes one mechanism for converting non-linear policies into linear policies.

But as the size of the policy grows, the cost of evaluation grows with it.  Sometimes the policy can grow large enough that even the linear-fragment fails to meet the performance budget.

In the linear fragment, OPA includes special algorithms that index rules efficiently, sometimes making evaluation constant-time, even as the policy grows.  The indexer looks for simple equality statements, so the more simple equality checks that appear in rules, the more effective the indexer is, and the fewer rules actually need to be evaluated.  Here is an example policy from the [rule-indexing blog](https://blog.openpolicyagent.org/optimizing-opa-rule-indexing-59f03f17caf3) giving the details for these algorithms.

```ruby
allow {
  some user
  input.method == "GET"
  input.path = ["accounts", user]
  input.user == user
}

allow {
  input.method == "GET"
  input.path == ["accounts", "report"]
  roles[input.user][_] = "admin"
}

allow {
  input.method = "POST"
  input.path = ["accounts"]
  roles[input.user][_] = "admin"
}
```

**Key takeaways**

For high-performance use cases:

* Write your policies to minimize iteration and search.
  * Use objects instead of arrays when you have a unique identifier for the elements of the array.
  * Consider [partial-evaluation](https://blog.openpolicyagent.org/partial-evaluation-162750eaf422) to compile non-linear policies to linear policies.
* Write your policies with simple equality statements so that [rule-indexing](https://blog.openpolicyagent.org/optimizing-opa-rule-indexing-59f03f17caf3) is effective.

## Functions Versus Rules
Rego lets you factor out common logic in 2 different and complementary ways.

One is the *function*, which is conceptually identical to functions from most programming languages.  It takes any input and returns any output.  Importantly, a function can take infinitely many inputs, e.g. any string.

```ruby
trim_and_split(s) = result {
     t := trim(s, " ")
     result := split(t, ".")
}
```
The other way to factor out common logic is with a *rule*.  Rules differ in that (i) they support automatic iteration and (ii) they are only defined for finitely many inputs.  (Those obviously go hand-in-hand.)  For example, you could define a rule that maps an application to the hostnames that app is running on:

```ruby
app_to_hostnames[app_name] = hostnames {
    app := apps[_]
    app_name := app.name
    hostnames := [hostname | name := app.servers[_]
                            s := sites[_].servers[_]
                            s.name == name
                            hostname := s.hostname]
}
```

And then we can iterate over all the key/value pairs of that app-to-hostname mapping (just like we could iterate over all key/value pairs of a hardcoded JSON object).  You can also iterate over just the keys or just the values or you can look up the value for a key or lookup all the keys for a single value.

```ruby
# iterate over all key/value pairs
> app_to_hostnames[app]
+-----------+------------------------------------------------------+
|    app    |                app_to_hostnames[app]                 |
+-----------+------------------------------------------------------+
| "web"     | ["hydrogen","helium","beryllium","boron","nitrogen"] |
| "mysql"   | ["lithium","carbon"]                                 |
| "mongodb" | ["oxygen"]                                           |
+-----------+------------------------------------------------------+
```

```ruby
# iterate over all values
> app_to_hostnames[_]
+------------------------------------------------------+
|                 app_to_hostnames[_]                  |
+------------------------------------------------------+
| ["hydrogen","helium","beryllium","boron","nitrogen"] |
| ["lithium","carbon"]                                 |
| ["oxygen"]                                           |
+------------------------------------------------------+
```

```ruby
# iterate over all keys
> app_to_hostnames[x] = _
+-----------+
|     x     |
+-----------+
| "web"     |
| "mysql"   |
| "mongodb" |
+-----------+
```

```ruby
# lookup the value for key "web"
> app_to_hostnames["web"]
[
  "hydrogen",
  "helium",
  "beryllium",
  "boron",
  "nitrogen"
]
```

```ruby
# lookup keys where value includes "lithium"
> app_to_hostnames[k][_] == "lithium"
+---------+
|    k    |
+---------+
| "mysql" |
+---------+
```


Obviously with the `trim_and_split` function we cannot ask for all the inputs/outputs since there are infinitely many.  We can't provide 1 input and ask for all the other inputs that make the function return true, again, because there could be infinitely many.  The only thing we can do with a function is provide it all the inputs and ask for the output.

```ruby
> trim_and_split("   foo.bar.baz  ")
[
  "foo",
  "bar",
  "baz"
]
```

Functions allow you to factor out common logic that has infinitely-many input/output pairs; rules allow you to factor out common logic with finitely many input/outputs and allow you to iterate over them in the same way as native JSON objects.

To achieve automatic iteration, there is an additional syntactic requirement on a rule that is NOT present for a function: `safety`.  See the FAQ entry on safety for technical details.  Every rule must be `safe`, which guarantees that OPA can figure out a finite list of possible values for every variable in the body and head of a rule.

We recommend using rules where possible and using functions when rules do not work.

## Safety

The compiler will sometimes throw errors that say a rule is not `safe`.  The goal of safety is to ensure that every rule has finitely many inputs/outputs.  Safety ensures that every variable has finitely many possible values, so that OPA can iterate over them to find those values that make the rule true.  Technically:

```
Safety: every variable appearing in the head or in a builtin or inside a negation must appear in a non-negated, non-builtin expression in the body of the rule.
```

Examples:
```
# Unsafe: x in head does not appear in body.
#   There are infinitely many values that make p true
p[x] { some y; q[y]; r[y] }

# Safe.  q and r are both rules
#   Both q and r are finite; therefore p is also finite.
p[x] = y { some x, y; q[x]; r[y] }

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
```
x := q[_]
not p[x]
```

## JSON Web Tokens (JWTs)

[JSON Web Tokens (JWTs)](https://jwt.io/) are an industry standard for exchanging information between services.  Often they are used to represent information about the users logged into a system.  OPA has special-purpose code for dealing with JWTs.

All JWTs with OPA come in as strings.  That string is a JSON Web Token encoded with JWS Compact Serialization. JWE and JWS JSON Serialization are not supported.

You can verify tokens are properly signed.
```ruby
# RS256 signature
valid := io.jwt.verify_rs256(string, certificate)
# PS256 signature
valid := io.jwt.verify_ps256(string, certificate)
# ES256 signature
valid := io.jwt.verify_es256(string, certificate)
# HS256 signature
valid := io.jwt.verify_hs256(string, certificate)
```

You can decode JWTs and use the contents of the JWT to make policy decisions.

```ruby
# If nested signing was used, the header, payload and signature will represent the most deeply nested token.
[header, payload, signature] = io.jwt.decode(string)

# Verify and decode, where constraints include cert, secret, the
#   algorithm name to use, and additional parameters for the verification
[valid, header, payload] = io.jwt.decode_verify(string, constraints)
```

For details see the [language reference section on tokens](https://www.openpolicyagent.org/docs/language-reference.html#tokens).

To get certificates into the policy, you can either hardcode them or provide them as environmental variables to OPA and then use the `opa.runtime` builtin to retrieve those variables.

```ruby
# all runtime information
runtime := opa.runtime()
# environment variables provided when OPA started
runtime.env
# the env variable PROD_CERTIFICATE
runtime.env.PROD_CERTIFICATE
```