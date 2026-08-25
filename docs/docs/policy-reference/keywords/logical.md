---
sidebar_label: and, or
title: 'Rego Keywords: and, or'
---

The `and`/`or` keywords are logical operators whose evaluation either succeeds or fails, and [neither produces a value](#expressions-not-values);
i.e. they don't result in a `true` or `false` value that can be bound to a variable or be used as a function argument.

<PlaygroundExample dir={require.context('./_examples/logical/overview')} />

:::note
If you're looking for the built-in functions for set intersection and union,
see the built-ins section for [intersection (&)](/docs/policy-reference/builtins/sets#builtin-sets-and) and [union (|)](/docs/policy-reference/builtins/sets#builtin-sets-or), respectively.
:::

## Enabling

The `and`/`or` keywords are not part of the standard v0 and v1 Rego syntax, and must be [imported](./import#importing-future-keywords) to be enabled:

* and: `import future.keywords.and`
* or: `import future.keywords.or`
* both, along with every other future keyword: `import future.keywords`

Importing `rego.v1` does _not_ enable them.

## or

The `or` keyword represents a logical disjunction in Rego. An `or` expression:

* is an infix operator with a left-hand and a right-hand operand: `<LHS> or <RHS>`
* is `defined` if _either_ operand succeeds; an operand fails when it is `undefined` _or_ evaluates to `false`
* [produces no value](#expressions-not-values) — only `defined` or `undefined`
* [cannot bind variables](#variables-and-scope) that are visible outside the expression

```rego
package example

import future.keywords.or

p if {
    input.a or input.b
}
```

### or vs. multiple rules

In Rego, disjunction can be expressed through [incremental rules](../../policy-language#incremental-definitions).
The `or` operator is _not_ a drop-in replacement for incremental rules.
`or` operands [cannot bind variables](#variables-and-scope) visible outside the expression, making `or` expressions only suitable for control flow.

Prefer expressing disjunction with `or` when:

* The disjunction is deeply embedded inside a rule and extracting it into a separate rule hurts readability
* Creating small one-line rules where the expression doesn't need to bind a value: `p if x or y`

Prefer expressing disjunction with incremental rules when:

* The disjunction must result in a value, and not only control flow
* The disjunction is complex and involves multiple conditions
* The disjunction can be re-used across your policy

## and

The `and` keyword represents a logical conjunction in Rego. An `and` expression:

* is an infix operator with a left-hand and a right-hand operand: `<LHS> and <RHS>`
* is `defined` only if _both_ operands succeed; an operand fails when it is `undefined` _or_ evaluates to `false`
* [produces no value](#expressions-not-values) — only `defined` or `undefined`
* [cannot bind variables](#variables-and-scope) that are visible outside the expression

```rego
package example

import future.keywords.and

p if {
    input.a and input.b
}
```

### and vs. multiple expressions

In Rego, rules are conjunctive operations where all expressions in the rule's body are AND-ed together; if all expressions
succeed, the rule succeeds. The `and` operator is _not_ a drop-in replacement for conjunctive rule expressions.
`and` operands [cannot bind variables](#variables-and-scope) visible outside the expression, making `and` expressions only suitable for control flow.

Prefer expressing conjunction with `and` when:

* Creating composite logical expressions together with [or](#or): `x or y and z`
* Creating small one-line rules where the expression doesn't need to bind a value: `p if x and y`

Prefer expressing conjunction with multiple expressions within a rule body when:

* The conjunction must result in a value, and not only control flow
* The conjunction is not part of a disjunctive expression ([or](#or))
* The conjunction can be re-used across your policy

## Expressions, not values

`and` and `or` are control-flow operators that do not return a value. When evaluated, the expression is either `defined` (successfully evaluated)
or `undefined` (failed to evaluate). Their operands are ordinary rule-body expressions, so an operand fails either by being
`undefined` or by evaluating to `false` — `false` is a value like any other, but an expression evaluating to it does not succeed.

Because there is no value, an `and`/`or` expression cannot appear anywhere a value is expected:

```
p := a or b                # an assigned value
f(a or b)                  # a function argument
[a or b | some x in xs]    # a comprehension head
every x in (a or b) { x }  # an every domain
```

## No evaluation branching

On evaluation, `and` and `or` expressions do not branch evaluation for the outer scope.
Meaning: even when both operands of an `or` expression are `defined`, the expression as a whole yields a single result.

```rego
package example

import future.keywords.or

s := {"do", "re", "mi"}

# p is assigned an array comprehension
# Even though both operands of the or-expression succeed, evaluation is not branching and no duplicates are added to the array
p := [x |
    some x in s
    count(x) > 1 or count(x) < 3 # all entries in s satisfy both the left-hand and right-hand sides
]
```

### Short-circuit evaluation

`and` and `or` expressions are short-circuited: if the evaluation result of one operand makes evaluation of the second redundant, it won't be evaluated.

| Expression              | Left-hand evaluated | Right-hand evaluated |
|-------------------------|---------------------|----------------------|
| `<succeeds> and <any>`  | :white_check_mark:  | :white_check_mark:   |
| `<fails> and <any>`     | :white_check_mark:  | :x:                  |
| `<succeeds> or <any>`   | :white_check_mark:  | :x:                  |
| `<fails> or <any>`      | :white_check_mark:  | :white_check_mark:   |

## Precedence and grouping

`and` and `or` bind more loosely than the expressions they combine: anything that isn't
`not`, `and`, `or`, or `with` is folded into the nearest operand.

Tightest to loosest binding:

1. any other expression — arithmetic, comparison, `in`, function calls, references
2. `not`
3. `and`
4. `or`
5. [`with`](#using-with) — applies to the whole expression unless scoped to an operand

Use parentheses to override the default binding:

| Expression                 | Reads as                                                             |
|----------------------------|----------------------------------------------------------------------|
| `1 + 1 == 2 and x in s`    | `(1 + 1 == 2) and (x in s)`                                          |
| `not a and b`              | `(not a) and b` (only the operand is negated)                        |
| `not (a and b)`            | `not (a and b)` (the whole conjunction is negated)                   |
| `a and b or c`             | `(a and b) or c`                                                     |
| `a or b and c`             | `a or (b and c)`                                                     |
| `(a or b) and c`           | `(a or b) and c` (the `or` binds first)                              |
| `a or b or c`              | `(a or b) or c` (left-associative)                                   |
| `a or b with input as x`   | `(a or b) with input as x` (`with` applies to the whole disjunction) |
| `a or (b with input as x)` | `a or (b with input as x)` (`with` applies to `b` only)              |

To negate a whole expression, group it with `not (...)`. This requires
`import future.keywords.not` in addition to the `and`/`or` imports; without it the
parentheses are read as an ordinary grouped expression, which cannot contain `and`
or `or`:

```
not (input.a or input.b)
# rego_parse_error: non-terminated expression
```

## Explicit bodies

By default, an operand is a single expression. Wrapping an operand in braces (`{}`) makes it an
_explicit body_: a sequence of expressions that must all succeed evaluation, just like a rule body.

Use an explicit body when an operand needs logic that a single expression can't express:
iteration, intermediate values, or several conditions that belong together. Any variables
it declares are [local to the body](#variables-and-scope).

```rego
package example

import future.keywords.or

items := [1, 2, 3]

p if {
    { some x in items; x > 2 } or input.b
}
```

### Variables and scope

Operands read variables from the enclosing rule body freely, but only an explicit body may
bind new ones. Variables bound in an explicit body are local to it, and not visible outside.

| Expression            | Result                                                                                                    |
|-----------------------|-----------------------------------------------------------------------------------------------------------|
| `x := 1 or input.b`   | `rego_compile_error: cannot assign vars inside implicit or operand`                                       |
| `x = 1 or input.b`    | `rego_unsafe_var_error: var x is unsafe`                                                                  |
| `{x := 1} or input.b` | allowed; `x` is not visible outside the braces                                                            |
| `{x := 1} and x == 1` | `rego_unsafe_var_error: var x is unsafe` (`x` declared in the left-hand is not visible in the right-hand) |

```rego
package example

import future.keywords.or

p if {
    y := 3                        # bound in the rule body
    { x := y; x > 1 } or input.b  # y is read; x is local to the braces
}
```

## Using `with`

The [`with` keyword](../../policy-language#with-keyword) can be applied to a whole `and`/`or`
expression, or to an operand that is a group or an [explicit body](#explicit-bodies), but not
to a single-expression operand:

| Placement                     | Applies to                                                          |
|-------------------------------|---------------------------------------------------------------------|
| `a or b with input as x`      | the whole expression                                                |
| `a or (b with input as x)`    | that operand only                                                   |
| `a or {b with input as x}`    | that operand only ([explicit body](#explicit-bodies))               |
| `a with input as x or b`      | rejected — `with` may not be applied to a single-expression operand |
| `a or b with input as x or c` | rejected — a trailing `with` must come last                         |
