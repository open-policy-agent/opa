---
title: Evaluating a Data Filter Policy
sidebar_position: 2
---

To understand how SQL WHERE clauses can be derived from a partially evaluated Rego policy, it's beneficial to have a basic idea about how _partial evaluation_ (PE) works.
This walk-through of a PE run starts with a basic filter policy:

```rego title="filters.rego"
# METADATA
# scope: package
# compile:
#   unknowns:
#     - input.users
#     - input.products
package filters

user := input.user

include if {
        input.users.name == user
        input.budget != "low"
}

include if {
        input.users.name == user
        input.budget == "low"
        input.products.price < 500
}

include if input.products.price == "free"
```

The `compile` annotation declares which references are treated as _unknowns_ for
this policy. See the [`compile` metadata annotation](../policy-language#metadata-compile)
reference for the full set of supported fields.

This walkthrough follows the policy in the same way the evaluator does, using the following input:

```json
{
  "user": "dana",
  "budget": "low"
}
```

---

<SideBySideContainer>
<SideBySideColumn>

```rego
user := input.user

# highlight-next-line
include if {
        input.users.name == user
        input.budget != "low"
}

include if {
        input.users.name == user
        input.budget == "low"
        input.products.price < 500
}

include if input.products.price == "free"
```

</SideBySideColumn>
<SideBySideColumn>
The first `include` rule is evaluated.
</SideBySideColumn>
</SideBySideContainer>

---

<SideBySideContainer>
<SideBySideColumn>

```rego
# highlight-next-line
user := input.user

include if {
        # highlight-next-line
        input.users.name == user
        input.budget != "low"
}

include if {
        input.users.name == user
        input.budget == "low"
        input.products.price < 500
}

include if input.products.price == "free"
```

</SideBySideColumn>
<SideBySideColumn>
The expression `input.users.name == user` uses `user`, which is _known_, "dana".
The <abbr title="left-hand side">LHS</abbr> `input.users.name` is part of the _unknowns_ (`input.users`), so the expression contributes to the conditions:

```rego
input.users.name == "dana"
```

Since `input.user` is known, it's been dereferenced.
</SideBySideColumn>
</SideBySideContainer>

---

<SideBySideContainer>
<SideBySideColumn>

```rego
user := input.user

include if {
        input.users.name == user
        # highlight-next-line
        input.budget != "low"
}

include if {
        input.users.name == user
        input.budget == "low"
        input.products.price < 500
}

include if input.products.price == "free"
```

</SideBySideColumn>
<SideBySideColumn>
The expression's <abbr title="left-hand side">LHS</abbr> is known, "low", which is not different from "low".
When all parts of an expression are known and it evaluates to false, this rule path is abandoned (regardless of any further expressions),
and the set of conditions aggregated for this rule body is discarded.

Partial evaluation continues with the next rule body.
</SideBySideColumn>
</SideBySideContainer>

---

<SideBySideContainer>
<SideBySideColumn>

```rego
user := input.user

include if {
        input.users.name == user
        input.budget != "low"
}

# highlight-next-line
include if {
        # highlight-next-line
        input.users.name == user
        input.budget == "low"
        input.products.price < 500
}

include if input.products.price == "free"
```

</SideBySideColumn>
<SideBySideColumn>
The second rule body is evaluated. A condition is again derived from the comparison with `user`, which is `input.user`, and known to be "dana":

```rego
input.users.name == "dana"
```

</SideBySideColumn>
</SideBySideContainer>

---

<SideBySideContainer>
<SideBySideColumn>

```rego
user := input.user

include if {
        input.users.name == user
        input.budget != "low"
}

include if {
        input.users.name == user
        # highlight-next-line
        input.budget == "low"
        input.products.price < 500
}

include if input.products.price == "free"
```

</SideBySideColumn>
<SideBySideColumn>
The expression `input.budget == "low"` has only known parts, `input.budget`, and "low", and is indeed true.
It doesn't add any condition, and lets us continue evaluating this rule body.
</SideBySideColumn>
</SideBySideContainer>

---

<SideBySideContainer>
<SideBySideColumn>

```rego
user := input.user

include if {
        input.users.name == user
        input.budget != "low"
}

include if {
        input.users.name == user
        input.budget == "low"
        # highlight-next-line
        input.products.price < 500
}

include if input.products.price == "free"
```

</SideBySideColumn>
<SideBySideColumn>
The next expression, `input.products.price < 500`, involves a number literal and an unknown, and thus adds a condition:

```rego
input.users.name == "dana"
input.products.price < 500
```

There are no further expressions in this rule body, so the conditions are saved, and partial evaluation proceeds to the next rule body.
</SideBySideColumn>
</SideBySideContainer>

---

<SideBySideContainer>
<SideBySideColumn>

```rego
user := input.user

include if {
        input.users.name == user
        input.budget != "low"
}

include if {
        input.users.name == user
        input.budget == "low"
        input.products.price < 500
}

# highlight-next-line
include if input.products.price == "free"
```

</SideBySideColumn>
<SideBySideColumn>
As with every new rule body, the set of conditions is _reset_.

This expression includes one unknown and one literal, so it adds a condition to the set:

```rego
input.products.price == "free"
```

There are no further expressions, so this condition also contributes to the PE result.
</SideBySideColumn>
</SideBySideContainer>

---

<SideBySideContainer>
<SideBySideColumn>
The partial evaluation of `data.filters.include` with the given (known) inputs is now complete.
It has yielded two sets of conditions, **A** and **B**, which form the basis of translation into SQL queries.
</SideBySideColumn>

<SideBySideColumn>

```rego title="A (Rego)"
input.users.name == "dana"
input.products.price < 500
```

```rego title="B (Rego)"
input.products.price == "free"
```

</SideBySideColumn>
</SideBySideContainer>

---

<SideBySideContainer>
<SideBySideColumn>
When translating, each of the sets is translated into SQL expressions:
</SideBySideColumn>

<SideBySideColumn>

```sql title="A (SQL)"
users.name = "dana" AND products.price < 500
```

```sql title="B (SQL)"
products.price = "free"
```

</SideBySideColumn>
</SideBySideContainer>

---

Finally, the two are combined with `OR`:

```sql title="A OR B"
(users.name = "dana" AND products.price < 500) OR products.price = "free"
```

## Next Steps

- To learn more about which Rego expressions can be used in filter policies, continue to [Writing valid data filtering policies](./fragment).
- Find all information about supported options and tweaks for translation in the [Data Filters Compilation API reference](../rest-api#compiling-a-rego-policy-and-query-into-data-filters).
