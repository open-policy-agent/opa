---
title: Annotations
kind: misc
weight: 18
---

The package and individual rules in a module can be annotated with a rich set of metadata.

```live:rego/metadata:module:read_only
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
custom | mapping of arbitrary data | A custom mapping of named parameters holding arbitrary data. Read more [here](#custom).

### Scope

Annotations can be defined at the rule or package level. The `scope` annotation in
a metadata block determines how that metadata block will be applied. If the
`scope` field is omitted, it defaults to the scope for the statement that
immediately follows the annotation. The `scope` values that are currently
supported are:

* `rule` - applies to the individual rule statement (within the same file). Default, when metadata block precedes rule.
* `document` - applies to all of the rules with the same name in the same package (across multiple files)
* `package` - applies to all of the rules in the package (within the same file). Default, when metadata block precedes package.
* `subpackages` - applies to all of the rules in the package and all subpackages (recursively, across multiple files)

Since the `document` scope annotation applies to all rules with the same name in the same package 
and the `subpackages` scope annotation applies to all packages with a matching path, metadata blocks with 
these scopes are applied over all files with applicable package- and rule paths. 
As there is no ordering across files in the same package, the `document` and `subpackages` scope annotations 
can only be specified **once** per path. 
The `document` scope annotation can be applied to any rule in the set (i.e., ordering does not matter.)

#### Example

```live:rego/metadata/scope:module:read_only
# METADATA
# scope: document
# description: A set of rules that determines if x is allowed.

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

### Title

The `title` annotation is a string value giving a human-readable name to the annotation target.

#### Example

```live:rego/metadata/title:module:read_only
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

```live:rego/metadata/description:module:read_only
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

```live:rego/metadata/related_resources1:module:read_only
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

```live:rego/metadata/related_resources2:module:read_only
# METADATA
# related_resources:
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

```live:rego/metadata/authors1:module:read_only
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

```live:rego/metadata/authors2:module:read_only
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

```live:rego/metadata/organizations:module:read_only
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

```live:rego/metadata/schemas:module:read_only
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

The `custom` annotation is a mapping of user-defined data, mapping string keys to arbitrarily typed values.

#### Example

```live:rego/metadata/custom:module:read_only
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

### Rego

In the example below, you can see how to access an annotation from within a policy.

Given the input:

```live:example/metadata/1:input
{
    "number": 11,
    "subject": {
        "name": "John doe",
        "role": "customer"
    }
}
```

The following policy

```live:example/metadata/1:module
package example

# METADATA
# title: Deny invalid numbers
# description: Numbers may not be higher than 5
# custom:
#  severity: MEDIUM
output := decision {
	input.number > 5

	annotation := rego.metadata.rule()
	decision := {
		"severity": annotation.custom.severity,
		"message": annotation.description,
	}
}
```

will output

```live:example/metadata/1:output
```

If you'd like more examples and information on this, you can see more here under the [Rego](../policy-reference/#rego) policy reference.
### Inspect command

Annotations can be listed through the `inspect` command by using the `-a` flag:

```shell
opa inspect -a
```

### Go API

The ``ast.AnnotationSet`` is a collection of all ``ast.Annotations`` declared in a set of modules. 
An ``ast.AnnotationSet`` can be created from a slice of compiled modules:

```go
var modules []*ast.Module
...
as, err := ast.BuildAnnotationSet(modules)
if err != nil {
    // Handle error.
}
```

or can be retrieved from an ``ast.Compiler`` instance:

```go
var modules []*ast.Module
...
compiler := ast.NewCompiler()
compiler.Compile(modules)
as := compiler.GetAnnotationSet()
```

The ``ast.AnnotationSet`` can be flattened into a slice of ``ast.AnnotationsRef``, which is a complete, sorted list of all 
annotations, grouped by the path and location of their targeted package or -rule.

```go
flattened := as.Flatten()
for _, entry := range flattened {
    fmt.Printf("%v at %v has annotations %v\n",
        entry.Path,
        entry.Location,
        entry.Annotations)
}

// Output:
// data.foo at foo.rego:5 has annotations {"scope":"subpackages","organizations":["Acme Corp."]}
// data.foo.bar at mod:3 has annotations {"scope":"package","description":"A couple of useful rules"}
// data.foo.bar.p at mod:7 has annotations {"scope":"rule","title":"My Rule P"}
//
// For modules:
// # METADATA
// # scope: subpackages
// # organizations:
// # - Acme Corp.
// package foo
// ---
// # METADATA
// # description: A couple of useful rules
// package foo.bar
// 
// # METADATA
// # title: My Rule P
// p := 7
```

Given an ``ast.Rule``, the ``ast.AnnotationSet`` can return the chain of annotations declared for that rule, and its path ancestry.
The returned slice is ordered starting with the annotations for the rule, going outward to the farthest node with declared annotations 
in the rule's path ancestry.

```go
var rule *ast.Rule
...
chain := ast.Chain(rule)
for _, link := range chain {
    fmt.Printf("link at %v has annotations %v\n",
        link.Path,
        link.Annotations)
}

// Output:
// data.foo.bar.p at mod:7 has annotations {"scope":"rule","title":"My Rule P"}
// data.foo.bar at mod:3 has annotations {"scope":"package","description":"A couple of useful rules"}
// data.foo at foo.rego:5 has annotations {"scope":"subpackages","organizations":["Acme Corp."]}
//
// For modules:
// # METADATA
// # scope: subpackages
// # organizations:
// # - Acme Corp.
// package foo
// ---
// # METADATA
// # description: A couple of useful rules
// package foo.bar
// 
// # METADATA
// # title: My Rule P
// p := 7
```
