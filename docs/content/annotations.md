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

The set of annotations must be specified as YAML within a comment block that **must** start with `# METADATA`. 
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
authors | list of strings | A list of authors for the annotation target. Read more [here](#authors).
custom | list of arbitrary data | A custom list of named parameters holding arbitrary data. Read more [here](#custom).
description | string | A description of the annotation target. Read more [here](#description).
organizations | list of strings | A list of organizations related to the annotation target. Read more [here](#organizations).
related_resources | list of URLs | A list of URLs pointing to related resources/documentation. Read more [here](#related-resources).
schemas | list of object | A list of associations between value paths and schema definitions. Read more [here](#schemas).
scope | string; one of `package`, `rule`, `document`, `subpackages` | The scope on which the `schemas` annotation is applied. Read more [here](./#scope).

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
# related-resources:
# - 
#  ref: https://example.com
# ...
# - 
#  ref: https://example.com/foo
#  description: A text describing this resource
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

### Scope

The `scope` annotation determines how the `schema` annotation will be applied.
If the `scope` annotation is omitted, it defaults to the scope for the statement that immediately follows the annotation.
In-depth information on this topic can be found [here](../schemas#annotation-scopes).

#### Example

```rego
# METADATA
# scope: rule
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

## Accessing annotations
