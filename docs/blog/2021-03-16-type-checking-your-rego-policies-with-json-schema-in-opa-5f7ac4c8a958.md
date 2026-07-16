---
title: "Type checking your Rego policies with JSON schema in OPA"
authors: ["aavarghese", "mandanavaziri", "tsandall"]
date: 2021-03-16
slug: type-checking-your-rego-policies-with-json-schema-in-opa-5f7ac4c8a958
---

![Diagram illustrating JSON schema type checking for Rego](/img/blog/type-checking-your-rego-policies-with-json-schema-in-opa-5f7ac4c8a958/banner.webp)

_**IBM Research & Styra**_

The Open Policy Agent (OPA) is an open-source engine that unifies policy enforcement across the cloud native stack. It provides a query language called Rego that lets the user specify policy as code and an engine that evaluates the queries given input data.

Rego is a powerful declarative language that does not require the user to specify a query strategy. This is achieved by the OPA runtime, leaving users free to reason about policies at a higher level.

Rego has a gradual type system meaning that types can be partially known statically. For example, an object could have certain fields whose types are known and others that are unknown statically. OPA type checks what it knows statically and leaves the unknown parts to be type checked at runtime. However, the input handed to OPA could by design be any JSON value and hence gradual type-checking has no way of catching policy authoring mistakes, even when the policy author knows the intended schema.

In this article, we introduce a new feature that enhances OPA's ability to statically type check Rego code by taking into account schemas for input documents. This improves programmer productivity and helps Rego programmers catch errors earlier.

To achieve this, we enable `opa eval` to take the schema for the input document, specified in JSON Schema format. The input schema is passed with the flag `--schema` (`-s`). Armed with this information, OPA's enhanced type checker can detect bugs stemming from incorrect usage of the input.

This feature is now available in OPA v0.27.0. Let's check it out!

![Rego type checking demo](/img/blog/type-checking-your-rego-policies-with-json-schema-in-opa-5f7ac4c8a958/1.webp)

_Rego type checking demo_

Consider the following Rego code, which assumes as input a Kubernetes admission review. For resources that are Pods, it checks that the image name starts with a specific prefix.

```rego
package kubernetes.admission


deny[msg] {
  input.request.kind.kinds == "Pod"
  image := input.request.object.spec.containers[_].image
  not startswith(image, "hooli.com/")
  msg := sprintf("image '%v' comes from untrusted registry", [image])
}
```

Notice that this code has a typo in it: `input.request.kind.kinds` is undefined and should have been `input.request.kind.kind`.

Consider the following input document:

```json
{
  "kind": "AdmissionReview",
  "request": {
    "kind": {
      "kind": "Pod",
      "version": "v1"
    },
    "object": {
      "metadata": {
        "name": "myapp"
      },
      "spec": {
        "containers": [
          { "image": "nginx", "name": "nginx-frontend" },
          { "image": "mysql", "name": "mysql-backend" }
        ]
      }
    }
  }
}
```

```bash
% opa eval -format pretty -i admission-review.json -d pod.rego
[]
```

The empty value returned is indistinguishable from a situation where the input did not violate the policy. This error is therefore causing the policy not to catch violating inputs appropriately.

If we fix the Rego code and change `input.request.kind.kinds` to `input.request.kind.kind`, then we obtain the expected result:

```
[
  "image 'nginx' comes from untrusted registry"
  "image 'mysql' comes from untrusted registry"
]
```

With this feature, it is possible to pass a schema to `opa eval`, written in JSON Schema.

Consider the [Kubernetes admission review input schema](https://github.com/aavarghese/opa-schema-examples/blob/main/kubernetes/admission-schema.json). We can pass this schema to the evaluator as follows:

```bash
% opa eval -format pretty -i admission-review.json -d pod.rego -s admission-schema.json
```

With the erroneous Rego code, we now obtain the following type error:

```
1 error occurred: pod.rego:5: rego_type_error: undefined ref: input.request.kind.kinds
  input.request.kind.kinds
                     ^
                     have: "kinds"
                     want (one of): ["kind" "version"]
```

This indicates the error to the Rego developer right away, without having the need to observe the results of runs on actual data, thereby improving productivity.

With this new feature, Rego developers will be able to provide JSON Schemas for their input documents and get the most out of static type checking. If you don't have a JSON Schema handy, you can easily obtain one from a sample JSON file using online tools (see links below). In the future, we will add to this feature the support to allow users to specify a directory of schemas and assign schemas to data documents, as well. This will be done via annotations (global, rule-level, rule output) that will indicate what schema is associated with what Rego path expression.

Happy Rego type checking!

For more information and limitations, see [documentation](https://www.openpolicyagent.org/docs/policy-language#schema) and [examples](https://github.com/aavarghese/opa-schema-examples/).
