---
title: "Write Policy in OPA. Enforce Policy in SQL."
authors: ["tsandall"]
date: 2018-08-17
slug: write-policy-in-opa-enforce-policy-in-sql-d9d24db93bf4
---

This post explains how to use OPA and SQL to protect access to sensitive data in your services without impacting consistency, performance, or scalability. We show how to translate OPA policies into SQL and enforce them within the database.

Throughout this post we will refer to a hypothetical service (`petprofilesv1`) used by a chain of veterinary clinics. The service exposes an HTTP API that serves profiles for pets at the clinics. The service implements the HTTP API by querying a SQL database and returning the results.

![Client sends GET /pets/fluffy to petprofilesv1, which queries the DB](/img/blog/write-policy-in-opa-enforce-policy-in-sql-d9d24db93bf4/1.webp)

Services like `petprofilesv1` often implement authorization policies that depend on attributes of the objects being accessed. They often also include authorization policies that must be applied to filter data in API results.

In this post, we dive into how services can integrate with OPA to enforce these kinds of authorization policies.

## Replicating Context Is Hard

Imagine we want to enforce a simple role-based authorization policy in our hypothetical service that says:

> "Only veterinarians are allowed to read pet profiles."

![Client and petprofilesv1 query OPA with method/path/subject roles and receive an allow response](/img/blog/write-policy-in-opa-enforce-policy-in-sql-d9d24db93bf4/2.webp)

To implement this policy we could create a simple `allow` rule in OPA:

```rego
default allow = false

allow = true {
  input.method = "GET"
  input.path = ["pets", name]              # name unused for now
  input.subject.roles[_] = "veterinarian"
}
```

When the service queries OPA, it provides the identity of the caller (`input.subject`), the operation being performed (`input.method`), and the resource being operated on (`input.path`). The response from OPA indicates whether the request should be allowed (`true`) or denied (`false`).

**This model works well when all of the context required for the decision is carried in the request.** But what happens if the incoming request does not include the necessary context? For example, suppose the policy should say:

> "Only the treating veterinarian is allowed to read a pet's profile."

In this case, a mapping from pet to treating veterinarian is required but not included in incoming HTTP GET requests.

One way of solving this would be to have the service fetch relevant context and provide it as input to the policy query:

```rego
allow = true {
  input.method = "GET"
  input.path = ["pets", name]                  # name unused for now
  input.subject.user = input.pet.veterinarian
}
```

This approach works well for small inputs and keeps the enforcement model relatively simple.

The downside is that it requires the service to know the exact context required by the policy. Each API endpoint exposed by the service (or set of services) could need custom logic to fetch the context to provide as input to the policy query. **Tight coupling between the service and the policy is difficult to maintain over time.** Moreover, as the size of the input grows, it may become prohibitive to fetch and supply context on every query.

Another approach is to have the pet-veterinarian mapping replicated into OPA and cached in-memory. This approach requires an additional component to act as a _data source (DS)_ that gathers the extra context required by policies and loads it into OPA.

![A DS component feeds pet-veterinarian mappings into the DB, which OPA and petprofilesv1 use to make the allow decision](/img/blog/write-policy-in-opa-enforce-policy-in-sql-d9d24db93bf4/3.webp)

If we used this approach, we could rewrite the OPA rule as follows:

```rego
allow = true {
  input.method = "GET"
  input.path = ["pets", name]
  data.pets[name] = input.subject.user
}
```

This avoids directly coupling the service with the policy. However, the obvious challenge is that OPA must maintain a cache of all pet-veterinarian mappings to authorize requests correctly. Depending on the size of the mapping and the consistency requirements of the service, this may not be feasible: OPA may not have the up-to-date mappings while evaluating the policy.

We could also use built-in functions in OPA to execute HTTP requests or query external databases during policy evaluation however this approach makes policies harder to test in isolation and the extra network hop negatively impacts latency and availability. Similarly, this approach may also violate the service's consistency requirements.

## Lists Require Filtering

In addition to challenges with replication, we also have to deal with APIs that return lists of resources.

When designing service APIs, it's common to expose a _list_ operation to return all the resources in a collection (e.g., the `petprofilesv1` service needs to expose `GET /pets`). Since list operations frequently return the same information (for each resource) as reads for individual resources, it's important that we apply the authorization policy to filter elements in the result.

While we could execute a policy query for each resource (or structure the policy to accept a list of resources to authorize access to), this approach complicates pagination models and does not scale well compared to filtering in the data store.

## Partial Evaluation

In the first section of this post we explained why it may be difficult to replicate context into OPA in a reliable, maintainable, and performant manner.

As of the latest release of OPA (v0.9), services can leverage the [Partial Evaluation](/blog/partial-evaluation-162750eaf422) feature to avoid replicating context into OPA. When services use Partial Evaluation, they specify what portions of the data or input documents are _unknown_. When OPA evaluates the policy, any statements that depend on unknown values are not evaluated — they are saved and returned to the caller.

By partially evaluating authorization policies, OPA can treat context that's unavailable during evaluation as unknown. For example, statements that depend on the pet-veterinarian mapping would be saved and returned to the caller.

![Partial evaluation output: the unresolved condition data.pets.fluffy.veterinarian = "alice" is returned alongside OPA](/img/blog/write-policy-in-opa-enforce-policy-in-sql-d9d24db93bf4/4.webp)

The table below shows the output when the service queries OPA for a request from `alice` trying to access `fluffy`'s profile.

```
+----------+-------------------------------------------------------+
| Policy   | allow {                                               |
|          |   input.method = "GET"                                |
|          |   input.path = ["pets", name]                         |
|          |   data.pets[name].veterinarian = input.subject.user   |
|          | }                                                     |
+----------+-------------------------------------------------------+
| Input    | {                                                     |
|          |   "method": "GET",                                    |
|          |   "path": ["pets", "fluffy"],                         |
|          |   "subject": {"user": "alice"}                        |
|          | }                                                     |
+----------+-------------------------------------------------------+
| Unknowns | [data.pets]                                           |
+----------+-------------------------------------------------------+
| Output   | data.pets["fluffy"].veterinarian = "alice"            |
+----------+-------------------------------------------------------+
```

When OPA partially evaluates policies, the output is a simplified version of the policy. The result of partial evaluation can be interpreted as a sequence of Rego expressions ANDed and ORed together:

```
( expr-1 AND expr-2 AND … ) OR ( expr-N AND expr-N+1 AND … ) OR …
```

If we extend the policy to allow pet owners to access their pet's profiles and require that veterinarians be signed in from a device at the pet's clinic, the output would include two queries (which can be ORed):

```
+----------+-------------------------------------------------------+
| Policy   | allow {                                               |
|          |   input.method = "GET"                                |
|          |   input.path = ["pets", name]                         |
|          |   data.pets[name].owner = input.subject.user          |
|          | }                                                     |
|          |                                                       |
|          | allow {                                               |
|          |   input.method = "GET"                                |
|          |   input.path = ["pets", name]                         |
|          |   data.pets[name].veterinarian = input.subject.user   |
|          |   data.pets[name].clinic = input.subject.location     |
|          | }                                                     |
+----------+-------------------------------------------------------+
| Input    | {                                                     |
|          |   "method": "GET",                                    |
|          |   "path": ["pets", "fluffy"],                         |
|          |   "subject": {                                        |
|          |     "user": "alice",                                  |
|          |     "location": "SOMA"                                |
|          |   }                                                   |
|          | }                                                     |
+----------+-------------------------------------------------------+
| Unknowns | [data.pets]                                           |
+----------+-------------------------------------------------------+
| Output 1 | data.pets["fluffy"].owner = "alice"                   |
+----------+-------------------------------------------------------+
| Output 2 | data.pets["fluffy"].veterinarian = "alice";           |
|          | data.pets["fluffy"].clinic = "SOMA"                   |
+----------+-------------------------------------------------------+
```

In some cases, OPA can still determine that a request should be allowed or denied unconditionally. In these cases, OPA returns a single empty query or no queries at all (respectively.) For example, if the input above was missing the subject field, neither allow rule would match (even partially) and the result would be empty.

By returning the simplified remainder of the policy to the service, we avoid evaluating the entire policy in one place — which means we do not have to replicate all of the context into OPA. The tradeoff is that the response from OPA is not simply an allow (`true`) or deny (`false`) value anymore — it's a set of Rego queries that must be evaluated by something (eventually).

This section explained how we can avoid replicating context into OPA using Partial Evaluation. However, this only solves part of the problem. If the service were to evaluate the result of Partial Evaluation itself, the service would still have to fetch the additional context from the data store (which would likely result in a solution with the same issues as before.)

## Enforcing OPA policies with SQL

To overcome the issues we outlined above, the remainder of the policy needs to be evaluated as close to the data as possible — inside the database.

[Since OPA policies are essentially just queries](/blog/opas-full-stack-policy-language-caeaadb1e077), the translation from Rego into another query language, like SQL, is relatively easy. As long as the policies expressed in Rego do not perform joins, we can translate sets of Rego queries into SQL expressions that get appended onto WHERE clauses. For example, the last policy from above says that:

> "Pet owners can access their own pet's profiles."
> "Veterinarians can access pet profiles from devices at the clinic."

We can express this policy in OPA as follows:

```rego
package petclinic.authz

default allow = false

allow {
  input.method = "GET"
  input.path = ["pets", name]
  allowed[pet]
  pet.name = name
}

allowed[pet] {
  pet = data.pets[_]
  pet.owner = input.subject.user
}

allowed[pet] {
  pet = data.pets[_]
  pet.veterinarian = input.subject.user
  pet.clinic = input.subject.location
}
```

In this example we have factored the authorization decision into helper rules named `allowed`. The `allowed` rules generate a _set_ of pets the user is allowed to see.

To integrate with OPA, the service invokes the Compile API and marks the `data.pets` path as unknown. For example, when `fluffy`'s veterinarian `alice` requests the profile from a device at the clinic, the query to OPA looks like this:

```json
{
  // The policy query to run.
  "query": "data.petclinic.authz.allow = true",
  // The input document to use.
  "input": {
    "method": "GET",
    "path": ["pets", "fluffy"],
    "subject": {
      "user": "alice",
      "location": "SOMA"
    }
  },
  // The values to treat as unknown during evaluation.
  "unknowns": ["data.pets"]
}
```

The response from OPA contains two queries:

```rego
# Query 1
pet = data.pets[_];
pet.owner = "alice";
pet.name = "fluffy"

# Query 2
pet = data.pets[_];
pet.veterinarian = "alice";
pet.clinic = "SOMA";
pet.name = "fluffy"
```

The service can consume these responses by translating them into SQL WHERE clauses. To keep things simple we can interpret references like `data.pets[_].owner` as follows:

- The prefix `data.pets` refers to the `pets` table in SQL.
- The variable `_` refers to a row in the `pets` table.
- The suffix `owner` refers to a column in the `pets` table.

With this interpretation we would produce the following SQL expression:

```sql
(pets.owner = "alice" AND pets.name = "fluffy") OR
(pets.veterinarian = "alice" AND
 pets.clinic = "SOMA" AND
 pets.name = "fluffy")
```

To show how you can implement a library that converts a fragment of Rego into SQL WHERE clauses, we have prepared an example that includes a library. You can find the example on [GitHub in the OPA contrib repository](https://github.com/open-policy-agent/contrib/tree/master/data_filter_example).

## Data Filtering with OPA

In addition to the API to get individual pet profiles, our service also exposes an API to list pet profiles (e.g,. GET /pets). In many APIs, list operations return the full extent of the resources in the collection. Because of this, it's important to apply the same data authorization policy there.

Even if the list operations only returned a subset of fields (e.g., the resource ID), it's a common requirement to not show clients the IDs of resources they are not allowed to access, so the filtering policy should still be applied.

We can extend the policy from above to cover list operations by adding the following rule:

```rego
allow {
  input.method = "GET"
  input.path = ["pets"]
  allowed[pet]
}
```

This rule is similar to the ones from earlier. The only differences are that it matches on the path `["pets"]` (instead of `["pets", name]`) and there is no condition on `pet.name`.

The result sent back to the service from OPA will be similar to the cases above. For example, if `alice` tries to list the pet profiles from the same device at the `SOMA` clinic the response from OPA will be:

```rego
# Output #1
pet = data.pets[_]
pet.owner = "alice"

# Output #2
pet = data.pets[_]
pet.veterinarian = "alice";
pet.clinic = "SOMA"
```

These statements will get translated into a SQL WHERE clause that return all of the pet profiles that `alice` is allowed to see at the `SOMA` clinic as well as any profiles of her own pets at any clinic:

```sql
(pets.owner = "alice") OR
(pets.veterinarian = "alice" AND pets.clinic = "SOMA")
```

This example highlights how you can reuse the same data authorization policy across both the _get_ and _list_ APIs.

## Try It Out

You can try out this new capability with the latest release of OPA. If you integrate with OPA in Go, you can use the [rego.Rego#Partial](https://godoc.org/github.com/open-policy-agent/opa/rego#example-Rego-Partial) function to invoke Partial Evaluation and obtain a set of conditions to apply to incoming requests. Similarly, if you integrate with OPA via HTTP, you can use the new [Compile API](https://www.openpolicyagent.org/docs/rest-api.html#compile-api) to obtain identical conditions (represented in JSON) that you can process in any language.

We have also published a small [example](https://github.com/open-policy-agent/contrib/tree/master/data_filter_example) that shows how to integrate a simple Python service backed by SQLite. The example includes a Python module that translates Rego queries into SQL predicates. The example includes additional support for policies that perform joins as well as relational operators like `!=`, `<=`, `>=`, etc.

## Wrap Up

This post explored how you can leverage OPA to enforce context-aware authorization policies that go way beyond simple protocol-level approaches. By using Partial Evaluation to obtain conditions that you evaluate in your service or via database queries, you can implement data protection and filtering policies in your service that are correct, maintainable, consistent, performant, and scalable.

We anticipate these OPA features will be used to solve a number of interesting problems in the authorization space, such as multi-tenancy support and protection of PII and other sensitive data.

Next we plan to tackle:

- Integrations with other kinds of databases, e.g., Elasticsearch.
- Tooling to identify when policies exceed the supported fragments of Rego.
- Indirection layers between the policy and underlying DB schema.
- Other use cases that build on partial evaluation like _column masking_.

If you have questions or comments, or you are interested in contributing, please reach out via [Slack](https://slack.openpolicyagent.org) or create tickets on [GitHub](https://github.com/open-policy-agent/opa).
