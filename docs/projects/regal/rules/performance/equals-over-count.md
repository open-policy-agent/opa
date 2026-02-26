# equals-over-count

**Summary**: Prefer direct use of `==`/`!=` over `count` to check for empty collections

**Category**: Performance

**Avoid**

```rego
package policy

denied if count(deny) > 0

users_with_no_emails_provided contains user if {
    some user in data.store.users

    count(user.emails) == 0
}
```

**Prefer**

```rego
package policy

denied if deny != set()

users_with_no_emails_provided contains user if {
    some user in data.store.users

    user.emails == []
}
```

## Rationale

**Note!** This is an **optional** rule (disabled by default) for optimizing performance in hot paths, and is not
recommended for general use in most projects. As our general guideline on policy authoring states: policy authors should
optimize for readability and to communicate intent. Policy evaluation with OPA is generally **very** fast. Additionally,
this rule comes with a few caveats of its own, as described further down. Read on to learn how you may benefit from this
rule even if you leave it disabled by default.

All function calls come with some cost, and while `count` is generally very cheap to use, it goes without saying that
avoiding a function call always is faster than making one. A common — and by all means idiomatic — pattern in Rego is
to evaluate something conditionally based on whether a collection (or string) is empty or not. The most straightforward
way to do this is probably to use `count` and compare the result to `0`, using either `==`, `!=` or `>`. This entails
making two function calls: one to `count` and one to the function behind the comparison operator.

**Cheap**

```rego
a := count(deny) > 0          # calls `count` and `gt` (`>`)
b := count(input.roles) != 0  # calls `count` and `neq` (`!=`)
c := count(data.users) == 0   # calls `count` and `equal` (`==`)
```

While this is perfectly idiomatic Rego, expressing the same logic using direct comparison to an empty collection or
string is more efficient, as it needs only the comparison function call. The cheaper form is also perfectly idiomatic,
but may not communicate intent as clearly as the `count` form.

**Cheaper**

```rego
a := deny != set()             # calls only `neq` (`!=`)
b := input.roles != []         # calls only `neq` (`!=`)
c := data.users == []          # calls only `equal` (`==`)
```

A small benefit of using the direct comparison form besides performance is that it communicates type
information at the call site, which the `count` form does not. While this rarely is necessary, it makes a pretty good
case for why some may prefer this form over the `count` alternative even for non-performance-critical code.

### Caveats

- This rule assumes a collection (or string) is always of the same type. While this is generally the case — and a good
  practice to follow — this rule will emit **false positives** on collections that may either be e.g. an array or a set,
  as the `count` form then isn't directly replaceable with the direct comparison form.
- This rule only applies to comparisons to "empty" or "not empty", and not e.g. `count(x) > 1` or `count(x) == 2`
- Just as when using `count`, evaluation still halts in the case of a non-existent (i.e. undefined) collection or
  string.
- Future versions of OPA could potentially perform this optimization as part of compilation, which would make both forms
  equally efficient. This wouldn't mean you'd have to change your code back — only that there no longer would be a
  performance benefit to using the direct comparison form. Some may still prefer it for other reasons though, like the
  extra type information it provides at call sites. Should this happen, we'll update this documentation accordingly.

### Recommended Use

Unless you're working in a project where every microsecond counts, you probably shouldn't enable this rule. What you
can do is to **occasionally** run the linter with `equals-over-count` enabled, and see if any of the reported
locations possibly may be in a hot path, and decide on a case-by-case basis whether to change the code or not. Don't
forget to benchmark to verify your assumptions!

It could also be that you prefer the direct comparison form over the `count` form for aesthetic reasons! In which case
you may choose to enable it by default. Just keep the caveats described above in mind.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  performance:
    equals-over-count:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/performance/equals-over-count/equals_over_count.rego)
