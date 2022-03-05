---
title: Annotations
kind: misc
weight: 18
---

The package and individual rules in a module can be annotated with a rich set of metadata.

```rego
# METADATA
# title: My rule
# description: A rule that determines if x is allowed.
# authors:
# - John Doe <john@example.com>
allow {
  ...
}
```

Annotations are grouped within a *metadata block*, and must be specified as YAML within a comment block that **must** start with `# METADATA`. 
Also, every line in the comment block containing the annotation **must** start at Column 1 in the module/file, or otherwise, they will be ignored.

{{< danger >}}
OPA will attempt to parse the YAML document in comments following the
initial `# METADATA` comment. If the YAML document cannot be parsed, OPA will
return an error. If you need to include additional comments between the
comment block and the next statement, include a blank line immediately after
the comment block containing the YAML document. This tells OPA that the
comment block containing the YAML document is finished
{{< /danger >}}

## Annotations

Name | Type | Description
--- | --- | ---
scope | string; one of `package`, `rule`, `document`, `subpackages` | The scope on which the `schemas` annotation is applied. Read more [here](./#scope).
title | string | A human-redable name for the annotation target. Read more [here](#title).
description | string | A description of the annotation target. Read more [here](#description).
related_resources | list of URLs | A list of URLs pointing to related resources/documentation. Read more [here](#related-resources).
authors | list of strings | A list of authors for the annotation target. Read more [here](#authors).
organizations | list of strings | A list of organizations related to the annotation target. Read more [here](#organizations).
schemas | list of object | A list of associations between value paths and schema definitions. Read more [here](#schemas).
custom | list of arbitrary data | A custom list of named parameters holding arbitrary data. Read more [here](#custom).

### Scope

Annotations can be defined at the rule or package level. The `scope` annotation in
a metadata block determines how that metadata block will be applied. If the
`scope` field is omitted, it defaults to the scope for the statement that
immediately follows the annotation. The `scope` values that are currently
supported are:

* `rule` - applies to the individual rule statement. Default, when metadata block precedes rule.
* `document` - applies to all of the rules with the same name in the same package
* `package` - applies to all of the rules in the package, Default, when metadata block precedes package.
* `subpackages` - applies to all of the rules in the package and all subpackages (recursively)

In case of overlap, schema annotations override each other as follows:

```
rule overrides document
document overrides package
package overrides subpackages
```

The following sections explain how the different scopes work.

#### Rule and Document Scopes

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

allow {
    access := data.acl["bob"]
    access[_] == input.operation
}
```

In the example above, the second rule does not include an annotation, so type
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
define the metadata block once on a rule with scope `document`:

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

In this example, the metadata with `document` scope has the same affect as the
two `rule` scoped metadata blocks in the previous example.

Since the `document` scope annotation applies to all rules with the same name in
the same package (which can span multiple files) and there is no ordering across
files in the same package, `document` scope annotations can only be specified
**once** per rule set. The `document` scope annotation can be applied to any
rule in the set (i.e., ordering does not matter.)

#### Package and Subpackage Scopes

Annotations can be defined at the `package` level and are then applied to all rules
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

#### Example

```rego
# METADATA
# scope: document
# schemas:
#   - input: schema.input
#   - data.acl: schema["acl-schema"]
allow {
    access := data.acl["alice"]
    access[_] == input.operation
}
```

### Title

The `title` annotation is a string value giving a human-readable name to the annotation target.

#### Example

```rego
# METADATA
# title: Allow Ones
allow {
  x == 1
}

# METADATA
# title: Allow Twos
allow {
  x == 2
}
```

### Description

The `description` annotation is a string value describing the annotation target, such as its purpose.

#### Example

```rego
# METADATA
# description: |
#  The 'allow' rule...
#  Is about allowing things.
#  Not denying them.
allow {
  ...
}
```

### Related Resources

The `related_resources` annotation is a list of *related-resource* entries, where each links to some related external resource; such as RFCs and other reading material.
A *related-resource* entry can either be an object or a short-form string holding a single URL.

#### Object Related-resource Format
When a *related-resource* entry is presented as an object, it has two fields:

* `ref`: a URL pointing to the resource (required).
* `description`: a text describing the resource.

#### String Related-resource Format
When a *related-resource* entry is presented as a string, it needs to be a valid URL.

#### Examples

```rego
# METADATA
# related_resources:
# - ref: https://example.com
# ...
# - ref: https://example.com/foo
#   description: A text describing this resource
allow {
  ...
}
```

```rego
# METADATA
# organizations:
# - https://example.com/foo
# ...
# - https://example.com/bar
allow {
  ...
}
```

### Authors

The `authors` annotation is a list of author entries, where each entry denotes an *author*. 
An *author* entry can either be an object or a short-form string.

#### Object Author Format
When an *author* entry is presented as an object, it has two fields:

* `name`: the name of the author
* `email`: the email of the author

At least one of the above fields are required for a valid `author` entry.

#### String Author Format
When an *author* entry is presented as a string, it has the format `{ name } [ "<" email ">"]`; 
where the name of the author is a sequence of whitespace-separated words. 
Optionally, the last word may represent an email, if enclosed with `<>`.

#### Examples

```rego
# METADATA
# authors:
# - name: John Doe
# ...
# - name: Jane Doe
#   email: jane@example.com
allow {
  ...
}
```

```rego
# METADATA
# authors:
# - John Doe
# ...
# - Jane Doe <jane@example.com>
allow {
  ...
}
```

### Organizations

The `organizations` annotation is a list of string values representing the organizations associated with the annotation target.

#### Example

```rego
# METADATA
# organizations:
# - Acme Corp.
# ...
# - Tyrell Corp.
allow {
  ...
}
```

### Schemas

The `schemas` annotation is a list of key value pairs, associating schemas to data values.
In-depth information on this topic can be found [here](../schemas#schema-annotations).

#### Example

```rego
# METADATA
# schemas:
#   - input: schema.input
#   - data.acl: schema["acl-schema"]
allow {
    access := data.acl["alice"]
    access[_] == input.operation
}
```

### Custom

The `custom` annotation is an associative array of user-defined data, mapping string keys to arbitrarily typed values.

#### Example

```rego
# METADATA
# custom:
#  my_int: 42
#  my_string: Some text
#  my_bool: true
#  my_list:
#   - a
#   - b
#  my_map:
#   a: 1
#   b: 2
allow {
  ...
}
```

## Accessing annotations

### Inspect command

Annotations can be listed through the `inspect` command by using the `-a` flag:

```shell
opa inspect -a
```

### Go API

The `ast.AnnotationSet` is a collection of all `ast.Annotations` declared in a set of modules. 
An `ast.AnnotationSet` can be created from a slice of compiled modules:

```go
var modules []*ast.Module
...
as, err := ast.BuildAnnotationSet(modules)
if err != nil {
    // Handle error.
}
```

or can be retrieved from an `ast.Compiler` instance:

```go
var modules []*ast.Module
...
compiler := ast.NewCompiler()
compiler.Compile(modules)
as := compiler.GetAnnotationSet()
```

The `ast.AnnotationSet` can be flattened into a slice of `ast.AnnotationsRef`, which is a complete, sorted list of all 
annotations, grouped by the path and location of their targeted package or -rule.

```go
flattened := as.Flatten()
for _, entry := range flattened {
    fmt.Printf("%v at %v has annotations %v\n",
        entry.Path,
        entry.Location,
        entry.Annotations)
}
```