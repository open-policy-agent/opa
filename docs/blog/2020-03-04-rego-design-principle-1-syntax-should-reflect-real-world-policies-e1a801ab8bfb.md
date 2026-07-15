---
title: "Rego design principle #1: Syntax should reflect real-world policies"
authors: ["timhinrichs"]
date: 2020-03-04
slug: rego-design-principle-1-syntax-should-reflect-real-world-policies-e1a801ab8bfb
---

![Banner image for Rego design principle number one blog post](/img/blog/rego-design-principle-1-syntax-should-reflect-real-world-policies-e1a801ab8bfb/banner.webp)

Sometimes people ask why Rego, OPA's policy language, looks or behaves the way it does. Part of the answer is that Rego came about after having built two other general-purpose policy languages, and lessons learned from that process shaped this one. This multi-part post lays out the results of that journey — the key design principles for Rego, why they're important, and how they influenced the language.

Related posts in the series:

- [Design principle #2: Embrace Hierarchical Data](/blog/rego-design-principle-2-embrace-hierarchical-data-8a4913bdfea1)
- [Design principle #3: Optimize Performance Automatically](/blog/rego-design-principle-3-optimize-performance-automatically-2d29ad3ce96d)

The first design principle holds that Rego's syntax "should NOT be designed as a general-purpose programming language that reads from disk, writes to network sockets, supports multi-threading, defines custom datastructures, etc."

## Refresher on OPA

OPA is a general purpose policy engine that separates policy decisions from the software services that enforce them. It bases decisions on input data, a Rego policy, and optionally external data reflecting real-world state (e.g., on-call schedules or resource ownership).

These functional requirements ensure that OPA has enough flexibility and generality to make context-aware decisions across a broad range of use-cases: admission control, API authorization, risk-analysis, data-filtering. OPA runs as a lightweight agent or library on the same server as the software service, thereby achieving both the availability and performance needed for policy decision-making in modern, cloud-native computing environments. It has been integrated with over 20 popular software systems and is at the time of writing an incubating project within the Cloud Native Computing Foundation.

## Natural encoding of real-world policies

The goal is that Rego should map closely onto plain-language rules and regulations. A useful litmus test: reading Rego aloud should sound close to the source documentation.

This readability test is especially important because of the broad range of stakeholders who are responsible for policy: developers, operations, security, and compliance. The less of a translation there is from the PDFs and wikis the easier it is to believe that your Rego policies are correct, that operationally you're on solid ground, that your auditors will be convinced the policies do what they should, and that your security vulnerabilities are properly mitigated.

> A Rego policy is a collection of if statements

### If statements in Rego

Nearly every Rego statement functions as an if-statement, but with different proportions than typical programming languages:

"Programming languages typically have small `if` conditions and relatively large `then` blocks... in Rego the `if` condition is a potentially large block of expressions, and the `then` part is a single expression."

```rego
# Rego example.
# An API call is allowed if the method is a GET
allow {
    input.method == "GET"
}
```

Multiple statements inside a rule are ANDed; ORs are expressed via multiple rules:

```rego
# Rego example.
# An API call is allowed if the method is a GET
allow {
    input.method == "GET"
}

# An API call is allowed if the method is POST and
#   the user is an admin
allow {
    # Only admins can create new objects
    input.method == "POST"
    input.user_is_admin == true
}
```

An equivalent JavaScript version, given for contrast:

```javascript
// Not a Rego example.  A JavaScript example.
function allow() {
  return allow1() || allow2();
}

function allow1() {
  return input.method == "GET";
}

function allow2() {
  return input.method == "POST"
    && input.is_admin == true;
}
```

Rego "has no need for explicit ANDs and ORs" within a rule, and includes "an explicit NOT operator." A self-documenting example:

```rego
# Rego example.
allow {
    operation_is_create
    user_is_admin
}
```

## Rich policy decisions

Real-world policies sometimes make a decision as simple as `allow` or `deny` but often they go far beyond that. What about `warn` or `error`? Or what if the decision is a rate-limit (number), a permitted hostname (a string), or the clusters to deploy an application (an array). Since OPA is a general-purpose policy engine (not an authorization engine) it needs to handle a rich collection of policy decisions.

Because inputs are typically JSON, decisions in Rego can likewise be any JSON type—not just booleans.

> A Rego decision is a JSON document

### Rich Policy Decisions in Rego

`allow` and `deny` are plain variables, not keywords, defaulting to `true`:

```rego
# Rego example.
allow = true {
    operation_is_create
    user_is_admin
}
```

Non-boolean decisions work the same way:

```rego
risk = 100 {
    input.method == "DELETE"
}
```

Partial sets can build up collections, such as error messages:

```rego
deny[msg] {
    input.method == "DELETE"
    not user_is_resource_owner
    msg := "only the owner of a resource may delete it"
}
```

## Collaboration

Real world policies are decided upon by multiple individuals and even teams. The security team might put global requirements in place across all application development teams and each application development team might put policy in place for their app. The k8s cluster administrator puts global policies in place but also delegates policy responsibilities to namespace-level admins. Policy is by its nature a collaborative endeavor, and Rego should recognize and support that.

At some level collaboration is supported simply because Rego is a text-based policy language (aka "policy-as-code"). Teams can check Rego policies into source control, and use peer-review to manage changes to it. We knew, however, that there are all too many examples where teams want to work more independently than that, e.g. putting global policies in place for an entire cluster and empowering team leads to manage policy for their portion of the cluster. That means that different teams should be able to write their policies independently from each other, and then combine those policies after the fact.

If different teams write different policies independently, they will inevitably end up with conflicts from time to time (e.g. one allows the decision and the other denys it). And so there must be a way to resolve those conflicts, based on a variety of different factors. Resolving conflicts is not always easy. It may depend on the teams involved, the kind of decision being made, the resource and its attributes, the time of day, and many other factors. Ergo the conflict resolution mechanism must be tantamount to a policy itself. Sometimes languages are designed to avoid the problem of conflict resolution by designing the language to not express conflicts at all. But this approach inevitably leads to problems because when two teams disagree in the real world and there is no way for them to express that disagreement in the policy language, they simply can't write the policy that they truly mean — the language is ambiguous in terms of the author's intent.

> Rego policies are composable; conflict resolution is a policy itself.

### Collaboration in Rego

Each policy lives in a package:

```rego
package microservice.authorization
```

Separate teams can define their own packages:

```rego
package developer
allow { … }
…
```

```rego
package security
allow { … }
…
```

A combining policy can reference other packages via the `data` keyword:

```rego
package main
allow {
    data.developer.allow
    data.security.allow
}
```

A more nuanced conflict-resolution example, where security's decision takes precedence when it has an opinion:

```rego
package main
# allow if the security team allows (and does not deny)
allow {
    data.security.allow
    not data.security.deny
}
# allow if the security team has no opinion and
#   the developer team allows (and does not deny)
allow {
    not data.security.allow
    not data.security.deny
    data.developer.allow
    not data.developer.deny
}
```

## Summary

The post recaps three requirements: policies are mostly if-statements and should read naturally; decisions can be any JSON value rather than just booleans; and composition/conflict-resolution use the same rule mechanics as ordinary policy logic.

Further reading:

- [Design principle #2: Embrace Hierarchical Data](/blog/rego-design-principle-2-embrace-hierarchical-data-8a4913bdfea1)
- [Design principle #3: Optimize Performance Automatically](/blog/rego-design-principle-3-optimize-performance-automatically-2d29ad3ce96d)
