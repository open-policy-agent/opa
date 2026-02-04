---
sidebar_label: Overview
sidebar_position: 1
image: /img/opa-errors.png
---

# OPA Errors Guide

This guide is designed to help you understand the most common errors you'll encounter when working with OPA. Each
document provides examples of the error, why it's an error, and how to fix it. A perfect companion for your debugging
session!

The errors currently documented are:

| Stage       | Category              | Message                                                                                                                              |
| ----------- | --------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| parsing     | rego_parse_error      | [var cannot be used for rule name](./errors/rego-parse-error/var-cannot-be-used-for-rule-name)                                       |
| parsing     | rego_parse_error      | [unexpected `{name}` keyword](./errors/rego-parse-error/unexpected-name-keyword)                                                     |
| parsing     | rego_parse_error      | [unexpected assign token](./errors/rego-parse-error/unexpected-assign-token)                                                         |
| parsing     | rego_parse_error      | [unexpected `{` token](./errors/rego-parse-error/unexpected-left-curly-token)                                                        |
| parsing     | rego_parse_error      | [unexpected identifier token](./errors/rego-parse-error/unexpected-identifier-token)                                                 |
| parsing     | rego_parse_error      | [unexpected `}` token](./errors/rego-parse-error/unexpected-right-curly-token)                                                       |
| parsing     | rego_parse_error      | [unexpected string token](./errors/rego-parse-error/unexpected-string-token)                                                         |
| compilation | rego_recursion_error  | [rule `{name}` is recursive](./errors/rego-recursion-error/rule-name-is-recursive)                                                   |
| compilation | rego_type_error       | [conflicting rules `{name}` found](./errors/rego-type-error/conflicting-rules-name-found)                                            |
| compilation | rego_type_error       | [match error](./errors/rego-type-error/match-error)                                                                                  |
| compilation | rego_type_error       | [arity mismatch](./errors/rego-type-error/arity-mismatch)                                                                            |
| compilation | rego_type_error       | [function has arity](./errors/rego-type-error/function-has-arity-got-argument)                                                       |
| compilation | rego_type_error       | [unsafe built-in function calls in expression: `{name}`](./errors/rego-type-error/unsafe-built-in-function-calls-in-expression-name) |
| compilation | rego_unsafe_var_error | [var `{name}` is unsafe](./errors/rego-unsafe-var-error/var-name-is-unsafe)                                                          |
| compilation | rego_compile_error    | [assigned var `{name}` unused](./errors/rego-compile-error/assigned-var-name-unused)                                                 |
| evaluation  | eval_conflict_error   | [complete rules must not produce multiple outputs](./errors/eval-conflict-error/complete-rules-must-not-produce-multiple-outputs)    |
| evaluation  | eval_conflict_error   | [object keys must be unique](./errors/eval-conflict-error/object-keys-must-be-unique)                                                |

## How To Read Pages in this Section

Each page in the OPA errors guide goes into detail about a single OPA error
type. For each detailed page, it contains the following information to help you
both check it applies and to resolve the error.

### Metadata

- **Category**: The category of the error. This is a high-level grouping of errors that are related to each other.
- **Message**: The error message you'll see OPA emit (normally following the category).

### Stage

Evaluation of Rego policies happens in three distinct stages — parsing, compilation and evaluation. Errors reported in
any of these stages will stop the evaluation process and have the error(s) reported.

#### Parsing

The first stage is **parsing**. In this step, OPA takes the raw Rego policy and parses it into an abstract syntax tree
(AST), which is then handed to the compiler. Errors at this stage are normally syntax errors, meaning the Rego provided
in a policy simply isn't valid. An example of this might be forgetting to terminate a string with a closing quote:

```rego
package policy

import future.keywords.contains
import future.keywords.if

deny contains message if {
    # ...

    message := "This won't work!
}
```

As expected, OPA will report a syntax error:

```txt
2 errors occurred:
policy.rego:9: rego_parse_error: non-terminated string
        message := "This won't work!
                   ^
policy.rego:9: rego_parse_error: illegal token
        message := "This won't work!
                   ^
```

At this point, further processing isn't possible, and the error must be fixed before we can proceed.

#### Compilation

While we may not think of Rego as a "compiled language", any policy passes through a compilation step before it can be
evaluated. During compilation, OPA will run several stages of analysis on the policy (which is now an AST) to ensure
that it's valid. This includes things like checking that functions are called with the right number of arguments,
that types are used correctly, or that variables are defined before they're used. A typical example of a compilation
error would be referencing a rule that isn't defined:

```rego
package policy

x := y
```

Since `y` isn't defined in our policy, the compiler considers it unsafe:

```txt
1 error occurred: policy.rego:3: rego_unsafe_var_error: var y is unsafe
```

**Tip:** when using `opa eval`, you can pass the `--strict` flag to enable additional compiler checks — like unused
variables or function arguments. This is a great way to spot mistakes and errors as soon as possible, and is highly
recommended.

#### Evaluation

The last stage in which errors may appear is evaluation. Errors at this stage normally involve `input` or `data`
that isn't known to OPA during parsing or compilation. Consider the following simplified example:

```rego
package policy

x := input.x

x := input.y
```

This policy _might_ work, if only one of `x` or `y` is provided in the input. If _both_ are provided, and they have
different, conflicting, values — an error will be reported during the evaluation stage:

```sh
policy.rego:3: eval_conflict_error: complete rules must not produce multiple outputs
```

Important to know is that not all "errors" at this stage will be reported as errors! Some things that would be
considered an error during compilation, like passing the wrong type of value in a function argument, would simply
result in evaluation being undefined during evaluation time.

```rego
startswith("100", 1)
```

As the `startswith` function expects two strings — and this is known by the compiler — this would fail during
compilation. But if we replace the `1` with a value from `input`:

```rego
startswith("100", input.x)
```

The compiler can't know the value of `input.x`. During evaluation, we _do_ know the value of `input.x`,
but does it mean we want a malformed value to stop policy evaluation entirely? Probably not! By default, evaluation
will simply consider that case to be _undefined_, and move on with evaluating the rest of the policy.

**Tip:** If you're using `opa eval` to evaluate policies, you can pass the `--strict-builtin-errors` flag to have
an error from a built-in function halt evaluation and have the error reported. Additionally, the
`--show-builtin-errors` flag may be used to collect _all_ errors from calling built-in functions and have them
reported. Both of these flags can be very useful for debugging!

### How To Fix It

This section provides guidance on how to fix the error.

### More Information

Some pages may provide additional information about the error, as well as links to resources for further reading.
