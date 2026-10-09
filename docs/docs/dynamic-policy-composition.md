---
title: Dynamic Policy Composition
---

Dynamic policy composition is a way to structure a policy library so that one entrypoint, the **router**, decides at
evaluation time which policies apply to a request. Rather than referring to every package by name, the router builds a
reference from the input and evaluates whatever policies it finds there. Teams can then add, remove and own their
policies without anyone editing the router.

The pattern is a good fit when:

- Many teams or components each own a set of rules, and a request only concerns one of them.
- The set of policies changes often, for example when each team publishes its own bundle.
- You want one decision point, with a single query path and input format, in front of all of them.

## Routing requests to policies

Policies live under a common prefix, grouped by an input attribute. This example has two teams, `payments` and
`search`, and each team's policies sit under its name:

```text
data.policies.payments.pci
data.policies.payments.audit
data.policies.search.rate
...
```

Each team policy is an ordinary package. By convention, every package in the library uses the same rule names, `deny`
in this example, so the router can collect them without knowing what is inside:

```rego title="policies/payments/pci.rego"
package policies.payments.pci

deny contains "card data must be encrypted" if {
	not input.resource.encrypted
}
```

```rego title="policies/payments/audit.rego"
package policies.payments.audit

deny contains "audit logging must be enabled" if {
	not input.resource.audited
}
```

```rego title="policies/search/rate.rego"
package policies.search.rate

deny contains "public indexes need a rate limit" if {
	input.resource.public
	not input.resource.rate_limit
}
```

The router uses `input.team` to pick the team's packages, iterates over them with `_`, and aggregates their `deny`
rules:

```rego title="main.rego"
package main

# Route to the policies of the team named in the input, and collect their denies.
deny contains msg if {
	some msg in data.policies[input.team][_].deny
}

# Requests for a team without policies must not be allowed by default.
deny contains sprintf("no policies for team %q", [input.team]) if {
	not data.policies[input.team]
}

default allow := false

allow if count(deny) == 0
```

Querying `data.main.allow` with this input only evaluates the payments policies. The search policies are loaded, but
never evaluated:

```json title="input.json"
{
  "team": "payments",
  "resource": { "public": true }
}
```

```json title="data.main.deny"
["audit logging must be enabled", "card data must be encrypted"]
```

You can route on more than one attribute by adding levels to the reference, for example
`data.policies[input.kind][input.namespace][_].deny`. The
[AWS CloudFormation Hooks tutorial](./aws-cloudformation-hooks#dynamic-policy-composition) routes on the resource
type, mapping `AWS::S3::Bucket` to `data.aws.s3.bucket`.

### Unknown routes

:::warning
A reference that matches nothing is undefined. Without a guard, a team that has no policies gets an empty `deny` set,
and `allow` succeeds.
:::

Decide what a request for an unknown route should get. You can reject it, as the second `deny` rule in `main.rego`
does, or send it to a default set of policies. For a default, resolve the route once, and fall back with `else` when
the team has no policies of its own. Keep the fallback policies outside `data.policies`, so no team name can select
them directly:

```rego title="main.rego"
package main

route := data.policies[input.team] if data.policies[input.team]

else := data.fallback

deny contains msg if {
	some msg in route[_].deny
}

default allow := false

allow if count(deny) == 0
```

```rego title="fallback/baseline.rego"
package fallback.baseline

deny contains "resource must have an owner" if {
	not input.resource.owner
}
```

For a team without policies, `route` is `data.fallback`, and `route[_].deny` collects the `deny` rules of every package
under it, the same way it does for a team's own packages. A request for `"team": "marketing"`, which has no policies,
is now checked against `fallback.baseline` instead of being allowed or rejected outright.

## Testing

Test each team policy on its own, like any other package. To test the router without depending on the real policy
library, replace `data.policies` with stand-in values using [`with`](./policy-language#with-keyword):

```rego title="main_test.rego"
package main_test

import data.main

test_routes_to_the_team_policies if {
	main.deny == {"from payments"} with input as {"team": "payments"}
		with data.policies as {"payments": {"p": {"deny": {"from payments"}}}, "search": {"s": {"deny": {"from search"}}}}
}

test_unknown_team_is_denied if {
	not main.allow with input as {"team": "unknown"}
}
```

Outside of tests, avoid using `with` inside the router to change `input` or `data` for each policy it dispatches to.
Rules evaluated under `with` aren't cached, which can be expensive. Regal's
[`with-outside-test-context`](/projects/regal/rules/performance/with-outside-test-context) rule flags this.

## Performance

For regular queries, such as the Data API or `opa eval`, the routing attributes are known, so the dynamic reference
resolves like a lookup and only the selected packages are evaluated. The number of loaded policies doesn't affect the
cost of a decision.

[Partial evaluation](./filtering/partial-evaluation) can combine with dynamic composition too: mark the attributes
you filter on as unknown, and keep the routing attributes known. For example, partially evaluate `data.main.deny[msg]`
with `input.resource` unknown and this input:

```json title="input.json"
{ "team": "payments" }
```

Because `input.team` is known, the router's reference `data.policies[input.team]` resolves to
`data.policies.payments`, so only the payments policies are evaluated. Their `deny` rules depend on the unknown
`input.resource`, so OPA returns those conditions instead of a decision, plus one for the guard rule (see the tip
below):

```text
not input.resource.audited
not input.resource.encrypted
```

:::tip
The `not data.policies[input.team]` guard can't be decided while the team policies depend on unknown input, so it
appears in the partial evaluation result as an extra condition. If you use the Compile API, keep the list of routes as
plain data instead, for example `not input.team in data.teams`, which partial evaluation resolves.
:::
