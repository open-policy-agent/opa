---
title: Evaluating a Data Filter Policy
sidebar_position: 2
---

To understand how SQL WHERE clauses can be derived from a partially evaluated Rego policy, it's beneficial to have a basic idea about how _partial evaluation_ (PE) works.
In this walk-through of a PE run, we'll start with a basic filter policy:

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

In this walkthrough, we'll go through the policy in the same way the evaluator does, and use the following input:

```json
{
  "user": {
    "name": "dana"
  },
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
The <abbr title="left-hand side">LHS</abbr> `input.users.name` is part of the _unknowns_ (`input.users`), so the expression contributes to our conditions:

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
Our expression's <abbr title="left-hand side">LHS</abbr> is known, "low", which is not different from "low".
An expression with all known parts that evaluates to false makes us give up on this rule path (regardless of eventual extra expressions following),
and we discard the set of conditions aggregated for this rule body.

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
Evaluating our second rule body, we again get a condition from the comparison with `user`, which is `input.user`, and known to be "dana":

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

This expression includes one unknown and one literal, so it adds a condition to our set:

```rego
input.products.price == "free"
```

There are no further expressions, so this condition also contributes to our PE result.
</SideBySideColumn>
</SideBySideContainer>

---

<SideBySideContainer>
<SideBySideColumn>
We're now done with the partial evaluation of our `data.filters.include` rule with the given (known) inputs.
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
