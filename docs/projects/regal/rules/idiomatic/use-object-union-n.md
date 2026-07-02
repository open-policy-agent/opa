# use-object-union-n

**Summary**: Prefer using `object.union_n` over nested `object.union` calls

**Category**: Idiomatic

**Avoid**

```rego
package policy

obj := object.union(obj1, object.union(obj2, obj3))
```

**Prefer**

```rego
package policy

obj := object.union_n([obj1, obj2, obj3])
```

## Rationale

Prefer to use `object.union_n` over nested `object.union` calls, as that is both easier to read and more efficient.

## (Optional) Always prefer `object.union_n` over `object.union`

Since there is nothing that `object.union` can do that `object.union_n` cannot, some users and teams may prefer the
consistency of using a single function for all their union needs. Set the `flag-all-union` configuration option
to `true` to always recommend replacing `object.union` with `object.union_n`.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  idiomatic:
    use-object-union-n:
      # one of "error", "warning", "ignore"
      level: error
      # recommend replacing all calls to `object.union` with `object.union_n`
      #
      # default: false
      flag-all-union: false
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/idiomatic/use-object-union-n/use_object_union_n.rego)
