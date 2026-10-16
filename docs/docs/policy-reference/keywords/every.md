---
sidebar_label: every
title: 'Rego Keyword: every'
---

Rego rules and statements are existentially quantified by default. This means
that if there is any solution then the rule is true, or a value is bound. Some
policies require checking all elements in an array or object. The `every`
keyword makes this
[universal quantification](/docs/policy-language#universal-quantification-for-all)
easier.

The following two equivalent rules achieve universal quantification. Note how
much easier to read the one using `every` is.

```rego
package play

allow1 if {
    every e in [1, 2, 3] {
        e < 4
    }
}

# without every, don't do this!
allow2 if {
    {r | some e in [1, 2, 3]; r := e < 4} == {true}
}
```

<sub>
`allow2` works by generating a set of 'results' testing elements from the
array `[1,2,3]`. The resulting set is tested against `{true}` to verify all
elements are `true`. `every` is a much better option!
</sub>

## Partial evaluation

During [partial evaluation](../../filtering/partial-evaluation), an `every` over a _known_ collection of at most 32 elements is evaluated once per element, and the result is the conjunction of the body for each.
That is what lets the [Compile API](../../rest-api#compile-api) turn it into a filter: with `input.banned` set to `["a", "b"]`, `every x in input.banned { input.fruits.name != x }` gives `WHERE (fruits.name <> 'a' AND fruits.name <> 'b')` for the SQL targets.
An empty collection leaves no condition: the `every` is true, and the query is `true`.
If the body is false for one of the elements, the `every` is false, and no query remains.
With more elements, or an unknown collection, the `every` stays in the result.

## Examples

<PlaygroundExample dir={require.context('./_examples/every/feature-flags/')} />

<PlaygroundExample dir={require.context('./_examples/every/internal-meetings/')} />
