# top-level-iteration

**Summary**: Iteration in top-level assignment

**Category**: Bugs

**Avoid**
```rego
package policy

user := input.users[_]
```

## Rationale

While OPA allows this construct — it probably shouldn't. Performing iteration outside of a rule or function body
doesn't make any sense, and traversing **any** collection containing more than one item in this context will result
in an error:

```shell
eval_conflict_error: complete rules must not produce multiple outputs
```

If the collection only contains a single item, the assignment will succeed, and the result will be the single element
assigned to the variable. As such, it is possible that a policy passing all tests still will fail when provided real
data.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    top-level-iteration:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/top-level-iteration/top_level_iteration.rego)
