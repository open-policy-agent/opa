---
title: Type Checking
kind: misc
weight: 2
---

## Using schemas to enhance the Rego type checker

You can provide one or more input schema files and/or data schema files to `opa eval` to improve static type checking and get more precise error reports as you develop Rego code.

The `-s` flag can be used to upload schemas for input and data documents in JSON Schema format. You can either load a single JSON schema file for the input document or directory of schema files.

```
-s, --schema string set schema file path or directory path
```

### Passing a single file with -s

When a single file is passed, it is a schema file associated with the input document globally. This means that for all rules in all packages, the `input` has a type derived from that schema. There is no constraint on the name of the file, it could be anything.

Example:
```
opa eval data.envoy.authz.allow -i opa-schema-examples/envoy/input.json -d opa-schema-examples/envoy/policy.rego -s opa-schema-examples/envoy/schemas/my-schema.json
```


### Passing a directory with -s

When a directory path is passed, annotations will be used in the code to indicate what expressions map to what schemas (see below).
Both input schema files and data schema files can be provided in the same directory, with different names. The directory of schemas may have any sub-directories. Notice that when a directory is passed the input document does not have a schema associated with it globally. This must also
be indicated via an annotation.


Example:
```
opa eval data.kubernetes.admission -i opa-schema-examples/kubernetes/input.json -d opa-schema-examples/kubernetes/policy.rego -s opa-schema-examples/kubernetes/schemas

```

Schemas can also be provided for policy and data files loaded via `opa eval --bundle`


Example:
```
opa eval data.kubernetes.admission -i opa-schema-examples/kubernetes/input.json -b opa-schema-examples/bundle.tar.gz -s opa-schema-examples/kubernetes/schemas

```

Samples provided at: https://github.com/aavarghese/opa-schema-examples/



## Usage scenario with a single schema file

Consider the following Rego code, which assumes as input a Kubernetes admission review. For resources that are Pods, it checks that the image name
starts with a specific prefix.

`pod.rego`
```
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


`input.json`
```
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
            {
              "image": "nginx",
              "name": "nginx-frontend"
            },
            {
              "image": "mysql",
              "name": "mysql-backend"
            }
          ]
        }
      }
    }
  }
  ```

Clearly there are 2 image names that are in violation of the policy. However, when we evaluate the erroneous Rego code against this input we obtain:
  ```
  % opa eval data.kubernetes.admission --format pretty -i opa-schema-examples/kubernetes/input.json -d opa-schema-examples/kubernetes/policy.rego
  []
  ```

The empty value returned is indistinguishable from a situation where the input did not violate the policy. This error is therefore causing the policy not to catch violating inputs appropriately.

If we fix the Rego code and change `input.request.kind.kinds` to `input.request.kind.kind`, then we obtain the expected result:
  ```
  [
  "image 'nginx' comes from untrusted registry",
  "image 'mysql' comes from untrusted registry"
  ]
  ```

With this feature, it is possible to pass a schema to `opa eval`, written in JSON Schema. Consider the admission review schema provided at:
https://github.com/aavarghese/opa-schema-examples/blob/main/kubernetes/schemas/input.json

We can pass this schema to the evaluator as follows:
  ```
  % opa eval data.kubernetes.admission --format pretty -i opa-schema-examples/kubernetes/input.json -d opa-schema-examples/kubernetes/policy.rego -s opa-schema-examples/kubernetes/schemas/input.json
  ```

With the erroneous Rego code, we now obtain the following type error:
  ```
  1 error occurred: ../../aavarghese/opa-schema-examples/kubernetes/policy.rego:5: rego_type_error: undefined ref: input.request.kind.kinds
	input.request.kind.kinds
	                   ^
	                   have: "kinds"
	                   want (one of): ["kind" "version"]
  ```

This indicates the error to the Rego developer right away, without having the need to observe the results of runs on actual data, thereby improving productivity.

## Schema annotations

When passing a directory of schemas to `opa eval`, schema annotations become handy to associate a Rego expression with a corresponding schema within a given scope:

```
# METADATA
# schemas:
#   - <path-to-value>:<path-to-schema>
#   ...
#   - <path-to-value>:<path-to-schema>
allow {
  ...
}
```

See the [annotations documentation](../annotations) for general information relating to annotations.

The `schemas` field specifies an array associating schemas to data values. Paths must start with `input` or `data` (i.e., they must be fully-qualified.)

The type checker derives a Rego Object type for the schema and an appropriate entry is added to the type environment before type checking the rule. This entry is removed upon exit from the rule.

Example:

Consider the following Rego code which checks if an operation is allowed by a user, given an ACL data document:

```
package policy

import data.acl

default allow := false

# METADATA
# schemas:
#   - input: schema.input
#   - data.acl: schema["acl-schema"]
allow {
    access := data.acl["alice"]
    access[_] == input.operation
}

allow {
    access := data.acl["bob"]
    access[_] == input.operation
}
```

Consider a directory named `mySchemasDir` with the following structure, provided via `opa eval --schema opa-schema-examples/mySchemasDir`

```$ tree mySchemasDir/
mySchemasDir/
├── input.json
└── acl-schema.json
```

For actual code samples, see https://github.com/aavarghese/opa-schema-examples/tree/main/acl.

In the first `allow` rule above, the input document has the schema `input.json`, and `data.acl` has the schema `acl-schema.json`. Note that we use the relative path inside the `mySchemasDir` directory to identify a schema, omit the `.json` suffix, and use the global variable `schema` to stand for the top-level of the directory.
Schemas in annotations are proper Rego references. So `schema.input` is also valid, but `schema.acl-schema` is not.

If we had the expression `data.acl.foo` in this rule, it would result in a type error because the schema contained in `acl-schema.json` only defines object properties `"alice"` and `"bob"` in the ACL data document.

On the other hand, this annotation does not constrain other paths under `data`. What it says is that we know the type of `data.acl` statically, but not that of other paths. So for example, `data.foo` is not a type error and gets assigned the type `Any`.

Note that the second `allow` rule doesn't have a METADATA comment block attached to it, and hence will not be type checked with any schemas.

On a different note, schema annotations can also be added to policy files part of a bundle package loaded via `opa eval --bundle` alongwith the `--schema` parameter for type checking a set of `*.rego` policy files.

The *scope* of the `schema` annotation can be controlled through the [scope](../annotations#scope) annotation

In case of overlap, schema annotations override each other as follows:

```
rule overrides document
document overrides package
package overrides subpackages
```

The following sections explain how the different scopes affect `schema` annotation
overriding for type checking.

#### Rule and Document Scopes

In the example above, the second rule does not include an annotation so type
checking of the second rule would not take schemas into account. To enable type
checking on the second (or other rules in the same file) we could specify the
annotation multiple times:

```
# METADATA
# scope: rule
# schemas:
#   - input: schema.input
#   - data.acl: schema["acl-schema"]
allow {
    access := data.acl["alice"]
    access[_] == input.operation
}

# METADATA
# scope: rule
# schemas:
#   - input: schema.input
#   - data.acl: schema["acl-schema"]
allow {
    access := data.acl["bob"]
    access[_] == input.operation
}
```

This is obviously redundant and error-prone. To avoid this problem, we can
define the annotation once on a rule with scope `document`:

```
# METADATA
# scope: document
# schemas:
#   - input: schema.input
#   - data.acl: schema["acl-schema"]
allow {
    access := data.acl["alice"]
    access[_] == input.operation
}

allow {
    access := data.acl["bob"]
    access[_] == input.operation
}
```

In this example, the annotation with `document` scope has the same affect as the
two `rule` scoped annotations in the previous example.

#### Package and Subpackage Scopes

Annotations can be defined at the `package` level and then applied to all rules
within the package:

```
# METADATA
# scope: package
# schemas:
#   - input: schema.input
#   - data.acl: schema["acl-schema"]
package example

allow {
    access := data.acl["alice"]
    access[_] == input.operation
}

allow {
    access := data.acl["bob"]
    access[_] == input.operation
}
```

`package` scoped schema annotations are useful when all rules in the same
package operate on the same input structure. In some cases, when policies are
organized into many sub-packages, it is useful to declare schemas recursively
for them using the `subpackages` scope. For example:

```
# METADTA
# scope: subpackages
# schemas:
# - input: schema.input
package kubernetes.admission
```

This snippet would declare the top-level schema for `input` for the
`kubernetes.admission` package as well as all subpackages. If admission control
rules were defined inside packages like `kubernetes.admission.workloads.pods`,
they would be able to pick up that one schema declaration.

### Overriding

JSON Schemas are often incomplete specifications of the format of data. For example, a Kubernetes Admission Review resource has a field `object` which can contain any other Kubernetes resource. A schema for Admission Review has a generic type `object` for that field that has no further specification. To allow more precise type checking in such cases, we support overriding existing schemas.

Consider the following example:

```
package kubernetes.admission

# METADATA
# scope: rule
# schemas:
# - input: schema.input
# - input.request.object: schema.kubernetes.pod
deny[msg] {
    input.request.kind.kind == "Pod"
    image := input.request.object.spec.containers[_].image
    not startswith(image, "hooli.com/")
    msg := sprintf("image '%v' comes from untrusted registry", [image])
}
```

In this example, the `input` is associated with an Admission Review schema, and furthermore `input.request.object` is set to have the schema of a Kubernetes Pod. In effect, the second schema annotation overrides the first one. Overriding is a schema transformation feature and combines existing schemas. In this case, we are combining the Admission Review schema with that of a Pod.

Notice that the order of schema annotations matter for overriding to work correctly.

Given a schema annotation, if a prefix of the path already has a type in the environment, then the annotation has the effect of merging and overriding the existing type with the type derived from the schema. In the example above, the prefix `input` already has a type in the type environment, so the second annotation overrides this existing type. Overriding affects the type of the longest prefix that already has a type. If no such prefix exists, the new path and type are added to the type environment for the scope of the rule.

In general, consider the existing Rego type:

```
object{a: object{b: object{c: C, d: D, e: E}}}
```
If we override this type with the following type (derived from a schema annotation of the form `a.b.e: schema-for-E1`):

```
object{a: object{b: object{e: E1}}}
```

It results in the following type:

```
object{a: object{b: object{c: C, d: D, e: E1}}}
```

Notice that `b` still has its fields `c` and `d`, so overriding has a merging effect as well. Moreover, the type of expression `a.b.e` is now `E1` instead of `E`.

We can also use overriding to add new paths to an existing type, so if we override the initial type with the following:

```
object{a: object{b: object{f: F}}}
```

we obtain the following type:

```
object{a: object{b: object{c: C, d: D, e: E, f: F}}}
```



We use schemas to enhance the type checking capability of OPA, and not to validate the input and data documents against desired schemas. This burden is still on the user and care must be taken when using overriding to ensure that the input and data provided are sensible and validated against the transformed schemas.


### Multiple input schemas

It is sometimes useful to have different input schemas for different rules in the same package. This can be achieved as illustrated by the following example:

```
package policy

import data.acl

default allow := false

# METADATA
# scope: rule
# schemas:
#  - input: schema["input"]
#  - data.acl: schema["acl-schema"]
allow {
    access := data.acl[input.user]
    access[_] == input.operation
}

# METADATA for whocan rule
# scope: rule
# schemas:
#   - input: schema["whocan-input-schema"]
#   - data.acl: schema["acl-schema"]
whocan[user] {
    access := acl[user]
    access[_] == input.operation
}
```

The directory that is passed to `opa eval` is the following:
```$ tree mySchemasDir/
mySchemasDir/
├── input.json
└── acl-schema.json
└── whocan-input-schema.json
```

In this example, we associate the schema `input.json` with the input document in the rule `allow`, and the schema `whocan-input-schema.json`
with the input document for the rule `whocan`.

### Translating schemas to Rego types and dynamicity

Rego has a gradual type system meaning that types can be partially known statically. For example, an object could have certain fields whose types are known and others that are unknown statically. OPA type checks what it knows statically and leaves the unknown parts to be type checked at runtime. An OPA object type has two parts: the static part with the type information known statically, and a dynamic part, which can be nil (meaning everything is known statically) or non-nil and indicating what is unknown.

When we derive a type from a schema, we try to match what is known and unknown in the schema. For example, an `object` that has no specified fields becomes the Rego type `Object{Any: Any}`. However, currently `additionalProperties` and `additionalItems` are ignored. When a schema is fully specified, we derive a type with its dynamic part set to nil, meaning that we take a strict interpretation in order to get the most out of static type checking. This is the case even if `additionalProperties` is set to `true` in the schema. In the future, we will take this feature into account when deriving Rego types.

When overriding existing types, the dynamicity of the overridden prefix is preserved.

### Supporting JSON Schema composition keywords

JSON Schema provides keywords such as `anyOf` and `allOf` to structure a complex schema. For `anyOf`, at least one of the subschemas must be true, and for `allOf`, all subschemas must be true. The type checker is able to identify such keywords and derive a more robust Rego type through more complex schemas.

#### `anyOf`

Specifically, `anyOf` acts as an Rego Or type where at least one (can be more than one) of the subschemas is true. Consider the following Rego and schema file containing `anyOf`:

`policy-anyOf.rego`
```
package kubernetes.admission

# METADATA
# scope: rule
# schemas:
#   - input: schema["input-anyOf"]
deny {
    input.request.servers.versions == "Pod"
}
```

`input-anyOf.json`
```
{
    "$schema": "http://json-schema.org/draft-07/schema",
    "type": "object",
    "properties": {
        "kind": {"type": "string"},
        "request": {
            "type": "object",
            "anyOf": [
                {
                   "properties": {
                       "kind": {
                           "type": "object",
                           "properties": {
                               "kind": {"type": "string"},
                               "version": {"type": "string" }
                           }
                       }
                   }
                },
                {
                   "properties": {
                       "server": {
                           "type": "object",
                           "properties": {
                               "accessNum": {"type": "integer"},
                               "version": {"type": "string"}
                           }
                       }
                   }
                }
            ]
        }
    }
}

```

We can see that `request` is an object with two options as indicated by the choices under `anyOf`:
* contains property `kind`, which has properties `kind` and `version`
* contains property `server`, which has properties `accessNum` and `version`

The type checker finds the first error in the Rego code, suggesting that `servers` should be either `kind` or `server`.
```
	input.request.servers.versions
	              ^
	              have: "servers"
	              want (one of): ["kind" "server"]
```
Once this is fixed, the second typo is highlighted, prompting the user to choose between `accessNum` and `version`.
```
	input.request.server.versions
	                     ^
	                     have: "versions"
	                     want (one of): ["accessNum" "version"]
```

#### `allOf`

Specifically, `allOf` keyword implies that all conditions under `allOf` within a schema must be met by the given data. `allOf` is implemented through merging the types from all of the JSON subSchemas listed under `allOf` before parsing the result to convert it to a Rego type. Merging of the JSON subSchemas essentially combines the passed in subSchemas based on what types they contain. Consider the following Rego and schema file containing `allOf`:

`policy-allOf.rego`
```
package kubernetes.admission

# METADATA
# scope: rule
# schemas:
#   - input: schema["input-allof"]
deny {
    input.request.servers.versions == "Pod"
}
```

`input-allof.json`
```
{
    "$schema": "http://json-schema.org/draft-07/schema",
    "type": "object",
    "properties": {
        "kind": {"type": "string"},
        "request": {
            "type": "object",
            "allOf": [
                {
                   "properties": {
                       "kind": {
                           "type": "object",
                           "properties": {
                               "kind": {"type": "string"},
                               "version": {"type": "string" }
                           }
                       }
                   }
                },
                {
                   "properties": {
                       "server": {
                           "type": "object",
                           "properties": {
                               "accessNum": {"type": "integer"},
                               "version": {"type": "string"}
                           }
                       }
                   }
                }
            ]
        }
    }
}

```

We can see that `request` is an object with properties as indicated by the elements listed under `allOf`:
* contains property `kind`, which has properties `kind` and `version`
* contains property `server`, which has properties `accessNum` and `version`

The type checker finds the first error in the Rego code, suggesting that `servers` should be `server`.
```
	input.request.servers.versions
	              ^
	              have: "servers"
	              want (one of): ["kind" "server"]
```
Once this is fixed, the second typo is highlighted, informing the user that `versions` should be one of `accessNum` or `version`.
```
	input.request.server.versions
	                     ^
	                     have: "versions"
	                     want (one of): ["accessNum" "version"]
```
Because the properties `kind`, `version`, and `accessNum` are all under the `allOf` keyword, the resulting schema that the given data must be validated against will contain the types contained in these properties children (string and integer).

### Remote references in JSON schemas

It is valid for JSON schemas to reference other JSON schemas via URLs, like this:
```json
{
  "description": "Pod is a collection of containers that can run on a host.",
  "type": "object",
  "properties": {
    "metadata": {
      "$ref": "https://kubernetesjsonschema.dev/v1.14.0/_definitions.json#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta",
      "description": "Standard object's metadata. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata"
    }
  }
}
```

OPA's type checker will fetch these remote references by default.
To control the remote hosts schemas will be fetched from, pass a capabilities
file to your `opa eval` or `opa check` call.

Starting from the capabilities.json of your OPA version (which can be found [in the
repository](https://github.com/open-policy-agent/opa/tree/main/capabilities)), add
an `allow_net` key to it: its values are the IP addresses or host names that OPA is
supposed to connect to for retrieving remote schemas.

```json
{
  "builtins": [ ... ],
  "allow_net": [ "kubernetesjsonschema.dev" ]
}
```

#### Note

- To forbid all network access in schema checking, set `allow_net` to `[]`
- Host names are checked against the list as-is, so adding `127.0.0.1` to `allow_net`,
  and referencing a schema from `http://localhost/` will _fail_.
- Metaschemas for different JSON Schema draft versions are not subject to this
  constraint, as they are already provided by OPA's schema checker without requiring
  network access. These are:

    - `http://json-schema.org/draft-04/schema`
    - `http://json-schema.org/draft-06/schema`
    - `http://json-schema.org/draft-07/schema`


## Limitations

Currently this feature admits schemas written in JSON Schema but does not support every feature available in this format.
In particular the following features are not yet supported:

* additional properties for objects
* pattern properties for objects
* additional items for arrays
* contains for arrays
* oneOf, not
* enum
* if/then/else

A note of caution: overriding is a powerful capability that must be used carefully. For example, the user is allowed to write:

```
# METADATA
# scope: rule
# schema:
#  - data: schema["some-schema"]
```

In this case, we are overriding the root of all documents to have some schema. Since all Rego code lives under `data` as virtual documents, this in practice renders all of them inaccessible (resulting in type errors). Similarly, assigning a schema to a package name is not a good idea and can cause problems. Care must also be taken when defining overrides so that the transformation of schemas is sensible and data can be validated against the transformed schema.

## References

For more examples, please see https://github.com/aavarghese/opa-schema-examples

This contains samples for Envoy, Kubernetes, and Terraform including corresponding JSON Schemas.

For a reference on JSON Schema please see: http://json-schema.org/understanding-json-schema/reference/index.html

For a tool that generates JSON Schema from JSON samples, please see: https://jsonschema.net/home