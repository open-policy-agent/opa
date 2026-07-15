---
title: "Partial Evaluation"
authors: ["tsandall"]
date: 2018-02-05
slug: partial-evaluation-162750eaf422
---

We'd like to introduce a new OPA feature called _partial evaluation_ which has several interesting applications. In this post we'll explain how partial evaluation works, and how it leverages rule indexing to optimize policy evaluation for low-latency use cases (e.g. microservice API authorization).

## What Is Partial Evaluation?

With partial evaluation, callers specify that certain inputs or pieces of data are _unknown_. OPA evaluates as much of the policy as possible without touching parts that depend on unknown values. The result of partial evaluation is _a new policy_ that can be evaluated more efficiently than the original.

![Diagram comparing Normal Evaluation and Partial Evaluation flows](/img/blog/partial-evaluation-162750eaf422/banner.webp)

By partially evaluating policies, OPA can perform computation at compile time instead of runtime.

For example, in API authorization use cases, expensive operations that are independent of incoming API requests are evaluated once (during partial evaluation) and their results can be cached for subsequent evaluation runs. This helps remove costly operations from the critical path of API request processing.

The big difference between partial evaluation and normal evaluation is the result. Normal evaluation produces a decision that can be enforced (e.g. allow or deny), but partial evaluation produces new policies that can be evaluated later when the unknowns become known.

![Diagram showing New Policies produced by Partial Evaluation feeding into normal Evaluation to produce Decisions](/img/blog/partial-evaluation-162750eaf422/2.webp)

Let's look at an example.

## Partial Evaluation and Rule Indexing

Imagine we have a simple authorization policy that decides whether to allow requests against appliances.

```rego
package smart_home

default allow = false

allow = true {
    op = allowed_operations[_]
    input.method = op.method
    input.resource = op.resource
}

allowed_operations = [
    {"method": "PUT", "resource": "air-conditioner"},
    {"method": "GET", "resource": "security-camera"},
    {"method": "POST", "resource": "garage-door"},
]
```

To use this policy, callers query OPA by asking for the value of **smart_home.allow** which will be **true** or **false** depending on whether the request should be allowed or denied. To answer the policy query, OPA _searches_ the allowed_operations data to find a match.

The nice thing about this policy is that we only have to write the search logic (**allow**) once, up front. After that, we can expand the data set (**allowed_operations**) to cover more and more appliances.

> In real scenarios, **allowed_operations** would be loaded into OPA as _data_ instead of hardcoded in the policy. It's hardcoded into the policy to make the example simpler.

The downside is that OPA needs to potentially search over the entire data set each time an API request is received. As we automate more of our home, the authorization decision will take longer to compute.

Performance is an important requirement for OPA: we need to minimize the latency introduced into the request path as much as possible. OPA has addressed use cases like this in the past by [precomputing a data structure that quickly looks up applicable rules instead of searching through them](/blog/optimizing-opa-rule-indexing-59f03f17caf3). However to accomplish this the rules have to be written in a manner that OPA's indexer can understand. For example, we would have to take the policy above rewrite it as follows:

```rego
package smart_home

default allow = false

allow {
    input.method = "PUT"
    input.resource = "air-conditioner"
}

allow {
    input.method = "GET"
    input.resource = "security-camera"
}

allow {
    input.method = "POST"
    input.resource = "garage-door"
}
```

While this works, no one wants to write out these kinds of rules by hand. Moreover, the policy becomes painful to read because we end up with thousands of duplicated rules. What we want is a simple, concise policy that OPA evaluates efficiently.

This is where partial evaluation comes in. Looking at the **allow** rule from earlier, we can see that there are two pieces of potentially unknown information: **input.method** and **input.resource**.

```rego
allow {
    op = allowed_operations[_]
    input.method = op.method
    input.resource = op.resource
}
```

With partial evaluation, we can evaluate everything in the policy that does not depend on these two input values. The result of partial evaluation, used internally by OPA, looks almost identical to the unrolled version we made above by hand.

Of course, not all policies are this simple. In some cases, expressions that depend on unknown values may produce outputs that are required in the rest of the policy:

```rego
allow {
    risk_score = (input.num_deletes * 10) + input.num_adds
    risk_score < risk_limit
}

risk_limit = 100
```

In this case, OPA will determine that **risk_score** cannot be computed if **input** is unknown and so the second expression will not be processed during partial evaluation. Instead it gets saved as part of the partial evaluation result.

This was a quick overview of how partial evaluation works. Let's look at how you can use partial evaluation today!

## Using Partial Evaluation

You enable partial evaluation by specifying the **partial** query parameter when you ask OPA for policy decisions.

OPA performs partial evaluation lazily: the first time you ask for a policy decision with partial evaluation, OPA will compute the partially evaluated policy and then cache it for later. If the data or policy in OPA changes such that partial evaluation needs to be re-run, OPA invalidates the cache entry.

Let's look at some examples.

Suppose we have the policy from above, but instead of 3 resources, we have hundreds. We can construct a query to see if we're allowed to GET the data from one of security cameras:

```json
{
  "input": {
    "method": "GET",
    "resource": "security-camera-493"
  }
}
```

This object is saved into a file called req.json that we can use to execute the query below with cURL:

```bash
curl localhost:8181/v1/data/smart_home/allow?metrics -d @req.json
```

In this case, the request is allowed and normal evaluation takes around **5ms** on an Intel Core i7 @ 2.9Ghz:

```json
{
  "metrics": {
    "timer_rego_query_compile_ns": 71412,
    "timer_rego_query_eval_ns": 5185152,
    "timer_rego_query_parse_ns": 194950
  },
  "result": true
}
```

We'll change the caller to specify the **partial** query parameter.

```bash
curl localhost:8181/v1/data/smart_home/allow?metrics&partial \
    -d @req.json
```

Now when we run the call, we get back the same results, but we can see the query latency (**timer_rego_query_eval_ns**) is much lower, around **50µs**.

```json
{
  "metrics": {
    "timer_rego_partial_eval_ns": 10613556,
    "timer_rego_query_compile_ns": 137427,
    "timer_rego_query_eval_ns": 43921,
    "timer_rego_query_parse_ns": 200871
  },
  "result": true
}
```

Since the above call was the first with the **partial** parameter set, the response included the time it took to partially evaluate the policy, around 10 milliseconds (**timer_rego_partial_eval_ns**). If we query OPA again with **partial** set the step is skipped because the result was cached:

```bash
curl localhost:8181/v1/data/smart_home/allow?metrics&partial \
    -d @req.json
```

```json
{
  "metrics": {
    "timer_rego_query_compile_ns": 78901,
    "timer_rego_query_eval_ns": 50405,
    "timer_rego_query_parse_ns": 22203
  },
  "result": true
}
```

You can also use partial evaluation if you embed OPA as a library. In that case:

- You must invoke partial evaluation yourself.
- You should invoke partial evaluation offline, outside the request path.
- You should cache the partial evaluation result.
- You can use the partial evaluation result when input values are known (e.g., when an API request is received.)

For more information on how to embed OPA as a library and leverage the partial evaluation feature, see [this GoDoc example](https://godoc.org/github.com/open-policy-agent/opa/rego#example-Rego-PartialEval).

## Benchmark: Role-based Access Control (RBAC)

An immediate application for partial evaluation is [RBAC](https://en.wikipedia.org/wiki/Role-based_access_control) policy enforcement. RBAC provides a simple, coarse-grained way of granting permissions by groupings. Determining whether to allow requests under RBAC involves identifying whether the caller has been associated with a role that grants permission to the perform the operation.

In projects like [Kubernetes](https://kubernetes.io/docs/admin/authorization/rbac/) and [Istio](https://istio.io/docs/concepts/security/rbac.html), RBAC configuration is specified using _roles_ and _role bindings_. Roles grant permission to perform operations and role bindings associate subjects (e.g., users or service accounts) to roles. Below is an example of some role and role binding data:

```json
{
  "roles": [
    {
      "operation": "read",
      "resource": "widgets",
      "name": "widget-reader"
    },
    {
      "operation": "write",
      "resource": "widgets",
      "name": "widget-writer"
    }
  ],
  "bindings": [
    {
      "user": "inspector-alice",
      "role": "widget-reader"
    },
    {
      "user": "maker-bob",
      "role": "widget-writer"
    }
  ]
}
```

In OPA, we can define an RBAC policy that evaluates the configuration above as follows:

```rego
package example_rbac

default allow = false

allow {
    user_has_role[role_name]
    role_has_permission[role_name]
}

user_has_role[role_name] {
    role_binding = data.bindings[_]
    role_binding.role = role_name
    role_binding.user = input.subject.user
}

role_has_permission[role_name] {
    role = data.roles[_]
    role.name = role_name
    role.operation = input.action.operation
    role.resource = input.action.resource
}
```

First, the policy searches for "bindings" that match the subject. Second, the policy checks to see if the role associated with the binding grants permission to perform the operation on the resource.

In this example, applying partial evaluation would yield a set of rules where the roles and role bindings have been inlined and the implicit loops that perform the search have been unrolled. For example:

```rego
# … more rules
allow {
    "admin" = input.subject.user
    "read" = input.action.operation
    "resource-lktj" = input.action.resource
}
allow {
    "admin" = input.subject.user
    "write" = input.action.operation
    "resource-rsiq" = input.action.resource
}
# … more rules
```

The table below summarizes the performance when using OPA to evaluate this RBAC policy on increasingly large RBAC configuration data sets.

```
# Roles | # Bindings | Normal Eval (ms) | With Partial Eval (ms)
--------+------------+------------------+-----------------------
    250 |        250 |        5.4982303 |              0.0468107
    500 |        500 |       11.8662272 |              0.0591411
  1,000 |      1,000 |       21.6441107 |              0.0542551
  2,000 |      2,000 |       45.4870310 |              0.0623962
```

This benchmark demonstrates the power of partial evaluation. Not only is the policy quick to evaluate but the evaluation latency remains relatively stable as the data set grows ([because of rule indexing](/blog/optimizing-opa-rule-indexing-59f03f17caf3)).

But it's only fair to warn that nothing comes for free. As the data set grows the time taken to run partial evaluation (and the memory required to store the result) also grow:

```
# Roles | # Bindings | Partial Eval + Compile (seconds) | Heap (MB)
--------+------------+----------------------------------+----------
    250 |        250 |                      0.503920151 | 11.714843
    500 |        500 |                      1.998275287 | 13.496093
  1,000 |      1,000 |                      8.085092923 | 18.449218
  2,000 |      2,000 |                     31.972901290 | 29.097656
```

It's important to reiterate that the partial evaluation step is done offline, outside of the request path: this means the partial evaluation latency only affects the time taken to propagate policy updates. From our point of view, given other system behaviour (such as caching and failures), this is an acceptable price to pay.

## Future Work and Wrap Up

With v0.6, we've laid the groundwork for partial evaluation. Going forward we're planning to work on extending coverage over the kinds of expressions that can be partially evaluated, and looking at strategies like performing partial evaluation in multiple passes.

Hopefully this post was useful (or at least interesting)! Partial evaluation is a big step for OPA as it allows policies to be expressed concisely while still benefiting from optimizations like rule indexing. We're looking forward to applying partial evaluation to other use cases in the near future.
