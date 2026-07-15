---
title: "Optimizing OPA: Rule Indexing"
authors: ["tsandall"]
date: 2017-06-06
slug: optimizing-opa-rule-indexing-59f03f17caf3
---

One of the hallmarks of declarative languages is that people shouldn't worry about performance — the software should. People write down the logic of a policy decision (e.g. should this API be allowed), and the software figures out how to evaluate that logic efficiently.

For example, it's common to use OPA to write whitelist (or blacklist) policies by defining multiple rules that all say to 'allow' (or 'deny') some operation. The example below defines a simple whitelist authorization policy for HTTP API operations (see the [HTTP API Authorization tutorial](https://www.openpolicyagent.org/docs/latest/http-api-authorization/) for more details). Notice how there are multiple statements that 'allow' the operation, any of which could apply to a given input.

```rego
package acmecorp.api

import data.acmecorp.roles

default allow = false

allow {
  input.method = "GET"
  input.path = ["accounts", user]
  input.user = user
}

allow {
  input.method = "GET"
  input.path = ["accounts", "report"]
  roles[input.user][_] = "admin"
}

allow {
  input.method = "POST"
  input.path = ["accounts"]
  roles[input.user][_] = "admin"
}
```

To ask if user "felix" is authorized to GET the /accounts API, you would query OPA as shown below.

```
POST /v1/data/acmecorp/api/allow HTTP/1.1
Content-Type: application/json

{
  "input": {
    "method": "POST",
    "user": "felix",
    "path": ["accounts"]
  }
}
```

If "felix" has the "admin" role, OPA responds with true, since the 3rd "allow" rule matches the path ["accounts"] and requires the user to be in the "admin" role. Otherwise, none of the "allow" rules match the input above, and OPA responds with false (because of the "default" rule).

In this blog post we describe an optimization that improves how efficiently OPA computes the answer to such a query. The simplest way to compute an answer is to evaluate each of the "allow" rules one-by-one. This is exactly what OPA did before v0.4.9. The obvious problem with this approach is that query latency increases as the total size of the rule set grows, even if the additional rules are irrelevant to most queries.

In many cases, rule sets share similar expressions that differ slightly but in ways that make some of them mutually exclusive. This means that if one rule would generate a value, another could not. In the example above, the 3rd rule only applies to the path ["accounts"], whereas the other two rules only apply to longer paths, e.g. ["accounts", "report"].

As of v0.4.9, OPA indexes rule sets to quickly determine which subset of rules must be evaluated given data stored in OPA or passed as input to the query. This allows OPA to evaluate only the rules required to generate the correct result. Rules that would not generate a value for the query input are ignored.

Conceptually, the index is a function that takes the query input as an argument and returns the rules OPA must evaluate:

```go
func (index Index) GetRules(input Value) []Rule {
  // perform index lookup
}
```

To make the index lookup efficient, OPA builds a [Trie data structure](https://en.wikipedia.org/wiki/Trie) from equality expressions contained in rule sets. For example, OPA would build the following structure for the rule set above:

![Trie structure built from the example rule set](/img/blog/optimizing-opa-rule-indexing-59f03f17caf3/1.webp)

When OPA looks up the rules to evaluate the query above, it traverses the Trie and collects rules that are required for the input. For simple equality expressions involving scalars this is straightforward however it becomes more complex when taking into account composites (arrays, objects), vars, and undefined values.

![Trie traversal handling composites, vars, and undefined values](/img/blog/optimizing-opa-rule-indexing-59f03f17caf3/2.png)

As we would expect, this approach provides an asymptotic performance improvement on queries that match the form supported by the index:

![Benchmark chart showing query latency improvement from indexing](/img/blog/optimizing-opa-rule-indexing-59f03f17caf3/3.png)

Currently, OPA can build this rule index for expressions of the form:

```
<ref> = <scalar | array | var> # or vice-versa
```

Here are a few examples:

```
input.foo = "bar"
["foo", "bar", x] = input.baz
data.foo.bar = x
```

There's an additional constraint on the `<ref>` term: it must:

- be ground (e.g., `data.example.foo[x]` is excluded)
- be non-nested (e.g., `data.example.foo[data.example.bar]` is excluded)
- refer to a base document (e.g., if `data.example.p` refers to a rule, it is excluded)

Nonetheless, in practice, there are many cases covered by the currently supported form and we're pleased to have this optimization in place. In the future, we'll extend the index to support other types of terms (e.g., objects, non-ground refs, etc.) as needed.
