---
sidebar_label: match error
image: /img/opa-errors.png
---

# `rego_type_error`: match error

Just like the category suggests, this error is emitted by the _type checker_ during the compilation stage. This error
is commonly triggered by comparing two values of different types, like a string and an integer (`"1" == 1`). In order
for the error to be reported, the compiler must be able to determine the types involved in the expression.

This may sound obvious, but remember that some values and references aren't known until evaluation time. Comparing
something to e.g. a value from `input` will therefore not be reported by the type checker (unless the type checker
[has been informed](https://www.openpolicyagent.org/docs/policy-language/#using-schemas-to-enhance-the-rego-type-checker)
about the schema of `input`).

| Stage         | Category          | Message                                    |
| ------------- | ----------------- | ------------------------------------------ |
| `compilation` | `rego_type_error` | `match error left: <type1> right: <type2>` |

## Examples

The first example is trivial, but serves well to demonstrate the error. An equality check between two literals of
different type can never be true, and the compiler knows this:

```rego
package policy

import future.keywords.if

same if 1 == "1"
```

The error message reported will include the types compared in the match error:

```txt
1 error occurred: policy.rego:5: rego_type_error: match error
    left  : number
    right : string
```

Some cases are more tricky however, and even comparison between two values of the same "simple" type can render a match
error when composite types are on both sides of the comparison. Consider the following example:

```rego
package policy

import future.keywords.if

user1 := {"name": "joe", "age": 55}
user2 := {"name": "jane"}

same_user if user1 == user2
```

We're clearly comparing two objects, so should this really be a match error? The compiler would say yes:

```txt
1 error occurred: policy.rego:8: rego_type_error: match error
    left  : object<age: number, name: string>
    right : object<name: string>
```

The reason for this is that while we may have objects on both sides, the type checker compares _recursively_. In doing
this it'll see that one object has an attribute (`age`) that the other doesn't, and hence considered to be of
different type after all. This is sometimes considered
[confusing](https://github.com/open-policy-agent/opa/issues/2132), but given that it also allows catching some bugs at
compile time, it's likely the right behavior.

## How To Fix It

Fixing this is simple: just change the types to match on both sides of a comparison. For the few cases where one
_really_ wants to compare two values of different types, a helper function can be used to "wash" off the type
information from the comparison:

```rego
package policy

import future.keywords.if

user1 := {"name": "joe", "age": 55}
user2 := {"name": "jane"}

untyped_equals(o1, o2) if o1 == o2

# this will be undefined, not a compile time error
same_user if untyped_equals(user1, user2)
```

Do note — this is not a recommended approach! The type checker is there to help you, and circumventing it should be
done only in exceptional cases.
