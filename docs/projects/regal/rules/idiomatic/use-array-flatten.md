# use-array-flatten

**Summary**: Prefer using `array.flatten` over nested `array.concat` calls

**Category**: Idiomatic

**Avoid**

```rego
package policy

flat1 := array.concat(arr1, array.concat(arr2, arr3))

flat2 := array.concat(arr1, array.concat(arr2, array.concat(arr3, arr4)))
```

**Prefer**

```rego
package policy

flat1 := array.flatten([arr1, arr2, arr3])

flat2 := array.flatten([arr1, arr2, arr3, arr4])
```

## Rationale

Since the introduction of
[array.flatten](https://www.openpolicyagent.org/docs/policy-reference/builtins/array#builtin-array-arrayflatten)
(OPA v1.13.0), nested `array.concat` calls can be replaced by a single call to `array.flatten`, which is both easier to
read and more efficient. Double win!

## (Optional) Recommend replacing `array.concat` calls wrapping arguments

The `array.concat` function is sometimes used to prepend or append a non-array value to an array, by wrapping the
non-array value in an array literal. Since the `array.flatten` function accepts values of any type, it can be used in
place of `array.concat` in these cases too, which arguably looks cleaner. Set the `flag-wrapped-concat` configuration
option to `true` to enable this check.

**Avoid**

```rego
package policy

flat := array.concat([not_arr], arr) # => [not_arr, arr...]
```

**Prefer**

```rego
package policy

flat := array.flatten([not_arr, arr]) # => [not_arr, arr...]
```

## (Optional) Always prefer `array.flatten` over `array.concat`

Since there is nothing that `array.concat` can do that `array.flatten` cannot, some users and teams may prefer the
consistency of using a single function for all their concatenation needs. Set the `flag-all-concat` configuration option
to `true` to always recommend replacing `array.concat` with `array.flatten`.

## Exceptions

If you are targeting an OPA version prior to v1.13.0, you'll need to provide Regal with the capabilities of your OPA,
so that only rules applicable to your OPA version are enforced. See the
[capabilities](https://www.openpolicyagent.org/projects/regal/configuration/capabilities) page of the documentation for
more details.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  idiomatic:
    use-array-flatten:
      # one of "error", "warning", "ignore"
      level: error
      # recommend also replacing calls to `array.concat` where at least one argument is an array literal,
      # e.g. `array.concat([a], b)` -> `array.flatten([a, b])`
      #
      # default: false
      flag-wrapped-concat: false
      # recommend replacing all calls to `array.concat` with `array.flatten`
      #
      # default: false
      flag-all-concat: false
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/idiomatic/use-array-flatten/use_array_flatten.rego)
