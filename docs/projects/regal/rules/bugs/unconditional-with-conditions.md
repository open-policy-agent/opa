# unconditional-with-conditions

**Summary**: Rule defined unconditionally alongside conditional definitions

**Category**: Bugs

**Automatically fixable**: No

**Avoid**
```rego
package policy

allow := true if input.user.is_admin

# `allow` is now defined for every input, so the conditions above never decide
# anything. If `input.user.is_admin` is true, evaluation fails with a conflict.
allow := false
```

**Prefer**
```rego
package policy

default allow := false

allow := true if input.user.is_admin
```

## Rationale

A definition written without `if` is defined for every input. When a rule has
one of those *and* other definitions that carry conditions, those conditions
cannot change the outcome, and only two things can happen:

- the values differ, and evaluation fails with `eval_conflict_error:
  complete rules must not produce multiple outputs`; or
- the values agree, and the conditional definitions are dead code.

Neither is what the author meant. Almost always the intent was a fallback, and
the fallback belongs in a `default` — which is a different construct, applying
only when no other definition produces a value, rather than competing with them.

The mistake is easy to make because a body-less definition reads like one. In
`allow := false` there is no `if`, nothing obviously conditional, and the line
looks like a declaration of a starting value rather than a third rule in the
set.

Static detection matters because of where the failure surfaces. The conflict is
raised at evaluation time, and only on the inputs that reach the conditional
branch, so a policy can pass `opa check --strict`, pass a test suite whose
fixtures never take that branch, and fail later on real data. In a policy used
as a gate, a rule that errors returns no decision at all, which at the boundary
is indistinguishable from a policy that stopped denying.

The same pattern also defeats `default`. Given

```rego
default f(_) := false

f(x) if x > 10

f(_) := false
```

the third definition is a complete definition, not a default, so `f` now has two
unconditional answers and the `default` is doing nothing.

## Known limitations

- Only definitions **within one file** are compared. A rule set split across
  files in the same package is not detected, since each file is linted on its
  own.
- The unconditional definition must assign a **constant** value. A definition
  such as `allow := input.override` can itself be undefined, and then the
  conditional definitions are genuinely doing work rather than being pointless:

  ```rego
  allow := input.override

  allow := false if not input.override
  ```

- Multi-value rules (`deny contains msg if ...`) are not reported, because their
  definitions union rather than conflict.
- Function definitions whose arguments are not all distinct variables are not
  reported. `f("a") := 1` has no body but applies only to the argument `"a"`, so
  the other definitions in the set still do real work. This does leave a case
  uncovered: `f("a") := 1` beside `f("a") := 2 if input.cond` can conflict, and
  telling that apart needs the arguments compared rather than just counted.
- Rules whose head contains a variable (`config[k] := "loose"`) are not
  reported. Deciding whether two such definitions can ever collide means
  reasoning about what the variable is bound to, which needs more than this
  rule attempts.
- At least one definition must carry conditions. Two definitions that both lack
  them conflict without any condition being made pointless, and that is left to
  a rule of its own:

  ```rego
  allow := true

  allow := false
  ```

  `duplicate-rule` does not report this either, as it compares rule text.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    unconditional-with-conditions:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [Default Keyword](https://www.openpolicyagent.org/docs/policy-language/#default-keyword)
- OPA Docs: [Complete Definitions](https://www.openpolicyagent.org/docs/policy-language/#complete-definitions)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/unconditional-with-conditions/unconditional_with_conditions.rego)

## Community

If you think you've found a problem with this rule or its documentation, would like to suggest improvements, new rules,
or just talk about Regal in general, please join us in the `#regal` channel in the Styra Community
[Slack](https://communityinviter.com/apps/styracommunity/signup)!
