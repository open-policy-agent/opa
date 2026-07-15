---
title: "Enhanced Type Checking for OPA with JSON Schema Annotations"
authors: ["mandanavaziri", "aavarghese", "tsandall"]
date: 2021-04-27
slug: enhanced-type-checking-for-opa-with-json-schema-annotations-826acb0f575
---

![Enhanced type checking for OPA with JSON schema](/img/blog/enhanced-type-checking-for-opa-with-json-schema-annotations-826acb0f575/banner.webp)

_**IBM Research & Styra**_

## What's happened?

In a [previous Medium blog](/blog/type-checking-your-rego-policies-with-json-schema-in-opa-5f7ac4c8a958), a feature released in [OPA v0.27.0](https://github.com/open-policy-agent/opa/releases/tag/v0.27.0) was introduced that lets OPA's static type checker take JSON schemas for input documents into account, improving how errors from misused data are caught during policy authoring.

The article explains that `opa eval` can take a schema for the input document via the `--schema(-s)` flag, applied globally across the module, allowing OPA's checker to catch issues like undefined objects.

## What's new?

This section covers extending type checking to support **multiple JSON schema files** for both input and data documents.

Example Rego code (based on a Kubernetes admission review input) is shown with a typo bug:

```rego
package kubernetes.admission

deny[msg] {
  input.request.kind.kinds == "Pod"                       # This line has a typo, should be input.request.kind.kind
  image := input.request.object.spec.containers[_].image
  not startswith(image, "hooli.com/")
  msg := sprintf("image '%v' comes from untrusted registry", [image])
}
```

```rego
input.request.kind.kinds
```

which should be:

```rego
input.request.kind.kind
```

Running:

```bash
% opa eval --format pretty -i admission-input.json -d policy.rego -s schemas/admission-input.json
```

returns:

```
1 error occurred: policy.rego:4: rego_type_error: undefined ref: input.request.kind.kinds
  input.request.kind.kinds
                     ^
                     have: "kinds"
                     want (one of): ["kind" "version"]
```

The article notes that a similar typo in `input.request.object.spec.containers[_].image` would go undetected, since the admission review schema leaves `input.request.object` generically typed.

To solve this, `opa eval` now supports a **directory of schema files** via the same `--schema (-s)` flag, while still supporting single schema files. New Rego **Metadata** blocks allow specifying **schema annotations and scope**, improving bug detection for undefined fields.

An **override feature** is also introduced, described as letting users merge existing schemas and subschemas "for more precise type checking."

Additionally, schema loading is enabled for `opa eval — bundle`, supporting type checking of data documents across a bundle — useful for "batch type analysis of Rego policies as part of any CI/CD pipelines."

These features are available in [**OPA v0.28.0**](https://github.com/open-policy-agent/opa/releases/tag/v0.28.0).

## What's the big deal you say?

Example policies and schemas are available in the [opa-schema-examples repository](https://github.com/aavarghese/opa-schema-examples).

The Kubernetes Admission Review example is revisited to demonstrate annotations and schema overriding. The `object` field in an Admission Review can contain any Kubernetes resource, and its [schema](https://github.com/aavarghese/opa-schema-examples/blob/main/kubernetes/schemas/input.json) leaves that field generically typed.

Annotations associate a Rego expression with an input or data schema loaded via `opa eval -s`, within a given scope.

Annotations use `METADATA` comment blocks in YAML syntax, where "every line in the block must start at Column 1."

Example schema directory structure:

```
mySchemasDir/
├── input.json
└── kubernetes
└──────pod.json
```

Loading the schema directory can be done via:

```bash
% opa eval data.kubernetes.admission --format pretty -i opa-schema-examples/kubernetes/input.json -d opa-schema-examples/kubernetes/policy.rego -s opa-schema-examples/kubernetes/mySchemasDir
```

```bash
% opa eval data.kubernetes.admission -format pretty -i opa-schema-examples/kubernetes/input.json -b opa-schema-examples/bundle.tar.gz -s opa-schema-examples/kubernetes/mySchemasDir
```

In this example, `input` is associated with the Admission Review schema (`input.json`), and `input.request.object` is set to the Kubernetes Pod schema (`pod.json`), with the second annotation overriding the first. The order of annotations is stated to matter "for overriding to work correctly."

Notes on schema reference syntax:

- Relative paths inside `mySchemasDir` are used, omitting the `.json` suffix
- The global variable `schema` represents the top level of the directory
- `schema.input` is valid; `schema.pod-schema` is invalid due to the hyphen — the correct syntax is `schema["pod-schema"]`

Combining annotations and overriding allows catching type errors in `input.request.object.spec.containers[_].image`:

```rego
package kubernetes.admission

# METADATA
# scope: rule
# schemas:
#   - input: schema["input"]
#   - input.request.object: schema.kubernetes["pod"]
deny[msg] {
  input.request.kind.kinds == "Pod"                       # This line has a typo, should be input.request.kind.kind
  image := input.request.object.spec.containers[_].images # This line has a typo, should be input.request.object.spec.containers[_].image
  not startswith(image, "hooli.com/")
  msg := sprintf("image '%v' comes from untrusted registry", [image])
}
```

```
2 errors occurred:
policy.rego:9: rego_type_error: undefined ref: input.request.kind.kinds
  input.request.kind.kinds
                     ^ have: "kinds"
                     want (one of): ["kind" "version"]
policy.rego:10: rego_type_error: undefined ref: input.request.object.spec.containers[_].images
  input.request.object.spec.containers[_].images
                                          ^ have: "images"
                                          want (one of): ["args" "command" "env" "envFrom" "image" "imagePullPolicy" "lifecycle" "livenessProbe" "name" "ports" "readinessProbe" "resources" "securityContext" "stdin" "stdinOnce" "terminationMessagePath" "terminationMessagePolicy" "tty" "volumeDevices" "volumeMounts" "workingDir"]
```

A second example checks whether an operation is allowed for a user, given an ACL data document. In the first `allow` rule, `input` uses `input.json` and `data.acl` uses `acl-schema.json`; an invalid expression like `data.acl.typo` would trigger a type error.

```rego
package policy

import data.acl

default allow = false

# METADATA
# scope: rule
# schemas:
#   - input: schema["input"]
#   - data.acl: schema["acl-schema"]
allow {
        access = data.acl["alice"]
        access[_] == input.operation
}

allow {
        access = data.acl["bob"]
        access[_] == input.operation
}
```

The article clarifies that this annotation "does not constrain other paths under data" — only the type of `data.acl` is statically known.

The second `allow` rule in the same example has no schema annotations, so it isn't type-checked against any loaded schema. Different rules in the same module can use different input schemas.

Annotations can also apply at different scopes via the `scope` field in Metadata, defaulting to the following statement if omitted. Supported scope values:

- **rule** - applies to the individual rule statement
- **document** - applies to all of the rules with the same name in the same package
- **package** - applies to all of the rules in the package
- **subpackages** - applies to all of the rules in the package and all subpackages (recursively)

More details: [Annotation scopes documentation](https://www.openpolicyagent.org/docs/latest/schemas/#annotation-scopes)

## What's Next?

The article closes by noting future plans to extend schema support to additional JSON Schema features such as `additionalProperties`, with more updates promised in upcoming OPA releases.

Further reading: [OPA schemas documentation](https://www.openpolicyagent.org/docs/latest/schemas/)

## Links

- Documentation: [OPA schemas documentation](https://www.openpolicyagent.org/docs/latest/schemas/)
- Examples: [opa-schema-examples repository](https://github.com/aavarghese/opa-schema-examples/)
- JSON to JSON schema online tool: [jsonschema.net](https://jsonschema.net/)
- JSON schema reference: [Understanding JSON Schema reference](http://json-schema.org/understanding-json-schema/reference/index.html)
- Related blog: /blog/type-checking-your-rego-policies-with-json-schema-in-opa-5f7ac4c8a958
