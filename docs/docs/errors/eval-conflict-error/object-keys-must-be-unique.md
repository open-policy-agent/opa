---
sidebar_label: object keys must be unique
image: /img/opa-errors.png
---

# eval_conflict_error: object keys must be unique

As in all programming languages, keys in object values must be unique. In Rego, if an object or rule is being
constructed with duplicate keys, this error will be raised. The same error is raised for both partial rules
constructing objects and for variables that are assigned objects where duplicate keys are present.

| Stage        | Category              | Message                      |
| ------------ | --------------------- | ---------------------------- |
| `evaluation` | `eval_conflict_error` | `object keys must be unique` |

## Examples

A simple example of this case would be creating an object where the key is the same for all values:

```rego
package policy

import rego.v1

obj := {k: v |
	k := "foo"
	some v in [1, 2]
}
```

In this example, we are attempting to create an object like `{"foo": 1, "foo": 2}` which contains duplicate keys.
This issue is also commonly seen with partial rules constructing objects. For example:

```rego
package policy

import rego.v1

obj_rule[k] := v if {
	k := "foo"
	some v in [1, 2]
}
```

Both of these simplified examples will raise the same error and are easy to spot
here in isolation. When this error appears in real-world policies, it's often
harder to find the source of the issue. For example, consider the following
policy:

```rego
package policy

import rego.v1

deny[input.document] := msg if {
	doc := data.documents[input.document]
	some permission in input.permissions

	not permission in doc.permissions[input.user]

	msg := sprintf("missing %s permission", [permission])
}
```

and the following data:

```json
{
  "documents": {
    "doc1": {
      "permissions": {
        "user1": [
          "read"
        ]
      }
    }
  }
}
```

This policy is intended to deny access to one or more documents if the user does not have the required permissions.
When evaluating the policy, where there's a single missing permission for a single document, the policy works as expected:

```rego
{
    "user": "user1",
    "actions": [
        {
            "document": "doc1",
            "permissions": [
                "read",
                "write" # user does not have this permission
            ]
        }
    ]
}
```

However, when evaluating the policy where there are multiple permission errors for a single document, the policy
will raise the `eval_conflict_error` error:

```rego
{
    "user": "user1",
    "actions": [
        {
            "document": "doc1",
            "permissions": [
                "read",
                "write", # user does not have this permission
                "delete" # user also does not have this permission
            ]
        }
    ]
}
```

Instead of the output being:

```json
{
  "deny": {
    "doc1": "missing write permission"
  }
}
```

We're trying to create an impossible output like this:

```json
{
  "deny": {
    "doc1": "missing write permission",
    "doc1": "missing delete permission"
  }
}
```

Even when a condition like this is deemed "impossible" — perhaps because another system's constraints forbid it, it
should be considered a best practice to account for this type of scenario in your policies.

## How To Fix It

To fix this error — ensure that all object keys are unique. If you're using values from `data.*` or `input.*` to
construct objects, ensure that there are not duplicated values. If you're using
`some x in y` in rules to create values for an object, this is a common source of this error as any more than one
value in `y` could result in duplicate keys.

Sometimes, you might need to restructure your policy to avoid this error. For example, in the policy above, we could
change it to work like this:

```rego
package policy

import rego.v1

deny contains msg if {
	some action in input.actions
	doc := data.documents[action.document]

	some permission in action.permissions

	not permission in doc.permissions[input.user]

	msg := sprintf("missing %s permission for document %s", [permission, action.document])
}
```

Though the output will have a different format too:

```json
{
  "deny": [
    "missing delete permission for document doc1",
    "missing write permission for document doc1"
  ]
}
```
