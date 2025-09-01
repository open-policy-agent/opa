---
sidebar_position: 2
---


# Roast (Regal's Optimized AST)

Roast is an optimized JSON format for [Rego](https://www.openpolicyagent.org/docs/latest/policy-language/) ASTs, as well
as common utilities for working with the both the Roast format and OPA's AST APIs.

Roast is used by [Regal](https://openpolicyagent.org/projects/regal), where the JSON representation of Rego's AST is used input for
static analysis [performed by Rego itself](https://www.styra.com/blog/linting-rego-with-rego/) to determine whether
policies conform to Regal's linter rules.

> [!IMPORTANT]
> This library changes frequently and provides no guarantees for what its API looks like. Depending on this library
> directly is not recommended! If you find any code here useful, feel free to use it as your own in your projects, but
> be aware that it may change at any time, and you may be better off copy-pasting whatever code you need.

## Goals

- Fast to traverse and process in Rego
- Usable without having to deal with quirks and inconsistencies
- As easy to read as the original AST JSON format

While this module provides a way to encode an `ast.Module` to an optimized JSON format, it does not provide a decoder.
In other words, there's currently no way to turn optimized AST JSON back into an `ast.Module` (or other AST types).
While this would be possible to do, there's no real need for that given our current use-case for this format, which is
to help work with the AST efficiently in Rego. Roast should not be considered a general purpose format for serializing
the Rego AST.

## Differences

The following section outlines the differences between the original AST JSON format and the Roast format.

### Compact `location` format

The perhaps most visually apparent change to the AST JSON format is how `location` attributes are represented. These
attributes are **everywhere** in the AST, so optimizing these for fast traversal has a huge impact on both the size of
the format and the speed at which it can be processed.

In the original AST, a `location` is represented as an object:

```json
{
  "file": "p.rego",
  "row": 5,
  "col": 1,
  "text": "Y29sbGVjdGlvbg=="
}
```

And in the optimized format as a string:

```json
"5:1:5:11"
```

The first two numbers are present in both formats, i.e. `row` and `col`. In the optimized format, the third and fourth
number is the end location, determined by the length of the `text` attribute decoded. In this case `Y29sbGVjdGlvbg==`
decodes to `collection`, which is 10 characters long. The end location is therefore `5:11`. The text can later be
retrieved when needed using the original source document as a lookup table of sorts.

While this may come with a small cost for when the `location` is actually needed, it's a huge win for when it's not.
Having to `split` the result and parse the row and column values when needed occurs some overhead, but only a small
percentage of `location` attributes are commonly used in practice.

Note that the `file` attribute is omitted entirely in the optimized format, as this would otherwise have to be repeated
for each `location` value. This can easily be retrieved by other means.

### "Empty" rule and `else` bodies

Rego rules don't necessarily have a body, or at least not one that's printed. Examples of this include:

```rego
package policy

default rule := "value"

map["key"] := "value"

collection contains "value"
```

OPA represents such rules internally (that is, in the AST) as having a body with a single expression containing the
boolean value `true`. This creates a uniform way to represent rules, so a rule like:

```rego
collection contains "value"
```

Would in the AST be identical to:

```rego
collection contains "value" if {
    true
}
```

And in the OPA JSON AST format:

```json
{
  "body": [
    {
      "index": 0,
      "location": {
        "file": "p.rego",
        "row": 5,
        "col": 1,
        "text": "Y29sbGVjdGlvbg=="
      },
      "terms": {
        "location": {
          "file": "p.rego",
          "row": 5,
          "col": 1,
          "text": "Y29sbGVjdGlvbg=="
        },
        "type": "boolean",
        "value": true
      }
    }
  ],
  "head": {
    "name": "collection",
    "key": {
      "location": {
        "file": "p.rego",
        "row": 5,
        "col": 21,
        "text": "InZhbHVlIg=="
      },
      "type": "string",
      "value": "value"
    },
    "ref": [
      {
        "location": {
          "file": "p.rego",
          "row": 5,
          "col": 1,
          "text": "Y29sbGVjdGlvbg=="
        },
        "type": "var",
        "value": "collection"
      }
    ],
    "location": {
      "file": "p.rego",
      "row": 5,
      "col": 1,
      "text": "Y29sbGVjdGlvbiBjb250YWlucyAidmFsdWUi"
    }
  },
  "location": {
    "file": "p.rego",
    "row": 5,
    "col": 1,
    "text": "Y29sbGVjdGlvbg=="
  }
}
```

Notice how there's 20 lines of JSON just to represent the body, even though there isn't really one!

The optimized Rego AST format discards generated bodies entirely, and the same rule would be represented as:

```json
{
  "head": {
    "location": "5:1:5:11",
    "ref": [
      {
        "location": "5:1:5:11",
        "type": "var",
        "value": "collection"
      }
    ],
    "key": {
      "type": "string",
      "value": "value",
      "location": "5:21:5:27"
    }
  },
  "location": "5:1:5:11"
}
```

Note that this applies equally to empty `else` bodies, which are represented the same way in the original AST, and
omitted entirely in the optimized format.

Similarly, Roast discards `location` attributes from attributes that don't have an actual location in the source code.
An example of this is the `data` term of a package path, which is present only in the AST.

### Removed `annotations` attribute from module

OPA already attaches `annotations` to rules. With the Roast format attaching `package` and `subpackages` scoped
`annotations` to the `package` as well, there is no need to store `annotations` at the module level, as that's
effectively just duplicating data. Having this removed can save a considerable amount of space in well-documented
policies, as they should be!

### Removed `index` attribute from body expressions

In the original AST, each expression in a body carries a numeric `index` attribute. While this doesn't take much space,
it is largely redundant, as the same number can be inferred from the order of the expressions in the body array. It's
therefore been removed from the Roast format.

### Removed `name` attribute from rule heads

The `name` attribute found in the OPA AST for `rules` is unreliable, as it's not always present. The `ref`
attribute however always is. While this doesn't come with any real cost in terms of AST size or performance, consistency
is key.

### Fixed inconsistencies in the original Rego AST

A few inconsistencies exist in the original AST JSON format:

- `comments` attributes having a `Text` attribute rather than the expected `text`
- `comments` attributes having a `Location` attribute rather than the expected `location`

Fixing these in the original format would be a breaking change. The Roast format corrects these inconsistencies, and
uses `text` and `location` consistently.

## Performance

While the numbers may vary some, the Roast format is currently about 40-50% smaller in size than the original AST JSON
format, and can be processed (in Rego, using `walk` and so on) about 1.25 times faster.
