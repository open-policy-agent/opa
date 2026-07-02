# walk-no-path

**Summary**: Call to `walk` can be optimized

**Category**: Performance

**Avoid**
```rego
package policy

allow if {
    # traverse potentially nested permissions structure looking
    # for an admin role, but notice how the path is never referenced
    # later
    walk(user.permissions, [path, value])

    value.type == "role"
    value.name == "admin"
}
```

**Prefer**
```rego
package policy

allow if {
    # replacing `path` with a wildcard variable tells the evaluator that it won't
    # have to build the path array for each node `walk` traverses, thereby avoiding
    # unnecessary allocations
    walk(user.permissions, [_, value])

    value.type == "role"
    value.name == "admin"
}
```

## Rationale

The primary purpose of the `walk` function is to traverse nested data structures, and often at an arbitrary depth.
Each node traversed "produces" a path/value pair, where the path is an array of keys that lead to the current node,
and the value is the current node itself. Most often, rules only need to account for the value of the node and not the
path, and when that is the case, using a wildcard variable (`_`) in place of path tells the evaluation engine that
there's no need to build the path array for each node traversed, thereby avoiding unnecessary allocations. This can
have a big impact on performance when huge data structures are traversed!

More concretely, `walk`ing without generating the path array cuts down evaluation time by about 33%, and reduces the
number of allocations by about 40%.

**Trivia**: this optimization was originally made in OPA to improve the performance of Regal, where `walk` is used
extensively to traverse the AST of the policy being linted.

## Exceptions

This rule can only optimize `walk` calls where the path/value array is provided as a second argument to `walk`, and
**not** when assigned using `:=`:

```rego
package policy

allow if {
    # this can't be optimized, as the `walk` function can't
    # "see" the array assignment on the left hand side
    [path, value] := walk(user.permissions)

    value.type == "role"
    value.name == "admin"
}
```

For this reason, and a few historic ones, using the second argument for the return value is the preferred way to use
`walk`, which is [unique](https://www.openpolicyagent.org/projects/regal/rules/style/function-arg-return#exceptions) for the walk built-in
function.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  performance:
    walk-no-path:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- Regal Docs: [function-arg-return](https://www.openpolicyagent.org/projects/regal/rules/style/function-arg-return)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/performance/walk-no-path/walk_no_path.rego)
