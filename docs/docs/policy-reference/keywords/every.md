---
sidebar_label: every
title: 'Rego Keyword Examples: every'
---

Rego rules and statements are existentially quantified by default. This means
that if there is any solution then the rule is true, or a value is bound. Some
policies require checking all elements in an array or object. The `every`
keyword makes this
[universal quantification](/docs/policy-language#universal-quantification-for-all)
easier.

Here we show two equivalent rules achieve universal quantification, note how
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
elements are `true`. As we can see `every` is a much better option!
</sub>

## Examples

<PlaygroundExample dir={require.context('./_examples/every/feature-flags/')} />

<PlaygroundExample dir={require.context('./_examples/every/internal-meetings/')} />
