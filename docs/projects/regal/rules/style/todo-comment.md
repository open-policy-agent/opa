# todo-comment

**Summary**: Avoid TODO Comments

**Category**: Style

**Avoid**
```rego
package policy

# TODO: implementation
allow := true

i := input.i + 1

# Fixme: surely there's a better way to do recursion
response := http.send({
    "url": "http://localhost:8080/v1/data/policy",
    "method": "POST",
    "body": {
        "input": {
            "i": i
        }
    }
})
```

**Prefer**

To fix the problem, or use an issue tracker to track it.

## Rationale

While TODO and FIXME comments are occasionally useful, they essentially provide a way to do issue tracking inside of
the code rather than where issues belong — in your issue tracker.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    todo-comment:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- DEV Community: [//TODO: Write a better comment](https://dev.to/adammc331/todo-write-a-better-comment-4c8c)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/style/todo-comment/todo_comment.rego)
