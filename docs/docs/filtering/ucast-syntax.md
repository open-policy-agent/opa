---
title: UCAST Syntax
description: UCAST Syntax
sidebar_position: 5
---

# UCAST Syntax

The data filtering support makes use of the [Universal Conditions AST (UCAST)](https://github.com/stalniy/ucast) project to represent (as JSON) a universal set of conditions that can be applied to filter data.

UCAST allows 3 types of nodes in the tree:

- **Document-level Condition** nodes: Used to apply an operator to the entire document/table, e.g. the EXISTS operator in SQL. These types of nodes are _not used_ by any of our interpreters.
- **Compound Condition** nodes: Used to apply an operator across N-many child nodes.
- **Field Condition** nodes: Used to apply an operator to a field and an optional value.

## Expanded Syntax

```python
START := EXPRS

EXPRS := COMPOUND_EXPR | FIELD_EXPR

COMPOUND_EXPR := {"type": "compound", "operator": COMPOUND_OP_NAME, "value": [EXPRS]}
COMPOUND_OP_NAME := "and" | "or" | "not"

FIELD_EXPR := {"type": "field", "field": FIELD_NAME, "operator": FIELD_OP_NAME, "value": VALUE}
FIELD_NAME := <string>

VALUE := <string> | <number> | ... | {"field": COMPARED_FIELD_NAME}
COMPARED_FIELD_NAME := <string>
```

The `FIELD_NAME` corresponds to the field being referenced in the database. `COMPARED_FIELD_NAME` is used when comparing one column to another column.

<details>
    <summary>Formal JSON Schema</summary>
```json
{
    "$schema": "https://json-schema.org/draft/2020-12/schema",
    "title": "UCAST expanded syntax",
    "description": "UCAST expanded syntax that is compatible with EOPA supported integrations",
    "$defs": {
        "compoundExpression": {
            "type": "object",
            "properties": {
                "type": { "const": "compound" },
                "operator": {
                    "anyOf": [
                        { "const": "and" },
                        { "const": "or" },
                        { "const": "not" }
                    ]
                },
                "value": {
                    "type": "array",
                    "items": { "$ref": "#" }
                }
            },
            "required": ["type", "operator", "value"]
        },
        "fieldExpression": {
            "type": "object",
            "properties": {
                "type": { "const": "field" },
                "field": { "type": "string" },
                "operator": { "enum": ["eq", "ne", "lt", "lte", "gt", "gte", "in", "nin", "contains", "startswith", "endswith"] }
            },
            "required": ["type", "field", "operator", "value"],
            "allOf": [
                {
                    "if": { "properties": { "operator": { "const": "eq" }} },
                    "then": { "properties": { "value": { "type": ["string", "number", "boolean", "null"] }}}
                },
                {
                    "if": { "properties": { "operator": { "const": "ne" }} },
                    "then": { "properties": { "value": { "type": ["string", "number", "boolean", "null"] }}}
                },
                {
                    "if": { "properties": { "operator": { "const": "lt" }} },
                    "then": { "properties": { "value": { "type": "number" }}}
                },
                {
                    "if": { "properties": { "operator": { "const": "lte" }} },
                    "then": { "properties": { "value": { "type": "number" }}}
                },
                {
                    "if": { "properties": { "operator": { "const": "gt" }} },
                    "then": { "properties": { "value": { "type": "number" }}}
                },
                {
                    "if": { "properties": { "operator": { "const": "gte" }} },
                    "then": { "properties": { "value": { "type": "number" }}}
                },
                {
                    "if": { "properties": { "operator": { "const": "in" }} },
                    "then": { "properties": { "value": { "type": ["array"] }}}
                },
                {
                    "if": { "properties": { "operator": { "const": "nin" }} },
                    "then": { "properties": { "value": { "type": ["array"] }}}
                },
                {
                    "if": { "properties": { "operator": { "const": "contains" }} },
                    "then": { "properties": { "value": { "type": ["string", "number", "boolean"] }}}
                },
                {
                    "if": { "properties": { "operator": { "const": "startswith" }} },
                    "then": { "properties": { "value": { "type": "string" }}}
                },
                {
                    "if": { "properties": { "operator": { "const": "endswith" }} },
                    "then": { "properties": { "value": { "type": "string" }}}
                }
            ]
        }
    },
    "type": "object",
    "anyOf": [
        { "$ref": "#/$defs/compoundExpression" },
        { "$ref": "#/$defs/fieldExpression" }
    ]
}
```
</details>

## Concise syntax

UCAST supports an abbreviated "concise" syntax, which allows leaving out the often redundant "type" and "operation" fields from the nodes. It uses some implicit construction rules to make common collections of conditions easier to write.

Two assumptions made in the concise format:

- The default compound operation is **and**.
- The default field operation is **eq**.

```python
START := IMPLICIT_AND

EXPRS := IMPLICIT_AND | COMPOUND_EXPR | FIELD_EXPR

IMPLICIT_AND := '{' (FIELD_EXPR | COMPOUND_EXPR)... '}'

FIELD_EXPR := (FIELD_NAME ':' VALUE) | (FIELD_NAME ':' '{' FIELD_OP_NAME ':' VALUE '}')
FIELD_NAME := <string>
VALUE := <string> | <number> | ... | {"field": COMPARED_FIELD_NAME}
COMPARED_FIELD_NAME := <string>

COMPOUND_EXPR := COMPOUND_OP_NAME ':' [EXPRS...]
COMPOUND_OP_NAME := 'and' | 'or' | 'not'
```

The `FIELD_NAME` corresponds to the field being referenced in the database.

## Compound Operations

The `not` operation is not generally supported by all clients.

| Operation | `COMPOUND_OP_NAME` | Supported Types      | [`@open-policy-agent/ucast-prisma`](https://github.com/open-policy-agent/opa-typescript/tree/main/packages/ucast-prisma) | [`OpenPolicyAgent.Ucast.Linq`](https://github.com/open-policy-agent/ucast-linq) |
| --------- | ------------------ | -------------------- | ------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------- |
| And       | `and`              | `array`              | :white_check_mark:                                                                                                       | :white_check_mark:                                                              |
| Or        | `or`               | `array`              | :white_check_mark:                                                                                                       | :white_check_mark:                                                              |
| Not       | `not`              | `array` with 1 entry | :white_check_mark:                                                                                                       | :x:                                                                             |

## Field Operations

Not all **field operations** are supported by every database integration. The following is a (non-comprehensive) compatibility matrix for the EOPA supported UCAST interpreters.

| Operation              | `FIELD_OP_NAME` | Supported `VALUE` Types               | [`@open-policy-agent/ucast-prisma`](https://github.com/open-policy-agent/opa-typescript/tree/main/packages/ucast-prisma) | [`OpenPolicyAgent.Ucast.Linq`](https://github.com/open-policy-agent/ucast-linq) |
| ---------------------- | --------------- | ------------------------------------- | ------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------- |
| Equals                 | `eq`            | `string`, `number`, `boolean`, `null` | :white_check_mark:                                                                                                       | :white_check_mark:                                                              |
| Not Equals             | `ne`            | `string`, `number`, `boolean`, `null` | :white_check_mark:                                                                                                       | :white_check_mark:                                                              |
| Less Than              | `lt`            | `number`                              | :white_check_mark:                                                                                                       | :white_check_mark:                                                              |
| Less Than or Equals    | `lte`           | `number`                              | :white_check_mark:                                                                                                       | :white_check_mark:                                                              |
| Greater Than           | `gt`            | `number`                              | :white_check_mark:                                                                                                       | :white_check_mark:                                                              |
| Greater Than or Equals | `gte`           | `number`                              | :white_check_mark:                                                                                                       | :white_check_mark:                                                              |
| In                     | `in`            | `array`                               | :white_check_mark:                                                                                                       | :white_check_mark:                                                              |
| Not In                 | `nin`           | `array`                               | :white_check_mark:                                                                                                       | :white_check_mark:                                                              |
| Contains               | `contains`      | `string`, `number`, `boolean`         | :white_check_mark:                                                                                                       | :x:                                                                             |
| Starts With            | `startswith`    | `string`                              | :white_check_mark:                                                                                                       | :x:                                                                             |
| Ends With              | `endswith`      | `string`                              | :white_check_mark:                                                                                                       | :x:                                                                             |

## Examples

### Simple: Product Prices

Use case: Show products with a price less than or equal to $500

Expanded Format:

```json
{
  "type": "field",
  "field": "products.price",
  "operator": "lte",
  "value": 500
}
```

Concise Format:

```json
{
  "products.price": {
    "lte": 500
  }
}
```

### Compound: Support Tickets

Use Case: Show tickets that are assigned to "Alice Zimmerman" and have a severity of 1 or 2.

Expanded Format:

```json
{
  "type": "compound",
  "operator": "and",
  "value": [
    {
      "type": "field",
      "field": "tickets.assignee",
      "operator": "eq",
      "value": "Alice Zimmerman"
    },
    {
      "type": "field",
      "field": "tickets.severity",
      "operator": "in",
      "value": [1, 2]
    }
  ]
}
```

Concise Format:

```json
{
  "tickets.assignee": "Alice Zimmerman",
  "tickets.severity": { "in": [1, 2] }
}
```

### Nots and Column Comparisons: Support Tickets

Use Case: Show tickets where the assignee is not the resolver.

Expanded Format:

```json
{
  "type": "compound",
  "operator": "not",
  "value": [
    {
      "type": "field",
      "field": "tickets.assignee",
      "operator": "eq",
      "value": { "field": "tickets.resolver" }
    }
  ]
}
```

Concise Format:

```json
{
  "not": [{ "tickets.assignee": { "field": "tickets.resolver" } }]
}
```
