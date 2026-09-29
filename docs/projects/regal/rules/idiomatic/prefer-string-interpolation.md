# prefer-string-interpolation

**Summary**: Prefer string interpolation

**Category**: Idiomatic

**Avoid**

```rego
package policy

request_formatted := sprintf("%s: %s", [method, path]) if {
    # extract method and path from the request
}

full_name := concat(" ", [first_name, last_name])
```

**Prefer**

```rego
package policy

request_formatted := $"{method}: ${path}" if {
    # extract method and path from the request
}

full_name := $"{first_name} ${last_name}"
```

## Rationale

The introduction of [string interpolation](https://www.openpolicyagent.org/docs/policy-language#string-interpolation)
in OPA ([v1.12.0](https://github.com/open-policy-agent/opa/releases/tag/v1.12.0)) is one of the most significant
improvements to the Rego language since OPA 1.0, and interpolation should now be preferred over alternative methods,
like `concat` and `sprintf`, whenever possible.

String interpolation improves readability by placing variables, references and even whole expressions directly in the
string to be rendered. Coupled with sensible variable names, the result is code that is easier to both read and
write, while losing none of its expressiveness. String interpolation comes with many additional benefits, but also a few
caveats to be aware of.

### Handling of Undefined

String interpolation additionally simplifies rendering strings that contain references to potentially undefined
documents. Undefined references would typically cause evaluation to halt if provided as arguments to built-in functions
like `concat` or `sprintf`. While this is consistent with how Rego normally evaluates undefined, some scenarios benefit
from a more lenient approach, where for example rendering a message as part of a decision ideally doesn't impact the
decision itself, even if the message resolves a reference to an undefined document. Consider for example a rule building
a set of "deny" messages using the `sprintf` function:

```rego
package policy

deny contains message if {
    #
    # [ any number of conditions ]
    #
    # rendering message string fails on missing input attributes — may impact decision
    message := sprintf("user %s missing required attributes", [input.user.name])
}
```

Even with all conditions in the rule body above being met, the set returned by evaluating the `deny` rule could
still end up empty (typically interpreted as "not deny") in the case where `user.name` is missing from the `input`
document. This is unlikely what the policy author intended! While Rego provides many ways to help address this, adding
additional code just to render a message string is often undesirable. OPA's string interpolation feature was designed
with this in mind, and renders references to undefined documents simply as `<undefined>` without causing evaluation to
halt.

```rego
package policy

deny contains message if {
    #
    # [ any number of conditions ]
    #
    # rendering message string never fails — no impact on decision
    message := $"user ${input.user.name} missing required attributes"
}
```

While the more lenient handling of undefined references commonly is desired when rendering strings, this may not always
be the case. Since replacing a call to `concat` or `sprintf` with string interpolation potentially changes the outcome
of evaluation, Regal by default only reports call-sites where all arguments to the function are guaranteed to be defined,
as is the case of e.g. local variables or function arguments, but not nested references to attributes of the `data` or
`input` documents. See [Configuration Options](#configuration-options) if you want Regal to also consider non-variable
arguments passed to `concat` or `sprintf`, with the mentioned caveat in mind.

### Portability

As a built-in language feature, string interpolation additionally provides improved portability across different OPA and
Rego implementations, where `sprintf` in particular may not be available, or only partially supported. As noted under
[Exceptions](#exceptions) below, `sprintf` may still be required for more advanced string formatting needs. Regal
identifies such cases and will only report use of formatting functions replaceable by string interpolation.

## Exceptions

While we consider string interpolation to be the idiomatic option over alternatives like `concat` or `sprintf` whenever
applicable, it's simplicity means it won't handle all the advanced formatting options that `sprintf` provides. Regal
currently recognizes and reports only cases of `sprintf` use where the following conditions are met:

- No verbs besides `%s`, `%q`, `%d` or `%v` are used in the format string
- The arguments array contains only references to **variables** known to be defined at evaluation time,
  **or** Regal is configured to also consider calls with non-variable arguments (see
  [Configuration Options](#configuration-options) below)

Regal will suggest replacing most uses of the `concat` built-in function with string interpolation, applying the same
default for handling non-variable arguments as calls to `sprintf`.

We recommend setting `include-non-var-args: true` in your configuration only in combination with manual review of any
calls rewritten to use interpolation, and that you have tests to ensure the correctness of the result.

## Configuration Options

This linter rule provides the following [configuration options](https://www.openpolicyagent.org/projects/regal/configuration):

```yaml title=".regal/config.yaml or .regal.yaml"
rules:
  idiomatic:
    prefer-string-interpolation:
      # one of "error", "warning", "ignore"
      level: error
      # whether to include non-variable arguments such as references to
      # `data` or `input` or the result of evaluated expressions passed
      # to `sprintf`. For example, this rule would by default disqualify the
      # a call like `sprintf("%s:%d", [x, input.y])` since `input.y` could be
      # undefined at evaluation time and changing the call to use interpolation
      # would potentially change the outcome of evaluation
      #
      # while this is often acceptable, Regal ultimately leaves that decision
      # up to the user.
      #
      # default: false
      include-non-var-args: true
```

## Related Resources

- OPA Docs: [String Interpolation](https://www.openpolicyagent.org/docs/policy-language#string-interpolation)
- OPA Docs: [sprintf](https://www.openpolicyagent.org/docs/policy-reference/builtins/strings#sprintf)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/idiomatic/prefer-string-interpolation/prefer_string_interpolation.rego)
