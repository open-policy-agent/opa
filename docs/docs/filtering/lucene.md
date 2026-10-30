---
title: Lucene (Solr) Filters
description: Translating data filtering policies into Lucene queries for Solr
sidebar_position: 5.5
---

The Lucene target (`Accept: application/vnd.opa.lucene.solr+json`) renders the filter as a query for Solr's [standard query parser](https://solr.apache.org/guide/solr/latest/query-guide/standard-query-parser.html), to be used as `fq` or `q`.

```rego
# METADATA
# scope: package
# compile:
#   unknowns: [input.doc]
package filters

include if {
    input.doc.owner == input.user
    input.doc.level >= 3
    not input.doc.status == "draft"
}
```

For the input `{"user": "alice"}`, the response is

```json
{
  "result": {
    "query": "(owner:\"alice\" AND level:[3 TO *] AND NOT status:\"draft\")"
  }
}
```

- The one unknown is the document, and its fields are the references below it: `input.doc.owner` is the field `owner`, `input.doc.address.city` the field `address.city`.
  More than one document is an error, as a query searches one index. `import input.doc` lets the policy write `doc.owner`.
- A reference without the document, like `input.owner`, has to be declared a field of it: add it to the unknowns, and map it with `{"lucene": {"owner": {"$table": "doc"}}}`.
- `targetSQLTableMappings` (per-target key `lucene`) renames fields: `{"doc": {"owner": "owner_id"}}`, or `{"doc": {"address.city": "town"}}`.
- An unconditional "yes" is `*:*`; for an unconditional "no", the response has no `result`.
- Whether a field is [single-valued or multi-valued](#single-valued-and-multi-valued-fields) can be declared with a schema.
- The target is tested against Solr; other engines with Lucene syntax may differ in details.

## Translation

Each body of a rule, or each rule of the same name, becomes a conjunction, and they are joined by `OR`:

| Rego                                                           | Lucene target                   |
| -------------------------------------------------------------- | ------------------------------- |
| `input.doc.name == "a"` and `input.doc.level > 1` in one body  | `(name:"a" AND level:{1 TO *})` |
| `input.doc.name == "a"` and `input.doc.level > 1` as two rules | `(name:"a" OR level:{1 TO *})`  |
| a single condition                                             | `name:"a"`, without parentheses |

A purely negative conjunction is anchored with the match-all `*:*`, and negative conditions come after the positive ones:

| Rego                                                            | Lucene target                               |
| --------------------------------------------------------------- | ------------------------------------------- |
| `not input.doc.status == "x"`                                   | `(*:* AND NOT status:"x")`                  |
| `input.doc.name == "a"`, then `not input.doc.status == "x"`     | `(name:"a" AND NOT status:"x")`             |
| `not input.doc.status == "x"`, then `not input.doc.name == "a"` | `(*:* AND NOT status:"x" AND NOT name:"a")` |

**Conditions.** The field is on the left, whichever side the unknown is on: `3 <= input.doc.level` is `level:[3 TO *]`.

| Rego                                                     | Lucene target                           |
| -------------------------------------------------------- | --------------------------------------- |
| `input.doc.name == "x"`                                  | `name:"x"`                              |
| `input.doc.level == 3`                                   | `level:3`                               |
| `input.doc.active == true`                               | `active:true`                           |
| `input.doc.name != "x"`                                  | `(name:[* TO *] AND NOT name:"x")`      |
| `input.doc.level < 3`                                    | `level:{* TO 3}`                        |
| `input.doc.level <= 3`                                   | `level:[* TO 3]`                        |
| `input.doc.level > 3`                                    | `level:{3 TO *}`                        |
| `input.doc.level >= 3`                                   | `level:[3 TO *]`                        |
| `input.doc.created >= "2024-01-01T00:00:00Z"`            | `created:["2024-01-01T00:00:00Z" TO *]` |
| `input.doc.name in {"a", "b"}`                           | `name:("a" OR "b")`                     |
| `input.doc.name != null`, `input.doc.name = _`           | `name:[* TO *]`                         |
| `not input.doc.name != null`                             | `(*:* AND NOT name:[* TO *])`           |
| `not input.doc.name == "x"`                              | `(*:* AND NOT name:"x")`                |
| `not input.doc.name in {"a", "b"}`                       | `(*:* AND NOT name:("a" OR "b"))`       |
| `startswith(input.doc.name, "ap")`                       | `name:ap*`                              |
| `endswith(input.doc.name, "le")`                         | `name:*le`                              |
| `contains(input.doc.name, "pl")`                         | `name:*pl*`                             |
| `strings.any_prefix_match(input.doc.name, ["ap", "ch"])` | `name:(ap* OR ch*)`                     |
| `strings.any_suffix_match(input.doc.name, ["le", "ry"])` | `name:(*le OR *ry)`                     |

- `{` and `}` exclude a bound, `[` and `]` include it. Only numbers and strings can be bounds.
- As in Rego, `input.doc.name != "x"` does not hold for a document without `name`, and `not input.doc.name == "x"` does.
- Solr leaves a field without a value out of the document, so `input.doc.name == null` is rejected: it never holds. `!= null` means "has a value", and `not input.doc.name != null` "has none".
- Strings are quoted, with `\` and `"` escaped, so a `*`, `:` or `OR` in a value from the input is literal.
  A wildcard pattern can't be quoted, so in the pattern of `startswith` and the like, every character but letters and digits gets a `\`: `startswith(input.doc.name, "a b")` is `name:a\ b*`.
  On a `string` field, patterns match case-sensitively, as in Rego. A leading wildcard, as in `endswith` and `contains`, is slow on a large index.
- A field name may contain letters, digits, `_`, `-` and `.`, and starts with a letter or `_`; anything else is an error.

**What can't be translated** is rejected with an HTTP 400 and a `pe_fragment_error`: comparisons between two unknowns, other built-ins on unknowns, `with`, rules left unresolved by partial evaluation, and the [multi-valued forms](#single-valued-and-multi-valued-fields) listed there.
So is a bare reference, as in `include if input.doc.active`; write `input.doc.active != false`, which is what it means in Rego, or `input.doc.active == true`.

## Differences from Rego

Solr matches values according to the field's type in its schema, which OPA does not see. The filter agrees with the policy only if the schema does:

- **Use unanalyzed fields**, like `string`, numeric or date fields. On an analyzed text field, `name:"apple"` also matches `"Big Apple"`.
- **Use values of the field's type.** In Rego, `1 == "1"` is false; in Solr, `price:1` on a `string` field matches `"1"`, and `active:true` matches `"true"`.
- **Ranges** follow the field's type: numeric for numbers, lexicographic for strings and dates. Rego orders a number and a string by type.
- **Missing fields.** A `not` holds for a document without the field, as in Rego with the improved `not` (`import future.keywords.not`, or `rego.v2`), which the Compile API does not accept yet. With the default semantics, Rego fails some of these, like `not input.doc.level < 2`.
- **Objects** have no Lucene counterpart; flatten them into fields.

## Single-valued and multi-valued fields

A Solr field is single-valued, returned as a value (`"owner": "alice"`), or `multiValued`, returned as an array (`"tags": ["a", "b"]`).
The policy compares a single-valued field as it is, `input.doc.owner == "alice"`, and a multi-valued one by its elements, `"a" in input.doc.tags`.
Lucene matches both alike: `tags:"a"` means "some value is `a`".

Declare which a field is with [`schemas` annotations](../policy-language#metadata-schemas) in scope of the compiled rule:

```rego
# METADATA
# scope: package
# compile:
#   unknowns: [input.doc]
# schemas:
#   - input.doc.owner: {"type": "string"}
#   - input.doc.tags: {"type": "array", "items": {"type": "string"}}
#   - input.doc.address.city: {"type": "string"}
package filters
```

A field of type `array` is multi-valued, one of a scalar type single-valued. A schema for the whole document, `input.doc` or `input`, works too, with nested fields as `object` properties.
Using a field the other way is an error, like `"alice" in input.doc.owner` or `input.doc.tags == "a"`; the type checker catches some of these when the policy is loaded, and in `opa test`.
Only inline schemas are used.
Without a schema, a field has the shape the policy uses it with, and using one both ways is an error. Test the policy with documents shaped as Solr returns them.

[`solr_schema.rego`](/filtering/solr_schema.rego) converts the response of Solr's Schema API into these annotations, for `input.doc`:

```shell
curl -s http://localhost:8983/solr/<core>/schema > solr.json
opa eval -f raw -d solr_schema.rego -i solr.json data.solr.annotation
```

A `multiValued` field, or one of a `multiValued` type, becomes an array, and a name with dots, like `origin.country`, is the reference `input.doc.origin.country`. Dynamic fields, and types without a JSON counterpart, are left out.

**Multi-valued fields** can be compared like this:

| Rego                                                                                                                           | Lucene target                                                                     |
| ------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------- |
| `"a" in input.doc.tags`                                                                                                        | `tags:"a"`                                                                        |
| `some t in input.doc.tags; t == "a"`, `input.doc.tags[_] == "a"`                                                               | `tags:"a"`                                                                        |
| `some t in input.doc.tags; t in {"a", "b"}`                                                                                    | `tags:("a" OR "b")`                                                               |
| `some l in input.doc.levels; l >= 3`                                                                                           | `levels:[3 TO *]`                                                                 |
| `input.doc.tags == []`, `count(input.doc.tags) == 0`                                                                           | `(*:* AND NOT tags:[* TO *])`                                                     |
| `input.doc.tags != []`                                                                                                         | `tags:[* TO *]`                                                                   |
| `every t in input.doc.tags { t != "a" }`                                                                                       | `(*:* AND NOT tags:"a")`                                                          |
| `every t in input.doc.tags { not t in {"a", "b"} }`                                                                            | `(*:* AND NOT tags:("a" OR "b"))`                                                 |
| `some t in input.doc.tags; startswith(t, "a")`                                                                                 | `tags:a*`                                                                         |
| `strings.any_prefix_match(input.doc.tags, ["a", "b"])`, or `some t in input.doc.tags; strings.any_prefix_match(t, ["a", "b"])` | `tags:(a* OR b*)`                                                                 |
| `every t in input.doc.tags { not startswith(t, "a") }`                                                                         | `(*:* AND NOT tags:a*)`                                                           |
| `every l in input.doc.levels { not l < 3 }`                                                                                    | `(*:* AND NOT levels:{* TO 3})`                                                   |
| `every t in input.doc.tags { t == "a" }`                                                                                       | `(*:* AND NOT tags:({* TO "a"} OR {"a" TO *}))`                                   |
| `every t in input.doc.tags { t in {"a", "b"} }`                                                                                | `(*:* AND NOT tags:({* TO "a"} OR {"a" TO "b"} OR {"b" TO *}))`                   |
| `{t \| some t in input.doc.tags} == {"a", "b"}`                                                                                | `(tags:"a" AND tags:"b" AND NOT tags:({* TO "a"} OR {"a" TO "b"} OR {"b" TO *}))` |
| `every g in input.groups { not g in input.doc.tags }`, for the known `input.groups`                                            | `(*:* AND NOT tags:"g1" AND NOT tags:"g2")`                                       |

- "All values are" and "the values are exactly" exclude the ranges around the given values, so they need values that are all strings, or all numbers, on a field that orders them as Rego does.
- `{t | some t in input.doc.tags} == {"a", "b"}` also holds for `["b", "a", "a"]`. `input.doc.tags == {"a", "b"}` never holds: an array is not a set.
- Solr does not index an empty array, so to Solr, `"tags": []` is no `tags` at all. In Rego, `input.doc.tags == []`, `count(input.doc.tags) == 0` and `every` hold for the empty array, but not for a missing field; the filter selects both.

These are rejected, as Lucene can't say them:

- Comparing the whole array, as `input.doc.tags == "a"`, `input.doc.tags == ["a", "b"]`, `input.doc.tags in {"a"}`, or a bare `input.doc.tags`, which is always true.
- "Some value differs", `some t in input.doc.tags; t != "a"`, and "all values are above", `every t in input.doc.tags { t > "a" }` or `{ startswith(t, "a") }`.
- Two conditions on one element, `some l in input.doc.levels; l > 1; l < 3`, as each could be met by a different value. Two elements, `some l ...; l > 1` and `some m ...; m < 3`, are fine.
- A count other than zero, `count(input.doc.tags) == 2`.
- Elements tied by an index, like `tags[i]` and `roles[i]`.
- An `every` over a field with more than one comparison of the element in its body.
- Comparing an element with `null`: a Lucene field has no null value.
